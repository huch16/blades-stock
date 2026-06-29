//go:build integration

// Integration tests that require real services (DB, AI APIs, network, wails runtime).
// Run with: go test -tags=integration ./...
// Skip by default with: go test ./...

package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/util"
)

func TestCheckStockBaseInfo(t *testing.T) {
	db.Init("./data/stock.db")
	NewApp().CheckStockBaseInfo(context.Background())
}

func TestJson(t *testing.T) {
	db.Init("./data/stock.db")

	jsonStr := `{
		"id" : 3334,
		"created_at" : "2025-02-28 16:49:31.8342514+08:00",
		"updated_at" : "2025-02-28 16:49:31.8342514+08:00",
		"deleted_at" : null,
		"code" : "PUK.US",
		"name" : "英国保诚集团",
		"full_name" : "",
		"e_name" : "",
		"exchange" : "NASDAQ",
		"type" : "stock",
		"is_del" : 0,
		"bk_name" : null,
		"bk_code" : null
	}`

	v := &models.StockInfoUS{}
	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	t.Logf("v:%+v", v)

	if err := db.Dao.Model(v).Updates(v).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
}

func TestUpdateCheck(t *testing.T) {
	releaseVersion := &models.GitHubReleaseVersion{}
	_, err := resty.New().R().
		SetResult(releaseVersion).
		SetHeader("Accept", "application/vnd.github+json").
		SetHeader("X-GitHub-Api-Version", "2022-11-28").
		Get("https://api.github.com/repos/huch16/blades-stock/releases/latest")
	if err != nil {
		t.Fatalf("github release: %v", err)
	}
	t.Logf("releaseVersion:%+v", releaseVersion)
}

func TestGetScreenResolution(t *testing.T) {
	x, y, w, h, err := getScreenResolution()
	if err != nil {
		t.Fatalf("get screen resolution: %v", err)
	}
	t.Logf("x:%d,y:%d,w:%d,h:%d", x, y, w, h)
}

func TestGetAiRecommendStocksList(t *testing.T) {
	db.Init("./data/stock.db")

	const str = `{"startDate": "2026-03-20 00:00:00", "endDate": "2026-03-27 23:59:59", "page": 1, "pageSize": 5000}`
	query := &models.AiRecommendStocksQuery{}
	if err := json.Unmarshal([]byte(str), query); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	pageData, err := data.NewAiRecommendStocksService().GetAiRecommendStocksList(query)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	logger.SugaredLogger.Infof("pageData:%+v", pageData.List)

	dataExport := make([]models.AiRecommendStocksMdExport, 0, len(pageData.List))
	for _, v := range pageData.List {
		dataExport = append(dataExport, v.ToMdExportStruct())
	}
	content := util.MarkdownTableWithTitle("近期AI分析/推荐股票明细列表", dataExport)
	t.Log(content)
}

func TestSummaryStockNews(t *testing.T) {
	db.Init("./data/stock.db")
	question := "分析今日的市场行情走势是否和券商的观点一致"
	app := NewApp()
	msgs := data.NewDeepSeekOpenAi(app.ctx, 0).NewSummaryStockNewsStreamWithTools(question, nil, app.AiTools, true, nil)

	var content strings.Builder
	for msg := range msgs {
		t.Logf("msg:%+v", msg)
		if c, ok := msg["content"].(string); ok {
			content.WriteString(c)
		}
	}
	t.Log(content.String())
}

func TestFetchAiModels(t *testing.T) {
	models := NewApp().FetchAiModels("https://ark.cn-beijing.volces.com/api/v3", "")
	t.Log(models)
}

func TestGetLatestTradingDay(t *testing.T) {
	date := NewApp().GetLatestTradingDay()
	t.Log(date)
}
