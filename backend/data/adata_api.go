package data

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"blades-stock/backend/logger"
	"github.com/go-resty/resty/v2"
)

const defaultADataServiceURL = "http://115.190.72.22:8000"

type AdataClient struct {
	baseURL string
	client  *resty.Client
}

func NewAdataClient() *AdataClient {
	cfg := GetSettingConfig()
	u := strings.TrimSpace(cfg.Settings.ADataServiceUrl)
	if u == "" {
		u = defaultADataServiceURL
	}
	return &AdataClient{
		baseURL: u,
		client:  resty.New().SetTimeout(30 * time.Second),
	}
}

func (c *AdataClient) SetBaseURL(baseURL string) {
	c.baseURL = strings.TrimRight(baseURL, "/")
}

// adataValueResponse 是 AData 服务实际返回的格式
// {"value": [...], "count": N}
type adataValueResponse struct {
	Value []map[string]any `json:"value"`
	Count int              `json:"Count"`
}

func (c *AdataClient) get(path string) ([]map[string]any, error) {
	r, err := c.client.R().
		SetHeader("User-Agent", "blades-stock/1.0").
		Get(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("adata HTTP GET %s: %w", path, err)
	}
	if r.StatusCode() >= 400 {
		return nil, fmt.Errorf("adata HTTP %d GET %s: %s", r.StatusCode(), path, string(r.Body()))
	}

	// AData 返回 {"value": [...]} 格式
	var resp adataValueResponse
	if err := json.Unmarshal(r.Body(), &resp); err != nil {
		// 尝试直接解析为数组
		var arr []map[string]any
		if err2 := json.Unmarshal(r.Body(), &arr); err2 != nil {
			return nil, fmt.Errorf("adata response parse failed: %w (body: %s)", err, truncateStr(string(r.Body()), 200))
		}
		return arr, nil
	}
	return resp.Value, nil
}

func (c *AdataClient) Ping() error {
	var resp struct {
		Status string `json:"status"`
	}
	_, err := c.client.R().SetResult(&resp).Get(c.baseURL + "/health")
	if err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("adata service status: %s", resp.Status)
	}
	logger.SugaredLogger.Infof("[AdataClient] connected to %s", c.baseURL)
	return nil
}

func (c *AdataClient) GetMarketCurrent(codeList []string) ([]map[string]any, error) {
	codes := strings.Join(codeList, ",")
	return c.get(fmt.Sprintf("/api/stock/market_current?code_list=%s", url.QueryEscape(codes)))
}

func (c *AdataClient) GetKLineData(stockCode, startDate, endDate string) ([]map[string]any, error) {
	path := fmt.Sprintf("/api/stock/market?stock_code=%s&start_date=%s&end_date=%s",
		url.QueryEscape(stockCode), url.QueryEscape(startDate), url.QueryEscape(endDate))
	return c.get(path)
}

func (c *AdataClient) GetTradeCalendar() ([]map[string]any, error) {
	return c.get("/api/stock/trade_calendar")
}

func (c *AdataClient) GetNorthFlow() ([]map[string]any, error) {
	return c.get("/api/sentiment/north_flow_current")
}

// GetTradeCalendarDays 提取交易日列表（trade_status=1 的日期）
func (c *AdataClient) GetTradeCalendarDays() ([]string, error) {
	data, err := c.GetTradeCalendar()
	if err != nil {
		return nil, err
	}
	var days []string
	for _, item := range data {
		status, _ := item["trade_status"].(float64)
		if int(status) == 1 {
			if dateStr, ok := item["trade_date"].(string); ok {
				days = append(days, dateStr)
			}
		}
	}
	return days, nil
}
