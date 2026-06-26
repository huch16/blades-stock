package data

import (
	"fmt"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
	"go-stock/backend/logger"
	"strings"
	"time"
)

// @Author spark
// @Date 2025/2/17 12:33
// @Desc
//-----------------------------------------------------------------------------------

type TushareApi struct {
	client *resty.Client
	config *SettingConfig
}

type TushareKLineResponse struct {
	TushareResponse
	Data TushareKLineData `json:"data"`
}

type TushareKLineData struct {
	Fields []string `json:"fields"`
	Items  [][]any  `json:"items"`
}

func NewTushareApi(config *SettingConfig) *TushareApi {
	return &TushareApi{
		client: SharedHTTPClient,
		config: config,
	}
}

func tushareApiNameFromKlt(klt string) string {
	switch klt {
	case "101":
		return "daily"
	case "102":
		return "weekly"
	case "103":
		return "monthly"
	default:
		return ""
	}
}

func (receiver *TushareApi) GetKLineData(stockCode, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if receiver.config.TushareToken == "" {
		return result
	}

	stockType := getStockType(stockCode)
	if stockType != "daily" {
		return result
	}

	apiName := tushareApiNameFromKlt(klt)
	if apiName == "" {
		return result
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}

	fields := "ts_code,trade_date,open,high,low,close,pre_close,change,pct_chg,vol,amount"
	tsCode := getTsCode(stockCode)
	timeout := receiver.config.CrawlTimeOut
	if timeout <= 0 {
		timeout = 30
	}

	endDate := time.Now().Format("20060102")
	startDate := time.Now().AddDate(0, 0, -limit*2).Format("20060102")

	resp := &TushareKLineResponse{}
	_, err := receiver.client.SetTimeout(time.Duration(timeout)*time.Second).R().
		SetHeader("content-type", "application/json").
		SetBody(&TushareRequest{
			ApiName: apiName,
			Token:   receiver.config.TushareToken,
			Params: map[string]any{
				"ts_code":    tsCode,
				"start_date": startDate,
				"end_date":   endDate,
			},
			Fields: fields}).
		SetResult(resp).
		Post(tushareApiUrl)
	if err != nil {
		logger.SugaredLogger.Errorf("Tushare KLine error: %v", err)
		return result
	}
	if resp.Code != 0 {
		logger.SugaredLogger.Errorf("Tushare KLine API error: code=%d msg=%s", resp.Code, resp.Msg)
		return result
	}

	return tushareItemsToKLineData(&resp.Data.Items, resp.Data.Fields)
}

func tushareItemsToKLineData(items *[][]any, fields []string) *[]KLineData {
	result := &[]KLineData{}
	if items == nil || len(*items) == 0 {
		return result
	}

	fieldMap := make(map[string]int, len(fields))
	for i, f := range fields {
		fieldMap[f] = i
	}

	getField := func(name string) func([]any) string {
		idx, ok := fieldMap[name]
		if !ok {
			return func([]any) string { return "0" }
		}
		return func(item []any) string {
			if idx < len(item) {
				return fmt.Sprintf("%v", item[idx])
			}
			return "0"
		}
	}
	getTradeDate := getField("trade_date")
	getOpen := getField("open")
	getHigh := getField("high")
	getLow := getField("low")
	getClose := getField("close")
	getVol := getField("vol")
	getAmount := getField("amount")

	n := len(*items)
	converted := make([]KLineData, 0, n)
	for i := n - 1; i >= 0; i-- {
		item := (*items)[i]
		kd := KLineData{
			Day:    getTradeDate(item),
			Open:   getOpen(item),
			Close:  getClose(item),
			High:   getHigh(item),
			Low:    getLow(item),
			Volume: getVol(item),
			Amount: getAmount(item),
		}
		converted = append(converted, kd)
	}

	for i := range converted {
		if i > 0 {
			prevClose, _ := parseFloatToFloat(converted[i-1].Close)
			curClose, _ := parseFloatToFloat(converted[i].Close)
			curHigh, _ := parseFloatToFloat(converted[i].High)
			curLow, _ := parseFloatToFloat(converted[i].Low)
			if prevClose > 0 {
				converted[i].ChangePercent = fmt.Sprintf("%.2f", (curClose-prevClose)/prevClose*100)
				converted[i].ChangeValue = fmt.Sprintf("%.2f", curClose-prevClose)
				converted[i].Amplitude = fmt.Sprintf("%.2f", (curHigh-curLow)/prevClose*100)
			}
		}
	}

	return &converted
}

// GetDaily tushare A股日线行情
func (receiver TushareApi) GetDaily(tsCode, startDate, endDate string, crawlTimeOut int64) string {
	//logger.SugaredLogger.Debugf("tushare daily request: ts_code=%s, start_date=%s, end_date=%s", tsCode, startDate, endDate)
	fields := "ts_code,trade_date,open,high,low,close,pre_close,change,pct_chg,vol,amount"
	resp := &TushareStockBasicResponse{}
	stockType := getStockType(tsCode)
	tsCodeNEW := getTsCode(tsCode)
	//logger.SugaredLogger.Debugf("tushare daily request: %s,tsCode:%s,tsCodeNEW:%s", stockType, tsCode, tsCodeNEW)
	_, err := receiver.client.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("content-type", "application/json").
		SetBody(&TushareRequest{
			ApiName: stockType,
			Token:   receiver.config.TushareToken,
			Params: map[string]any{
				"ts_code":    tsCodeNEW,
				"start_date": startDate,
				"end_date":   endDate,
			},
			Fields: fields}).
		SetResult(resp).
		Post(tushareApiUrl)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return ""
	}
	res := ""
	if len(resp.Data.Items) > 0 {
		fieldsStr := slice.JoinFunc(resp.Data.Fields, ",", func(s string) string {
			return "\"" + convertor.ToString(s) + "\""
		})
		res += fieldsStr + "\n"
		for _, item := range resp.Data.Items {
			//logger.SugaredLogger.Debugf("%s", slice.Join(item, ","))
			t := slice.JoinFunc(item, ",", func(s any) any {
				return "\"" + convertor.ToString(s) + "\""
			})
			res += t + "\n"
		}
	}
	//logger.SugaredLogger.Debugf("tushare response: %s", res)
	return res
}

func getTsCode(code string) any {
	if strutil.HasPrefixAny(code, []string{"US", "us", "gb_"}) {
		code = strings.Replace(code, "gb_", "", 1)
		code = strings.Replace(code, "us", "", 1)
		return code
	}
	return code
}

func getStockType(code string) string {
	if strutil.HasSuffixAny(code, []string{"SZ", "SH", "sh", "sz"}) {
		return "daily"
	}
	if strutil.HasSuffixAny(code, []string{"HK", "hk"}) {
		return "hk_daily"
	}
	if strutil.HasPrefixAny(code, []string{"US", "us", "gb_"}) {
		return "us_daily"
	}
	return ""
}
