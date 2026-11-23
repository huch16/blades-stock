package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/logger"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
)

// 分时数据
func (receiver StockDataApi) GetStockMinutePriceData(stockCode string) (*[]MinuteData, string) {

	stockCode = FormatMarketStockCode(strings.TrimSpace(stockCode))

	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/minute/query?code=%s", stockCode)
	if strutil.HasPrefixAny(stockCode, []string{"gb_", "GB_"}) {
		stockCode = strings.Replace(strings.ToUpper(stockCode), "GB_", "us", 1) + ".OQ"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "US"}) {
		url = fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/UsMinute/query?code=%s", stockCode)
	}
	//logger.SugaredLogger.Infof("GetStockMinutePriceData url:%s", url)
	res := make(map[string]interface{})
	// 请求级 context 超时：receiver.client 是共享 SharedHTTPClient，在此调
	// SetTimeout 会改写所有并发请求的超时（数据竞态，见 httpclient.go
	// crawlTimeoutContext），超时语义不变。
	ctx, cancel := crawlTimeoutContext(receiver.config.CrawlTimeOut)
	defer cancel()
	resp, err := receiver.client.R().SetContext(ctx).
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)

	date := ""
	minuteDatas := &[]MinuteData{}

	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return minuteDatas, date
	}
	//logger.SugaredLogger.Infof("resp:%s", resp.Body())
	json.Unmarshal(resp.Body(), &res)
	code, _ := convertor.ToInt(res["code"])
	if res["data"] != nil && code == 0 {
		if dataMap, ok := res["data"].(map[string]interface{}); ok && dataMap != nil {
			var stockData any
			pure := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(stockCode), "sh"), "sz")
			for k, v := range dataMap {
				kLower := strings.ToLower(k)
				if kLower == strings.ToLower(stockCode) || strings.Contains(kLower, pure) {
					stockData = v
					break
				}
			}
			if stockData != nil {
				if m, ok := stockData.(map[string]interface{}); ok && m != nil {
					if d, ok := m["data"]; ok && d != nil {
						if m2, ok := d.(map[string]any); ok && m2 != nil {
							if minutePriceData, ok := m2["data"]; ok && minutePriceData != nil {
								if datas, ok := minutePriceData.([]any); ok {
									for _, item := range datas {
										if str, ok := item.(string); ok {
											minuteDataSplit := strutil.SplitEx(strutil.ReplaceWithMap(str, map[string]string{
												"\r\n": " ",
											}), " ", true)
											if len(minuteDataSplit) >= 3 {
												price, _ := convertor.ToFloat(minuteDataSplit[1])
												volume, _ := convertor.ToFloat(minuteDataSplit[2])
												amount := float64(0)
												if len(minuteDataSplit) >= 4 {
													amount, _ = convertor.ToFloat(minuteDataSplit[3])
												}
												tStr := minuteDataSplit[0]
												if len(tStr) >= 4 {
													tStr = tStr[0:2] + ":" + tStr[2:4]
												}
												minuteData := &MinuteData{
													Time:   tStr,
													Price:  price,
													Volume: volume,
													Amount: amount,
												}
												*minuteDatas = append(*minuteDatas, *minuteData)
											}
										}
									}
									if dateRaw, ok := m2["date"]; ok && dateRaw != nil {
										date = convertor.ToString(dateRaw)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return minuteDatas, date
}

func sinaScaleFromKLineType(kLineType string) string {
	switch kLineType {
	case "day", "240":
		return "240"
	case "week", "1200":
		return "1200"
	case "month":
		return "7200"
	case "5":
		return "5"
	case "15":
		return "15"
	case "30":
		return "30"
	case "60":
		return "60"
	default:
		return kLineType
	}
}

func (receiver StockDataApi) GetKLineData(stockCode string, kLineType string, days int64) *[]KLineData {
	scale := sinaScaleFromKLineType(kLineType)
	ts := time.Now().UnixMilli()
	callback := fmt.Sprintf("callback_%d", ts)
	url := fmt.Sprintf("https://quotes.sina.cn/cn/api/jsonp_v2.php/%s/CN_MarketDataService.getKLineData?symbol=%s&scale=%s&ma=no&datalen=%d", callback, stockCode, scale, days)
	K := &[]KLineData{}
	// 请求级 context 超时（不用共享 client 的 SetTimeout，见上）。
	ctx, cancel := crawlTimeoutContext(receiver.config.CrawlTimeOut)
	defer cancel()
	resp, err := receiver.client.R().SetContext(ctx).
		SetHeader("Host", "quotes.sina.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
		return K
	}
	body := string(resp.Body())
	// 解析 JSONP 响应：提取第一个 [ 到最后一个 ] 之间的内容
	start := strings.Index(body, "[")
	end := strings.LastIndex(body, "]")
	if start < 0 || end <= start {
		logger.SugaredLogger.Errorf("GetKLineData: no array found in response body: %s", truncateStr(body, 200))
		return K
	}
	jsonStr := body[start : end+1]
	if err := json.Unmarshal([]byte(jsonStr), K); err != nil {
		logger.SugaredLogger.Errorf("GetKLineData: json unmarshal error: %v, body: %s", err, truncateStr(body, 200))
	}
	return K
}

func (receiver StockDataApi) GetHK_KLineData(stockCode string, kLineType string, days int64) *[]KLineData {

	//logger.SugaredLogger.Infof("GetHK_KLineData stockCode:%s,kLineType:%s,days:%d", stockCode, kLineType, days)
	if strutil.HasPrefixAny(stockCode, []string{"gb_", "GB_"}) {
		stockCode = strings.Replace(stockCode, "gb_", "us", 1) + ".OQ"
	}

	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d,qfq", stockCode, kLineType, days)
	//logger.SugaredLogger.Infof("url:%s", url)
	K := &[]KLineData{}
	res := make(map[string]interface{})
	// 请求级 context 超时（不用共享 client 的 SetTimeout，见 GetKLineData）。
	ctx, cancel := crawlTimeoutContext(receiver.config.CrawlTimeOut)
	defer cancel()
	resp, err := receiver.client.R().SetContext(ctx).
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return K
	}
	//logger.SugaredLogger.Infof("resp:%s", resp.Body())
	json.Unmarshal(resp.Body(), &res)
	code, _ := convertor.ToInt(res["code"])
	if code != 0 {
		return K
	}
	if res["data"] != nil && code == 0 {
		if dataMap, ok := res["data"].(map[string]interface{}); ok && dataMap != nil {
			if stockData, ok := dataMap[stockCode].(map[string]interface{}); ok && stockData != nil {
				data := stockData
				var day []any
				if data["qfqday"] != nil {
					if qday, ok := data["qfqday"].([]any); ok {
						day = qday
					}
				}
				if data["day"] != nil {
					if dday, ok := data["day"].([]any); ok {
						day = dday
					}
				}
				for _, v := range day {
					if vv, ok := v.([]any); ok && len(vv) >= 6 {
						KLine := &KLineData{
							Day:    convertor.ToString(vv[0]),
							Open:   convertor.ToString(vv[1]),
							Close:  convertor.ToString(vv[2]),
							High:   convertor.ToString(vv[3]),
							Low:    convertor.ToString(vv[4]),
							Volume: convertor.ToString(vv[5]),
						}
						*K = append(*K, *KLine)
					}
				}
			}
		}
	}
	return K
}

//lint:ignore U1000 getDCStockInfo is kept for legacy compatibility

func (receiver StockDataApi) GetCommonKLineData(stockCode string, kLineType string, days int64) *[]KLineData {

	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d,qfq", stockCode, kLineType, days)
	//logger.SugaredLogger.Infof("url:%s", url)
	K := &[]KLineData{}
	res := make(map[string]interface{})
	// 请求级 context 超时（不用共享 client 的 SetTimeout，见 GetKLineData）。
	ctx, cancel := crawlTimeoutContext(receiver.config.CrawlTimeOut)
	defer cancel()
	resp, err := receiver.client.R().SetContext(ctx).
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return K
	}
	//logger.SugaredLogger.Infof("resp:%s", resp.Body())
	json.Unmarshal(resp.Body(), &res)
	code, _ := convertor.ToInt(res["code"])
	if code != 0 {
		return K
	}
	if res["data"] != nil && code == 0 {
		if dataMap, ok := res["data"].(map[string]interface{}); ok && dataMap != nil {
			if stockData, ok := dataMap[stockCode].(map[string]interface{}); ok && stockData != nil {
				data := stockData
				var day []any
				if data["qfqday"] != nil {
					if qday, ok := data["qfqday"].([]any); ok {
						day = qday
					}
				}
				if data["day"] != nil {
					if dday, ok := data["day"].([]any); ok {
						day = dday
					}
				}
				for _, v := range day {
					if vv, ok := v.([]any); ok && len(vv) >= 6 {
						KLine := &KLineData{
							Day:    convertor.ToString(vv[0]),
							Open:   convertor.ToString(vv[1]),
							Close:  convertor.ToString(vv[2]),
							High:   convertor.ToString(vv[3]),
							Low:    convertor.ToString(vv[4]),
							Volume: convertor.ToString(vv[5]),
						}
						*K = append(*K, *KLine)
					}
				}
			}
		}
	}
	return K
}

// GetStockHistoryMoneyData 获取股票历史资金流向数据
