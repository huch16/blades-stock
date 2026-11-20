package datasource

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"blades-stock/backend/logger"
	"github.com/go-resty/resty/v2"
)

const tickFlowBaseURL = "https://api.tickflow.org"

// TickFlowClient wraps the TickFlow REST API.
type TickFlowClient struct {
	apiKey string
	client *resty.Client
}

// NewTickFlowClient creates a client with the given API key.
// The key is resolved by the caller (backend/data's NewTickFlowClient
// forwarding function reads it from settings); this package never reads
// the settings table and never imports data.
func NewTickFlowClient(apiKey string) *TickFlowClient {
	return &TickFlowClient{
		apiKey: strings.TrimSpace(apiKey),
		client: SharedHTTPClient,
	}
}

// ── K lines ───────────────────────────────────────────────────────────────────

type TickFlowKlinesData struct {
	Timestamp []int64   `json:"timestamp"`
	Open      []float64 `json:"open"`
	High      []float64 `json:"high"`
	Low       []float64 `json:"low"`
	Close     []float64 `json:"close"`
	Volume    []int64   `json:"volume"`
	Amount    []float64 `json:"amount"`
}

type tickFlowKlinesResp struct {
	Data *TickFlowKlinesData `json:"data"`
}

// GetKlines fetches historical K lines from TickFlow.
func (c *TickFlowClient) GetKlines(symbol, period string, count int, adjust string) (*TickFlowKlinesData, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("TickFlow API密钥未配置，请在设置中填写TickFlowAPIKey")
	}
	period = strings.TrimSpace(period)
	if period == "" {
		period = "1d"
	}
	if count <= 0 {
		count = 200
	}

	var result tickFlowKlinesResp
	resp, err := c.client.R().
		SetHeader("x-api-key", c.apiKey).
		SetQueryParams(map[string]string{
			"symbol": symbol,
			"period": period,
			"count":  fmt.Sprintf("%d", count),
		}).
		SetResult(&result).
		Get(tickFlowBaseURL + "/v1/klines")

	if err != nil {
		return nil, fmt.Errorf("TickFlow API请求失败: %v", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("TickFlow API返回HTTP %d: %s", resp.StatusCode(), strings.TrimSpace(string(resp.Body())))
	}
	if result.Data == nil || len(result.Data.Timestamp) == 0 {
		return nil, fmt.Errorf("TickFlow 未返回K线数据（标的代码是否正确？%s）", symbol)
	}
	return result.Data, nil
}

// GetKlinesAsMarkdown fetches klines and returns a markdown table (last N rows).
func (c *TickFlowClient) GetKlinesAsMarkdown(symbol, period string, count int, adjust string) string {
	data, err := c.GetKlines(symbol, period, count, adjust)
	if err != nil {
		if strings.Contains(err.Error(), "NO_KLINE_PERMISSION") || strings.Contains(err.Error(), "403") {
			logger.SugaredLogger.Debugf("TickFlow %s 无 %s K线权限，跳过", symbol, period)
			return ""
		}
		logger.SugaredLogger.Warnf("TickFlow GetKlinesAsMarkdown failed: %v", err)
		return fmt.Sprintf("获取 %s K线失败: %v", symbol, err)
	}

	n := len(data.Timestamp)
	if n == 0 {
		return fmt.Sprintf("%s 无K线数据", symbol)
	}

	show := n
	if show > 30 {
		show = 30
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool {
		return data.Timestamp[idx[i]] > data.Timestamp[idx[j]]
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s K线（%s，共 %d 根，显示最近 %d 根）\n\n", symbol, period, n, show))
	sb.WriteString("| 日期 | 开盘 | 最高 | 最低 | 收盘 | 成交量(手) | 成交额(元) |\n")
	sb.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")

	for i := 0; i < show; i++ {
		k := idx[i]
		ts := time.UnixMilli(data.Timestamp[k]).Format("2006-01-02")
		sb.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.2f | %.2f | %d | %.0f |\n",
			ts, data.Open[k], data.High[k], data.Low[k], data.Close[k], data.Volume[k], data.Amount[k]))
	}
	sb.WriteString("\n> 数据来源: TickFlow API\n")
	return sb.String()
}

// ── Universes (symbol lists) ────────────────────────────────────────────────

type tickFlowUniverseResp struct {
	Symbols []string `json:"symbols"`
	Name    string   `json:"name"`
}

// GetUniverse fetches the list of symbols for a given universe.
func (c *TickFlowClient) GetUniverse(name string) (*tickFlowUniverseResp, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("TickFlow API密钥未配置")
	}

	var result tickFlowUniverseResp
	resp, err := c.client.R().
		SetHeader("x-api-key", c.apiKey).
		SetQueryParams(map[string]string{"name": name}).
		SetResult(&result).
		Get(tickFlowBaseURL + "/v1/universes")

	if err != nil {
		return nil, fmt.Errorf("TickFlow API请求失败: %v", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("TickFlow API返回HTTP %d: %s", resp.StatusCode(), strings.TrimSpace(string(resp.Body())))
	}
	return &result, nil
}

func (c *TickFlowClient) GetUniverseAsMarkdown(name string, limit int) string {
	data, err := c.GetUniverse(name)
	if err != nil {
		return fmt.Sprintf("获取标的信息失败: %v", err)
	}
	n := len(data.Symbols)
	if n == 0 {
		return fmt.Sprintf("标的信息为空（%s）", name)
	}
	if limit <= 0 || limit > n {
		limit = n
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 标的池: %s（共 %d 只，显示前 %d 只）\n\n", name, n, limit))
	for i := 0; i < limit; i++ {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, data.Symbols[i]))
	}
	if n > limit {
		sb.WriteString(fmt.Sprintf("\n... 共 %d 只，仅显示前 %d 只\n", n, limit))
	}
	sb.WriteString("\n> 数据来源: TickFlow API\n")
	return sb.String()
}

// ── Instruments (metadata for a single symbol) ──────────────────────────────

type TickFlowInstrument struct {
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	Market      string `json:"market"`
	Type        string `json:"type"`
	ListingDate string `json:"listing_date"`
	Industry    string `json:"industry"`
}

type tickFlowInstrumentResp struct {
	Data *TickFlowInstrument `json:"data"`
}

// GetInstrument fetches metadata for a single symbol.
func (c *TickFlowClient) GetInstrument(symbol string) (*TickFlowInstrument, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("TickFlow API密钥未配置")
	}

	var result tickFlowInstrumentResp
	resp, err := c.client.R().
		SetHeader("x-api-key", c.apiKey).
		SetQueryParams(map[string]string{"symbol": symbol}).
		SetResult(&result).
		Get(tickFlowBaseURL + "/v1/instruments")

	if err != nil {
		return nil, fmt.Errorf("TickFlow API请求失败: %v", err)
	}
	if resp.StatusCode() == 400 {
		return nil, fmt.Errorf("TickFlow instruments请求参数错误(400): %s（标的代码格式是否正确？如 600000.SH）",
			strings.TrimSpace(string(resp.Body())))
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("TickFlow API返回HTTP %d: %s", resp.StatusCode(), strings.TrimSpace(string(resp.Body())))
	}
	if result.Data == nil {
		return nil, fmt.Errorf("TickFlow 未找到标的: %s", symbol)
	}
	return result.Data, nil
}

func (c *TickFlowClient) GetInstrumentAsMarkdown(symbol string) string {
	inst, err := c.GetInstrument(symbol)
	if err != nil {
		return fmt.Sprintf("获取标的信息失败: %v", err)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s - %s\n\n", inst.Symbol, inst.Name))
	sb.WriteString(fmt.Sprintf("- 市场: %s\n", inst.Market))
	sb.WriteString(fmt.Sprintf("- 类型: %s\n", inst.Type))
	sb.WriteString(fmt.Sprintf("- 上市日期: %s\n", inst.ListingDate))
	sb.WriteString(fmt.Sprintf("- 行业: %s\n", inst.Industry))
	sb.WriteString("\n> 数据来源: TickFlow API\n")
	return sb.String()
}

// ── Batch K lines (multiple symbols) ────────────────────────────────────────

func (c *TickFlowClient) GetKlinesBatch(symbols []string, period string, count int, adjust string) map[string]*TickFlowKlinesData {
	results := make(map[string]*TickFlowKlinesData, len(symbols))
	for _, sym := range symbols {
		data, err := c.GetKlines(sym, period, count, adjust)
		if err != nil {
			logger.SugaredLogger.Warnf("TickFlow batch: %s failed: %v", sym, err)
			continue
		}
		results[sym] = data
	}
	return results
}

// ── Real-time quotes ────────────────────────────────────────────────────────

// TickFlowQuote represents a single real-time quote from /v1/quotes.
type TickFlowQuote struct {
	Symbol       string  `json:"symbol"`
	Region       string  `json:"region"`
	LastPrice    float64 `json:"last_price"`
	PrevClose    float64 `json:"prev_close"`
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	Volume       int64   `json:"volume"`
	Amount       float64 `json:"amount"`
	Timestamp    int64   `json:"timestamp"`
	ChangePct    float64 `json:"change_pct"`
	ChangeAmount float64 `json:"change_amount"`
	Amplitude    float64 `json:"amplitude"`
	TurnoverRate float64 `json:"turnover_rate"`
	Name         string  `json:"name"`
}

type tickFlowQuotesResp struct {
	Data []TickFlowQuote `json:"data"`
}

// GetRealtimeQuotes fetches real-time quotes for given symbols.
// symbols: e.g. ["600000.SH", "000001.SZ"].
func (c *TickFlowClient) GetRealtimeQuotes(symbols []string) ([]TickFlowQuote, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("TickFlow API密钥未配置，请在设置中填写TickFlowAPIKey")
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("请提供至少一个标的代码")
	}

	var result tickFlowQuotesResp
	resp, err := c.client.R().
		SetHeader("x-api-key", c.apiKey).
		SetQueryParams(map[string]string{
			"symbols": strings.Join(symbols, ","),
		}).
		SetResult(&result).
		Get(tickFlowBaseURL + "/v1/quotes")

	if err != nil {
		return nil, fmt.Errorf("TickFlow 实时行情请求失败: %v", err)
	}
	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("TickFlow API Key 无效或已过期（401）")
	}
	if resp.StatusCode() == 403 {
		return nil, fmt.Errorf("TickFlow API Key 无实时行情权限（403），请检查订阅计划")
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("TickFlow 实时行情返回HTTP %d: %s", resp.StatusCode(), strings.TrimSpace(string(resp.Body())))
	}
	return result.Data, nil
}

// GetRealtimeQuotesAsMarkdown fetches quotes and returns a markdown table.
func (c *TickFlowClient) GetRealtimeQuotesAsMarkdown(symbols []string) string {
	quotes, err := c.GetRealtimeQuotes(symbols)
	if err != nil {
		return fmt.Sprintf("获取实时行情失败: %v", err)
	}
	if len(quotes) == 0 {
		return "无实时行情数据（标的代码是否正确？）"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 实时行情（%d 只）\n\n", len(quotes)))
	sb.WriteString("| 代码 | 名称 | 最新价 | 涨跌幅 | 涨跌额 | 最高 | 最低 | 成交量(手) | 成交额(元) |\n")
	sb.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")

	for _, q := range quotes {
		name := q.Name
		if name == "" {
			name = "-"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f%% | %.2f | %.2f | %.2f | %d | %.0f |\n",
			q.Symbol, name, q.LastPrice, q.ChangePct*100, q.ChangeAmount,
			q.High, q.Low, q.Volume, q.Amount))
	}
	sb.WriteString("\n> 数据来源: TickFlow API /v1/quotes\n")
	return sb.String()
}
