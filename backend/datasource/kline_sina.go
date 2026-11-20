package datasource

import (
	"blades-stock/backend/logger"

	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type SinaKLineApi struct {
	client *resty.Client
}

type SinaKLineItem struct {
	Day    string `json:"day"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

// NewSinaKLineApi 构造新浪 K 线客户端。crawlTimeoutSec 为爬取超时（秒），
// 由调用方（backend/data 的降级链）从设置传入；本包不读设置表。
func NewSinaKLineApi(crawlTimeoutSec int64) *SinaKLineApi {
	client := SharedHTTPClient
	client.SetTimeout(time.Duration(crawlTimeoutSec) * time.Second)
	return &SinaKLineApi{
		client: client,
	}
}

func sinaScaleFromKlt(klt string) string {
	switch klt {
	case "1":
		return "1"
	case "5":
		return "5"
	case "15":
		return "15"
	case "30":
		return "30"
	case "60":
		return "60"
	case "101":
		return "240"
	case "102":
		return "1200"
	default:
		return ""
	}
}

// SinaSymbolFromStockCode 将股票代码转为新浪符号（R-25a 自 data 移入；data 侧留同名小写转发）。
func SinaSymbolFromStockCode(stockCode string) string {
	code := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(code, ".") {
		parts := strings.Split(code, ".")
		if len(parts) == 2 {
			market := parts[1]
			pureCode := parts[0]
			switch market {
			case "SH", "SS":
				return "sh" + pureCode
			case "SZ":
				return "sz" + pureCode
			case "BJ":
				return "bj" + pureCode
			}
		}
	}
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") || strings.HasPrefix(code, "BJ") {
		return strings.ToLower(code[:2]) + code[2:]
	}
	if len(code) >= 1 {
		first := code[0:1]
		switch first {
		case "6", "5":
			return "sh" + code
		case "0", "3", "1":
			return "sz" + code
		case "4", "8", "9":
			return "bj" + code
		}
	}
	return strings.ToLower(code)
}

func (s *SinaKLineApi) GetKLineData(stockCode, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	scale := sinaScaleFromKlt(klt)
	if scale == "" {
		logger.SugaredLogger.Warnf("SinaKLine: unsupported klt %s", klt)
		return result
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 1023 {
		limit = 1023
	}
	symbol := SinaSymbolFromStockCode(stockCode)
	if symbol == "" {
		logger.SugaredLogger.Errorf("SinaKLine: invalid stock code: %s", stockCode)
		return result
	}

	ts := time.Now().UnixMilli()
	callback := fmt.Sprintf("callback_%d", ts)
	baseURL := "https://quotes.sina.cn/cn/api/jsonp_v2.php/" + callback + "/CN_MarketDataService.getKLineData"
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("scale", scale)
	params.Set("ma", "no")
	params.Set("datalen", fmt.Sprintf("%d", limit))
	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	req := s.client.R()
	req.SetHeader("User-Agent", GetRandomUA())
	req.SetHeader("Accept", "*/*")
	req.SetHeader("Referer", "https://finance.sina.com.cn")
	req.SetHeader("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := req.Get(reqURL)
	if err != nil {
		logger.SugaredLogger.Errorf("SinaKLine HTTP error: %v", err)
		return result
	}
	if resp.StatusCode() != 200 {
		logger.SugaredLogger.Errorf("SinaKLine HTTP %d", resp.StatusCode())
		return result
	}

	body := string(resp.Body())
	items := s.parseJSONPResponse(body)
	if items == nil {
		return result
	}

	converted := s.convertToKLineData(items, klt)

	if klt == "101" || klt == "102" || klt == "103" {
		converted = s.appendTodayKLine(converted, stockCode, klt)
	}

	return &converted
}

var jsonpArrayStartRe = regexp.MustCompile(`\[\s*\{`)

func (s *SinaKLineApi) parseJSONPResponse(body string) []SinaKLineItem {
	var items []SinaKLineItem
	trimmed := strings.TrimSpace(body)

	// 如果包含脚本重定向或注释（例如 /*<script>...</script>*/ ），先跳过
	if idx := strings.Index(trimmed, "*/"); idx != -1 {
		trimmed = strings.TrimSpace(trimmed[idx+2:])
	}

	startLoc := jsonpArrayStartRe.FindStringIndex(trimmed)
	if len(startLoc) == 2 {
		jsonStart := startLoc[0]
		lastBracket := strings.LastIndex(trimmed, "]")
		if lastBracket < jsonStart {
			logger.SugaredLogger.Errorf("SinaKLine: no closing ] found, body prefix: %s", TruncateStr(trimmed, 200))
			return nil
		}
		jsonStr := trimmed[jsonStart : lastBracket+1]
		if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
			logger.SugaredLogger.Errorf("SinaKLine JSONP parse error: %v, jsonStr len=%d, body prefix: %s", err, len(jsonStr), TruncateStr(trimmed, 200))
			return nil
		}
		return items
	}

	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		logger.SugaredLogger.Errorf("SinaKLine JSON parse error: %v, body prefix: %s", err, TruncateStr(trimmed, 200))
		return nil
	}
	return items
}

// TruncateStr 截断字符串到指定长度（R-25a 自 data 移入；data 侧留同名小写转发）。
func TruncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (s *SinaKLineApi) appendTodayKLine(data []KLineData, stockCode, klt string) []KLineData {
	if len(data) == 0 {
		return data
	}

	now := time.Now()
	todayStr := now.Format("2006-01-02")

	lastDay := data[len(data)-1].Day
	if len(lastDay) >= 10 && lastDay[:10] == todayStr {
		return data
	}

	hour, minute := now.Hour(), now.Minute()
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return data
	}
	if hour < 9 || (hour == 9 && minute < 30) {
		return data
	}

	symbol := SinaSymbolFromStockCode(stockCode)
	sinaCode := symbol
	if strings.HasPrefix(sinaCode, "bj") {
		sinaCode = "sb" + sinaCode[2:]
	}

	url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", now.UnixMilli(), sinaCode)
	resp, err := s.client.R().
		SetHeader("Host", "hq.sinajs.cn").
		SetHeader("Referer", "https://finance.sina.com.cn/").
		SetHeader("User-Agent", GetRandomUA()).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Warnf("SinaKLine appendTodayKLine hq error: %v", err)
		return data
	}

	body := GB18030ToUTF8(resp.Body())
	parts := strings.SplitN(body, "\"", 3)
	if len(parts) < 2 {
		return data
	}
	fields := strings.Split(parts[1], ",")
	if len(fields) < 32 {
		return data
	}

	openPrice := strings.TrimSpace(fields[1])
	prevClose := strings.TrimSpace(fields[2])
	curPrice := strings.TrimSpace(fields[3])
	highPrice := strings.TrimSpace(fields[4])
	lowPrice := strings.TrimSpace(fields[5])
	volume := strings.TrimSpace(fields[8])
	amount := strings.TrimSpace(fields[9])
	tradeDate := strings.TrimSpace(fields[30])

	if tradeDate != todayStr {
		return data
	}

	curPriceF, _ := strconv.ParseFloat(curPrice, 64)
	prevCloseF, _ := strconv.ParseFloat(prevClose, 64)
	openF, _ := strconv.ParseFloat(openPrice, 64)
	highF, _ := strconv.ParseFloat(highPrice, 64)
	lowF, _ := strconv.ParseFloat(lowPrice, 64)
	volF, _ := strconv.ParseFloat(volume, 64)
	amountF, _ := strconv.ParseFloat(amount, 64)

	if curPriceF <= 0 || openF <= 0 {
		return data
	}

	day := todayStr
	if klt == "102" {
		_, isoWeek := now.ISOWeek()
		day = fmt.Sprintf("%s-W%02d", now.Format("2006"), isoWeek)
	} else if klt == "103" {
		day = now.Format("2006-01")
	}

	todayKd := KLineData{
		Day:    day,
		Open:   fmt.Sprintf("%.2f", openF),
		Close:  fmt.Sprintf("%.2f", curPriceF),
		High:   fmt.Sprintf("%.2f", highF),
		Low:    fmt.Sprintf("%.2f", lowF),
		Volume: fmt.Sprintf("%.0f", volF/100),
		Amount: fmt.Sprintf("%.2f", amountF),
	}
	if prevCloseF > 0 {
		todayKd.ChangePercent = fmt.Sprintf("%.2f", (curPriceF-prevCloseF)/prevCloseF*100)
		todayKd.ChangeValue = fmt.Sprintf("%.2f", curPriceF-prevCloseF)
		todayKd.Amplitude = fmt.Sprintf("%.2f", (highF-lowF)/prevCloseF*100)
	}

	if klt == "101" {
		return append(data, todayKd)
	}

	if klt == "102" || klt == "103" {
		lastDayStr := data[len(data)-1].Day
		samePeriod := false
		if klt == "102" {
			samePeriod = lastDayStr == day
		} else if klt == "103" {
			samePeriod = len(lastDayStr) >= 7 && lastDayStr[:7] == day[:7]
		}
		if samePeriod {
			data[len(data)-1] = todayKd
		} else {
			data = append(data, todayKd)
		}
	}

	return data
}

func (s *SinaKLineApi) convertToKLineData(items []SinaKLineItem, klt string) []KLineData {
	result := make([]KLineData, 0, len(items))
	for i, item := range items {
		kd := KLineData{
			Day:    item.Day,
			Open:   safeStr(item.Open),
			Close:  safeStr(item.Close),
			High:   safeStr(item.High),
			Low:    safeStr(item.Low),
			Volume: safeStr(item.Volume),
			Amount: "0",
		}
		if i > 0 {
			prevClose, _ := ParseFloatToFloat(items[i-1].Close)
			curClose, _ := ParseFloatToFloat(item.Close)
			curHigh, _ := ParseFloatToFloat(item.High)
			curLow, _ := ParseFloatToFloat(item.Low)
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (curClose-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", curClose-prevClose)
			}
			if prevClose > 0 {
				kd.Amplitude = fmt.Sprintf("%.2f", (curHigh-curLow)/prevClose*100)
			}
		}
		result = append(result, kd)
	}
	return result
}

func safeStr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "null" {
		return "0"
	}
	return s
}

type TencentKLineApi struct {
	client *resty.Client
}

type TencentStockKLinePayload struct {
	Day      [][]string `json:"day,omitempty"`
	Week     [][]string `json:"week,omitempty"`
	Month    [][]string `json:"month,omitempty"`
	Qfqday   [][]string `json:"qfqday,omitempty"`
	Qfqweek  [][]string `json:"qfqweek,omitempty"`
	Qfqmonth [][]string `json:"qfqmonth,omitempty"`
}

type TencentKLineResponse struct {
	Code int                                 `json:"code"`
	Msg  string                              `json:"msg"`
	Data map[string]TencentStockKLinePayload `json:"-"`
}

func (r *TencentKLineResponse) UnmarshalJSON(b []byte) error {
	type rawResp struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	var raw rawResp
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	r.Code = raw.Code
	r.Msg = raw.Msg
	trimmedData := strings.TrimSpace(string(raw.Data))
	if strings.HasPrefix(trimmedData, "{") {
		var m map[string]TencentStockKLinePayload
		if err := json.Unmarshal(raw.Data, &m); err == nil {
			r.Data = m
		}
	} else {
		r.Data = make(map[string]TencentStockKLinePayload)
	}
	return nil
}

// NewTencentKLineApi 构造腾讯 K 线客户端。crawlTimeoutSec 为爬取超时（秒），
// 由调用方（backend/data 的降级链）从设置传入；本包不读设置表。
func NewTencentKLineApi(crawlTimeoutSec int64) *TencentKLineApi {
	client := SharedHTTPClient
	client.SetTimeout(time.Duration(crawlTimeoutSec) * time.Second)
	return &TencentKLineApi{
		client: client,
	}
}

func tencentPeriodFromKlt(klt string) string {
	switch klt {
	case "101":
		return "day"
	case "102":
		return "week"
	case "103":
		return "month"
	default:
		return ""
	}
}

func tencentSymbolFromStockCode(stockCode string) string {
	code := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(code, ".") {
		parts := strings.Split(code, ".")
		if len(parts) == 2 {
			market := parts[1]
			pureCode := parts[0]
			switch market {
			case "SH", "SS":
				return "sh" + pureCode
			case "SZ":
				return "sz" + pureCode
			case "BJ":
				return "bj" + pureCode
			}
		}
	}
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") || strings.HasPrefix(code, "BJ") {
		return strings.ToLower(code[:2]) + code[2:]
	}
	if len(code) >= 1 {
		first := code[0:1]
		switch first {
		case "6", "5":
			return "sh" + code
		case "0", "3", "1":
			return "sz" + code
		case "4", "8", "9":
			return "bj" + code
		}
	}
	return strings.ToLower(code)
}

func (t *TencentKLineApi) GetKLineData(stockCode, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	period := tencentPeriodFromKlt(klt)
	if period == "" {
		logger.SugaredLogger.Warnf("TencentKLine: unsupported klt %s (only day/week/month)", klt)
		return result
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 800 {
		limit = 800
	}
	symbol := tencentSymbolFromStockCode(stockCode)
	if symbol == "" {
		logger.SugaredLogger.Errorf("TencentKLine: invalid stock code: %s", stockCode)
		return result
	}

	varName := fmt.Sprintf("kline_%sqfq", period)
	param := fmt.Sprintf("%s,%s,,,%d,qfq", symbol, period, limit)
	params := url.Values{}
	params.Set("_var", varName)
	params.Set("param", param)
	reqURL := "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?" + params.Encode()

	req := t.client.R()
	req.SetHeader("User-Agent", GetRandomUA())
	req.SetHeader("Accept", "*/*")
	req.SetHeader("Referer", "https://gu.qq.com")

	resp, err := req.Get(reqURL)
	if err != nil {
		logger.SugaredLogger.Errorf("TencentKLine HTTP error: %v", err)
		return result
	}
	if resp.StatusCode() != 200 {
		logger.SugaredLogger.Errorf("TencentKLine HTTP %d", resp.StatusCode())
		return result
	}

	body := string(resp.Body())
	jsonStr := t.stripVarPrefix(body, varName)
	if jsonStr == "" {
		logger.SugaredLogger.Errorf("TencentKLine: failed to strip var prefix, body prefix: %s", TruncateStr(body, 200))
		return result
	}

	var tResp TencentKLineResponse
	if err := json.Unmarshal([]byte(jsonStr), &tResp); err != nil {
		logger.SugaredLogger.Errorf("TencentKLine JSON parse error: %v", err)
		return result
	}
	if tResp.Code != 0 {
		logger.SugaredLogger.Errorf("TencentKLine API error: code=%d msg=%s", tResp.Code, tResp.Msg)
		return result
	}

	return t.extractKLineData(&tResp, symbol, period)
}

func (t *TencentKLineApi) stripVarPrefix(body, varName string) string {
	trimmed := strings.TrimSpace(body)
	prefix := varName + "="
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix), ";")
	}
	return trimmed
}

func (t *TencentKLineApi) extractKLineData(tResp *TencentKLineResponse, symbol, period string) *[]KLineData {
	result := &[]KLineData{}
	for _, stockData := range tResp.Data {
		var rows [][]string
		switch period {
		case "day":
			if len(stockData.Qfqday) > 0 {
				rows = stockData.Qfqday
			} else {
				rows = stockData.Day
			}
		case "week":
			if len(stockData.Qfqweek) > 0 {
				rows = stockData.Qfqweek
			} else {
				rows = stockData.Week
			}
		case "month":
			if len(stockData.Qfqmonth) > 0 {
				rows = stockData.Qfqmonth
			} else {
				rows = stockData.Month
			}
		}
		if len(rows) == 0 {
			continue
		}
		converted := t.convertRowsToKLineData(rows)
		return &converted
	}
	return result
}

func (t *TencentKLineApi) convertRowsToKLineData(rows [][]string) []KLineData {
	result := make([]KLineData, 0, len(rows))
	for i, row := range rows {
		if len(row) < 6 {
			continue
		}
		kd := KLineData{
			Day:    row[0],
			Open:   safeStr(row[1]),
			Close:  safeStr(row[2]),
			High:   safeStr(row[3]),
			Low:    safeStr(row[4]),
			Volume: safeStr(row[5]),
			Amount: "0",
		}
		if len(row) >= 7 {
			kd.Amount = safeStr(row[6])
		}
		if i > 0 {
			prevClose, _ := ParseFloatToFloat(rows[i-1][2])
			curClose, _ := ParseFloatToFloat(row[2])
			curHigh, _ := ParseFloatToFloat(row[3])
			curLow, _ := ParseFloatToFloat(row[4])
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (curClose-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", curClose-prevClose)
				kd.Amplitude = fmt.Sprintf("%.2f", (curHigh-curLow)/prevClose*100)
			}
		}
		result = append(result, kd)
	}
	return result
}
