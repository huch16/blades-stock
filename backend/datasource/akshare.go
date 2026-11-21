package datasource

import (
	"fmt"
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
