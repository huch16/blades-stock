package datasource

import (
	"blades-stock/backend/logger"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/validator"
	"github.com/go-resty/resty/v2"
)

// 模拟 Windows 上 Chrome 从 quote.eastmoney.com 请求 push2his 行情接口（与 DevTools Network 常见字段对齐）。
// 不显式设置 Accept-Encoding：由 net/http 默认协商 gzip 并自动解压；若声明 br/zstd 而 Transport 不解压会导致乱码/失败。

// Enhanced headers with more realistic browser characteristics
// SetEastMoneyKlineBrowserHeaders 为东财请求设置浏览器特征头。
// R-25c 自 data 移入时导出；data 侧留同名小写转发（stock_money.go 在用）。
func SetEastMoneyKlineBrowserHeaders(r *resty.Request, referer string) {
	r.SetHeader("User-Agent", GetRandomUA())
	r.SetHeader("Accept", "*/*")
	r.SetHeader("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	r.SetHeader("Connection", "keep-alive")
	r.SetHeader("Referer", referer)
}

// fetchKLineJSONBytesByHTTP 每次调用均发起真实 GET，不缓存 K 线响应；cookieHeader 仅来自 chromedp 缓存或当次刷新。
// 由于 Transport 设置了 DisableCompression=true，需要手动处理 gzip 解压。
// 注意：超时在构造时已固化到独立 client（见 NewEastMoneyKLineApi），这里
// 不再调 client.SetTimeout——receiver.client 曾是共享单例，每次请求改写
// 其 http.Client.Timeout 是并发 K 线拉取的数据竞态根因之一。
// ctx 用于 K 线降级链的源级超时取消（见 data.runKLineSourceWithTimeout），
// 经请求级 SetContext 生效。
func (receiver *EastMoneyKLineApi) fetchKLineJSONBytesByHTTP(ctx context.Context, reqURL string) ([]byte, error) {
	req := receiver.client.R().SetContext(ctx)
	SetEastMoneyKlineBrowserHeaders(req, "https://quote.eastmoney.com")
	// 使用缓存的 Cookie，pageURL 参数传空字符串由函数内部使用默认值
	//cookieHeader, err := FetchEastMoneyCookiesViaChromedp("", time.Second*5, reqURL)
	//if err != nil {
	//	logger.SugaredLogger.Errorf("FetchEastMoneyCookiesViaChromedp error: %v", err)
	//}
	//if err == nil {
	//	//logger.SugaredLogger.Infof("Cookie: %s", cookieHeader)
	//	req.SetHeader("Cookie", cookieHeader)
	//}

	resp, err := req.Get(reqURL)
	if err != nil {
		logger.SugaredLogger.Errorf("HTTP error: %v", err)
		return nil, err
	}
	if resp.StatusCode() != 200 {
		b := resp.Body()
		if len(b) > 500 {
			b = b[:500]
		}
		return nil, fmt.Errorf("HTTP %d: %q", resp.StatusCode(), string(b))
	}

	// 读取响应体
	rawBody := resp.Body()

	// 检查 Content-Encoding 并处理 gzip 压缩
	contentEncoding := resp.Header().Get("Content-Encoding")
	if strings.ToLower(contentEncoding) == "gzip" {
		reader, err := gzip.NewReader(bytes.NewReader(rawBody))
		if err != nil {
			return nil, fmt.Errorf("gzip.NewReader error: %w", err)
		}
		defer reader.Close()
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("gzip decompress error: %w", err)
		}
		return decompressed, nil
	}

	return rawBody, nil
}

// @Author spark
// @Date 2026/3/15
// @Desc 东方财富 K 线数据 API 工具

// EastMoneyKLineApi 东方财富 K 线 API 结构体
type EastMoneyKLineApi struct {
	client *resty.Client
	// crawlTimeoutSec 与 browserPath 由调用方传入（见 NewEastMoneyKLineApi）；
	// 本包不读设置表。crawlTimeoutSec <= 0 时取默认 10 秒。
	crawlTimeoutSec int64
	browserPath     string
}

// KLineType K 线类型枚举
type KLineType string

const (
	KLineType1Min     KLineType = "1"   // 1 分钟
	KLineType5Min     KLineType = "5"   // 5 分钟
	KLineType15Min    KLineType = "15"  // 15 分钟
	KLineType30Min    KLineType = "30"  // 30 分钟
	KLineType60Min    KLineType = "60"  // 60 分钟
	KLineType120Min   KLineType = "120" // 120 分钟
	KLineTypeDay      KLineType = "101" // 日 K
	KLineTypeWeek     KLineType = "102" // 周 K
	KLineTypeMonth    KLineType = "103" // 月 K
	KLineTypeQuarter  KLineType = "104" // 季 K
	KLineTypeHalfYear KLineType = "105" // 半年 K
	KLineTypeYear     KLineType = "106" // 年 K
)

// EastMoneyKLineResponse 东方财富 K 线响应结构
type EastMoneyKLineResponse struct {
	Rc      int    `json:"rc"` // 接口实际返回 rc，成功为 0
	Version string `json:"version"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID       int      `json:"id"`
		Klines   []string `json:"klines"` // K 线数据数组，逗号分隔的字符串
		Name     string   `json:"name"`
		Code     string   `json:"code"`
		Market   any      `json:"market"`
		Period   string   `json:"period"`
		Pair     int      `json:"pair"`
		PrePrice string   `json:"prePrice"`
	} `json:"data"`
}

// CallAuctionData 竞价数据结构
type CallAuctionData struct {
	Time         string // 时间 (HH:MM:SS)
	Price        string // 撮合价格
	Volume       string // 撮合数量 (手)
	Amount       string // 撮合金额 (元)
	ChangeNum    string // 增减量
	ChangeRatio  string // 增减比例 (%)
	MatchedVol   string // 匹配量
	UnmatchedVol string // 未匹配量
	AskPrice1    string // 卖一价
	AskVol1      string // 卖一量
	BidPrice1    string // 买一价
	BidVol1      string // 买一量
}

// NewEastMoneyKLineApi 创建东方财富 K 线 API 实例
// NewEastMoneyKLineApi 构造东方财富 K 线客户端。
// crawlTimeoutSec 为爬取超时（秒，<=0 取默认 10 秒），browserPath 为浏览器路径
// （空表示未配置，HTTP 请求不带 chromedp cookie）；均由调用方
// （backend/data 的转发函数）从设置传入，本包不读设置表。
func NewEastMoneyKLineApi(crawlTimeoutSec int64, browserPath string) *EastMoneyKLineApi {
	// 超时在构造时固化到独立 client（共享 transport，见
	// CreateHTTPClientWithTimeout；与 settings_service.go 的做法一致）：
	// 不再用 SharedHTTPClient——并发拉取时每次请求调 SetTimeout 会改写
	// 共享 http.Client.Timeout（数据竞态）。超时取值与原来每次请求时
	// 的计算一致（<=0 取默认 10 秒），语义不变。
	timeout := 10 * time.Second
	if crawlTimeoutSec > 0 {
		timeout = time.Duration(crawlTimeoutSec) * time.Second
	}

	return &EastMoneyKLineApi{
		client:          CreateHTTPClientWithTimeout(timeout),
		crawlTimeoutSec: crawlTimeoutSec,
		browserPath:     browserPath,
	}
}

// GetKLineData 获取 K 线数据（最新一段，等价于 end=20500101）
func (receiver *EastMoneyKLineApi) GetKLineData(stockCode, kLineType, adjustFlag string, days int) *[]KLineData {
	return receiver.GetKLineDataBefore(stockCode, kLineType, adjustFlag, days, "20500101")
}

// GetKLineDataBefore 获取 end 时间点之前的 limit 根 K 线。
// end 为空或 "20500101" 表示取到最新；否则为东方财富格式：日/周等为 YYYYMMDD，分钟线多为 YYYYMMDDHHmmss（与 f51 字段一致即可）。
func (receiver *EastMoneyKLineApi) GetKLineDataBefore(stockCode, kLineType, adjustFlag string, limit int, end string) *[]KLineData {
	return receiver.GetKLineDataBeforeWithContext(context.Background(), stockCode, kLineType, adjustFlag, limit, end)
}

// GetKLineDataBeforeWithContext 是 GetKLineDataBefore 的 context 版：ctx 用于
// K 线降级链的源级超时取消（见 data.runKLineSourceWithTimeout），经请求级
// SetContext 生效；本客户端的超时仍在构造时固化，不调 SetTimeout。
func (receiver *EastMoneyKLineApi) GetKLineDataBeforeWithContext(ctx context.Context, stockCode, kLineType, adjustFlag string, limit int, end string) *[]KLineData {
	kLines := &[]KLineData{}

	// 转换股票代码格式
	secid := receiver.ConvertStockCode(stockCode)
	if secid == "" {
		logger.SugaredLogger.Errorf("invalid stock code: %s", stockCode)
		return kLines
	}

	if limit <= 0 {
		return kLines
	}
	if strings.TrimSpace(end) == "" {
		end = "20500101"
	}

	// 构建 fields 参数
	fields := "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f116"
	if adjustFlag != "" {
		fields = "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f116,f113,f114,f115"
	}

	// 构建 URL
	baseURL := "https://push2his.eastmoney.com/api/qt/stock/kline/get"
	params := url.Values{}
	params.Set("secid", secid)
	params.Set("klt", kLineType)
	params.Set("fqt", adjustFlag)
	params.Set("end", end)
	params.Set("lmt", convertor.ToString(limit))
	params.Set("fields1", "f1,f2,f3,f4,f5,f6")
	params.Set("fields2", fields)
	params.Set("wbp2u", "|0|0|0|web")
	params.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	//logger.SugaredLogger.Infof("GetKLineDataBefore url: %s", reqURL)

	if strings.TrimSpace(receiver.browserPath) == "" {
		logger.SugaredLogger.Infof("东财 K 线未配置 BrowserPath，HTTP 请求不带 chromedp cookie")
	}

	var body []byte
	var fetchErr error
	body, fetchErr = receiver.fetchKLineJSONBytesByHTTP(ctx, reqURL)

	if fetchErr != nil {
		logger.SugaredLogger.Errorf("GetKLineData error: %v", fetchErr)
		return kLines
	}
	var response EastMoneyKLineResponse
	err := json.Unmarshal(body, &response)
	if err != nil {
		preview := body
		if len(preview) > 400 {
			preview = preview[:400]
		}
		logger.SugaredLogger.Errorf("json.Unmarshal error: %v body_prefix=%q", err, string(preview))
		return kLines
	}

	if response.Rc != 0 {
		logger.SugaredLogger.Errorf("API error: rc=%d code=%d message=%s", response.Rc, response.Code, response.Message)
		return kLines
	}
	if response.Code != 0 {
		logger.SugaredLogger.Errorf("API error: code=%d, message=%s", response.Code, response.Message)
		return kLines
	}

	// 解析 K 线数据
	for _, klineStr := range response.Data.Klines {
		kline := receiver.parseKLine(klineStr, adjustFlag)
		if kline != nil {
			*kLines = append(*kLines, *kline)
		}
	}

	//logger.SugaredLogger.Infof("GetKLineData success, count: %d", len(*kLines))
	return kLines
}

// GetMinuteKLine 获取分时 K 线数据 (1 分钟、5 分钟等)
func (receiver *EastMoneyKLineApi) GetMinuteKLine(stockCode string, minuteType KLineType, days int) *[]KLineData {
	return receiver.GetKLineData(stockCode, string(minuteType), "", days)
}

// GetDayKLine 获取日 K 线数据
func (receiver *EastMoneyKLineApi) GetDayKLine(stockCode string, days int) *[]KLineData {
	return receiver.GetKLineData(stockCode, "101", "", days)
}

// GetWeekKLine 获取周 K 线数据
func (receiver *EastMoneyKLineApi) GetWeekKLine(stockCode string, weeks int) *[]KLineData {
	return receiver.GetKLineData(stockCode, "102", "", weeks)
}

// GetMonthKLine 获取月 K 线数据
func (receiver *EastMoneyKLineApi) GetMonthKLine(stockCode string, months int) *[]KLineData {
	return receiver.GetKLineData(stockCode, "103", "", months)
}

// GetQuarterKLine 获取季 K 线数据
// GetYearKLine 获取年 K 线数据
// GetAdjustedKLine 获取复权 K 线数据
// adjustType: qfq=前复权，hfq=后复权
func (receiver *EastMoneyKLineApi) GetAdjustedKLine(stockCode, adjustType string, days int) *[]KLineData {
	return receiver.GetKLineData(stockCode, "101", adjustType, days)
}

// convertStockCode 转换股票代码为东方财富格式
// 输入：000001 或 sz000001 或 000001.SZ
// 输出：0.000001 或 1.600000
// ConvertStockCode 将股票代码转为东财接口格式。
// R-25c 移入 datasource 时导出（eastmoney_kline_api_test.go 的集成测试在用）。
func (receiver *EastMoneyKLineApi) ConvertStockCode(stockCode string) string {
	stockCode = CleanCodeOnly(stockCode)
	stockCode = strings.ToUpper(strings.TrimSpace(stockCode))

	// 如果已经包含点号，说明是标准格式
	if strings.Contains(stockCode, ".") {
		parts := strings.Split(stockCode, ".")
		if len(parts) == 2 {
			code := parts[0]
			market := parts[1]

			switch market {
			case "SH", "SS":
				return "1." + code
			case "SZ":
				return "0." + code
			case "BJ":
				return "0." + code
			case "HK":
				return "128." + code
			case "BK":
				return "90." + code

			default:
				return stockCode
			}
		}
	}

	// 处理带市场前缀的代码
	if strings.HasPrefix(stockCode, "SH") || strings.HasPrefix(stockCode, "SZ") || strings.HasPrefix(stockCode, "BJ") {
		market := stockCode[:2]
		code := stockCode[2:]

		switch market {
		case "SH":
			return "1." + code
		case "SZ":
			return "0." + code
		case "BJ":
			return "0." + code
		case "HK":
			return "128." + code
		case "BK":
			return "90." + code
		default:
			return stockCode
		}
	}

	// 纯数字代码，根据代码规则判断市场（支持股票与ETF基金）
	if len(stockCode) >= 1 && validator.IsNumberStr(stockCode) {
		firstChar := stockCode[0:1]
		switch firstChar {
		case "6", "5": // 6: 沪市主板/科创板; 5: 沪市ETF/封闭式基金 (如510300, 588000)
			return "1." + stockCode
		case "0", "3", "1": // 0: 深市主板; 3: 创业板; 1: 深市ETF/LOF/基金 (如159915, 161725)
			return "0." + stockCode
		case "4", "8", "9": // 北交所 / 新三板
			return "0." + stockCode
		default:
			// 其他情况默认按深市处理
			return "0." + stockCode
		}
	}

	return stockCode
}

// parseKLine 解析单条 K 线数据
// K 线数据格式：日期，开盘价，收盘价，最高价，最低价，成交量，成交额，振幅，涨跌幅，涨跌额，换手率，市盈率 TTM
func (receiver *EastMoneyKLineApi) parseKLine(klineStr, adjustFlag string) *KLineData {
	//logger.SugaredLogger.Debugf("parseKLine: %s", klineStr)
	parts := strings.Split(klineStr, ",")
	if len(parts) < 11 {
		logger.SugaredLogger.Warnf("invalid kline format: %s", klineStr)
		return nil
	}

	kline := &KLineData{
		Day:           parts[0],                       // 日期
		Open:          receiver.parseFloat(parts[1]),  // 开盘价
		Close:         receiver.parseFloat(parts[2]),  // 收盘价
		High:          receiver.parseFloat(parts[3]),  // 最高价
		Low:           receiver.parseFloat(parts[4]),  // 最低价
		Volume:        receiver.parseFloat(parts[5]),  // 成交量 (手)
		Amount:        receiver.parseFloat(parts[6]),  // 成交额 (元)
		Amplitude:     receiver.parseFloat(parts[7]),  // 振幅 (%)
		ChangePercent: receiver.parseFloat(parts[8]),  // 涨跌幅 (%)
		ChangeValue:   receiver.parseFloat(parts[9]),  // 涨跌额 (元)
		TurnoverRate:  receiver.parseFloat(parts[10]), // 换手率 (%)
	}

	// f113: 前复权开盘价, f114: 前复权收盘价, f115: 前复权最高价, f116: 前复权最低价
	_ = adjustFlag != "" && len(parts) >= 14

	return kline
}

// parseFloat 安全转换浮点数字符串
func (receiver *EastMoneyKLineApi) parseFloat(s string) string {
	if s == "" || s == "-" || s == "null" {
		return "0"
	}
	return s
}

// GetBatchKLineData 批量获取多只股票的 K 线数据
func (receiver *EastMoneyKLineApi) GetBatchKLineData(stockCodes []string, kLineType string, days int) map[string]*[]KLineData {
	result := make(map[string]*[]KLineData)

	for i, stockCode := range stockCodes {
		kLines := receiver.GetKLineData(stockCode, kLineType, "", days)
		result[stockCode] = kLines

		// 使用更智能的延迟策略
		if i < len(stockCodes)-1 { // 最后一个不需要延迟
			// 随机延迟 200-800ms，模拟人类操作
			delay := 200 + rand.Intn(600)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	return result
}

// GetLatestKLine 获取最新一条 K 线数据
func (receiver *EastMoneyKLineApi) GetLatestKLine(stockCode string, kLineType string) *KLineData {
	kLines := receiver.GetKLineData(stockCode, kLineType, "", 1)
	if len(*kLines) > 0 {
		return &(*kLines)[0]
	}
	return nil
}

// GetKLineWithMA 获取带均线的 K 线数据，支持任意周期的简单移动平均（SMA，以收盘价计算）。
// maPeriods 为均线周期，如 5,10,20,60,120；若未传则默认 5,10,20,60。
func (receiver *EastMoneyKLineApi) GetKLineWithMA(stockCode string, kLineType string, days int, maPeriods ...int) (*[]KLineData, error) {
	periods := maPeriods
	if len(periods) == 0 {
		periods = []int{5, 10, 20, 60}
	}
	maxPeriod := GetMaxPeriod(periods)
	fetchDays := days + maxPeriod
	if fetchDays < 1 {
		fetchDays = maxPeriod + 60
	}
	full := receiver.GetKLineData(stockCode, kLineType, "", fetchDays)
	if full == nil || len(*full) == 0 {
		return full, nil
	}
	total := len(*full)
	// 收盘价序列（完整长度，用于计算均线）
	closes := make([]float64, total)
	for i, k := range *full {
		v, _ := ParseFloatToFloat(k.Close)
		closes[i] = v
	}
	// 只返回最后 days 条
	if total > days {
		*full = (*full)[total-days:]
		total = len(*full)
	}
	offset := len(closes) - total // 截取后在 closes 中的起始下标
	// 对每个周期计算 SMA，并写回每条 K 线的 MA
	for _, p := range periods {
		if p <= 0 {
			continue
		}
		for i := 0; i < total; i++ {
			idx := offset + i
			ma := ComputeSMA(closes, idx, p)
			if ma < 0 {
				continue
			}
			if (*full)[i].MA == nil {
				(*full)[i].MA = make(map[string]string)
			}
			(*full)[i].MA[fmt.Sprintf("%d", p)] = fmt.Sprintf("%.4f", ma)
		}
	}
	return full, nil
}

// ComputeSMA 计算 closes 从 idx 往前 period 根的收盘价简单移动平均；不足 period 根返回 -1。
// R-25c 自 data 移入时导出；data 侧留同名小写转发（pure_helpers_test.go 在用）。
func ComputeSMA(closes []float64, idx, period int) float64 {
	start := idx - period + 1
	if start < 0 {
		return -1
	}
	sum := 0.0
	for j := start; j <= idx; j++ {
		sum += closes[j]
	}
	return sum / float64(period)
}

// GetMaxPeriod 返回 periods 中的最大值。
// R-25c 自 data 移入时导出；data 侧留同名小写转发（pure_helpers_test.go 在用）。
func GetMaxPeriod(periods []int) int {
	max := 0
	for _, p := range periods {
		if p > max {
			max = p
		}
	}
	return max
}

// ValidateStockCode 验证股票代码是否有效（定长校验，与前端 isValidEastMoneyCode 同规则，
// 另接受无歧义的裸码/市场前缀 A 股形式）。
// 此前实现只看 ConvertStockCode 返回是否为空，而 ConvertStockCode 对任意非空输入
// 几乎总返回非空（"invalid"→"INVALID"、"600500000119.SH"→"1.600500000119"），
// 导致乱码代码可直达东财请求（见 2026-11-21 线上 600500000119.SH 事件）。
// ConvertStockCode 本身保持宽容（它是转换器，调用方另有用途）；校验只收紧此处。
func (receiver *EastMoneyKLineApi) ValidateStockCode(stockCode string) bool {
	code := CleanCodeOnly(stockCode)
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return false
	}
	// 带后缀的标准形式：A 股 6 位 + .SH/.SZ/.BJ/.SS；港股 5 位 + .HK；美股 1–5 字母 + .US
	if strings.Contains(code, ".") {
		parts := strings.Split(code, ".")
		if len(parts) != 2 {
			return false
		}
		num, suffix := parts[0], parts[1]
		switch suffix {
		case "SH", "SZ", "BJ", "SS":
			return len(num) == 6 && validator.IsNumberStr(num)
		case "HK":
			return len(num) == 5 && validator.IsNumberStr(num)
		case "US":
			return isAllLetters(num) && len(num) <= 5
		default:
			return false
		}
	}
	// 无后缀：只接受无歧义的 A 股形式（裸 6 位数字、SH/SZ/BJ + 6 位数字）。
	// 裸 5 位数字（如 00700）有歧义（港股/深市），裸字母（如 AAPL）东财路径本就无法处理，均拒绝。
	if len(code) == 6 && validator.IsNumberStr(code) {
		return true
	}
	for _, p := range []string{"SH", "SZ", "BJ"} {
		if strings.HasPrefix(code, p) && len(code) == 8 && validator.IsNumberStr(code[2:]) {
			return true
		}
	}
	return false
}

// isAllLetters 报告 s 是否全部由字母组成（空串返回 false）。
func isAllLetters(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}

// GetKLineCount 获取指定时间段内的 K 线数量
