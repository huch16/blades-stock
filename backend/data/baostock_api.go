package data

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"blades-stock/backend/logger"
)

// ============================================================================
// Baostock 数据服务客户端
// 部署在 VPS 115.190.72.22:8090
// 提供交易日历、季频财务数据、行业分类等
// ============================================================================

const defaultBaostockServiceURL = "http://115.190.72.22:8090"

// BaostockApi Baostock 数据服务客户端
type BaostockApi struct {
	baseURL string
	client  *http.Client
}

// NewBaostockApi 创建客户端
func NewBaostockApi() *BaostockApi {
	baseURL := defaultBaostockServiceURL
	// 从配置读取（如果有）
	if settings := GetSettingConfig(); settings != nil && settings.Settings != nil {
		// 暂时使用默认地址，后续可加配置字段
		_ = settings
	}
	return &BaostockApi{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ---- 通用 HTTP 请求 ----

func (b *BaostockApi) get(path string, params map[string]string) ([]byte, error) {
	u := b.baseURL + path
	if len(params) > 0 {
		v := url.Values{}
		for k, val := range params {
			v.Set(k, val)
		}
		u += "?" + v.Encode()
	}
	resp, err := b.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("baostock request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("baostock read body failed: %w", err)
	}
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("baostock HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// ---- 响应结构 ----

type baostockResponse struct {
	Code  int                    `json:"code"`
	Msg   string                 `json:"msg"`
	Data  []map[string]interface{} `json:"data"`
	Count int                    `json:"count"`
}

// ---- 交易日历 ----

// TradeCalendarEntry 交易日历条目
type TradeCalendarEntry struct {
	CalendarDate  string `json:"calendar_date"`
	IsTradingDay  string `json:"is_trading_day"` // "1" = 交易日, "0" = 非交易日
}

// tradeCalendarYearResponse 整年交易日历响应
type tradeCalendarYearResponse struct {
	Code          int                  `json:"code"`
	Msg           string               `json:"msg"`
	Data          []TradeCalendarEntry `json:"data"`
	TradingDays   []string             `json:"trading_days"`
	TradingCount  int                  `json:"trading_count"`
}

// GetTradeCalendar 获取指定日期范围的交易日历
func (b *BaostockApi) GetTradeCalendar(startDate, endDate string) ([]TradeCalendarEntry, error) {
	body, err := b.get("/api/v1/trade_calendar", map[string]string{
		"start_date": startDate,
		"end_date":   endDate,
	})
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse trade_calendar failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock trade_calendar error: %s", resp.Msg)
	}
	var entries []TradeCalendarEntry
	for _, m := range resp.Data {
		entries = append(entries, TradeCalendarEntry{
			CalendarDate: fmt.Sprintf("%v", m["calendar_date"]),
			IsTradingDay: fmt.Sprintf("%v", m["is_trading_day"]),
		})
	}
	return entries, nil
}

// GetTradeCalendarYear 获取整年交易日历
func (b *BaostockApi) GetTradeCalendarYear(year int) (*tradeCalendarYearResponse, error) {
	body, err := b.get(fmt.Sprintf("/api/v1/trade_calendar/year/%d", year), nil)
	if err != nil {
		return nil, err
	}
	var resp tradeCalendarYearResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse trade_calendar_year failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock trade_calendar_year error: %s", resp.Msg)
	}
	return &resp, nil
}

// ---- 季频财务数据 ----

// FinancialData 季频财务数据（通用结构）
type FinancialData struct {
	Code       string  `json:"code"`
	PubDate    string  `json:"pubDate"`
	StatDate   string  `json:"statDate"`
	RoeAvg     float64 `json:"roeAvg,omitempty"`     // 净资产收益率
	NpMargin   float64 `json:"npMargin,omitempty"`   // 销售净利率
	GpMargin   float64 `json:"gpMargin,omitempty"`   // 销售毛利率"
	NetProfit  float64 `json:"netProfit,omitempty"`  // 净利润
	NetProfitGrowth float64 `json:"netProfitGrowth,omitempty"` // 净利润增长率
	YoYProfit  float64 `json:"yoyProfit,omitempty"`  // 同比利润
	YoYRevenue float64 `json:"yoyRevenue,omitempty"` // 同比营收
	Revenue    float64 `json:"revenue,omitempty"`    // 营业收入
	RevenueGrowth float64 `json:"revenueGrowth,omitempty"` // 营收增长率
}

// GetProfitData 获取季频盈利能力数据
func (b *BaostockApi) GetProfitData(code string, year, quarter int) ([]map[string]interface{}, error) {
	// 格式转换：sh600671 → sh.600671
	c := normalizeBaostockCode(code)
	body, err := b.get("/api/v1/financial/profit", map[string]string{
		"code":    c,
		"year":    fmt.Sprintf("%d", year),
		"quarter": fmt.Sprintf("%d", quarter),
	})
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse profit failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock profit error: %s", resp.Msg)
	}
	return resp.Data, nil
}

// GetGrowthData 获取季频成长能力数据
func (b *BaostockApi) GetGrowthData(code string, year, quarter int) ([]map[string]interface{}, error) {
	c := normalizeBaostockCode(code)
	body, err := b.get("/api/v1/financial/growth", map[string]string{
		"code":    c,
		"year":    fmt.Sprintf("%d", year),
		"quarter": fmt.Sprintf("%d", quarter),
	})
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse growth failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock growth error: %s", resp.Msg)
	}
	return resp.Data, nil
}

// GetOperationData 获取季频营运能力数据
func (b *BaostockApi) GetOperationData(code string, year, quarter int) ([]map[string]interface{}, error) {
	c := normalizeBaostockCode(code)
	body, err := b.get("/api/v1/financial/operation", map[string]string{
		"code":    c,
		"year":    fmt.Sprintf("%d", year),
		"quarter": fmt.Sprintf("%d", quarter),
	})
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse operation failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock operation error: %s", resp.Msg)
	}
	return resp.Data, nil
}

// GetCashFlowData 获取季频现金流量数据
func (b *BaostockApi) GetCashFlowData(code string, year, quarter int) ([]map[string]interface{}, error) {
	c := normalizeBaostockCode(code)
	body, err := b.get("/api/v1/financial/cash_flow", map[string]string{
		"code":    c,
		"year":    fmt.Sprintf("%d", year),
		"quarter": fmt.Sprintf("%d", quarter),
	})
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse cash_flow failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock cash_flow error: %s", resp.Msg)
	}
	return resp.Data, nil
}

// GetDupontData 获取季频杜邦分析数据
func (b *BaostockApi) GetDupontData(code string, year, quarter int) ([]map[string]interface{}, error) {
	c := normalizeBaostockCode(code)
	body, err := b.get("/api/v1/financial/dupont", map[string]string{
		"code":    c,
		"year":    fmt.Sprintf("%d", year),
		"quarter": fmt.Sprintf("%d", quarter),
	})
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse dupont failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock dupont error: %s", resp.Msg)
	}
	return resp.Data, nil
}

// ---- 行业分类 ----

// StockIndustry 股票行业分类
type StockIndustry struct {
	Code          string `json:"code"`
	CodeName      string `json:"code_name"`
	Industry      string `json:"industry"`
	IndustryClassification string `json:"industryClassification"`
}

// GetStockIndustry 获取股票行业分类
func (b *BaostockApi) GetStockIndustry(code string) ([]StockIndustry, error) {
	params := map[string]string{}
	if code != "" {
		params["code"] = normalizeBaostockCode(code)
	}
	body, err := b.get("/api/v1/stock_industry", params)
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse industry failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock industry error: %s", resp.Msg)
	}
	var result []StockIndustry
	for _, m := range resp.Data {
		result = append(result, StockIndustry{
			Code:          fmt.Sprintf("%v", m["code"]),
			CodeName:      fmt.Sprintf("%v", m["code_name"]),
			Industry:      fmt.Sprintf("%v", m["industry"]),
			IndustryClassification: fmt.Sprintf("%v", m["industryClassification"]),
		})
	}
	return result, nil
}

// ---- 证券基本信息 ----

// BaostockStockBasic Baostock 证券基本信息
type BaostockStockBasic struct {
	Code        string `json:"code"`
	CodeName    string `json:"code_name"`
	IpoDate     string `json:"ipoDate"`
	OutDate     string `json:"outDate"`
	Type        string `json:"type"`         // 1=股票 2=指数 3=其他
	Status      string `json:"status"`       // 1=正常 0=退市
}

// GetStockBasic 获取证券基本信息
func (b *BaostockApi) GetStockBasic(code string) ([]BaostockStockBasic, error) {
	params := map[string]string{}
	if code != "" {
		params["code"] = normalizeBaostockCode(code)
	}
	body, err := b.get("/api/v1/stock_basic", params)
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse stock_basic failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock stock_basic error: %s", resp.Msg)
	}
	var result []BaostockStockBasic
	for _, m := range resp.Data {
		result = append(result, BaostockStockBasic{
			Code:     fmt.Sprintf("%v", m["code"]),
			CodeName: fmt.Sprintf("%v", m["code_name"]),
			IpoDate:  fmt.Sprintf("%v", m["ipoDate"]),
			OutDate:  fmt.Sprintf("%v", m["outDate"]),
			Type:     fmt.Sprintf("%v", m["type"]),
			Status:   fmt.Sprintf("%v", m["status"]),
		})
	}
	return result, nil
}

// ---- 历史K线（含复权和PE/PB） ----

// BaostockKLine Baostock K线数据（含复权、估值指标）
type BaostockKLine struct {
	Date       string  `json:"date"`
	Code       string  `json:"code"`
	Open       string  `json:"open"`
	High       string  `json:"high"`
	Low        string  `json:"low"`
	Close      string  `json:"close"`
	Preclose   string  `json:"preclose"`
	Volume     string  `json:"volume"`
	Amount     string  `json:"amount"`
	Adjustflag string  `json:"adjustflag"`
	Turn       string  `json:"turn"`       // 换手率
	Tradestatus string `json:"tradestatus"` // 1=正常 0=停牌
	PctChg     string  `json:"pctChg"`     // 涨跌幅%
	PETTM      string  `json:"peTTM"`      // 市盈率TTM
	PBMRQ      string  `json:"pbMRQ"`      // 市净率MRQ
	PSTTM      string  `json:"psTTM"`      // 市销率TTM
	PCFNcfTTM  string  `json:"pcfNcfTTM"`  // 市现率TTM
	IsST       string  `json:"isST"`       // 1=ST 0=正常
}

// GetKLine 获取历史K线（含复权、PE/PB等估值指标）
func (b *BaostockApi) GetKLine(code, startDate, endDate, frequency, adjustflag string) ([]BaostockKLine, error) {
	c := normalizeBaostockCode(code)
	params := map[string]string{
		"code":       c,
		"frequency":  frequency,
		"adjustflag": adjustflag,
	}
	if startDate != "" {
		params["start_date"] = startDate
	}
	if endDate != "" {
		params["end_date"] = endDate
	}
	body, err := b.get("/api/v1/kline", params)
	if err != nil {
		return nil, err
	}
	var resp baostockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("baostock parse kline failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("baostock kline error: %s", resp.Msg)
	}
	var result []BaostockKLine
	for _, m := range resp.Data {
		result = append(result, BaostockKLine{
			Date:        fmt.Sprintf("%v", m["date"]),
			Code:        fmt.Sprintf("%v", m["code"]),
			Open:        fmt.Sprintf("%v", m["open"]),
			High:        fmt.Sprintf("%v", m["high"]),
			Low:         fmt.Sprintf("%v", m["low"]),
			Close:       fmt.Sprintf("%v", m["close"]),
			Preclose:    fmt.Sprintf("%v", m["preclose"]),
			Volume:      fmt.Sprintf("%v", m["volume"]),
			Amount:      fmt.Sprintf("%v", m["amount"]),
			Adjustflag:  fmt.Sprintf("%v", m["adjustflag"]),
			Turn:        fmt.Sprintf("%v", m["turn"]),
			Tradestatus: fmt.Sprintf("%v", m["tradestatus"]),
			PctChg:      fmt.Sprintf("%v", m["pctChg"]),
			PETTM:       fmt.Sprintf("%v", m["peTTM"]),
			PBMRQ:       fmt.Sprintf("%v", m["pbMRQ"]),
			PSTTM:       fmt.Sprintf("%v", m["psTTM"]),
			PCFNcfTTM:   fmt.Sprintf("%v", m["pcfNcfTTM"]),
			IsST:        fmt.Sprintf("%v", m["isST"]),
		})
	}
	return result, nil
}

// ---- 健康检查 ----

// HealthCheck 检查 Baostock 服务是否可用
func (b *BaostockApi) HealthCheck() bool {
	body, err := b.get("/health", nil)
	if err != nil {
		logger.SugaredLogger.Warnf("baostock health check failed: %v", err)
		return false
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	status, ok := resp["status"]
	return ok && status == "ok"
}

// ---- 工具函数 ----

// normalizeBaostockCode 将 blades-stock 代码格式转换为 baostock 格式
// sh600671 → sh.600671, sz000100 → sz.000100, 600671 → sh.600671
func normalizeBaostockCode(code string) string {
	c := strings.TrimSpace(code)
	// 已经是 baostock 格式
	if strings.Contains(c, ".") {
		return c
	}
	// 带市场前缀
	if len(c) > 2 {
		prefix := strings.ToLower(c[:2])
		if prefix == "sh" || prefix == "sz" || prefix == "bj" {
			return prefix + "." + c[2:]
		}
	}
	// 纯数字，根据首字符判断
	if len(c) >= 6 {
		first := c[0]
		switch first {
		case '6':
			return "sh." + c
		case '0', '3':
			return "sz." + c
		case '8', '9':
			return "bj." + c
		}
	}
	return c
}
