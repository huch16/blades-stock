// Command blades_mcp_server 将 blades-stock 的数据工具暴露为 MCP（Model Context
// Protocol）服务端，供外部客户端（如本机 opencode）通过 stdio 或 HTTP 调用。
//
// 默认以 stdio 模式运行（适合 opencode / Claude Desktop 本地 MCP 配置）；
// 通过 -http :8090 可切换为 Streamable HTTP 模式，配合 -token 设置鉴权。
//
// 注意：MCP stdio 协议走 stdout，运行时请设置 BLADES_STOCK_LOG_CONSOLE=0，
// 避免日志污染 stdout（backend/logger 已支持该环境变量）。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"blades-stock/backend/agent"
	"blades-stock/backend/agent/tools"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	httpAddr = flag.String("http", "", "可选：Streamable HTTP 模式监听地址（如 :8090），留空则使用 stdio 模式")
	token    = flag.String("token", "", "可选：HTTP 模式 Bearer 鉴权 token")
)

func main() {
	flag.Parse()

	// 复用 paths.BaseDir() 定位权威库 data/stock.db
	db.Init("")

	srv := server.NewMCPServer(
		"blades-stock", "0.1.0",
		server.WithInstructions("blades-stock 数据工具服务：提供 A 股行情、K线、财务、资金流、热点、涨停池、新闻资讯、AI 推荐与市场总结等能力。"),
	)

	converted := convertTools(collectTools())
	converted = append(converted, newAiMarketSummaryTool())
	for _, st := range converted {
		srv.AddTool(st.Tool, st.Handler)
	}
	logger.SugaredLogger.Infof("blades mcp server: registered %d tools", len(converted))

	if *httpAddr != "" {
		serveHTTP(srv)
		return
	}

	// stdio 模式：日志必须保持 stdout 纯净（BLADES_STOCK_LOG_CONSOLE=0）
	if err := server.ServeStdio(srv); err != nil {
		logger.SugaredLogger.Errorf("stdio mcp server error: %v", err)
		os.Exit(1)
	}
}

// collectTools 组装本服务暴露的 blades 工具集：基础工具 + 全量数据工具 +
// 节假日/回测/MCP元数据查询工具。刻意排除 GetProjectTools（文件读写/命令执行）
// 与外部 MCP 客户端工具（避免递归连接 Tushare/悟道等外部服务）。
func collectTools() []bladesTools.Tool {
	var all []bladesTools.Tool

	all = append(all, tools.GetQueryStockCodeInfoTool())
	all = append(all, tools.GetQueryStockNewsTool())
	all = append(all, tools.GetQueryBKDictTool())
	all = append(all, tools.GetWebSearchTool())
	all = append(all, tools.GetFetchWebPageTool())

	all = append(all, tools.GetAllDataTools()...)
	all = append(all, tools.GetHolidayTools()...)

	all = append(all, tools.GetMCPServerTools()...)
	all = append(all, tools.GetSkillTools()...)

	all = append(all, tools.GetBacktestStatsTool())
	all = append(all, tools.GetBacktestDetailTool())
	all = append(all, tools.GetRunBacktestTool())
	all = append(all, tools.GetVectorBacktestTool())
	all = append(all, tools.GetVectorGridSearchTool())

	return all
}

// convertTools 将 blades 工具转换为 MCP ServerTool（名称/描述/inputSchema + handler）。
func convertTools(all []bladesTools.Tool) []server.ServerTool {
	seen := make(map[string]bool)
	var out []server.ServerTool

	for _, bt := range all {
		name := bt.Name()
		if name == "" || seen[name] {
			logger.SugaredLogger.Warnf("skip duplicate/empty tool name: %q", name)
			continue
		}
		seen[name] = true

		raw, err := json.Marshal(bt.InputSchema())
		if err != nil || len(raw) == 0 {
			logger.SugaredLogger.Warnf("tool %s: marshal input schema failed: %v, fallback to object", name, err)
			raw = json.RawMessage(`{"type":"object"}`)
		}

		// 注意：不能用 mcp.NewTool + WithRawInputSchema（NewTool 会预置 InputSchema，
		// 与 RawInputSchema 同时存在会触发 errToolSchemaConflict），需直接构造 Tool。
		mt := mcp.Tool{
			Name:           name,
			Description:    bt.Description(),
			RawInputSchema: raw,
		}
		blade := bt
		out = append(out, server.ServerTool{
			Tool: mt,
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return callBladeTool(ctx, blade, req)
			},
		})
	}
	return out
}

// callBladeTool 将 MCP 调用参数转为 blades 工具的 JSON 参数串，并返回文本结果。
// 在调用前进行 JSON Schema 校验，并在调用前后写入审计日志。
func callBladeTool(ctx context.Context, bt bladesTools.Tool, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start := time.Now()
	argsJSON := "{}"
	switch v := req.GetRawArguments().(type) {
	case json.RawMessage:
		if len(v) > 0 && string(v) != "null" {
			argsJSON = string(v)
		}
	case map[string]any:
		if len(v) > 0 {
			b, err := json.Marshal(v)
			if err == nil {
				argsJSON = string(b)
			}
		}
	case nil:
		// keep "{}"
	}

	// Schema validation
	if schema := bt.InputSchema(); schema != nil {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(argsJSON), &data); err != nil {
			// invalid JSON
			entry := models.AuditLog{
				ToolName:    bt.Name(),
				ServerName:  "mcp",
				RequestBody: argsJSON,
				Success:     false,
				ErrorMsg:    fmt.Sprintf("invalid JSON: %v", err),
				DurationMs:  0,
			}
			agent.RecordAuditLogSync(entry)
			return &mcp.CallToolResult{
				Content: []mcp.Content{textContent(fmt.Sprintf("工具参数非法JSON: %v", err))},
				IsError: true,
			}, nil
		}
		resolved, err := schema.Resolve(nil)
		if err != nil {
			entry := models.AuditLog{
				ToolName:    bt.Name(),
				ServerName:  "mcp",
				RequestBody: argsJSON,
				Success:     false,
				ErrorMsg:    err.Error(),
				DurationMs:  0,
			}
			agent.RecordAuditLogSync(entry)
			return &mcp.CallToolResult{
				Content: []mcp.Content{textContent(fmt.Sprintf("工具参数解析失败: %v", err))},
				IsError: true,
			}, nil
		}
		if err := resolved.Validate(data); err != nil {
			entry := models.AuditLog{
				ToolName:    bt.Name(),
				ServerName:  "mcp",
				RequestBody: argsJSON,
				Success:     false,
				ErrorMsg:    err.Error(),
				DurationMs:  0,
			}
			agent.RecordAuditLogSync(entry)
			return &mcp.CallToolResult{
				Content: []mcp.Content{textContent(fmt.Sprintf("工具参数校验失败: %v", err))},
				IsError: true,
			}, nil
		}
	}

	out, err := bt.Handle(ctx, argsJSON)
	duration := time.Since(start).Milliseconds()

	// Audit log for success or failure
	entry := models.AuditLog{
		ToolName:    bt.Name(),
		ServerName:  "mcp",
		RequestBody: argsJSON,
		Response:    out,
		Success:     err == nil,
		ErrorMsg: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
		DurationMs: duration,
	}
	agent.RecordAuditLog(entry) // async

	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{textContent(fmt.Sprintf("工具调用失败: %v", err))},
			IsError: true,
		}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{textContent(out)},
	}, nil
}

// textContent 构造带 Type="text" 的文本内容（Type 缺失会导致客户端解析失败）。
func textContent(s string) mcp.Content {
	return mcp.TextContent{Type: "text", Text: s}
}

// serveHTTP 以 Streamable HTTP 模式启动服务（可选 Bearer token 鉴权）。
func serveHTTP(srv *server.MCPServer) {
	var h http.Handler = server.NewStreamableHTTPServer(srv)
	if *token != "" {
		h = authMiddleware(*token, h)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", h)

	httpSrv := &http.Server{Addr: *httpAddr, Handler: mux}
	go func() {
		logger.SugaredLogger.Infof("blades mcp server (HTTP) listening on %s/mcp", *httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.SugaredLogger.Errorf("http mcp server error: %v", err)
		}
	}()

	waitSignal()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func authMiddleware(t string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+t {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func waitSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}

// summaryMu 防止多个并发 AI 市场总结任务（该任务会调用大模型，耗时较长）。
var summaryMu sync.Mutex

// newAiMarketSummaryTool 暴露"AI市场资讯总结"重工具：复用 app.go SummaryStockNews
// 的默认提示词与 agent 流水线，收集完整文字报告后一次性返回。
func newAiMarketSummaryTool() server.ServerTool {
	mt := mcp.Tool{
		Name:        "AiMarketSummary",
		Description: "对当前A股市场进行全面AI资讯总结与分析：大盘指数、涨跌家数、热点板块、涨停池、消息面、资金面、主线方向与风险关注点。返回完整文字报告，需调用大模型与多个数据工具，耗时约1-5分钟。",
		RawInputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"question": {"type": "string", "description": "自定义分析问题（可选），为空则使用默认市场总结模板"},
				"thinking": {"type": "boolean", "description": "是否开启深度思考（可选，默认false）"}
			}
		}`),
	}

	return server.ServerTool{
		Tool: mt,
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := req.GetString("question", "")
			think := req.GetBool("thinking", false)
			out, err := runAiMarketSummary(ctx, q, think)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{textContent(fmt.Sprintf("AI市场总结失败: %v", err))},
					IsError: true,
				}, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{textContent(out)},
			}, nil
		},
	}
}

func runAiMarketSummary(ctx context.Context, question string, thinking bool) (string, error) {
	if !summaryMu.TryLock() {
		return "", fmt.Errorf("已有AI市场总结任务在执行，请稍后再试")
	}
	defer summaryMu.Unlock()

	// 预采集市场参考数据（与 app.go preFetchMarketSummaryData 一致）
	preFetchCtx := ""
	if api := data.NewMarketNewsApi(); api != nil {
		if s := api.GlobalStockIndexesReadable(30); s != "" {
			preFetchCtx = "\n\n【预采集参考数据】以下为当前市场概况，可供直接引用：\n" + s
		}
	}

	q := strings.TrimSpace(question)
	if q == "" {
		q = marketSummaryDefaultQuestion
	}
	q += preFetchCtx

	// 长任务保护：单次最长 10 分钟，客户端取消（ctx.Done）可提前终止
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	var sb strings.Builder
	for msg := range agent.NewStockAiAgentApi().ChatWithContext(runCtx, q, 0, nil, false, 0, thinking, "plan_execute") {
		if msg == nil {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" || content == "agent-DONE" {
			continue
		}
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("AI总结未产生内容（可能被取消或模型异常）")
	}
	return strings.TrimSpace(sb.String()), nil
}

// marketSummaryDefaultQuestion 与 app.go:2116 的默认市场总结提示词保持一致。
const marketSummaryDefaultQuestion = `请对当前A股市场进行全面的AI资讯总结和分析。

## 执行步骤
### 第1步：获取市场整体数据
- 用 GetMarketData 获取大盘指数行情、涨跌家数、涨跌分布
- 用 GlobalStockIndexesReadable 获取全球主要指数表现
- 用 GetStockChanges 获取今日异动数据（火箭发射、大笔买入等）

### 第2步：捕捉市场热点
- 用 GetUplimitHotPlates 获取今日涨停板块热度排名
- 用 HotStockTable 获取热门股票排名
- 用 GetUplimitHotStocks 获取涨停个股列表
- 用 HotspotDiscovery 发现当前市场热点题材

### 第3步：消息面分析
- 用 GetNewsListData 获取当天重要财经新闻
- 用 GetWallstreetcnLives 获取华尔街见闻7x24快讯
- 用 WebSearch 搜索最新的重要政策/行业消息

### 第4步：资金面分析
- 用 GetStockMoneyData 查看个股资金流入排名
- 用 GetIndustryMoneyRank 查看行业资金流向
- 用 GetMutualTop10Deal 查看北向资金动向

### 第5步：综合分析
- 当前市场整体趋势判断（多头/空头/震荡）
- 主线热点板块和持续性分析
- 资金偏好方向（大盘/小盘，价值/成长）
- 潜在风险和关注点
- 建议关注的方向/板块/个股

注意：每一步都必须调用对应的工具获取最新数据，严禁凭记忆编造。即使某些工具调用失败，也要基于已成功获取的数据进行分析，不要反复追问用户补充信息。`
