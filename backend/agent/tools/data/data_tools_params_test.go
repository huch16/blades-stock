package data

import (
	"context"
	"testing"

	bladesTools "github.com/go-kratos/blades/tools"
)

// findDataTool 在注册工具切片里按名取工具；找不到时测试失败。
func findDataTool(t *testing.T, tools []bladesTools.Tool, name string) bladesTools.Tool {
	t.Helper()
	for _, tool := range tools {
		if tool.Name() == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func handleDataTool(t *testing.T, tool bladesTools.Tool, args string) string {
	t.Helper()
	out, err := tool.Handle(context.Background(), args)
	if err != nil {
		t.Fatalf("%s Handle(%s) err = %v", tool.Name(), args, err)
	}
	return out
}

// TestDataToolsParams_EmptyPaths 验证 Batch 4 扫过的 data 子包工具在
// “缺参/别名为空”时都走原来的早期返回，而不是把空参数往下游
// （DB/网络/quant）传；别名形态的空值也不应再被误判。
// （空路径在副作用之前即返回；读库的几项读的是本包 TestMain 配的
// 临时 SQLite 空库，不触真实 data/stock.db。）
func TestDataToolsParams_EmptyPaths(t *testing.T) {
	cases := []struct {
		name string
		tool bladesTools.Tool
		args []string
		want string
	}{
		{
			"GetAIAnalysisContent empty", findDataTool(t, RegisterAITools(), "GetAIAnalysisContent"),
			[]string{`{}`, `{"stock_code": ""}`, `{"code": ""}`, `{"symbol": ""}`, `{"stockCode": null}`},
			"请提供股票代码",
		},
		{
			"GetFundInfo empty", findDataTool(t, RegisterFundTools(), "GetFundInfo"),
			[]string{`{}`, `{"fund_code": ""}`, `{"code": ""}`},
			"请输入基金代码",
		},
		{
			"GetFundKLine empty", findDataTool(t, RegisterFundTools(), "GetFundKLine"),
			[]string{`{}`, `{"fundCode": ""}`, `{"fund_code": ""}`},
			"请输入基金代码",
		},
		{
			"GetFundHistoryNetValue empty", findDataTool(t, RegisterFundTools(), "GetFundHistoryNetValue"),
			[]string{`{}`, `{"fundCode": ""}`},
			"请输入基金代码",
		},
		{
			"GetFundTop10Holdings empty", findDataTool(t, RegisterFundTools(), "GetFundTop10Holdings"),
			[]string{`{}`, `{"fund_code": ""}`},
			"请输入基金代码",
		},
		{
			"GetStockNotice empty", findDataTool(t, RegisterHotTools(), "GetStockNotice"),
			[]string{`{}`, `{"stockCodes": ""}`, `{"stock_codes": ""}`, `{"codes": []}`, `{"code": ""}`},
			"请输入股票代码",
		},
		{
			"GetEastMoneyKLine empty", findDataTool(t, RegisterKlineTools(), "GetEastMoneyKLine"),
			[]string{`{}`, `{"stock_code": ""}`, `{"code": ""}`, `{"stockCodes": []}`},
			"参数 stockCode 或 stockCodes 不能为空",
		},
		{
			"GetEastMoneyKLineWithMA empty", findDataTool(t, RegisterKlineTools(), "GetEastMoneyKLineWithMA"),
			[]string{`{}`, `{"stockCode": ""}`, `{"symbol": ""}`},
			"参数 stockCode 或 stockCodes 不能为空",
		},
		{
			"GetStockKLine empty", findDataTool(t, RegisterKlineTools(), "GetStockKLine"),
			[]string{`{}`, `{"stock_code": ""}`, `{"days": "30"}`},
			"未获取到 K 线数据",
		},
		{
			"GetAIAnalysisDetail empty id", findDataTool(t, RegisterAITools(), "GetAIAnalysisDetail"),
			[]string{`{}`, `{"id": 0}`, `{"recordId": 0}`, `{"id": "0"}`},
			"请提供有效的分析报告ID",
		},
		{
			"FilterStocks empty db", findDataTool(t, RegisterScreenTools(), "FilterStocks"),
			[]string{
				`{"macdGoldenFork": true}`,
				`{"macd_golden_fork": true}`,
				`{"upp_days": 5}`,
				`{"up_nday": 5}`,
				`{"keyword": "xyz"}`,
			},
			"未找到符合条件的股票",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, args := range tc.args {
				if got := handleDataTool(t, tc.tool, args); got != tc.want {
					t.Errorf("%s Handle(%s) = %q, want %q", tc.tool.Name(), args, got, tc.want)
				}
			}
		})
	}
}

// TestDataToolsParams_AIAnalysisDetail_NotFound 验证 id 别名解析后，
// 不存在的报告在临时空库里查不到时返回固定文案（不抛错）。
func TestDataToolsParams_AIAnalysisDetail_NotFound(t *testing.T) {
	tool := findDataTool(t, RegisterAITools(), "GetAIAnalysisDetail")
	for _, args := range []string{`{"id": 5}`, `{"recordId": 5}`} {
		if got := handleDataTool(t, tool, args); got != "未找到该分析报告" {
			t.Errorf("GetAIAnalysisDetail Handle(%s) = %q, want %q", args, got, "未找到该分析报告")
		}
	}
}
