package data

import (
	"encoding/json"
	"fmt"
	"blades-stock/backend/circuitbreaker"
	"blades-stock/backend/logger"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// klineFallbackCache 缓存 FetchKLineWithFallback 的最近结果，避免同一标的
// 在短时间窗内被重复走完整的慢速降级链（MAC 故障时尤甚）。
type klineFallbackCacheEntry struct {
	data      []KLineData
	src       string
	limit     int
	fetchedAt time.Time
	failed    bool // true: whole chain returned empty; served only within the fail TTL
}

const (
	klineCacheTTL      = 5 * time.Minute
	klineCacheFailTTL  = 60 * time.Second
	klineCacheMaxSize  = 4096
)

var (
	klineCacheMu sync.RWMutex
	klineCache   = make(map[string]*klineFallbackCacheEntry)
)

func klineCacheGet(code, klt string, limit int) *KLineSourceResult {
	klineCacheMu.RLock()
	e, ok := klineCache[code+"|"+klt]
	klineCacheMu.RUnlock()
	if !ok {
		return nil
	}
	if e.failed {
		// Failed entries are limit-independent: an empty result stays empty no
		// matter how many bars were requested. Serve within the cooldown to
		// avoid re-running the full dead chain on every call.
		if time.Since(e.fetchedAt) <= klineCacheFailTTL {
			return &KLineSourceResult{Data: &[]KLineData{}, Source: e.src}
		}
		return nil
	}
	if time.Since(e.fetchedAt) > klineCacheTTL || e.limit < limit {
		return nil
	}
	d := make([]KLineData, len(e.data))
	copy(d, e.data)
	return &KLineSourceResult{Data: &d, Source: e.src}
}

func klineCacheSet(code, klt string, res *KLineSourceResult, limit int) {
	if res == nil {
		return
	}
	empty := res.Data == nil || len(*res.Data) == 0
	d := make([]KLineData, 0)
	if !empty {
		d = make([]KLineData, len(*res.Data))
		copy(d, *res.Data)
	}
	klineCacheMu.Lock()
	defer klineCacheMu.Unlock()
	key := code + "|" + klt
	if old := klineCache[key]; old != nil && old.limit >= limit && !old.failed {
		return
	}
	if len(klineCache) >= klineCacheMaxSize {
		for k := range klineCache {
			delete(klineCache, k)
			break
		}
	}
	klineCache[key] = &klineFallbackCacheEntry{data: d, src: res.Source, limit: limit, fetchedAt: time.Now(), failed: empty}
}

type SinaKLineApi struct {
	client *resty.Client
	config *SettingConfig
}

type SinaKLineItem struct {
	Day    string `json:"day"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

func NewSinaKLineApi(config *SettingConfig) *SinaKLineApi {
	client := SharedHTTPClient
	client.SetTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	return &SinaKLineApi{
		client: client,
		config: config,
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

func sinaSymbolFromStockCode(stockCode string) string {
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
	symbol := sinaSymbolFromStockCode(stockCode)
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
	req.SetHeader("User-Agent", getRandomUA())
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
			logger.SugaredLogger.Errorf("SinaKLine: no closing ] found, body prefix: %s", truncateStr(trimmed, 200))
			return nil
		}
		jsonStr := trimmed[jsonStart : lastBracket+1]
		if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
			logger.SugaredLogger.Errorf("SinaKLine JSONP parse error: %v, jsonStr len=%d, body prefix: %s", err, len(jsonStr), truncateStr(trimmed, 200))
			return nil
		}
		return items
	}

	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		logger.SugaredLogger.Errorf("SinaKLine JSON parse error: %v, body prefix: %s", err, truncateStr(trimmed, 200))
		return nil
	}
	return items
}

func truncateStr(s string, maxLen int) string {
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

	symbol := sinaSymbolFromStockCode(stockCode)
	sinaCode := symbol
	if strings.HasPrefix(sinaCode, "bj") {
		sinaCode = "sb" + sinaCode[2:]
	}

	url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", now.UnixMilli(), sinaCode)
	resp, err := s.client.R().
		SetHeader("Host", "hq.sinajs.cn").
		SetHeader("Referer", "https://finance.sina.com.cn/").
		SetHeader("User-Agent", getRandomUA()).
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
			prevClose, _ := parseFloatToFloat(items[i-1].Close)
			curClose, _ := parseFloatToFloat(item.Close)
			curHigh, _ := parseFloatToFloat(item.High)
			curLow, _ := parseFloatToFloat(item.Low)
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
	config *SettingConfig
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
	Code int                                   `json:"code"`
	Msg  string                                `json:"msg"`
	Data map[string]TencentStockKLinePayload   `json:"-"`
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

func NewTencentKLineApi(config *SettingConfig) *TencentKLineApi {
	client := SharedHTTPClient
	client.SetTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	return &TencentKLineApi{
		client: client,
		config: config,
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
	req.SetHeader("User-Agent", getRandomUA())
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
		logger.SugaredLogger.Errorf("TencentKLine: failed to strip var prefix, body prefix: %s", truncateStr(body, 200))
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
			prevClose, _ := parseFloatToFloat(rows[i-1][2])
			curClose, _ := parseFloatToFloat(row[2])
			curHigh, _ := parseFloatToFloat(row[3])
			curLow, _ := parseFloatToFloat(row[4])
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

type KLineSourceResult struct {
	Data   *[]KLineData `json:"data"`
	Source string       `json:"source"`
}

func cleanCodeOnly(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// 如果包含 - 或空格，例如 "天目药业 - 600671.SH" 或 "天目药业 600671"
	for _, sep := range []string{"-", "—", " "} {
		if strings.Contains(raw, sep) {
			parts := strings.Split(raw, sep)
			for i := len(parts) - 1; i >= 0; i-- {
				p := strings.TrimSpace(parts[i])
				if p != "" {
					raw = p
					break
				}
			}
		}
	}
	// 处理纯数字但长度不足 6 位的情况（如 JSON 解析丢失前导零，002906 变成 2906）
	isDigits := true
	for _, c := range raw {
		if c < '0' || c > '9' {
			isDigits = false
			break
		}
	}
	if isDigits && len(raw) > 0 && len(raw) < 6 {
		raw = fmt.Sprintf("%06s", raw)
	}
	return raw
}

// FetchKLineWithFallback 返回带短时缓存的 K 线降级链结果：同标的同周期在
// 5 分钟内重复请求直接命中缓存，避免重复走完整的慢速降级链。
func FetchKLineWithFallback(stockCode, stockName, klt string, limit int, end string) *KLineSourceResult {
	stockCode = cleanCodeOnly(stockCode)
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "101"
	}
	if limit <= 0 {
		limit = 60
	}
	if cached := klineCacheGet(stockCode, klt, limit); cached != nil {
		cached.Source = "cache:" + cached.Source
		fillVolumeRatio(cached.Data)
		return cached
	}
	res := fetchKLineWithFallbackInner(stockCode, stockName, klt, limit, end)
	klineCacheSet(stockCode, klt, res, limit)
	return res
}

func fetchKLineWithFallbackInner(stockCode, stockName, klt string, limit int, end string) *KLineSourceResult {
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "101"
	}
	if limit <= 0 {
		limit = 60
	}
	// 港美股：仅走 MAC/东财（新浪/腾讯/通达信不支持港美股）
	if IsHKStockCode(stockCode) || IsUSStockCode(stockCode) {
		if macResult := tryKLineSource("tdx-mac", klineSourceTimeout, stockCode, klt, func() *KLineSourceResult {
			return fetchFromMAC(stockCode, klt, limit)
		}); macResult != nil {
			macResult.Source = "tdx-mac"
			fillVolumeRatio(macResult.Data)
			return macResult
		}
		if eastMoneyResult := tryKLineSource("eastmoney", 0, stockCode, klt, func() *KLineSourceResult {
			return fetchFromEastMoney(stockCode, stockName, klt, limit, end)
		}); eastMoneyResult != nil {
			eastMoneyResult.Source = "eastmoney"
			fillVolumeRatio(eastMoneyResult.Data)
			return eastMoneyResult
		}
		return &KLineSourceResult{Data: &[]KLineData{}, Source: ""}
	}

	// A股：优先通达信 MAC 行情源（响应最快、极速直连），失败再尝试通达信标准源及远程源。
	macResult := tryKLineSource("tdx-mac", klineSourceTimeout, stockCode, klt, func() *KLineSourceResult {
		return fetchFromMAC(stockCode, klt, limit)
	})
	if macResult != nil {
		macResult.Source = "tdx-mac"
		fillVolumeRatio(macResult.Data)
		return macResult
	}

	tdxResult := tryKLineSource("tdx", klineSourceTimeout, stockCode, klt, func() *KLineSourceResult {
		return fetchFromTdx(stockCode, klt, limit)
	})
	if tdxResult != nil {
		tdxResult.Source = "tdx"
		fillVolumeRatio(tdxResult.Data)
		return tdxResult
	}

	eastMoneyResult := tryKLineSource("eastmoney", 0, stockCode, klt, func() *KLineSourceResult {
		return fetchFromEastMoney(stockCode, stockName, klt, limit, end)
	})
	if eastMoneyResult != nil {
		eastMoneyResult.Source = "eastmoney"
		fillVolumeRatio(eastMoneyResult.Data)
		return eastMoneyResult
	}

	tushareResult := tryKLineSource("tushare", 0, stockCode, klt, func() *KLineSourceResult {
		return fetchFromTushare(stockCode, klt, limit)
	})
	if tushareResult != nil {
		tushareResult.Source = "tushare"
		fillVolumeRatio(tushareResult.Data)
		return tushareResult
	}

	sinaResult := tryKLineSource("sina", 0, stockCode, klt, func() *KLineSourceResult {
		return fetchFromSina(stockCode, klt, limit)
	})
	if sinaResult != nil {
		sinaResult.Source = "sina"
		fillVolumeRatio(sinaResult.Data)
		return sinaResult
	}

	tencentResult := tryKLineSource("tencent", 0, stockCode, klt, func() *KLineSourceResult {
		return fetchFromTencent(stockCode, klt, limit)
	})
	if tencentResult != nil {
		tencentResult.Source = "tencent"
		fillVolumeRatio(tencentResult.Data)
		return tencentResult
	}

	logger.SugaredLogger.Warnf("所有K线数据源均不可用: code=%s klt=%s", stockCode, klt)
	if eastMoneyResult != nil {
		eastMoneyResult.Source = "eastmoney"
		return eastMoneyResult
	}
	return &KLineSourceResult{Data: &[]KLineData{}, Source: ""}
}

// klineSourceTimeout 单个数据源的探测超时。
const klineSourceTimeout = 5 * time.Second

// tryKLineSource 带断路器的数据源探测：
//   - 断路器断开（Open）时快速跳过，不发起请求；
//   - 超时/结果为 nil 视为数据源级故障，上报 Failure 计入连续失败，触发熔断；
//   - 数据源可达但该标的无数据视为软失败，不惩罚断路器，仅继续降级；
//   - 返回有效数据上报 Success。
func tryKLineSource(name string, timeout time.Duration, stockCode, klt string, fn func() *KLineSourceResult) *KLineSourceResult {
	cb := circuitbreaker.Get(name)
	if !cb.Allow() {
		logger.SugaredLogger.Debugf("断路器断开，跳过数据源 %s: code=%s klt=%s", name, stockCode, klt)
		return nil
	}

	var rs *KLineSourceResult
	ok := true
	if timeout > 0 {
		rs, ok = runKLineSourceWithTimeout(name, timeout, stockCode, klt, fn)
	} else {
		rs = fn()
	}
	if !ok {
		cb.Failure()
		return nil
	}
	if rs == nil {
		cb.Failure()
		return nil
	}
	if rs.Data == nil || len(*rs.Data) == 0 {
		logger.SugaredLogger.Warnf("%s 数据为空，降级到下一个数据源: code=%s klt=%s", name, stockCode, klt)
		return nil
	}
	cb.Success()
	return rs
}

func runKLineSourceWithTimeout(name string, timeout time.Duration, stockCode, klt string, fn func() *KLineSourceResult) (*KLineSourceResult, bool) {
	type result struct {
		rs *KLineSourceResult
	}
	ch := make(chan result, 1)
	go func() {
		// 重要：某些第三方库（gotdx）可能在响应解析时 panic（如上游 issue: ParseResponse
		// 返回的响应长度不足，仍访问 [:6]）。裸 panic 会沿着 goroutine 一路上溯，最终
		// 冲跨 Wails 进程，表现为「打开 K 线分析闪退」。在这里捕获并返回 nil，让
		// tryKLineSource 走断路器降级路径。
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("🛡️ %s 数据源 goroutine panic 已恢复: %v (code=%s klt=%s)", name, r, stockCode, klt)
				ch <- result{rs: nil}
			}
		}()
		ch <- result{rs: fn()}
	}()

	select {
	case r := <-ch:
		return r.rs, true
	case <-time.After(timeout):
		logger.SugaredLogger.Warnf("%s 超时 (%v)，降级到下一个数据源: code=%s klt=%s", name, timeout, stockCode, klt)
		return nil, false
	}
}

// fillVolumeRatio 补算量比：量比 = 当日成交量 / 过去5日平均成交量
func fillVolumeRatio(data *[]KLineData) {
	if data == nil || len(*data) < 6 {
		return
	}
	list := *data
	for i := 5; i < len(list); i++ {
		if list[i].VolumeRatio != "" {
			continue
		}
		curVol, err := strconv.ParseFloat(list[i].Volume, 64)
		if err != nil || curVol <= 0 {
			continue
		}
		var sum float64
		validDays := 0
		for j := i - 5; j < i; j++ {
			v, err := strconv.ParseFloat(list[j].Volume, 64)
			if err == nil && v > 0 {
				sum += v
				validDays++
			}
		}
		if validDays > 0 {
			avg := sum / float64(validDays)
			if avg > 0 {
				list[i].VolumeRatio = fmt.Sprintf("%.2f", curVol/avg)
			}
		}
	}
}

func fetchFromEastMoney(stockCode, stockName, klt string, limit int, end string) *KLineSourceResult {
	api := NewEastMoneyKLineApi(GetSettingConfig())
	var data *[]KLineData
	if strings.TrimSpace(end) == "" {
		data = api.GetKLineDataBefore(stockCode, klt, "", limit, "20500101")
	} else {
		data = api.GetKLineDataBefore(stockCode, klt, "", limit, end)
	}
	return &KLineSourceResult{Data: data}
}

func fetchFromTushare(stockCode, klt string, limit int) *KLineSourceResult {
	api := NewTushareApi(GetSettingConfig())
	data := api.GetKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromSina(stockCode, klt string, limit int) *KLineSourceResult {
	api := NewSinaKLineApi(GetSettingConfig())
	data := api.GetKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromTencent(stockCode, klt string, limit int) *KLineSourceResult {
	api := NewTencentKLineApi(GetSettingConfig())
	data := api.GetKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromTdx(stockCode, klt string, limit int) *KLineSourceResult {
	api := NewTdxKLineApi()
	data := api.GetKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromMAC(stockCode, klt string, limit int) *KLineSourceResult {
	api := NewTdxKLineApi()
	data := api.GetMACKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func AggregateSinaMonthKLine(dailyData *[]KLineData) *[]KLineData {
	if dailyData == nil || len(*dailyData) == 0 {
		return dailyData
	}
	arr := *dailyData
	grouped := make(map[string][]KLineData)
	var monthKeys []string
	for _, kd := range arr {
		monthKey := extractMonthKey(kd.Day)
		if monthKey == "" {
			continue
		}
		if _, exists := grouped[monthKey]; !exists {
			monthKeys = append(monthKeys, monthKey)
		}
		grouped[monthKey] = append(grouped[monthKey], kd)
	}
	result := make([]KLineData, 0, len(monthKeys))
	for _, mk := range monthKeys {
		group := grouped[mk]
		if len(group) == 0 {
			continue
		}
		first := group[0]
		last := group[len(group)-1]
		highF := -1e18
		lowF := 1e18
		volSum := 0.0
		amtSum := 0.0
		for _, k := range group {
			h, _ := parseFloatToFloat(k.High)
			l, _ := parseFloatToFloat(k.Low)
			v, _ := parseFloatToFloat(k.Volume)
			a, _ := parseFloatToFloat(k.Amount)
			if h > highF {
				highF = h
			}
			if l < lowF {
				lowF = l
			}
			volSum += v
			amtSum += a
		}
		highS := first.High
		lowS := first.Low
		if highF > -1e17 {
			highS = fmt.Sprintf("%.2f", highF)
		}
		if lowF < 1e17 {
			lowS = fmt.Sprintf("%.2f", lowF)
		}
		result = append(result, KLineData{
			Day:           last.Day,
			Open:          first.Open,
			Close:         last.Close,
			High:          highS,
			Low:           lowS,
			Volume:        fmt.Sprintf("%.0f", volSum),
			Amount:        fmt.Sprintf("%.2f", amtSum),
			ChangePercent: last.ChangePercent,
			ChangeValue:   last.ChangeValue,
			Amplitude:     last.Amplitude,
			TurnoverRate:  last.TurnoverRate,
		})
	}
	return &result
}

func extractMonthKey(dayStr string) string {
	s := strings.TrimSpace(dayStr)
	if len(s) >= 7 && s[4] == '-' {
		return s[:7]
	}
	if len(s) >= 6 {
		return s[:4] + "-" + s[4:6]
	}
	if len(s) >= 10 && (strings.Contains(s, "/") || strings.Contains(s, "-")) {
		t, err := time.Parse("2006-01-02", strings.ReplaceAll(s, "/", "-"))
		if err == nil {
			return t.Format("2006-01")
		}
	}
	return ""
}
