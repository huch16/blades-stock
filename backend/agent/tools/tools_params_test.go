package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"blades-stock/backend/data"

	bladesTools "github.com/go-kratos/blades/tools"
)

// findToolByName 在工具切片里按名取工具；找不到时测试失败。
func findToolByName(t *testing.T, tools []bladesTools.Tool, name string) bladesTools.Tool {
	t.Helper()
	for _, tool := range tools {
		if tool.Name() == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found in %v", name, tools)
	return nil
}

func handleEmpty(t *testing.T, tool bladesTools.Tool, args string) string {
	t.Helper()
	out, err := tool.Handle(context.Background(), args)
	if err != nil {
		t.Fatalf("%s Handle(%s) err = %v", tool.Name(), args, err)
	}
	return out
}

// TestToolsParams_EmptyPaths 验证 Batch 3 扫过的工具在“缺参/别名为空”
// 时都走原来的早期返回，而不是把空参数往下游（DB/网络/quant）传：
// 单键问题修掉后，别名形态的空值也不应再被误判。
// （空路径在副作用之前即返回，不触 DB/网络/外部 AI；
// List 类工具读的是包内 TestMain 配的临时 SQLite 空库。）
func TestToolsParams_EmptyPaths(t *testing.T) {
	cases := []struct {
		name string
		tool bladesTools.Tool
		args []string
		want string
	}{
		{
			"AnalyzeHotMoneyActivity empty", findToolByName(t, GetHotMoneyChipTools(), "AnalyzeHotMoneyActivity"),
			[]string{`{}`, `{"stock_code": ""}`, `{"code": ""}`, `{"symbol": ""}`, `{"stock_code": null}`, `{"stockCode": " "}`},
			"请提供股票代码",
		},
		{
			"AnalyzeChipConcentration empty", findToolByName(t, GetHotMoneyChipTools(), "AnalyzeChipConcentration"),
			[]string{`{}`, `{"stock_code": ""}`, `{"code": ""}`},
			"请提供股票代码",
		},
		{
			"AnalyzeCapitalGame empty", findToolByName(t, GetHotMoneyChipTools(), "AnalyzeCapitalGame"),
			[]string{`{}`, `{"stockCode": ""}`, `{"symbol": ""}`},
			"请提供股票代码",
		},
		{
			"GetADataMarketCurrent empty", findToolByName(t, GetAdataTools(), "GetADataMarketCurrent"),
			[]string{`{}`, `{"codeList": ""}`, `{"stock_codes": ""}`, `{"codes": []}`},
			"【提示】未指定有效股票代码，请提供 6 位股票代码（如 000001 或 600519）。",
		},
		{
			"TickFlowKlines empty", findToolByName(t, GetTickFlowToolWrappers(), "TickFlowKlines"),
			[]string{`{}`, `{"symbol": ""}`, `{"stockCode": ""}`, `{"code": ""}`},
			"请提供symbol参数",
		},
		{
			"TickFlowKlinesBatch empty", findToolByName(t, GetTickFlowToolWrappers(), "TickFlowKlinesBatch"),
			[]string{`{}`, `{"symbols": ""}`, `{"stockCodes": ""}`},
			"请提供symbols参数",
		},
		{
			"TickFlowUniverse empty", findToolByName(t, GetTickFlowToolWrappers(), "TickFlowUniverse"),
			[]string{`{}`, `{"name": ""}`},
			"请提供name参数（标的池名称）",
		},
		{
			"TickFlowInstrument empty", findToolByName(t, GetTickFlowToolWrappers(), "TickFlowInstrument"),
			[]string{`{}`, `{"symbol": ""}`, `{"stock_code": ""}`},
			"请提供symbol参数",
		},
		{
			"TickFlowRealtimeQuotes empty", findToolByName(t, GetTickFlowToolWrappers(), "TickFlowRealtimeQuotes"),
			[]string{`{}`, `{"symbols": ""}`},
			"请提供symbols参数",
		},
		{
			"GetStockThesis empty", GetStockThesisTool(),
			[]string{`{}`, `{"stock_code": ""}`, `{"code": ""}`, `{"symbol": ""}`},
			"❌ 股票代码不能为空",
		},
		{
			"SaveStockThesis empty", SaveStockThesisTool(),
			[]string{`{}`, `{"stockCode": ""}`, `{"code": ""}`},
			"❌ 股票代码不能为空",
		},
		{
			"ScreenByStrategy empty", GetStrategyScreenTool(),
			[]string{`{}`, `{"strategyName": ""}`, `{"strategy": ""}`, `{"name": ""}`},
			"请指定策略名称 strategyName",
		},
		{
			"QueryStockCodeInfo empty", GetQueryStockCodeInfoTool(),
			[]string{`{}`, `{"searchWord": ""}`, `{"keyword": ""}`, `{"code": ""}`},
			"未找到股票信息",
		},
		{
			"FetchWebPage empty", GetFetchWebPageTool(),
			[]string{`{}`, `{"url": ""}`, `{"link": ""}`, `{"href": ""}`},
			"URL不能为空",
		},
		{
			"WebSearch empty", GetWebSearchTool(),
			[]string{`{}`, `{"query": ""}`, `{"keyword": ""}`, `{"q": ""}`},
			"搜索关键词不能为空",
		},
		{
			"ExecutePaperTrade empty code", ExecutePaperTradeTool(),
			[]string{`{"side": "BUY"}`, `{"side": "BUY", "stock_code": ""}`, `{"side": "BUY", "code": ""}`},
			"❌ 股票代码不能为空",
		},
		{
			"GetIndustrySupplyGraph empty", GetIndustrySupplyGraphTool(),
			[]string{`{}`, `{"keyword": ""}`, `{"query": ""}`},
			"❌ 请提供查询关键词",
		},
		{
			"GetMCPServerDetail empty id", findToolByName(t, GetMCPServerTools(), "GetMCPServerDetail"),
			[]string{`{}`, `{"id": 0}`, `{"id": "0"}`, `{"serverId": 0}`},
			"请提供有效的MCP服务器ID",
		},
		{
			"DeleteMCPServer empty id", findToolByName(t, GetMCPServerTools(), "DeleteMCPServer"),
			[]string{`{}`, `{"id": 0}`},
			"请提供有效的MCP服务器ID",
		},
		{
			"TestMCPServer empty id", findToolByName(t, GetMCPServerTools(), "TestMCPServer"),
			[]string{`{}`, `{"id": 0}`},
			"请提供有效的MCP服务器ID",
		},
		{
			"GetMCPToolDetail empty id", findToolByName(t, GetMCPServerTools(), "GetMCPToolDetail"),
			[]string{`{}`, `{"toolId": 0}`},
			"请提供有效的MCP工具ID",
		},
		{
			"ListMCPServers empty db", findToolByName(t, GetMCPServerTools(), "ListMCPServers"),
			[]string{`{}`, `{"page": 1, "pageSize": 10}`},
			"未找到符合条件的MCP服务器",
		},
		{
			"ListMCPServerTools empty db", findToolByName(t, GetMCPServerTools(), "ListMCPServerTools"),
			[]string{`{}`},
			"未找到MCP工具",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, args := range tc.args {
				if got := handleEmpty(t, tc.tool, args); got != tc.want {
					t.Errorf("%s Handle(%s) = %q, want %q", tc.tool.Name(), args, got, tc.want)
				}
			}
		})
	}
}

// TestToolsParams_ProjectToolsAliases 验证项目工具的别名正向解析：
// 这些路径只做本地文件/命令操作，不触网络与外部 AI。
func TestToolsParams_ProjectToolsAliases(t *testing.T) {
	t.Run("ReadProjectFile filePath alias reads local file", func(t *testing.T) {
		out := handleEmpty(t, GetReadProjectFileTool(), `{"filePath": "backtest_tools.go"}`)
		if !strings.HasPrefix(out, "```\npackage tools") {
			t.Errorf("ReadProjectFile filePath alias: got %q, want code fence + package tools", truncateForTest(out, 80))
		}
	})
	t.Run("ReadProjectFile filepath alias missing file", func(t *testing.T) {
		out := handleEmpty(t, GetReadProjectFileTool(), `{"filepath": "no-such-file-xyz.go"}`)
		if out != "文件不存在: no-such-file-xyz.go" {
			t.Errorf("ReadProjectFile filepath alias: got %q", out)
		}
	})
	t.Run("SearchProjectFiles glob alias", func(t *testing.T) {
		out := handleEmpty(t, GetSearchProjectFilesTool(), `{"glob": "backtest_tools.go"}`)
		if !strings.Contains(out, "找到 1 个匹配文件") {
			t.Errorf("SearchProjectFiles glob alias: got %q", out)
		}
	})
	t.Run("SearchProjectFiles no match", func(t *testing.T) {
		out := handleEmpty(t, GetSearchProjectFilesTool(), `{"pattern": "no-such-*.go"}`)
		if !strings.Contains(out, "找到 0 个匹配文件") {
			t.Errorf("SearchProjectFiles no match: got %q", out)
		}
	})
	t.Run("GitOperation command alias runs git status", func(t *testing.T) {
		out := handleEmpty(t, GetGitOperationTool(), `{"command": "status"}`)
		if !strings.Contains(out, "On branch") {
			t.Errorf("GitOperation command alias: got %q", truncateForTest(out, 120))
		}
	})
	t.Run("GitOperation unknown op fails in git not in parsing", func(t *testing.T) {
		out := handleEmpty(t, GetGitOperationTool(), `{"op": "not-a-git-subcommand"}`)
		if !strings.HasPrefix(out, "git 操作失败: ") {
			t.Errorf("GitOperation unknown op: got %q", truncateForTest(out, 120))
		}
	})
	t.Run("RunCommand cmd alias hits whitelist", func(t *testing.T) {
		out := handleEmpty(t, GetRunCommandTool(), `{"cmd": "bogus-cmd"}`)
		if !strings.Contains(out, "不在白名单中") {
			t.Errorf("RunCommand cmd alias: got %q", out)
		}
	})
	t.Run("LearnProject url alias reaches handler check", func(t *testing.T) {
		out := handleEmpty(t, GetLearnProjectTool(), `{"url": "https://example.com/x.git"}`)
		if out != "学习功能尚未初始化，请重启应用" {
			t.Errorf("LearnProject url alias: got %q, want handler-not-initialized (proves source was parsed)", out)
		}
	})
	t.Run("ListProjectDir path alias", func(t *testing.T) {
		out := handleEmpty(t, GetListProjectDirTool(), `{"path": "."}`)
		if !strings.Contains(out, "项目目录结构: .") {
			t.Errorf("ListProjectDir: got %q", truncateForTest(out, 120))
		}
	})
}

// TestToolsParams_MCPServerCreateDeleteAliases 验证 MCP 创建/删除的
// 别名正向解析（link→url、serverId→id），写临时库、测完即删。
func TestToolsParams_MCPServerCreateDeleteAliases(t *testing.T) {
	create := findToolByName(t, GetMCPServerTools(), "CreateMCPServer")
	out := handleEmpty(t, create, `{"name": "params-test-server", "link": "http://127.0.0.1:9/x"}`)
	var id uint
	if _, err := fmt.Sscanf(out, "MCP服务器创建成功，ID: %d", &id); err != nil || id == 0 {
		t.Fatalf("CreateMCPServer link alias: got %q, want success with ID", out)
	}
	t.Cleanup(func() { _ = data.NewMCPServerApi().Delete(id) })

	del := findToolByName(t, GetMCPServerTools(), "DeleteMCPServer")
	delOut := handleEmpty(t, del, fmt.Sprintf(`{"serverId": %d}`, id))
	if !strings.Contains(delOut, "已删除") {
		t.Fatalf("DeleteMCPServer serverId alias: got %q", delOut)
	}
	if _, err := data.NewMCPServerApi().GetByID(id); err == nil {
		t.Errorf("DeleteMCPServer serverId alias: row %d still exists after delete", id)
	}
}

func truncateForTest(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
