package datasource

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"blades-stock/backend/logger"
	"github.com/go-resty/resty/v2"
)

type AkshareClient struct {
	baseURL string
	client  *resty.Client
}

// NewAkshareClient 构造纯 vendor 客户端。baseURL 由调用方（backend/data 的
// NewAkshareClient 转发）按“DB 设置 → 环境变量/默认值”解析后传入，
// 本包不读设置表、不 import data。
func NewAkshareClient(baseURL string) *AkshareClient {
	return &AkshareClient{
		baseURL: baseURL,
		client:  resty.New().SetTimeout(60 * time.Second),
	}
}

func (c *AkshareClient) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

type akShareResponse struct {
	Code int              `json:"code"`
	Msg  string           `json:"msg"`
	Data []map[string]any `json:"data"`
	Meta map[string]any   `json:"meta,omitempty"`
}

func (c *AkshareClient) get(path string) (*akShareResponse, error) {
	var resp akShareResponse
	r, err := c.client.R().
		SetResult(&resp).
		SetHeader("User-Agent", "blades-stock/1.0").
		Get(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("akshare HTTP GET %s: %w", path, err)
	}
	if r.StatusCode() >= 400 {
		return nil, fmt.Errorf("akshare HTTP %d GET %s", r.StatusCode(), path)
	}
	if resp.Code >= 400 {
		return nil, fmt.Errorf("akshare api err %s: %s", path, resp.Msg)
	}
	return &resp, nil
}

func (c *AkshareClient) post(path string, body map[string]any) (*akShareResponse, error) {
	var resp akShareResponse
	r, err := c.client.R().
		SetResult(&resp).
		SetHeader("User-Agent", "blades-stock/1.0").
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("akshare HTTP POST %s: %w", path, err)
	}
	if r.StatusCode() >= 400 {
		return nil, fmt.Errorf("akshare HTTP %d POST %s", r.StatusCode(), path)
	}
	if resp.Code >= 400 {
		return nil, fmt.Errorf("akshare api err %s: %s", path, resp.Msg)
	}
	return &resp, nil
}

func (c *AkshareClient) Ping() error {
	var resp struct {
		Status string `json:"status"`
	}
	_, err := c.client.R().SetResult(&resp).Get(c.baseURL + "/health")
	if err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("akshare service status: %s", resp.Status)
	}
	logger.SugaredLogger.Infof("[AkshareClient] connected to %s", c.baseURL)
	return nil
}

func (c *AkshareClient) FuturesSpot(market string) ([]map[string]any, error) {
	marketMap := map[string]string{
		"上期所": "SHFE", "大商所": "DCE", "郑商所": "CZCE",
		"广期所": "GFEX", "中金所": "CFFEX", "上海国际能源交易中心": "INE",
		"shfe": "SHFE", "dce": "DCE", "czce": "CZEX",
		"gfex": "GFEX", "cffex": "CFFEX", "ine": "INE",
	}
	if mapped, ok := marketMap[market]; ok {
		market = mapped
	}
	resp, err := c.get(fmt.Sprintf("/api/v1/futures/spot?market=%s", url.QueryEscape(market)))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) ETFSpot() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/etf/spot")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) ETFHistorical(symbol, period, startDate, endDate, adjust string) ([]map[string]any, error) {
	path := fmt.Sprintf("/api/v1/etf/historical?symbol=%s&period=%s&start_date=%s&end_date=%s&adjust=%s", symbol, period, startDate, endDate, adjust)
	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) OpenFundNetValue(symbol, startDate, endDate string) ([]map[string]any, error) {
	path := fmt.Sprintf("/api/v1/fund/open/net_value?symbol=%s&start_date=%s&end_date=%s", symbol, startDate, endDate)
	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) BoardConcept() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/board/concept")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) BoardIndustry() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/board/industry")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) MacroCPI() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/macro/cpi")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) MacroGDP() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/macro/gdp")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) MacroPMI() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/macro/pmi")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) MacroMoneySupply() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/macro/money_supply")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) LongHuToday() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/longhu/today")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) ConvertibleBondSpot() ([]map[string]any, error) {
	resp, err := c.get("/api/v1/convertible/bond_spot")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *AkshareClient) Call(function string, kwargs map[string]any) ([]map[string]any, error) {
	body := map[string]any{"function": function}
	if kwargs != nil {
		body["kwargs"] = kwargs
	}
	resp, err := c.post("/api/v1/call", body)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}
