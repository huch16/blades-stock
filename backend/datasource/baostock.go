package datasource

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"blades-stock/backend/circuitbreaker"
)

// ============================================================================
// Baostock 数据服务客户端
// 默认部署在 VPS 115.190.72.22:8090，可用环境变量 BLADES_STOCK_BAOSTOCK_URL 覆盖
// 提供交易日历、季频财务数据、行业分类等
// ============================================================================

// BaostockApi Baostock 数据服务客户端
type BaostockApi struct {
	baseURL string
	client  *http.Client
}

// NewBaostockApi 创建客户端。baseURL 由调用方（backend/data 的
// NewBaostockApi 转发）按环境变量解析后传入，本包不读设置表、不 import data。
func NewBaostockApi(baseURL string) *BaostockApi {
	u := baseURL
	// 熔断保护：已知失联的默认远程主机 (115.190.72.22) 快速失败，杜绝 30 秒超时阻塞。
	timeout := 30 * time.Second
	if strings.Contains(u, "115.190.72.22") {
		timeout = 2 * time.Second
	}
	return &BaostockApi{
		baseURL: u,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// ---- 通用 HTTP 请求 ----

func (b *BaostockApi) get(path string, params map[string]string) ([]byte, error) {
	cb := circuitbreaker.Get("baostock_api")
	if !cb.Allow() {
		return nil, fmt.Errorf("baostock circuit breaker open (fast fail)")
	}
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
		cb.Failure()
		return nil, fmt.Errorf("baostock request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		cb.Failure()
		return nil, fmt.Errorf("baostock read body failed: %w", err)
	}
	if resp.StatusCode >= 400 {
		cb.Failure()
		return body, fmt.Errorf("baostock HTTP %d: %s", resp.StatusCode, string(body))
	}
	cb.Success()
	return body, nil
}

// ---- 响应结构 ----

type baostockResponse struct {
	Code  int                      `json:"code"`
	Msg   string                   `json:"msg"`
	Data  []map[string]interface{} `json:"data"`
	Count int                      `json:"count"`
}

// ---- 交易日历 ----

// TradeCalendarEntry 交易日历条目
type TradeCalendarEntry struct {
	CalendarDate string `json:"calendar_date"`
	IsTradingDay string `json:"is_trading_day"` // "1" = 交易日, "0" = 非交易日
}

// tradeCalendarYearResponse 整年交易日历响应
type tradeCalendarYearResponse struct {
	Code         int                  `json:"code"`
	Msg          string               `json:"msg"`
	Data         []TradeCalendarEntry `json:"data"`
	TradingDays  []string             `json:"trading_days"`
	TradingCount int                  `json:"trading_count"`
}

// GetTradeCalendar 获取指定日期范围的交易日历
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
	Code            string  `json:"code"`
	PubDate         string  `json:"pubDate"`
	StatDate        string  `json:"statDate"`
	RoeAvg          float64 `json:"roeAvg,omitempty"`          // 净资产收益率
	NpMargin        float64 `json:"npMargin,omitempty"`        // 销售净利率
	GpMargin        float64 `json:"gpMargin,omitempty"`        // 销售毛利率"
	NetProfit       float64 `json:"netProfit,omitempty"`       // 净利润
	NetProfitGrowth float64 `json:"netProfitGrowth,omitempty"` // 净利润增长率
	YoYProfit       float64 `json:"yoyProfit,omitempty"`       // 同比利润
	YoYRevenue      float64 `json:"yoyRevenue,omitempty"`      // 同比营收
	Revenue         float64 `json:"revenue,omitempty"`         // 营业收入
	RevenueGrowth   float64 `json:"revenueGrowth,omitempty"`   // 营收增长率
}

// GetProfitData 获取季频盈利能力数据
// GetGrowthData 获取季频成长能力数据
// GetOperationData 获取季频营运能力数据
// GetCashFlowData 获取季频现金流量数据
// GetDupontData 获取季频杜邦分析数据
// ---- 行业分类 ----

// StockIndustry 股票行业分类
type StockIndustry struct {
	Code                   string `json:"code"`
	CodeName               string `json:"code_name"`
	Industry               string `json:"industry"`
	IndustryClassification string `json:"industryClassification"`
}

// GetStockIndustry 获取股票行业分类
// ---- 证券基本信息 ----

// BaostockStockBasic Baostock 证券基本信息
type BaostockStockBasic struct {
	Code     string `json:"code"`
	CodeName string `json:"code_name"`
	IpoDate  string `json:"ipoDate"`
	OutDate  string `json:"outDate"`
	Type     string `json:"type"`   // 1=股票 2=指数 3=其他
	Status   string `json:"status"` // 1=正常 0=退市
}

// GetStockBasic 获取证券基本信息
// ---- 历史K线（含复权和PE/PB） ----

// BaostockKLine Baostock K线数据（含复权、估值指标）
type BaostockKLine struct {
	Date        string `json:"date"`
	Code        string `json:"code"`
	Open        string `json:"open"`
	High        string `json:"high"`
	Low         string `json:"low"`
	Close       string `json:"close"`
	Preclose    string `json:"preclose"`
	Volume      string `json:"volume"`
	Amount      string `json:"amount"`
	Adjustflag  string `json:"adjustflag"`
	Turn        string `json:"turn"`        // 换手率
	Tradestatus string `json:"tradestatus"` // 1=正常 0=停牌
	PctChg      string `json:"pctChg"`      // 涨跌幅%
	PETTM       string `json:"peTTM"`       // 市盈率TTM
	PBMRQ       string `json:"pbMRQ"`       // 市净率MRQ
	PSTTM       string `json:"psTTM"`       // 市销率TTM
	PCFNcfTTM   string `json:"pcfNcfTTM"`   // 市现率TTM
	IsST        string `json:"isST"`        // 1=ST 0=正常
}

// GetKLine 获取历史K线（含复权、PE/PB等估值指标）
// ---- 健康检查 ----

// HealthCheck 检查 Baostock 服务是否可用
// ---- 工具函数 ----

// NormalizeBaostockCode 将 blades-stock 代码格式转换为 baostock 格式
// sh600671 → sh.600671, sz000100 → sz.000100, 600671 → sh.600671
func NormalizeBaostockCode(code string) string {
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
