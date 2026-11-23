package data

import (
	"blades-stock/backend/circuitbreaker"
	"blades-stock/backend/datasource"
	"blades-stock/backend/logger"

	"github.com/go-resty/resty/v2"

	"context"
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

// cleanCodeOnly 转发到 datasource.CleanCodeOnly（R-25c：实现已搬入 datasource）。
func cleanCodeOnly(raw string) string {
	return datasource.CleanCodeOnly(raw)
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
		if macResult := tryKLineSource("tdx-mac", klineSourceTimeout, stockCode, klt, func(ctx context.Context) *KLineSourceResult {
			return fetchFromMAC(ctx, stockCode, klt, limit)
		}); macResult != nil {
			macResult.Source = "tdx-mac"
			fillVolumeRatio(macResult.Data)
			return macResult
		}
		if eastMoneyResult := tryKLineSource("eastmoney", 0, stockCode, klt, func(ctx context.Context) *KLineSourceResult {
			return fetchFromEastMoney(ctx, stockCode, stockName, klt, limit, end)
		}); eastMoneyResult != nil {
			eastMoneyResult.Source = "eastmoney"
			fillVolumeRatio(eastMoneyResult.Data)
			return eastMoneyResult
		}
		return &KLineSourceResult{Data: &[]KLineData{}, Source: ""}
	}

	// A股：优先通达信 MAC 行情源（响应最快、极速直连），失败再尝试通达信标准源及远程源。
	macResult := tryKLineSource("tdx-mac", klineSourceTimeout, stockCode, klt, func(ctx context.Context) *KLineSourceResult {
		return fetchFromMAC(ctx, stockCode, klt, limit)
	})
	if macResult != nil {
		macResult.Source = "tdx-mac"
		fillVolumeRatio(macResult.Data)
		return macResult
	}

	tdxResult := tryKLineSource("tdx", klineSourceTimeout, stockCode, klt, func(ctx context.Context) *KLineSourceResult {
		return fetchFromTdx(ctx, stockCode, klt, limit)
	})
	if tdxResult != nil {
		tdxResult.Source = "tdx"
		fillVolumeRatio(tdxResult.Data)
		return tdxResult
	}

	eastMoneyResult := tryKLineSource("eastmoney", 0, stockCode, klt, func(ctx context.Context) *KLineSourceResult {
		return fetchFromEastMoney(ctx, stockCode, stockName, klt, limit, end)
	})
	if eastMoneyResult != nil {
		eastMoneyResult.Source = "eastmoney"
		fillVolumeRatio(eastMoneyResult.Data)
		return eastMoneyResult
	}

	tushareResult := tryKLineSource("tushare", 0, stockCode, klt, func(ctx context.Context) *KLineSourceResult {
		return fetchFromTushare(ctx, stockCode, klt, limit)
	})
	if tushareResult != nil {
		tushareResult.Source = "tushare"
		fillVolumeRatio(tushareResult.Data)
		return tushareResult
	}

	sinaResult := tryKLineSource("sina", 0, stockCode, klt, func(ctx context.Context) *KLineSourceResult {
		return fetchFromSina(ctx, stockCode, klt, limit)
	})
	if sinaResult != nil {
		sinaResult.Source = "sina"
		fillVolumeRatio(sinaResult.Data)
		return sinaResult
	}

	tencentResult := tryKLineSource("tencent", 0, stockCode, klt, func(ctx context.Context) *KLineSourceResult {
		return fetchFromTencent(ctx, stockCode, klt, limit)
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
//   - fn 收到带超时的 ctx：timeout>0 时由 runKLineSourceWithTimeout 以该超时
//     建 ctx，超时后取消，在途的 resty 请求（东财/Tushare/新浪/腾讯）立即
//     中断、worker 随即退出，不再留超时孤儿 goroutine 在后台跑完网络请求。
func tryKLineSource(name string, timeout time.Duration, stockCode, klt string, fn func(ctx context.Context) *KLineSourceResult) *KLineSourceResult {
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
		rs = fn(context.Background())
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

// runKLineSourceWithTimeout 在 worker goroutine 里跑 fn，timeout 内未返回
// 即判超时。超时后取消 ctx：在途的 resty 请求（东财/Tushare/新浪/腾讯，
// 经请求级 SetContext 接 ctx）立即中断，worker 随即退出、结果丢弃进带
// 缓冲的 ch，不再像以前那样超时后仍在后台把整次网络请求跑完（孤儿
// goroutine，见 ROUND4-REPORT §5/§7 第 9 条）。TDX（gotdx）不支持
// context，worker 仍需等 gotdx 自身超时返回，但该超时由 CrawlTimeOut 限定
// （kline_tdx.go newClient 的 WithTimeoutSec），不会无限挂起。
func runKLineSourceWithTimeout(name string, timeout time.Duration, stockCode, klt string, fn func(ctx context.Context) *KLineSourceResult) (*KLineSourceResult, bool) {
	type result struct {
		rs *KLineSourceResult
	}
	ch := make(chan result, 1)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
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
		ch <- result{rs: fn(ctx)}
	}()

	select {
	case r := <-ch:
		return r.rs, true
	case <-ctx.Done():
		// 超时：cancel()（defer 也会调一次，幂等）让在途请求中断、worker 退出；
		// 本函数不等 worker，降级链立即继续下一源。
		cancel()
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

func fetchFromEastMoney(ctx context.Context, stockCode, stockName, klt string, limit int, end string) *KLineSourceResult {
	api := NewEastMoneyKLineApi(GetSettingConfig())
	var data *[]KLineData
	if strings.TrimSpace(end) == "" {
		data = api.GetKLineDataBeforeWithContext(ctx, stockCode, klt, "", limit, "20500101")
	} else {
		data = api.GetKLineDataBeforeWithContext(ctx, stockCode, klt, "", limit, end)
	}
	return &KLineSourceResult{Data: data}
}

func fetchFromTushare(ctx context.Context, stockCode, klt string, limit int) *KLineSourceResult {
	api := NewTushareApi(GetSettingConfig())
	data := api.GetKLineDataWithContext(ctx, stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromSina(ctx context.Context, stockCode, klt string, limit int) *KLineSourceResult {
	api := datasource.NewSinaKLineApi(GetSettingConfig().CrawlTimeOut)
	data := api.GetKLineDataWithContext(ctx, stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromTencent(ctx context.Context, stockCode, klt string, limit int) *KLineSourceResult {
	api := datasource.NewTencentKLineApi(GetSettingConfig().CrawlTimeOut)
	data := api.GetKLineDataWithContext(ctx, stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

// fetchFromTdx / fetchFromMAC 不接 ctx：gotdx 不支持 context，超时由其自身
// WithTimeoutSec（CrawlTimeOut）限定；runKLineSourceWithTimeout 超时后
// worker 仍需等该调用返回，但不会无限挂起（见该函数注释）。
func fetchFromTdx(ctx context.Context, stockCode, klt string, limit int) *KLineSourceResult {
	api := NewTdxKLineApi()
	data := api.GetKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromMAC(ctx context.Context, stockCode, klt string, limit int) *KLineSourceResult {
	api := NewTdxKLineApi()
	data := api.GetMACKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

// truncateStr 转发到 datasource.TruncateStr（R-25a：实现已搬入 datasource）。
func truncateStr(s string, maxLen int) string {
	return datasource.TruncateStr(s, maxLen)
}

// sinaSymbolFromStockCode 转发到 datasource.SinaSymbolFromStockCode（R-25a）。
func sinaSymbolFromStockCode(stockCode string) string {
	return datasource.SinaSymbolFromStockCode(stockCode)
}

// getRandomUA 转发到 datasource.GetRandomUA（R-25a：实现已搬入 datasource；
// 本转发原在 eastmoney_kline_api.go，R-25c 该文件搬走后迁至此处，
// fund_data_* 等 25 处调用点不动）。
func getRandomUA() string {
	return datasource.GetRandomUA()
}

// parseFloatToFloat 转发到 datasource.ParseFloatToFloat（R-25a：实现已搬入 datasource；
// 本转发原在 eastmoney_kline_api.go，R-25c 该文件搬走后迁至此处，
// tushare_data_api.go 与 pure_helpers_test.go 的调用点不动）。
func parseFloatToFloat(s string) (float64, error) {
	return datasource.ParseFloatToFloat(s)
}

// AggregateKLineEveryN 转发到 datasource.AggregateKLineEveryN（R-25b：实现已搬入 datasource；
// 本转发原在 eastmoney_kline_api.go，R-25c 该文件搬走后迁至此处，
// pure_helpers_test.go 的调用点不动）。
func AggregateKLineEveryN(src *[]KLineData, n int) *[]KLineData {
	return datasource.AggregateKLineEveryN(src, n)
}

// setEastMoneyKlineBrowserHeaders 转发到 datasource.SetEastMoneyKlineBrowserHeaders
// （R-25c：实现随 eastmoney_kline_api.go 搬入 datasource；stock_money.go 在用）。
func setEastMoneyKlineBrowserHeaders(r *resty.Request, referer string) {
	datasource.SetEastMoneyKlineBrowserHeaders(r, referer)
}

// computeSMA 转发到 datasource.ComputeSMA（R-25c：实现随 eastmoney_kline_api.go
// 搬入 datasource；pure_helpers_test.go 在用）。
func computeSMA(closes []float64, idx, period int) float64 {
	return datasource.ComputeSMA(closes, idx, period)
}

// getMaxPeriod 转发到 datasource.GetMaxPeriod（R-25c：实现随 eastmoney_kline_api.go
// 搬入 datasource；pure_helpers_test.go 在用）。
func getMaxPeriod(periods []int) int {
	return datasource.GetMaxPeriod(periods)
}
