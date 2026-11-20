package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/strutil"
)

func (f *FundApi) CrawlFundNetEstimatedUnit(code string) {
	if IsOnExchangeFund(code) {
		f.crawlOnExchangeFundQuote(code)
		return
	}
	var fundNetUnitValue FundNetUnitValue
	response, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", "https://fund.eastmoney.com/").
		SetQueryParams(map[string]string{"rt": strconv.FormatInt(time.Now().UnixMilli(), 10)}).
		Get(fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", code))
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
	} else if response.StatusCode() == 200 {
		htmlContent := string(response.Body())
		if strings.Contains(htmlContent, "jsonpgz") {
			htmlContent = strutil.Trim(htmlContent, "jsonpgz(", ");")
			htmlContent = strutil.Trim(htmlContent, ");")
			err := json.Unmarshal([]byte(htmlContent), &fundNetUnitValue)
			if err == nil && fundNetUnitValue.Gsz != "" {
				fund := &FollowedFund{
					Code:             fundNetUnitValue.Fundcode,
					Name:             fundNetUnitValue.Name,
					NetEstimatedTime: fundNetUnitValue.Gztime,
				}
				netEstimatedUnit, err := convertor.ToFloat(fundNetUnitValue.Gsz)
				if err == nil {
					fund.NetEstimatedUnit = &netEstimatedUnit
				}
				netEstimatedRate, err := convertor.ToFloat(fundNetUnitValue.Gszzl)
				if err == nil {
					fund.NetEstimatedRate = &netEstimatedRate
				}
				netUnitValuePrev, err := convertor.ToFloat(fundNetUnitValue.Dwjz)
				if err == nil {
					fund.NetUnitValuePrev = &netUnitValuePrev
				}
				db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
				return
			}
		}
	}
	f.crawlFundEstimatedViaSina(code)
}

func (f *FundApi) crawlFundEstimatedViaSina(code string) {
	response, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", "https://finance.sina.com.cn/").
		Get(fmt.Sprintf("http://hq.sinajs.cn/list=fu_%s", code))
	if err == nil && response.StatusCode() == 200 {
		data := string(GB18030ToUTF8(response.Body()))
		datas := strutil.SplitAndTrim(data, "=", "\"")
		if len(datas) >= 2 {
			parts := strings.Split(datas[1], ",")
			if len(parts) >= 8 {
				gsz := strings.TrimSpace(parts[2])
				if gsz != "" && gsz != "0" {
					gszzl := strings.TrimSpace(parts[6])
					dwjz := strings.TrimSpace(parts[3])
					date := strings.TrimSpace(parts[7])
					tm := strings.TrimSpace(parts[1])
					fund := &FollowedFund{
						Code:             code,
						NetEstimatedTime: date + " " + tm,
					}
					netEstimatedUnit, err := convertor.ToFloat(gsz)
					if err == nil {
						fund.NetEstimatedUnit = &netEstimatedUnit
					}
					netEstimatedRate, err := convertor.ToFloat(gszzl)
					if err == nil {
						fund.NetEstimatedRate = &netEstimatedRate
					}
					netUnitValuePrev, err := convertor.ToFloat(dwjz)
					if err == nil {
						fund.NetUnitValuePrev = &netUnitValuePrev
					}
					db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
					return
				}
			}
		}
	}
	f.crawlFundEstimatedViaMobileAPI(code)
}

func (f *FundApi) crawlFundEstimatedViaMobileAPI(code string) {
	type MobileFundInfo struct {
		FCODE     string  `json:"FCODE"`
		SHORTNAME string  `json:"SHORTNAME"`
		PDATE     string  `json:"PDATE"`
		NAV       string  `json:"NAV"`
		ACCNAV    string  `json:"ACCNAV"`
		NAVCHGRT  string  `json:"NAVCHGRT"`
		GSZ       *string `json:"GSZ"`
		GSZZL     *string `json:"GSZZL"`
		GZTIME    *string `json:"GZTIME"`
	}
	type MobileAPIResponse struct {
		Datas   []MobileFundInfo `json:"Datas"`
		ErrCode int              `json:"ErrCode"`
		Success bool             `json:"Success"`
	}

	url := fmt.Sprintf("https://fundmobapi.eastmoney.com/FundMNewApi/FundMNFInfo?pageIndex=1&pageSize=1&plat=Android&appType=ttjj&product=EFund&Version=1&deviceid=1&Ession=1&Fcodes=%s", code)
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		Get(url)
	if err != nil || resp.StatusCode() != 200 {
		return
	}

	var result MobileAPIResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil || !result.Success || len(result.Datas) == 0 {
		return
	}

	data := result.Datas[0]
	fund := &FollowedFund{
		Code: data.FCODE,
		Name: data.SHORTNAME,
	}

	if data.GSZ != nil && *data.GSZ != "" && data.GZTIME != nil && *data.GZTIME != "" {
		gsz, err := convertor.ToFloat(*data.GSZ)
		if err == nil {
			fund.NetEstimatedUnit = &gsz
		}
		if data.GSZZL != nil {
			gszzl, err := convertor.ToFloat(*data.GSZZL)
			if err == nil {
				fund.NetEstimatedRate = &gszzl
			}
		}
		fund.NetEstimatedTime = *data.GZTIME
	}

	if data.PDATE != "" && data.NAV != "" {
		nav, err := convertor.ToFloat(data.NAV)
		if err == nil {
			fund.NetUnitValue = &nav
		}
		fund.NetUnitValueDate = data.PDATE
	}

	if data.NAVCHGRT != "" && data.PDATE == time.Now().Format("2006-01-02") {
		navChgRt, err := convertor.ToFloat(data.NAVCHGRT)
		if err == nil {
			fund.NetActualRate = &navChgRt
		}
	}

	db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
}

func (f *FundApi) crawlOnExchangeFundQuote(code string) {
	var sinaCode string
	if len(code) < 1 {
		return
	}
	switch code[0:1] {
	case "5", "6":
		sinaCode = "sh" + code
	case "1", "2":
		sinaCode = "sz" + code
	default:
		return
	}
	url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), sinaCode)
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "hq.sinajs.cn").
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", "https://finance.sina.com.cn").
		Get(url)
	if err != nil || resp.StatusCode() != 200 {
		return
	}
	data := string(GB18030ToUTF8(resp.Body()))
	parts := strings.SplitN(data, "=", 2)
	if len(parts) < 2 {
		return
	}
	content := strings.Trim(parts[1], " \"\n\r;")
	if content == "" {
		return
	}
	fields := strings.Split(content, ",")
	if len(fields) < 4 {
		return
	}
	currentPrice, err := convertor.ToFloat(fields[3])
	if err != nil || currentPrice == 0 {
		return
	}
	yesterdayPrice, err := convertor.ToFloat(fields[2])
	if err != nil || yesterdayPrice == 0 {
		return
	}
	changeRate := (currentPrice - yesterdayPrice) / yesterdayPrice * 100
	changeRate = mathutil.RoundToFloat(changeRate, 2)

	fund := &FollowedFund{
		Code:             code,
		NetEstimatedUnit: &currentPrice,
		NetEstimatedRate: &changeRate,
		NetEstimatedTime: strings.TrimSpace(fields[31]),
	}
	db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
}

func (f *FundApi) CrawlFundNetUnitValue(code string) {
	if IsOnExchangeFund(code) {
		f.crawlOnExchangeFundNetUnitValue(code)
		return
	}
	url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=f_%s", time.Now().UnixMilli(), code)
	response, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "hq.sinajs.cn").
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", "https://finance.sina.com.cn").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	if err == nil && response.StatusCode() == 200 {
		data := string(GB18030ToUTF8(response.Body()))
		datas := strutil.SplitAndTrim(data, "=", "\"")
		if len(datas) >= 2 {
			parts := strutil.SplitAndTrim(datas[1], ",", "\"")
			val, err := convertor.ToFloat(parts[1])
			if err == nil {
				fund := &FollowedFund{
					Name:             parts[0],
					Code:             code,
					NetUnitValue:     &val,
					NetUnitValueDate: parts[4],
				}
				db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
				return
			}
		}
	}
	f.crawlFundNetUnitValueViaEastMoney(code)
}

func (f *FundApi) crawlOnExchangeFundNetUnitValue(code string) {
	klineApi := NewFundKLineApi()
	result := klineApi.GetFundKLine(code, "101", 3)
	if result == nil || result.Data == nil || len(*result.Data) < 1 {
		return
	}
	data := *result.Data
	latest := data[len(data)-1]
	val, err := convertor.ToFloat(latest.Close)
	if err != nil || val == 0 {
		return
	}
	date := latest.Day
	if strings.Contains(date, " ") {
		date = strings.Split(date, " ")[0]
	}
	fund := &FollowedFund{
		Code:             code,
		NetUnitValue:     &val,
		NetUnitValueDate: date,
	}
	if len(data) >= 2 {
		prev := data[len(data)-2]
		prevVal, err := convertor.ToFloat(prev.Close)
		if err == nil && prevVal > 0 {
			fund.NetUnitValuePrev = &prevVal
		}
	}
	db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
}

func (f *FundApi) crawlFundNetUnitValueViaEastMoney(code string) {
	history, err := f.GetFundHistoryNetValue(code, 1, 2, "", "")
	if err != nil || len(history) == 0 {
		return
	}
	latest := history[0]
	val := latest.NetValue
	fund := &FollowedFund{
		Code:             code,
		NetUnitValue:     &val,
		NetUnitValueDate: latest.Date,
	}
	if len(history) >= 2 {
		prevVal := history[1].NetValue
		fund.NetUnitValuePrev = &prevVal
	}
	db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
}

func (f *FundApi) GetFundHistoryNetValue(fundCode string, pageIndex, pageSize int, startDate, endDate string) ([]FundHistoryNetValue, error) {
	if IsOnExchangeFund(fundCode) {
		return f.getOnExchangeFundHistoryNetValue(fundCode, pageSize)
	}
	url := fmt.Sprintf("http://api.fund.eastmoney.com/f10/lsjz?fundCode=%s&pageIndex=%d&pageSize=%d&startDate=%s&endDate=%s&_%d",
		fundCode, pageIndex, pageSize, startDate, endDate, time.Now().UnixMilli())
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", fmt.Sprintf("http://fundf10.eastmoney.com/jjjz_%s.html", fundCode)).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求历史净值失败: %w", err)
	}

	var result struct {
		Data struct {
			LSJZList []struct {
				FSRQ  string `json:"FSRQ"`
				DWJZ  string `json:"DWJZ"`
				LJJZ  string `json:"LJJZ"`
				JZZZL string `json:"JZZZL"`
				SGZT  string `json:"SGZT"`
				SHZT  string `json:"SHZT"`
			} `json:"LSJZList"`
			TotalCount int `json:"TotalCount"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("解析历史净值失败: %w", err)
	}

	var values []FundHistoryNetValue
	for _, item := range result.Data.LSJZList {
		dwjz, _ := convertor.ToFloat(item.DWJZ)
		ljjz, _ := convertor.ToFloat(item.LJJZ)
		jzzzl, _ := convertor.ToFloat(item.JZZZL)
		values = append(values, FundHistoryNetValue{
			Date:        item.FSRQ,
			NetValue:    dwjz,
			AccumValue:  ljjz,
			DailyGrowth: jzzzl,
			BuyStatus:   item.SGZT,
			SellStatus:  item.SHZT,
		})
	}
	return values, nil
}

func IsOnExchangeFund(code string) bool {
	if len(code) < 2 {
		return false
	}
	prefix := code[:2]
	switch prefix {
	case "15", "16", "18", "50", "51", "52", "56", "58":
		return true
	default:
		return false
	}
}

func (f *FundApi) getOnExchangeFundHistoryNetValue(fundCode string, pageSize int) ([]FundHistoryNetValue, error) {
	klineApi := NewFundKLineApi()
	result := klineApi.GetFundKLine(fundCode, "101", pageSize)
	if result == nil || result.Data == nil || len(*result.Data) == 0 {
		return nil, fmt.Errorf("获取场内基金历史行情失败")
	}
	klineData := *result.Data
	var values []FundHistoryNetValue
	for i := len(klineData) - 1; i >= 0; i-- {
		item := klineData[i]
		closePrice, _ := convertor.ToFloat(item.Close)
		var dailyGrowth float64
		if i > 0 {
			prevClose, _ := convertor.ToFloat(klineData[i-1].Close)
			if prevClose > 0 {
				dailyGrowth = (closePrice - prevClose) / prevClose * 100
				dailyGrowth = mathutil.RoundToFloat(dailyGrowth, 2)
			}
		} else {
			dailyGrowth, _ = convertor.ToFloat(item.ChangePercent)
		}
		date := item.Day
		if strings.Contains(date, " ") {
			date = strings.Split(date, " ")[0]
		}
		values = append(values, FundHistoryNetValue{
			Date:        date,
			NetValue:    closePrice,
			AccumValue:  0,
			DailyGrowth: dailyGrowth,
			BuyStatus:   "-",
			SellStatus:  "-",
		})
	}
	return values, nil
}

func fundKLineSecid(code string) string {
	if !IsOnExchangeFund(code) {
		return ""
	}
	if len(code) < 1 {
		return ""
	}
	first := code[0:1]
	switch first {
	case "5":
		return "1." + code
	case "1":
		return "0." + code
	default:
		return ""
	}
}
