package data

import (
	"blades-stock/backend/circuitbreaker"
	"blades-stock/backend/datasource"
	"blades-stock/backend/logger"

	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
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
	klineCacheTTL     = 5 * time.Minute
	klineCacheFailTTL = 60 * time.Second
	klineCacheMaxSize = 4096
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
	api := datasource.NewSinaKLineApi(GetSettingConfig().CrawlTimeOut)
	data := api.GetKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromTencent(stockCode, klt string, limit int) *KLineSourceResult {
	api := datasource.NewTencentKLineApi(GetSettingConfig().CrawlTimeOut)
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

// truncateStr 转发到 datasource.TruncateStr（R-25a：实现已搬入 datasource）。
func truncateStr(s string, maxLen int) string {
	return datasource.TruncateStr(s, maxLen)
}

// sinaSymbolFromStockCode 转发到 datasource.SinaSymbolFromStockCode（R-25a）。
func sinaSymbolFromStockCode(stockCode string) string {
	return datasource.SinaSymbolFromStockCode(stockCode)
}
