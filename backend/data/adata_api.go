package data

import (
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
		client:  resty.New().SetTimeout(60 * time.Second),
	}
}

func (c *AdataClient) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

type adataResponse struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data []map[string]any `json:"data"`
}

func (c *AdataClient) get(path string) (*adataResponse, error) {
	var resp adataResponse
	r, err := c.client.R().
		SetResult(&resp).
		SetHeader("User-Agent", "blades-stock/1.0").
		Get(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("adata HTTP GET %s: %w", path, err)
	}
	if r.StatusCode() >= 400 {
		return nil, fmt.Errorf("adata HTTP %d GET %s", r.StatusCode(), path)
	}
	if resp.Code >= 400 {
		return nil, fmt.Errorf("adata api err %s: %s", path, resp.Msg)
	}
	return &resp, nil
}

func (c *AdataClient) Ping() error {
	var resp struct {
		Status string `json:"status"`
	}
	_, err := c.client.R().SetResult(&resp).Get(c.baseURL + "/api/health")
	if err != nil {
		return err
	}
	logger.SugaredLogger.Infof("[AdataClient] connected to %s", c.baseURL)
	return nil
}

func (c *AdataClient) GetMarketCurrent(codeList []string) ([]map[string]any, error) {
	codes := strings.Join(codeList, ",")
	resp, err := c.get(fmt.Sprintf("/api/stock/market_current?code_list=%s", url.QueryEscape(codes)))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AdataClient) GetKLineData(stockCode, startDate, endDate string) ([]map[string]any, error) {
	path := fmt.Sprintf("/api/stock/market?stock_code=%s&start_date=%s&end_date=%s",
		url.QueryEscape(stockCode), url.QueryEscape(startDate), url.QueryEscape(endDate))
	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AdataClient) GetTradeCalendar(year int) ([]map[string]any, error) {
	resp, err := c.get(fmt.Sprintf("/api/stock/trade_calendar?year=%d", year))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AdataClient) GetNorthFlow() ([]map[string]any, error) {
	resp, err := c.get("/api/sentiment/north_flow_current")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}
