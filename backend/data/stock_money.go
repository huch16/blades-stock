package data

import (
	"encoding/json"
	"fmt"
	url2 "net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/robertkrimen/otto"
)

// GetStockHistoryMoneyData 获取股票历史资金流向数据
func (receiver StockDataApi) GetStockHistoryMoneyData(stockCode string) []models.StockMoneyDataHis {

	stockCode = ConvertStockCodeToTushareCode(stockCode)

	var hisData []models.StockMoneyDataHis

	if strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
			"SH": "1",
			"sh": "1",
			"SZ": "0",
			"sz": "0",
			"BJ": "0",
			"bj": "0",
		})
	} else {
		if strutil.HasPrefixAny(stockCode, []string{"60", "688"}) {
			stockCode = stockCode + ".1"
		} else {
			stockCode = stockCode + ".0"
		}
	}
	if strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = strings.Split(stockCode, ".")[1] + "." + strings.Split(stockCode, ".")[0]
	}

	baseURL := "https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get"

	params := url2.Values{}
	params.Set("cb", "data")
	params.Set("lmt", "0")
	params.Set("klt", "101")
	params.Set("fields1", "f1,f2,f3,f7")
	params.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65")
	params.Set("ut", eastMoneyUTParam)
	params.Set("secid", stockCode)
	params.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	//
	//// 配置强制 IPv4 优先的 Transport，解决 IPv6 连接问题
	//dialer := &net.Dialer{
	//	Timeout:       10 * time.Second,
	//	KeepAlive:     30 * time.Second,
	//	DualStack:     false, // 禁用双栈
	//	FallbackDelay: -1,    // 禁用 Happy Eyeballs
	//}
	//receiver.client.SetTransport(&http.Transport{
	//	DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
	//		// 强制只使用 IPv4
	//		host, port, err := net.SplitHostPort(addr)
	//		if err != nil {
	//			return nil, err
	//		}
	//		// 解析 A 记录（IPv4）
	//		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	//		if err != nil {
	//			return nil, err
	//		}
	//		if len(ips) == 0 {
	//			return nil, fmt.Errorf("no IPv4 address found for %s", host)
	//		}
	//		ipv4 := ips[0].String()
	//		return dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ipv4, port))
	//	},
	//	TLSClientConfig: &tls.Config{
	//		MinVersion: tls.VersionTLS12,
	//		ServerName: "push2.eastmoney.com",
	//	},
	//	DisableCompression:  true, // 禁用自动压缩，手动处理 gzip
	//	MaxIdleConns:        100,
	//	MaxIdleConnsPerHost: 10,
	//	IdleConnTimeout:     90 * time.Second,
	//	ForceAttemptHTTP2:   false, // 强制使用 HTTP/1.1
	//})

	//logger.SugaredLogger.Infof("url:%s", reqURL)
	req := receiver.client.SetHeader("User-Agent", getRandomUA()).R()
	setEastMoneyKlineBrowserHeaders(req, "https://quote.eastmoney.com")
	// 使用缓存的 Cookie，pageURL 参数传空字符串由函数内部使用默认值
	//cookieHeader, err := FetchEastMoneyCookiesViaChromedp("", time.Second*3, reqURL)
	//if err == nil {
	//	//logger.SugaredLogger.Infof("Cookie: %s", cookieHeader)
	//	req.SetHeader("Cookie", cookieHeader)
	//}

	resp, err := req.Get(reqURL)
	if err != nil {
		logger.SugaredLogger.Errorf("req.Get error: %v", err)
		return hisData
	}
	body := strings.TrimSpace(string(resp.Body()))
	// 剔除 JSONP data(...) 包装，直接原生高速反序列化，彻底根除 otto JS 虚拟机的性能瓶颈与语法解析崩溃
	if strings.HasPrefix(body, "data(") && strings.HasSuffix(body, ");") {
		body = body[5 : len(body)-2]
	} else if strings.HasPrefix(body, "data(") && strings.HasSuffix(body, ")") {
		body = body[5 : len(body)-1]
	}

	var resData models.StockHistoryMoneyDataResp
	err = json.Unmarshal([]byte(body), &resData)
	if err != nil {
		return hisData
	}
	if len(resData.Data.Klines) > 0 {
		for _, v := range resData.Data.Klines {
			vals := strings.Split(v, ",")
			//logger.SugaredLogger.Infof("kline:%v", vals)
			hisData = append(hisData, models.StockMoneyDataHis{
				Date: convertor.ToString(vals[0]),
				F62:  convertor.ToString(vals[1]),
				F84:  convertor.ToString(vals[2]),
				F78:  convertor.ToString(vals[3]),
				F72:  convertor.ToString(vals[4]),
				F66:  convertor.ToString(vals[5]),
				F184: convertor.ToString(vals[6]),
				F87:  convertor.ToString(vals[7]),
				F81:  convertor.ToString(vals[8]),
				F75:  convertor.ToString(vals[9]),
				F69:  convertor.ToString(vals[10]),
				F2:   convertor.ToString(vals[11]),
				F3:   convertor.ToString(vals[12]),
			})
		}
	}

	return hisData

}

// GetStockMoneyData 获取个股资金流数据

// GetStockMoneyData 获取个股资金流数据
func (receiver StockDataApi) GetStockMoneyData() models.StockMoneyDataResp {

	var resData models.StockMoneyDataResp
	url := "https://push2.eastmoney.com/api/qt/clist/get?cb=data&fid=f62&po=1&pz=50&pn=1&np=1&fltt=2&invt=2&ut=8dec03ba335b81bf4ebdf7b29ec27d15&fs=m:0+t:6+f:!2,m:0+t:13+f:!2,m:0+t:80+f:!2,m:1+t:2+f:!2,m:1+t:23+f:!2,m:0+t:7+f:!2,m:1+t:3+f:!2&fields=f12,f14,f2,f3,f62,f184,f66,f69,f72,f75,f78,f81,f84,f87,f204,f205,f124,f1,f13,f100,f265"
	req := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut) * time.Second).R()

	setEastMoneyKlineBrowserHeaders(req, "https://quote.eastmoney.com")
	// 使用缓存的 Cookie，pageURL 参数传空字符串由函数内部使用默认值
	//cookieHeader, err := FetchEastMoneyCookiesViaChromedp("", time.Second*3, quoteEastMoneyPage)
	//if err == nil {
	//	//logger.SugaredLogger.Infof("Cookie: %s", cookieHeader)
	//	req.SetHeader("Cookie", cookieHeader)
	//}

	resp, err := req.
		SetHeader("Host", "push2.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("req.Get error: %v", err)
	}
	body := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")
	val, err := vm.Run(body)
	if err != nil {
		logger.SugaredLogger.Errorf("vm.Run error: %v", err)
	}
	value, err := val.Export()
	if err != nil {
		logger.SugaredLogger.Errorf("val.Export error: %v", err)
	}
	marshal, err := json.Marshal(value)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Marshal error: %v", err)
		return models.StockMoneyDataResp{}
	}
	err = json.Unmarshal(marshal, &resData)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return models.StockMoneyDataResp{}
	}
	return resData
}

// GetMutualTop10Deal 获取互联互通（沪股通/深股通/港股通）十大成交股数据
// mutualType: 001=沪股通十大成交股, 002=港股通(沪)十大成交股 , 003=深股通十大成交股, 004=港股通(深)十大成交股
// tradeDate: 交易日期，格式如 2026-03-16

// GetMutualTop10Deal 获取互联互通（沪股通/深股通/港股通）十大成交股数据
// mutualType: 001=沪股通十大成交股, 002=港股通(沪)十大成交股 , 003=深股通十大成交股, 004=港股通(深)十大成交股
// tradeDate: 交易日期，格式如 2026-03-16
func (receiver StockDataApi) GetMutualTop10Deal(mutualType, tradeDate string, page, pageSize int) *models.MutualTop10DealResp {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	filter := fmt.Sprintf("(MUTUAL_TYPE=\"%s\")(TRADE_DATE='%s')", mutualType, tradeDate)
	encodedFilter := url2.QueryEscape(filter)

	url := fmt.Sprintf("https://datacenter-web.eastmoney.com/web/api/data/v1/get?callback=data&sortColumns=RANK&sortTypes=1&pageSize=%d&pageNumber=%d&reportName=RPT_MUTUAL_TOP10DEAL&columns=ALL&source=WEB&client=WEB&filter=%s&_=%d",
		pageSize, page, encodedFilter, time.Now().UnixMilli())

	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal err:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}

	body := string(resp.Body())
	//logger.SugaredLogger.Infof("GetMutualTop10Deal resp:%s", body)

	vm := otto.New()
	// 将 JSONP 回调 data(...) 转成普通对象
	_, err = vm.Run("function data(res){return res};")
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal vm func error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}
	val, err := vm.Run(body)
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal vm run error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}
	value, err := val.Export()
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal export error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}

	marshal, err := json.Marshal(value)
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal marshal error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}

	var resData models.MutualTop10DealResp
	err = json.Unmarshal(marshal, &resData)
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal unmarshal error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}
	return &resData
}

// 获取股票概念题材信息

// 获取股票概念题材信息
func (receiver StockDataApi) GetStockConceptInfo(stockCode string) models.StockConceptInfoResp {
	//601138.SH
	if !strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = ConvertStockCodeToTushareCode(stockCode)
	}
	url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_F10_CORETHEME_BOARDTYPE&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CNEW_BOARD_CODE%2CBOARD_NAME%2CSELECTED_BOARD_REASON%2CIS_PRECISE%2CBOARD_RANK%2CBOARD_YIELD%2CDERIVE_BOARD_CODE&quoteColumns=f3~05~NEW_BOARD_CODE~BOARD_YIELD&filter=(SECUCODE%3D%22" + stockCode + "%22)(IS_PRECISE%3D%221%22)&pageNumber=1&pageSize=&sortTypes=1&sortColumns=BOARD_RANK&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	//logger.SugaredLogger.Infof("url:%s", url2.QueryEscape(url))
	var data models.StockConceptInfoResp
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("Origin", "https://emweb.securities.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("req.Get error: %v", err)
	}
	err = json.Unmarshal(resp.Body(), &data)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error: %v", err)
		return models.StockConceptInfoResp{}
	}
	return data
}

func (receiver StockDataApi) GetStockFinancialInfo(stockCode string) *models.StockFinancialInfoResp {

	if !strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = ConvertStockCodeToTushareCode(stockCode)
	}

	url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_F10_FINANCE_DUPONT&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CORG_CODE%2CORG_TYPE%2CREPORT_DATE%2CREPORT_TYPE%2CREPORT_DATE_NAME%2CSECURITY_TYPE_CODE%2CNOTICE_DATE%2CUPDATE_DATE%2CCURRENCY%2CNETPROFIT%2CTOTAL_OPERATE_INCOME%2CTOTAL_ASSETS%2CTOTAL_LIABILITIES%2CTOTAL_CURRENT_ASSETS%2CTOTAL_NONCURRENT_ASSETS%2CPARENT_NETPROFIT%2CSALE_NPR%2CTOTAL_ASSETS_TR%2CJROA%2CPARENT_NETPROFIT_RATIO%2CEQUITY_MULTIPLIER%2CROE%2CDEBT_ASSET_RATIO%2CTOTAL_INCOME%2CTOTAL_COST%2CTOTAL_EXPENSE%2CMONETARYFUNDS%2CTRADE_FINASSET%2CNOTE_RECE%2CACCOUNTS_RECE%2CFINANCE_RECE%2COTHER_RECE%2CINVENTORY%2CCREDITOR_INVEST%2CLONG_EQUITY_INVEST%2CINVEST_REALESTATE%2CFIXED_ASSET%2CCIP%2CUSERIGHT_ASSET%2CINTANGIBLE_ASSET%2CDEVELOP_EXPENSE%2CGOODWILL%2CLONG_PREPAID_EXPENSE%2CDEFER_TAX_ASSET%2CINVEST_INCOME%2CEXCHANGE_INCOME%2CFAIRVALUE_CHANGE_INCOME%2CASSET_DISPOSAL_INCOME%2COPERATE_COST%2CSURRENDER_VALUE%2CNET_COMPENSATE_EXPENSE%2CNET_CONTRACT_RESERVE%2CPOLICY_BONUS_EXPENSE%2COPERATE_TAX_ADD%2CINCOME_TAX%2CASSET_IMPAIRMENT_INCOME%2CCREDIT_IMPAIRMENT_INCOME%2CNONBUSINESS_EXPENSE%2CFINANCE_EXPENSE%2CSALE_EXPENSE%2CMANAGE_EXPENSE%2CRESEARCH_EXPENSE%2CINTEREST_NI%2CFEE_COMMISSION_NI%2CEARNED_PREMIUM%2CBUSINESS_MANAGE_EXPENSE%2COTHER_CREDITOR_INVEST%2COTHER_EQUITY_INVEST%2CLONG_RECE%2CAVAILABLE_SALE_FINASSET%2CHOLD_MATURITY_INVEST%2CFEE_COMMISSION_EXPENSE&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=12&sortTypes=-1&sortColumns=REPORT_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	//logger.SugaredLogger.Infof("url:%s", url)
	var data models.StockFinancialInfoResp
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("Origin", "https://emweb.securities.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		//SetResult(&data).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("req.Get error: %v", err)
	}
	//logger.SugaredLogger.Infof("resp:%s", string(resp.Body()))
	err = json.Unmarshal(resp.Body(), &data)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error: %v", err)
		return &models.StockFinancialInfoResp{}
	}
	//logger.SugaredLogger.Infof("data:%v", data)
	return &data
}

func (receiver StockDataApi) GetStockHolderNum(stockCode string) *models.StockHolderNumResp {
	if !strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = ConvertStockCodeToTushareCode(stockCode)
	}
	url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_F10_EH_HOLDERNUM&columns=SECUCODE%2CSECURITY_CODE%2CEND_DATE%2CHOLDER_TOTAL_NUM%2CTOTAL_NUM_RATIO%2CAVG_FREE_SHARES%2CAVG_FREESHARES_RATIO%2CHOLD_FOCUS%2CPRICE%2CAVG_HOLD_AMT%2CHOLD_RATIO_TOTAL%2CFREEHOLD_RATIO_TOTAL&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=12&sortTypes=-1&sortColumns=END_DATE&source=HSF10&client=PC&v=" + strconv.Itoa(time.Now().Nanosecond())
	//logger.SugaredLogger.Infof("url:%s", url)
	var data models.StockHolderNumResp
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("Origin", "https://emweb.securities.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		//SetResult(&data).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("req.Get error: %v", err)
	}
	err = json.Unmarshal(resp.Body(), &data)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error: %v", err)
		return &models.StockHolderNumResp{}
	}
	return &data
}

func (receiver StockDataApi) GetIndustryValuation(bkName string) *models.IndustryValuationResp {
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&reportName=RPT_VALUEINDUSTRY_STA&columns=ALL&quoteColumns=&source=WEB&client=WEB&pageNumber=1&filter=%28BOARD_NAME%3D%22" + url2.QueryEscape(bkName) + "%22%29&_=" + strconv.Itoa(time.Now().Nanosecond())
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("req.Get error: %v", err)
	}
	body := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")
	val, err := vm.Run(body)
	if err != nil {
		logger.SugaredLogger.Errorf("vm.Run error: %v", err)
	}
	value, err := val.Export()
	if err != nil {
		logger.SugaredLogger.Errorf("val.Export error: %v", err)
	}
	marshal, err := json.Marshal(value)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Marshal error: %v", err)
	}
	//logger.SugaredLogger.Infof("data:%s", string(marshal))
	data := models.IndustryValuationResp{}
	err = json.Unmarshal(marshal, &data)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error: %v", err)
	}
	return &data
}

func (receiver StockDataApi) GetAllStocks(page int, pageSize int, name string, technicalIndicators models.TechnicalIndicators) *models.AllStocksResp {
	indicators := ""
	// 将 TechnicalIndicators 转换为 map 并遍历构建查询条件
	indicatorConditions := []string{}

	// 使用反射获取结构体字段值
	v := reflect.ValueOf(technicalIndicators)
	t := reflect.TypeOf(technicalIndicators)

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// 只处理布尔类型的字段
		if value.Kind() == reflect.Bool && value.Bool() {
			// 获取 JSON 标签作为字段名
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" {
				// 构建查询条件格式：(FIELD_NAME="1")
				condition := fmt.Sprintf("(%s=\"1\")", jsonTag)
				indicatorConditions = append(indicatorConditions, condition)
			}
		}
		if value.Kind() == reflect.Int && value.Int() != 0 {
			// 获取 JSON 标签作为字段名
			jsonTag := field.Tag.Get("json")
			operator := field.Tag.Get("operator")

			if jsonTag != "" {
				// 构建查询条件格式：如 (UPP_DAYS>=3)
				condition := fmt.Sprintf("(%s%s%d)", jsonTag, operator, value.Int())
				indicatorConditions = append(indicatorConditions, condition)
			}
		}
	}
	// 拼接所有条件
	if len(indicatorConditions) > 0 {
		indicators = strings.Join(indicatorConditions, "")
	}
	//logger.SugaredLogger.Infof("indicators:%s", indicators)

	//logger.SugaredLogger.Infof("GetAllStocks page:%d,pageSize:%d,name:%s", page, pageSize, name)
	search := ""
	if name != "" {
		search = fmt.Sprintf("(SECURITY_NAME_ABBR in (\"%s\"))", name)
	}
	url := "https://data.eastmoney.com/dataapi/xuangu/list?st=CHANGE_RATE&sr=-1&ps=" + convertor.ToString(pageSize) + "&p=" + convertor.ToString(page) + "&sty=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CNEW_PRICE%2CCHANGE_RATE%2CVOLUME_RATIO%2CHIGH_PRICE%2CLOW_PRICE%2CPRE_CLOSE_PRICE%2CVOLUME%2CDEAL_AMOUNT%2CTURNOVERRATE%2CMARKET%2CCONCEPT%2CINDUSTRY&filter=%28MARKET+in+%28%22%E4%B8%8A%E4%BA%A4%E6%89%80%E4%B8%BB%E6%9D%BF%22%2C%22%E6%B7%B1%E4%BA%A4%E6%89%80%E4%B8%BB%E6%9D%BF%22%2C%22%E6%B7%B1%E4%BA%A4%E6%89%80%E5%88%9B%E4%B8%9A%E6%9D%BF%22%2C%22%E4%B8%8A%E4%BA%A4%E6%89%80%E7%A7%91%E5%88%9B%E6%9D%BF%22%2C%22%E4%B8%8A%E4%BA%A4%E6%89%80%E9%A3%8E%E9%99%A9%E8%AD%A6%E7%A4%BA%E6%9D%BF%22%2C%22%E6%B7%B1%E4%BA%A4%E6%89%80%E9%A3%8E%E9%99%A9%E8%AD%A6%E7%A4%BA%E6%9D%BF%22%2C%22%E5%8C%97%E4%BA%AC%E8%AF%81%E5%88%B8%E4%BA%A4%E6%98%93%E6%89%80%22%29%29" + url2.QueryEscape(search+indicators) + "&source=SELECT_SECURITIES&client=WEB&hyversion=v2"
	//logger.SugaredLogger.Infof("url:%s", url)
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "data.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("req.Get error: %v", err)
	}
	data := models.AllStocksResp{}
	err = json.Unmarshal(resp.Body(), &data)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error: %v", err)
		return &models.AllStocksResp{}
	}
	//for _, info := range data.Result.Data {
	//	toAllStockInfo := info.ToAllStockInfo()
	//	oldInfo := NewStockDataApi().GetStockInfoByCode(info.SECUCODE)
	//	toAllStockInfo.ID = oldInfo.ID
	//	err := NewStockDataApi().AddAllStockInfo(toAllStockInfo)
	//	if err != nil {
	//		logger.SugaredLogger.Errorf("err:%s", err.Error())
	//	}
	//}
	return &data
}

// JSONToMarkdownTable 将JSON数据转换为Markdown表格

// GetStockRZRQInfo 获取融资融券信息
func (receiver StockDataApi) GetStockRZRQInfo(stockCode string) models.StockRZRQInfoResp {
	var StockRZRQInfoResp models.StockRZRQInfoResp
	if !strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = ConvertStockCodeToTushareCode(stockCode)
	}
	filter := url2.QueryEscape(fmt.Sprintf("(SECUCODE=\"%s\")", stockCode))
	url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_RZRQ_STOCKS_DETAIL&columns=MARKET_NAME%2CMARKET_CODE%2CTRADE_DATE%2CSECURITY_CODE%2CSECUCODE%2CSECURITY_NAME_ABBR%2CFIN_BALANCE%2CFIN_BUY_AMT%2CFIN_REPAY_AMT%2CLOAN_BALANCE%2CLOAN_SELL_VOL%2CLOAN_REPAY_VOL%2CMARGIN_BALANCE%2CLOAN_BALANCE_VOL%2CFIN_NETBUY_AMT&quoteColumns=&filter=" + filter + "&pageNumber=1&pageSize=50&sortTypes=-1&sortColumns=TRADE_DATE&source=Datacenter&client=PC&v=" + convertor.ToString(time.Now().Unix())
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
		return StockRZRQInfoResp
	}
	json.Unmarshal(resp.Body(), &StockRZRQInfoResp)
	return StockRZRQInfoResp
}

// AddTradingRecord 添加交易日志
