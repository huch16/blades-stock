package data

import (
	"context"
	"encoding/json"
	"fmt"
	url2 "net/url"
	"strings"
	"time"

	"blades-stock/backend/datasource"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	resty "github.com/go-resty/resty/v2"
	"github.com/robertkrimen/otto"
)

// FormatMarketStockCode 给纯6位股票/ETF代码自适应补充交易所前缀（sh/sz/bj）
func FormatMarketStockCode(code string) string {
	clean := models.CleanStockCode(code)
	if clean == "" {
		return strings.ToLower(code)
	}
	lower := strings.ToLower(code)
	if strutil.HasPrefixAny(lower, []string{"sh", "sz", "bj", "hk"}) {
		return lower
	}
	if strutil.HasPrefixAny(clean, []string{"60", "688", "51", "58"}) {
		return "sh" + clean
	}
	if strutil.HasPrefixAny(clean, []string{"00", "30", "15", "16", "18"}) {
		return "sz" + clean
	}
	if strutil.HasPrefixAny(clean, []string{"4", "8", "92"}) {
		return "bj" + clean
	}
	return "sh" + clean
}

func (receiver StockDataApi) GetStockCodeRealTimeData(StockCodes ...string) (*[]StockInfo, error) {
	normalizedCodes := make([]string, 0, len(StockCodes))
	for _, c := range StockCodes {
		c = strings.TrimSpace(c)
		if c != "" {
			normalizedCodes = append(normalizedCodes, FormatMarketStockCode(c))
		}
	}
	StockCodes = ConvertTushareCodeToStockCodes(normalizedCodes)

	stockInfos := make([]StockInfo, 0)

	hkcodes := slice.Filter(StockCodes, func(i int, s string) bool {
		return strutil.HasPrefixAny(s, []string{"hk", "HK", "sh", "sz"})
	})

	if len(hkcodes) > 0 {
		hkcodesStr := slice.JoinFunc(hkcodes, ",", func(s string) string {
			if strutil.HasPrefixAny(s, []string{"hk", "HK"}) {
				return "r_" + strings.ToLower(s)
			} else {
				return strings.ToLower(s)
			}
		})
		url := fmt.Sprintf(txStockUrl, time.Now().Unix(), hkcodesStr)
		resp, err := receiver.client.R().
			SetHeader("Host", "qt.gtimg.cn").
			SetHeader("Referer", "https://gu.qq.com/").
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
			Get(url)
		//logger.SugaredLogger.Infof("GetStockCodeRealTimeData %s", url)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return &[]StockInfo{}, err
		}
		str := GB18030ToUTF8(resp.Body())
		dataStr := strutil.SplitAndTrim(strings.Trim(str, "\n"), ";")

		for _, data := range dataStr {
			stockData, err := ParseTxStockData(data)
			if err != nil {
				logger.SugaredLogger.Error(err.Error())
				continue
			}
			if stockData == nil {
				continue
			}
			stockInfos = append(stockInfos, *stockData)
			go func(sd StockInfo) {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Warnf("Save StockInfo panic: %v", r)
					}
				}()
				var count int64
				db.Dao.Model(&StockInfo{}).Where("code = ?", sd.Code).Count(&count)
				if count == 0 {
					db.Dao.Model(&StockInfo{}).Create(&sd)
				} else {
					db.Dao.Model(&StockInfo{}).Where("code = ?", sd.Code).Updates(&sd)
				}
			}(*stockData)
		}
	}

	szzsusCodes := slice.Filter(StockCodes, func(i int, s string) bool {
		return !strutil.HasPrefixAny(s, []string{"hk", "HK", "sh", "sz"})
	})

	codes := slice.JoinFunc(szzsusCodes, ",", func(s string) string {
		if strings.HasPrefix(s, "us") {
			s = strings.Replace(s, "us", "gb_", 1)
		}
		if strings.HasPrefix(s, "US") {
			s = strings.Replace(s, "US", "gb_", 1)
		}
		return strings.ToLower(s)
	})

	url := fmt.Sprintf(sinaStockUrl, time.Now().Unix(), codes)
	//logger.SugaredLogger.Infof("GetStockCodeRealTimeData %s", url)
	resp, err := receiver.client.R().
		SetContext(context.Background()).
		SetHeader("Host", "hq.sinajs.cn").
		SetHeader("Referer", "https://finance.sina.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Debugf("Sina实时行情请求失败，切换到腾讯行情: %v", err)
		txCodes := slice.JoinFunc(szzsusCodes, ",", func(s string) string {
			s = strings.ToLower(s)
			if strutil.HasPrefixAny(s, []string{"us", "gb_"}) {
				return "gb_" + strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(s, "us"), "gb_"), ".")
			}
			return s
		})
		txFallbackURL := fmt.Sprintf(txStockUrl, time.Now().Unix(), txCodes)
		resp2, err2 := receiver.client.R().
			SetHeader("Host", "qt.gtimg.cn").
			SetHeader("Referer", "https://gu.qq.com/").
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
			Get(txFallbackURL)
		if err2 != nil {
			logger.SugaredLogger.Debugf("腾讯行情也失败（将跳过）: %v", err2)
			return &[]StockInfo{}, err
		}
		resp = resp2
		str2 := GB18030ToUTF8(resp.Body())
		dataStr2 := strutil.SplitAndTrim(strings.Trim(str2, "\n"), ";")
		for _, data := range dataStr2 {
			stockData, errTx := ParseTxStockData(data)
			if errTx != nil || stockData == nil {
				continue
			}
			stockInfos = append(stockInfos, *stockData)
			go func(sd StockInfo) {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Warnf("Save TxStockInfo panic: %v", r)
					}
				}()
				var count int64
				db.Dao.Model(&StockInfo{}).Where("code = ?", sd.Code).Count(&count)
				if count == 0 {
					db.Dao.Model(&StockInfo{}).Create(&sd)
				} else {
					db.Dao.Model(&StockInfo{}).Where("code = ?", sd.Code).Updates(&sd)
				}
			}(*stockData)
		}
		return &stockInfos, nil
	}

	str := GB18030ToUTF8(resp.Body())
	dataStr := strutil.SplitEx(str, "\n", true)

	for _, data := range dataStr {
		//logger.SugaredLogger.Info(data)
		stockData, err := ParseFullSingleStockData(data)
		//logger.SugaredLogger.Infof("GetStockCodeRealTimeData %v", stockData)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			continue
		}
		if stockData == nil {
			continue
		}
		stockInfos = append(stockInfos, *stockData)

		go func(sd StockInfo) {
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Warnf("Save SingleStockData panic: %v", r)
				}
			}()
			var count int64
			db.Dao.Model(&StockInfo{}).Where("code = ?", sd.Code).Count(&count)
			if count == 0 {
				db.Dao.Model(&StockInfo{}).Create(&sd)
			} else {
				db.Dao.Model(&StockInfo{}).Where("code = ?", sd.Code).Updates(&sd)
			}
		}(*stockData)

	}

	return &stockInfos, err
}

// GB18030ToUTF8 GB18030 转换为 UTF8（转发；实现已搬入 datasource，R-25a）。
func GB18030ToUTF8(bs []byte) string {
	return datasource.GB18030ToUTF8(bs)
}

func ParseTxStockData(data string) (*StockInfo, error) {
	//v_r_hk09660="100~地平线机器人-W~09660~6.240~5.690~5.800~192659034.0~0~0~6.240~0~0~0~0~0~0~0~0~0~6.240~0~0~0~0~0~0~0~0~0~192659034.0~2025/04/29
	//13:41:04~0.550~9.67~6.450~5.710~6.240~192659034.0~1180471843.140~0~32.51~~0~0~13.01~691.1364~823.6983~HORIZONROBOT-W~0.00~10.380~3.320~1.07~-16.03~0~0~0~0~0~32.51~6.40~1.74~600~73.33~17.96~GP~19.70~11.51~-0.95~-18.54~44.44~13200293682.00~11075904412.00~32.51~0.000~6.127~56.39~HKD~1~30";
	//v_sz002241="51~歌尔股份~002241~22.26~22.27~0.00~0~0~0~22.26~1004~0.00~0~0.00~0~0.00~0~0.00~0~22.26~1004~0.00~558~0.00~0~0.00~0~0.00~0~~20250509092233~-0.01~-0.04~0.00~0.00~22.26/0/0~0~0~0.00~28.21~~0.00~0.00~0.00~686.46~777.09~2.31~24.50~20.04~0.00~-558~0.00~41.44~29.16~~~1.24~0.0000~0.0000~0~
	//~GP-A~-13.75~6.76~1.09~8.18~3.39~30.63~15.70~6.87~17.47~-23.95~3083811231~3490989083~-21.75~12.02~3083811231~~~39.36~-0.04~~CNY~0~~0.00~0";

	datas := strutil.SplitAndTrim(data, "=", "\"")
	if len(datas) < 2 {
		return nil, fmt.Errorf("invalid data format")
	}
	var result map[string]string
	var err error
	if strutil.ContainsAny(datas[0], []string{"v_r_hk", "v_hk", "v_sz", "v_sh"}) {
		result, _ = ParseTxHKStockData(datas)
	}

	marshal, err := json.Marshal(result)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Marshal error:%s", err.Error())
		return nil, err
	}
	//logger.SugaredLogger.Infof("股票数据解析完成marshal: %s", marshal)
	stockInfo := &StockInfo{}
	err = json.Unmarshal(marshal, &stockInfo)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error:%s", err.Error())
		return nil, err
	}
	//logger.SugaredLogger.Infof("股票数据解析完成stockInfo: %+v", stockInfo)

	return stockInfo, nil

}

func ParseTxHKStockData(datas []string) (map[string]string, error) {
	//v_r_hk09660="
	//100~   0
	//地平线机器人-W~  1
	//09660~ 2
	//6.270~ 3 当前价
	//5.690~ 4 昨收价
	//5.800~ 5 开盘价
	//195083034.0~
	//0~
	//0~
	//6.270~
	//0~
	//0~
	//0~
	//0~
	//0~
	//0~
	//0~
	//0~
	//0~
	//6.270~
	//0~0~0~0~0~0~0~0~0~
	//195083034.0~
	//2025/04/29 13:45:41~  30 当前时间
	//0.580~
	//10.19~
	//6.450~  最高价
	//5.710~  最低价
	//6.270~
	//195083034.0~
	//1195673623.140~
	//0~
	//32.66
	//~~0~0~13.01~694.4592~827.6584~HORIZONROBOT-W~0.00~10.380~3.320~1.06~-18.71~0~0~0~0~0~32.66~6.43~1.76~600~74.17~18.53~GP~19.70~11.51~-0.48~-18.15~45.14~13200293682.00~11075904412.00~32.66~0.000~6.129~57.14~HKD~1~30";
	result := make(map[string]string)

	stockCode := strutil.ReplaceWithMap(datas[0], map[string]string{
		"v_r_": "",
		"v_":   "",
	})
	result["股票代码"] = stockCode

	parts := strutil.SplitAndTrim(datas[1], "~")
	//logger.SugaredLogger.Infof("股票数据解析完成 len: %v", len(parts))
	if len(parts) < 35 {
		return nil, fmt.Errorf("invalid data format")
	}
	result["股票名称"] = parts[1]
	result["当前价格"] = parts[3]
	result["昨日收盘价"] = parts[4]
	result["今日开盘价"] = parts[5]

	result["今日最高价"] = parts[33]
	result["今日最低价"] = parts[34]

	if strutil.HasPrefixAny(stockCode, []string{"sz", "sh"}) {
		result["买一报价"] = parts[9]
		result["买一申报"] = parts[10]
		result["买二报价"] = parts[11]
		result["买二申报"] = parts[12]
		result["买三报价"] = parts[13]
		result["买三申报"] = parts[14]
		result["买四报价"] = parts[15]
		result["买四申报"] = parts[16]
		result["买五报价"] = parts[17]
		result["买五申报"] = parts[18]

		result["卖一报价"] = parts[19]
		result["卖一申报"] = parts[20]
		result["卖二报价"] = parts[21]
		result["卖二申报"] = parts[22]
		result["卖三报价"] = parts[23]
		result["卖三申报"] = parts[24]
		result["卖四报价"] = parts[25]
		result["卖四申报"] = parts[26]
		result["卖五报价"] = parts[27]
		result["卖五申报"] = parts[28]

	}

	timestr := ""

	if strutil.ContainsAny(parts[30], []string{"/"}) {
		timestr = strutil.ReplaceWithMap(parts[30], map[string]string{
			"/":  "-",
			"\n": " ",
		})
		result["日期"] = strutil.SplitAndTrim(timestr, " ", "")[0]
		result["时间"] = strutil.SplitAndTrim(timestr, " ", "")[1]
	} else {
		result["日期"] = strutil.Trim(parts[29])[0:4] + "-" + strutil.Trim(parts[29])[4:6] + "-" + strutil.Trim(parts[29])[6:8]
		result["时间"] = strutil.Trim(parts[29])[8:10] + ":" + strutil.Trim(parts[29])[10:12] + ":" + strutil.Trim(parts[29])[12:14]
		result["今日最高价"] = parts[32]
		result["今日最低价"] = parts[33]
	}
	//logger.SugaredLogger.Infof("股票数据解析完成 %s %s 时间: %s,%s", parts[1], parts[3], parts[29], parts[30])

	//logger.SugaredLogger.Infof("股票数据解析完成 时间: %v", timestr)

	//logger.SugaredLogger.Infof("股票数据解析完成: %v", result)

	return result, nil
}

func ParseFullSingleStockData(data string) (*StockInfo, error) {
	datas := strutil.SplitAndTrim(data, "=", "\"")
	if len(datas) < 2 {
		return nil, fmt.Errorf("invalid data format")
	}
	var result map[string]string
	var err error
	if strutil.ContainsAny(datas[0], []string{"hq_str_sz", "hq_str_sh", "hq_str_bj", "hq_str_sb"}) {
		result, _ = ParseSHSZStockData(datas)
	}
	if strutil.ContainsAny(datas[0], []string{"hq_str_hk"}) {
		result, _ = ParseHKStockData(datas)
	}
	if strutil.ContainsAny(datas[0], []string{"hq_str_gb"}) {
		result, _ = ParseUSStockData(datas)
	}

	marshal, err := json.Marshal(result)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Marshal error:%s", err.Error())
		return nil, err
	}
	//logger.SugaredLogger.Infof("股票数据解析完成marshal: %s", marshal)
	stockInfo := &StockInfo{}
	err = json.Unmarshal(marshal, &stockInfo)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error:%s", err.Error())
		return nil, err
	}
	//logger.SugaredLogger.Infof("股票数据解析完成stockInfo: %+v", stockInfo)

	return stockInfo, nil
}

func ParseUSStockData(datas []string) (map[string]string, error) {
	code := strings.Split(datas[0], "hq_str_")[1]
	result := make(map[string]string)
	parts := strutil.SplitAndTrim(datas[1], ",", "\"", ";")
	//parts := strings.Split(data, ",")
	//logger.SugaredLogger.Infof("股票数据解析完成: parts:%d", len(parts))
	if len(parts) < 35 {
		return nil, fmt.Errorf("invalid data format")
	}
	/*
		谷歌,   0
		170.2100, 1 现价
		-2.57, 2 涨跌幅
		2025-02-28 09:38:50, 3 时间
		-4.4900, 4 涨跌额
		175.9400, 5 今日开盘价
		176.5900, 6 区间
		169.7520, 7 区间
		208.7000, 8 52周区间
		130.9500, 9 52周区间
		25930485, 10 成交量
		17083496, 11 10日均量
		2074859900000, 12 市值
		8.13, 13 每股收益
		20.940000 , 14 市盈率
		0.00,  15
		0.00,  16
		0.20,  17
		0.00,	18
		12190000000, 19
		71, 20
		170.2000, 21 盘前盘后盘
		-0.01, 22  盘前盘后涨跌幅
		-0.01, 23
		Feb 27 07:59PM EST, 24
		Feb 27 04:00PM EST, 25
		174.7000, 26 前收盘
		2917444, 27
		1, 28
		2025, 29
		4456143849.0000, 30
		176.1200, 31
		163.7039, 32
		496605933.1411, 33
		170.2100, 34 现价
		174.7000 35 前收盘
	*/
	result["股票代码"] = code
	result["股票名称"] = parts[0]
	result["今日开盘价"] = parts[5]

	if len(parts) >= 36 {
		result["昨日收盘价"] = strutil.ReplaceWithMap(strutil.RemoveNonPrintable(parts[26]), map[string]string{"\"": "", ";": ""})
	} else {
		result["昨日收盘价"] = strutil.ReplaceWithMap(strutil.RemoveNonPrintable(parts[len(parts)-1]), map[string]string{"\"": "", ";": ""})
	}

	result["今日最高价"] = parts[6]
	result["今日最低价"] = parts[7]
	result["当前价格"] = parts[1]
	result["盘前盘后"] = parts[21]
	result["盘前盘后涨跌幅"] = parts[22]
	result["日期"] = strutil.SplitAndTrim(parts[3], " ", "")[0]
	result["时间"] = strutil.SplitAndTrim(parts[3], " ", "")[1]
	//logger.SugaredLogger.Infof("美股股票数据解析完成: %v", result)
	return result, nil
}

func ParseHKStockData(datas []string) (map[string]string, error) {
	code := strings.Split(datas[0], "hq_str_")[1]
	result := make(map[string]string)
	parts := strutil.SplitAndTrim(datas[1], ",", "\"", ";")
	//parts := strings.Split(data, ",")
	if len(parts) < 19 {
		return nil, fmt.Errorf("invalid data format")
	}
	/*
		XIAOMI-W,    0
		小米集团－Ｗ,  1 股票名称
		50.050,		 2 今日开盘价
		49.150,		 3 昨日收盘价
		51.950,      4 今日最高价
		49.700,      5 今日最低价
		51.700,      6 当前价格
		2.550,       7 涨跌额
		5.188,		 8 涨跌幅
		51.65000,    9
		51.70000,    10
		15770408249, 11 成交额
		308362585,   12 成交量
		0.000,       13
		0.000,       14
		51.950,		 15 52周最高
		12.560,		 16 52周最低
		2025/02/21,  17
		16:08        18
	*/
	result["股票代码"] = code
	result["股票名称"] = parts[1]
	result["今日开盘价"] = parts[2]
	result["昨日收盘价"] = parts[3]
	result["今日最高价"] = parts[4]
	result["今日最低价"] = parts[5]
	result["当前价格"] = parts[6]
	result["日期"] = strings.ReplaceAll(parts[17], "/", "-")
	result["时间"] = strings.ReplaceAll(parts[18], "\";", ":00")
	//logger.SugaredLogger.Infof("股票数据解析完成: %v", result)
	return result, nil
}

func ParseSHSZStockData(datas []string) (map[string]string, error) {
	code := strings.Split(datas[0], "hq_str_")[1]
	result := make(map[string]string)
	parts := strutil.SplitAndTrim(datas[1], ",", "\"")
	//parts := strings.Split(data, ",")
	if len(parts) < 32 {
		return nil, fmt.Errorf("invalid data format")
	}
	/*
		0：”大秦铁路”，股票名字；
		1：”27.55″，今日开盘价；
		2：”27.25″，昨日收盘价；
		3：”26.91″，当前价格；
		4：”27.55″，今日最高价；
		5：”26.20″，今日最低价；
		6：”26.91″，竞买价，即“买一”报价；
		7：”26.92″，竞卖价，即“卖一”报价；
		8：”22114263″，成交的股票数，由于股票交易以一百股为基本单位，所以在使用时，通常把该值除以一百；
		9：”589824680″，成交金额，单位为“元”，为了一目了然，通常以“万元”为成交金额的单位，所以通常把该值除以一万；
		10：”4695″，“买一”申报4695股，即47手；
		11：”26.91″，“买一”报价；
		12：”57590″，“买二”
		13：”26.90″，“买二”
		14：”14700″，“买三”
		15：”26.89″，“买三”
		16：”14300″，“买四”
		17：”26.88″，“买四”
		18：”15100″，“买五”
		19：”26.87″，“买五”
		20：”3100″，“卖一”申报3100股，即31手；
		21：”26.92″，“卖一”报价
		(22, 23), (24, 25), (26,27), (28, 29)分别为“卖二”至“卖四的情况”
		30：”2008-01-11″，日期；
		31：”15:05:32″，时间；*/
	result["股票代码"] = code
	result["股票名称"] = parts[0]
	result["今日开盘价"] = parts[1]
	result["昨日收盘价"] = parts[2]
	result["当前价格"] = parts[3]
	result["今日最高价"] = parts[4]
	result["今日最低价"] = parts[5]
	result["竞买价"] = parts[6]
	result["竞卖价"] = parts[7]
	result["成交的股票数"] = parts[8]
	result["成交金额"] = parts[9]
	result["买一申报"] = parts[10]
	result["买一报价"] = parts[11]
	result["买二申报"] = parts[12]
	result["买二报价"] = parts[13]
	result["买三申报"] = parts[14]
	result["买三报价"] = parts[15]
	result["买四申报"] = parts[16]
	result["买四报价"] = parts[17]
	result["买五申报"] = parts[18]
	result["买五报价"] = parts[19]
	result["卖一申报"] = parts[20]
	result["卖一报价"] = parts[21]
	result["卖二申报"] = parts[22]
	result["卖二报价"] = parts[23]
	result["卖三申报"] = parts[24]
	result["卖三报价"] = parts[25]
	result["卖四申报"] = parts[26]
	result["卖四报价"] = parts[27]
	result["卖五申报"] = parts[28]
	result["卖五报价"] = parts[29]
	result["日期"] = parts[30]
	result["时间"] = parts[31]
	return result, nil
}

func SearchStockPriceInfo(stockName, stockCode string, crawlTimeOut int64) *[]string {
	stockData, err := NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		return &[]string{}
	}
	info := (*stockData)[0]
	var messages []string
	priceTime := info.Date + " " + info.Time
	name := info.Name
	if name == "" {
		name = stockName
	}
	messages = append(messages, fmt.Sprintf("%s:%s现价%s", priceTime, name, info.Price))
	messages = append(messages, fmt.Sprintf("开盘价: %s", info.Open))
	messages = append(messages, fmt.Sprintf("最高价: %s", info.High))
	messages = append(messages, fmt.Sprintf("最低价: %s", info.Low))
	messages = append(messages, fmt.Sprintf("昨收价: %s", info.PreClose))
	messages = append(messages, fmt.Sprintf("成交量: %s", info.Volume))
	messages = append(messages, fmt.Sprintf("成交额: %s", info.Amount))
	if info.ChangePercent != 0 {
		messages = append(messages, fmt.Sprintf("涨跌幅: %.2f%%", info.ChangePercent))
	}
	if info.ChangePrice != 0 {
		messages = append(messages, fmt.Sprintf("涨跌额: %.2f", info.ChangePrice))
	}
	return &messages
}

func SearchStockInfo(stock, msgType string, crawlTimeOut int64) *[]string {
	if crawlTimeOut <= 0 {
		crawlTimeOut = 10
	}
	apiURL := fmt.Sprintf("https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraphSearch&os=web&sv=8.7.9&keyword=%s&type=%s", url2.QueryEscape(RemoveAllBlankChar(stock)), url2.QueryEscape(msgType))
	client := resty.New()
	resp, err := client.SetTimeout(time.Duration(crawlTimeOut)*time.Second).
		SetRetryCount(1).
		SetRetryWaitTime(300*time.Millisecond).R().
		SetHeader("Host", "www.cls.cn").
		SetHeader("Origin", "https://www.cls.cn").
		SetHeader("Referer", "https://www.cls.cn/searchPage").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0").
		Get(apiURL)
	if err != nil || resp.StatusCode() != 200 {
		logger.SugaredLogger.Debugf("SearchStockInfo HTTP failed: %v", err)
		return &[]string{}
	}
	var result struct {
		Data []struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			Time    string `json:"ctime"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Debugf("SearchStockInfo JSON unmarshal failed: %v", err)
		return &[]string{}
	}
	var messages []string
	for _, item := range result.Data {
		text := item.Title
		if text == "" {
			text = item.Content
		}
		messages = append(messages, ReplaceSensitiveWords(text))
	}
	return &messages
}

// 分时数据

//lint:ignore U1000 getDCStockInfo is kept for legacy compatibility
func (receiver StockDataApi) getDCStockInfo(market string, page, pageSize int) {
	//m:105,m:106,m:107  //美股
	//m:128+t:3,m:128+t:4,m:128+t:1,m:128+t:2 //港股
	fs := "m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23,m:0+t:81+s:2048"
	switch market {
	case "hk":
		fs = "m:128+t:3,m:128+t:4,m:128+t:1,m:128+t:2"
	case "us":
		fs = "m:105,m:106,m:107"
	}

	url := "https://push2.eastmoney.com/api/qt/clist/get?np=1&fltt=1&invt=2&cb=data&fs=%s&fields=f12,f13,f14,f1,f2,f4,f3,f152,f5,f6,f7,f15,f18,f16,f17,f10,f8,f9,f23,f100,f265&fid=f3&pn=%d&pz=%d&po=1&dect=1&wbp2u=|0|0|0|web&_=%d"
	sprintfUrl := fmt.Sprintf(url, fs, page, pageSize, time.Now().UnixMilli())
	//logger.SugaredLogger.Infof("page:%d  url:%s", page, sprintfUrl)
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "push2.eastmoney.com").
		SetHeader("Referer", "https://quote.eastmoney.com/center/gridlist.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:146.0) Gecko/20100101 Firefox/146.0").
		Get(sprintfUrl)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return
	}
	body := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")
	val, err := vm.Run(body)
	if err != nil {
		logger.SugaredLogger.Errorf("vm.Run error: %v", err)
	}
	value, _ := val.Object().Value().Export()
	marshal, _ := json.Marshal(value)
	data := make(map[string]any)
	json.Unmarshal(marshal, &data)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error: %v", err)
	}
	//logger.SugaredLogger.Infof("resp:%s", data)
	if data["data"] != nil {
		datas := data["data"].(map[string]any)
		_ = datas["total"].(float64)
		diff := datas["diff"].([]any)
		//logger.SugaredLogger.Infof("total:%d", int(total))
		for _, item := range diff {
			stock := item.(map[string]any)
			//logger.SugaredLogger.Infof("k:%d,%s:%s:%s %s:%s", k, stock["f14"], stock["f12"], DCToTsCode(stock["f12"].(string)), stock["f100"], stock["f265"])

			if market == "" {
				stockInfo := &StockBasic{
					Symbol: stock["f12"].(string),
					TsCode: DCToTsCode(stock["f12"].(string)),
					Name:   stock["f14"].(string),
					BKName: stock["f100"].(string),
					BKCode: stock["f265"].(string),
				}
				db.Dao.Model(&StockBasic{}).Where("symbol = ?", stockInfo.Symbol).First(stockInfo)
				//logger.SugaredLogger.Infof("stockInfo:%+v", stockInfo)
				if stockInfo.ID == 0 {
					db.Dao.Model(&StockBasic{}).Create(stockInfo)
				} else {
					stockInfo = &StockBasic{
						Symbol: stock["f12"].(string),
						TsCode: DCToTsCode(stock["f12"].(string)),
						Name:   stock["f14"].(string),
						BKName: stock["f100"].(string),
						BKCode: stock["f265"].(string),
					}
					db.Dao.Model(&StockBasic{}).Where("symbol = ?", stockInfo.Symbol).Updates(stockInfo)
				}
			}

			if market == "hk" {
				stockInfo := &models.StockInfoHK{
					Code:   strutil.PadStart(stock["f12"].(string), 5, "0") + ".HK",
					Name:   stock["f14"].(string),
					BKName: stock["f100"].(string),
					BKCode: stock["f265"].(string),
				}
				db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stockInfo.Code).First(stockInfo)
				//logger.SugaredLogger.Infof("stockInfo:%+v", stockInfo)
				if stockInfo.ID == 0 {
					db.Dao.Model(&models.StockInfoHK{}).Create(stockInfo)
				} else {
					stockInfo = &models.StockInfoHK{
						Code:   strutil.PadStart(stock["f12"].(string), 5, "0") + ".HK",
						Name:   stock["f14"].(string),
						BKName: stock["f100"].(string),
						BKCode: stock["f265"].(string),
					}
					db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stockInfo.Code).Updates(stockInfo)
				}
			}

			if market == "us" {
				stockInfo := &models.StockInfoUS{
					Code:   strutil.PadStart(stock["f12"].(string), 5, "0") + ".US",
					Name:   stock["f14"].(string),
					BKName: stock["f100"].(string),
					BKCode: stock["f265"].(string),
				}
				db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stockInfo.Code).First(stockInfo)
				//logger.SugaredLogger.Infof("stockInfo:%+v", stockInfo)
				if stockInfo.ID == 0 {
					db.Dao.Model(&models.StockInfoUS{}).Create(stockInfo)
				} else {
					stockInfo = &models.StockInfoUS{
						Code:   strutil.PadStart(stock["f12"].(string), 5, "0") + ".US",
						Name:   stock["f14"].(string),
						BKName: stock["f100"].(string),
						BKCode: stock["f265"].(string),
					}
					db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stockInfo.Code).Updates(stockInfo)
				}
			}

		}

	}
}

func DCToTsCode(dcCode string) string {
	//北京证券交易所	8（83、87、88 等）	创新型中小企业（专精特新为主）
	//上海证券交易所	6（60、688 等）	大盘蓝筹、科创板（高新技术）
	//深圳证券交易所	0、3（000、002、30 等）	中小盘、创业板（成长型创新企业）
	switch dcCode[0:1] {
	case "8":
		return dcCode + ".BJ"
	case "9":
		return dcCode + ".BJ"
	case "6":
		return dcCode + ".SH"
	case "0":
		return dcCode + ".SZ"
	case "3":
		return dcCode + ".SZ"
	}
	return ""
}

func saveHKStockInfo(d map[string]any) {
	for _, v := range d["page_data"].([]any) {
		vv := v.(string)
		splits := strings.Split(vv, "~")
		stock := &models.StockInfoHK{
			Code: strutil.PadStart(splits[0], 5, "0") + ".HK",
			Name: splits[1],
		}
		//logger.SugaredLogger.Infof("vv:%s", vv)
		db.Dao.Model(stock).Where("code = ?", stock.Code).First(stock)
		if stock.ID == 0 {
			//logger.SugaredLogger.Infof("stock:%+v", stock)
			db.Dao.Model(&models.StockInfoHK{}).Create(stock)
		}
	}
}
