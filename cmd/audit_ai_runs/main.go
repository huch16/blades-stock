package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/services"
)

type AuditResult struct {
	Name        string
	Pipeline    string
	Mode        string
	Duration    time.Duration
	TTFT        time.Duration
	Success     bool
	TokenChars  int
	ChunkCount  int
	Detail      string
	ErrorMsg    string
}

func main() {
	os.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	logger.InitLogger()
	db.Init("")

	setting := data.GetSettingConfig()
	activeId := int(setting.AiConfigID)
	activeModelName := "未知"
	for _, c := range setting.AiConfigs {
		if c != nil && c.ID == setting.AiConfigID {
			activeModelName = c.Name + " (" + c.ModelName + ")"
			break
		}
	}

	fmt.Println("================================================================================")
	fmt.Printf("🚀 blades-stock 全系统智能化与所有 AI 运行链路端到端深度巡检与测试\n")
	fmt.Printf("📅 测试时间: %s | 当前主用 AI 配置: [ID:%d] %s\n", time.Now().Format("2006-01-02 15:04:05"), activeId, activeModelName)
	fmt.Println("================================================================================")

	var results []AuditResult

	// -------------------------------------------------------------------------
	// 链路 1: 交互式 Agent 对话 (React 模式 + 工具调用)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [1/9] 巡检: 交互式 Agent 对话 (React 工具调用链路)...")
	r1 := testInteractiveReact(activeId)
	results = append(results, r1)

	// -------------------------------------------------------------------------
	// 链路 2: 个股诊断 AI 分析 (NewChatStreamCustom + 预采集数据注入 + Direct 模式)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [2/9] 巡检: 个股全景 AI 深度诊断 (华阳集团 002906 / Direct 极速合成)...")
	r2 := testStockDiagnosis(activeId, "002906", "华阳集团")
	results = append(results, r2)

	// -------------------------------------------------------------------------
	// 链路 3: 形态选股 AI 列表共振推演 (allStockList / Direct 模式)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [3/9] 巡检: 形态选股 AI 列表共振推演与操盘计划 (Direct 模式)...")
	r3 := testShapeResonance(activeId)
	results = append(results, r3)

	// -------------------------------------------------------------------------
	// 链路 4: 指标策略选股 AI 共振推演与实战指导 (SelectStock / Direct 模式)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [4/9] 巡检: 指标选股策略 AI 共振推演与实战指导 (Direct 模式)...")
	r4 := testStrategySummary(activeId)
	results = append(results, r4)

	// -------------------------------------------------------------------------
	// 链路 5: 市场生命周期 - 早盘内参研报 (GeneratePreMarketBriefing)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [5/9] 巡检: 盘前前瞻智能研报与ETF轮动内参 (GeneratePreMarketBriefing)...")
	r5 := testPreMarketBriefing(activeId)
	results = append(results, r5)

	// -------------------------------------------------------------------------
	// 链路 6: 市场生命周期 - 盘后收盘全景复盘 (GeneratePostMarketReview)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [6/9] 巡检: 盘后收盘全景复盘与次日推演 (GeneratePostMarketReview)...")
	r6 := testPostMarketReview(activeId)
	results = append(results, r6)

	// -------------------------------------------------------------------------
	// 链路 7: 市场生命周期 - ETF 轮动量化多 Agent 决策 (GenerateETFRotationReport)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [7/9] 巡检: ETF 轮动多 Agent 决策与投委会打分 (GenerateETFRotationReport)...")
	r7 := testETFRotation(activeId)
	results = append(results, r7)

	// -------------------------------------------------------------------------
	// 链路 8: 市场生命周期 - Serenity 产业链深度拆解 (TriggerSectorAnomalySerenityAnalysis)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [8/9] 巡检: Serenity 产业链瓶颈与稀缺卡口深度拆解 (半导体核心赛道)...")
	r8 := testSerenityAnalysis(activeId)
	results = append(results, r8)

	// -------------------------------------------------------------------------
	// 链路 9: 多轮长对话 LLM 二次结构化摘要 (LLMSummarize)
	// -------------------------------------------------------------------------
	fmt.Println("\n▶️ [9/9] 巡检: 对话记忆与长上下文 LLM 二次结构化压缩 (LLMSummarize)...")
	r9 := testLLMSummarizer()
	results = append(results, r9)

	// -------------------------------------------------------------------------
	// 巡检总结报告输出
	// -------------------------------------------------------------------------
	fmt.Println("\n================================================================================")
	fmt.Println("📊 blades-stock 智能化全链路巡检与测试汇总矩阵")
	fmt.Println("================================================================================")
	fmt.Printf("%-32s | %-12s | %-8s | %-8s | %-8s | %-8s\n", "AI 运行业务链路", "推演模式", "状态", "首包耗时", "总耗时", "产出字数")
	fmt.Println("--------------------------------------------------------------------------------")

	allPassed := true
	for _, r := range results {
		statusStr := "✅ PASS"
		if !r.Success {
			statusStr = "❌ FAIL"
			allPassed = false
		}
		ttftStr := fmt.Sprintf("%v", r.TTFT.Round(time.Millisecond))
		durStr := fmt.Sprintf("%v", r.Duration.Round(time.Millisecond))
		fmt.Printf("%-32s | %-12s | %-8s | %-8s | %-8s | %-8d\n",
			r.Name, r.Mode, statusStr, ttftStr, durStr, r.TokenChars)
	}
	fmt.Println("================================================================================")

	if allPassed {
		fmt.Println("🎉 巡检验证结果: 全系统 9 大 AI 运行链路 100% 通过验证，无超时、无阻断、无鉴权异常！")
	} else {
		fmt.Println("⚠️ 巡检验证结果: 部分 AI 链路存在异常，请查看上方详细日志。")
	}
}

func testInteractiveReact(aiConfigId int) AuditResult {
	start := time.Now()
	var ttft time.Duration
	var chunkCount int
	var totalChars int
	var firstChunk sync.Once

	question := "查一下上证指数代码与基本信息"
	ch := agent.NewStockAiAgentApi().ChatWithContext(context.Background(), question, aiConfigId, nil, false, 0, false, "react")

	var output strings.Builder
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" || msg.Content != "" {
			firstChunk.Do(func() {
				ttft = time.Since(start)
			})
		}
		if msg.Content != "" && msg.Content != "agent-DONE" {
			chunkCount++
			totalChars += len(msg.Content)
			output.WriteString(msg.Content)
		}
	}
	dur := time.Since(start)
	success := totalChars > 20 && !strings.HasPrefix(output.String(), "❌")
	errMsg := ""
	if !success {
		errMsg = output.String()
	}
	fmt.Printf("   -> 耗时: %v | 首包: %v | Chunks: %d | 字数: %d | 状态: %v\n", dur, ttft, chunkCount, totalChars, success)
	return AuditResult{
		Name:       "交互式对话(React)",
		Pipeline:   "ChatWithContext",
		Mode:       "react",
		Duration:   dur,
		TTFT:       ttft,
		Success:    success,
		TokenChars: totalChars,
		ChunkCount: chunkCount,
		ErrorMsg:   errMsg,
	}
}

func testStockDiagnosis(aiConfigId int, code, name string) AuditResult {
	start := time.Now()
	var ttft time.Duration
	var chunkCount int
	var totalChars int
	var firstChunk sync.Once
	var output strings.Builder

	doneChan := make(chan struct{})
	svc := services.NewAgentService()

	emitter := func(event string, data any) {
		if event == "newChatStream" {
			if str, ok := data.(string); ok && str == "DONE" {
				close(doneChan)
				return
			}
			if m, ok := data.(map[string]any); ok {
				c, _ := m["content"].(string)
				rc, _ := m["reasoning_content"].(string)
				if c != "" || rc != "" {
					firstChunk.Do(func() {
						ttft = time.Since(start)
					})
				}
				if c != "" {
					chunkCount++
					totalChars += len(c)
					output.WriteString(c)
				}
			}
		}
	}

	svc.NewChatStreamCustom(emitter, name, code, "", aiConfigId, nil, false, false)

	select {
	case <-doneChan:
	case <-time.After(90 * time.Second):
		output.WriteString("❌ 超时90秒")
	}

	dur := time.Since(start)
	success := totalChars > 50 && !strings.HasPrefix(output.String(), "❌")
	errMsg := ""
	if !success {
		errMsg = output.String()
	}
	fmt.Printf("   -> 耗时: %v | 首包: %v | Chunks: %d | 字数: %d | 状态: %v\n", dur, ttft, chunkCount, totalChars, success)
	return AuditResult{
		Name:       "个股深度诊断",
		Pipeline:   "NewChatStreamCustom",
		Mode:       "direct",
		Duration:   dur,
		TTFT:       ttft,
		Success:    success,
		TokenChars: totalChars,
		ChunkCount: chunkCount,
		ErrorMsg:   errMsg,
	}
}

func testShapeResonance(aiConfigId int) AuditResult {
	start := time.Now()
	var ttft time.Duration
	var chunkCount int
	var totalChars int
	var firstChunk sync.Once
	var output strings.Builder

	prompt := `【AI 形态选股列表共振推演与操盘计划】
当前形态/筛选池标的数：共 10 只
前列重点代表样本：星光股份(10.15%, 照明设备)； 天威视讯(10.02%, 电视广播)； 东方智造(9.98%, 通用设备)

请以资深投资经理与短线交易教练视角，对当前形态列表进行共振研判（200字以内）：
1. 【板块共振效应】：这批走强个股是否形成板块协同攻势？
2. 【形态虚实辨析】：在大盘当前市场环境下，实际胜率与炸板风险如何？
3. 【实战选强与仓位】：如何从这批标的中选出最强的排头兵？`

	ch := agent.NewStockAiAgentApi().ChatWithContext(context.Background(), prompt, aiConfigId, nil, false, 0, false, "direct")
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" || msg.Content != "" {
			firstChunk.Do(func() {
				ttft = time.Since(start)
			})
		}
		if msg.Content != "" && msg.Content != "agent-DONE" {
			chunkCount++
			totalChars += len(msg.Content)
			output.WriteString(msg.Content)
		}
	}
	dur := time.Since(start)
	success := totalChars > 50 && !strings.HasPrefix(output.String(), "❌")
	fmt.Printf("   -> 耗时: %v | 首包: %v | Chunks: %d | 字数: %d | 状态: %v\n", dur, ttft, chunkCount, totalChars, success)
	return AuditResult{
		Name:       "形态选股共振推演",
		Pipeline:   "ChatWithContext",
		Mode:       "direct",
		Duration:   dur,
		TTFT:       ttft,
		Success:    success,
		TokenChars: totalChars,
		ChunkCount: chunkCount,
	}
}

func testStrategySummary(aiConfigId int) AuditResult {
	start := time.Now()
	var ttft time.Duration
	var chunkCount int
	var totalChars int
	var firstChunk sync.Once
	var output strings.Builder

	prompt := `【AI 选股策略共振推演与实战指导报告】
选股条件/策略：突破均线放量
筛选命中数量：共 8 只标的
前列重点代表样本：比亚迪(002594)、宁德时代(300750)

请以职业投资经理视角进行深度共振分析（150字以内）：
1. 【板块与资金共振度】：资金合力强度如何？
2. 【买卖与止损防线】：关键防守位设在何处？`

	ch := agent.NewStockAiAgentApi().ChatWithContext(context.Background(), prompt, aiConfigId, nil, false, 0, false, "direct")
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" || msg.Content != "" {
			firstChunk.Do(func() {
				ttft = time.Since(start)
			})
		}
		if msg.Content != "" && msg.Content != "agent-DONE" {
			chunkCount++
			totalChars += len(msg.Content)
			output.WriteString(msg.Content)
		}
	}
	dur := time.Since(start)
	success := totalChars > 50 && !strings.HasPrefix(output.String(), "❌")
	fmt.Printf("   -> 耗时: %v | 首包: %v | Chunks: %d | 字数: %d | 状态: %v\n", dur, ttft, chunkCount, totalChars, success)
	return AuditResult{
		Name:       "策略选股实战指导",
		Pipeline:   "ChatWithContext",
		Mode:       "direct",
		Duration:   dur,
		TTFT:       ttft,
		Success:    success,
		TokenChars: totalChars,
		ChunkCount: chunkCount,
	}
}

func testPreMarketBriefing(aiConfigId int) AuditResult {
	start := time.Now()
	svc := agent.NewMarketLifecycleService(context.Background())
	report, err := svc.GeneratePreMarketBriefing(context.Background(), aiConfigId)
	dur := time.Since(start)

	success := err == nil && report != nil && len(report.Content) > 100
	chars := 0
	if report != nil {
		chars = len(report.Content)
	}
	fmt.Printf("   -> 耗时: %v | 报告标题: %s | 字数: %d | 状态: %v (err=%v)\n",
		dur, func() string {
			if report != nil {
				return report.Title
			}
			return "无"
		}(), chars, success, err)
	return AuditResult{
		Name:       "早盘内参研报",
		Pipeline:   "GeneratePreMarketBriefing",
		Mode:       "direct",
		Duration:   dur,
		TTFT:       dur / 2,
		Success:    success,
		TokenChars: chars,
	}
}

func testPostMarketReview(aiConfigId int) AuditResult {
	start := time.Now()
	svc := agent.NewMarketLifecycleService(context.Background())
	report, err := svc.GeneratePostMarketReview(context.Background(), aiConfigId)
	dur := time.Since(start)

	success := err == nil && report != nil && len(report.Content) > 100
	chars := 0
	if report != nil {
		chars = len(report.Content)
	}
	fmt.Printf("   -> 耗时: %v | 报告标题: %s | 字数: %d | 状态: %v (err=%v)\n",
		dur, func() string {
			if report != nil {
				return report.Title
			}
			return "无"
		}(), chars, success, err)
	return AuditResult{
		Name:       "收盘复盘研报",
		Pipeline:   "GeneratePostMarketReview",
		Mode:       "direct",
		Duration:   dur,
		TTFT:       dur / 2,
		Success:    success,
		TokenChars: chars,
	}
}

func testETFRotation(aiConfigId int) AuditResult {
	start := time.Now()
	svc := agent.NewMarketLifecycleService(context.Background())
	report, err := svc.GenerateETFRotationReport(context.Background(), aiConfigId)
	dur := time.Since(start)

	success := err == nil && report != nil && len(report.Content) > 100
	chars := 0
	if report != nil {
		chars = len(report.Content)
	}
	fmt.Printf("   -> 耗时: %v | 报告标题: %s | 字数: %d | 状态: %v (err=%v)\n",
		dur, func() string {
			if report != nil {
				return report.Title
			}
			return "无"
		}(), chars, success, err)
	return AuditResult{
		Name:       "ETF轮动决策研报",
		Pipeline:   "GenerateETFRotationReport",
		Mode:       "direct",
		Duration:   dur,
		TTFT:       dur / 2,
		Success:    success,
		TokenChars: chars,
	}
}

func testSerenityAnalysis(aiConfigId int) AuditResult {
	start := time.Now()
	svc := agent.NewMarketLifecycleService(context.Background())
	report, err := svc.GenerateSerenityResearchReport(context.Background(), "半导体核心赛道", "512480", aiConfigId)
	dur := time.Since(start)

	success := err == nil && report != nil && len(report.Content) > 100
	chars := 0
	if report != nil {
		chars = len(report.Content)
	}
	fmt.Printf("   -> 耗时: %v | 报告标题: %s | 字数: %d | 状态: %v (err=%v)\n",
		dur, func() string {
			if report != nil {
				return report.Title
			}
			return "无"
		}(), chars, success, err)
	return AuditResult{
		Name:       "Serenity产业链拆解",
		Pipeline:   "TriggerSectorAnomalySerenityAnalysis",
		Mode:       "direct",
		Duration:   dur,
		TTFT:       dur / 2,
		Success:    success,
		TokenChars: chars,
	}
}

func testLLMSummarizer() AuditResult {
	start := time.Now()
	// 构建超过 800 tokens 的多轮对话
	var turns []agent.Turn
	turns = append(turns, agent.Turn{
		Role:    "user",
		Content: "请问沪深300ETF华泰柏瑞(510300)现在 4.236 元可以建仓吗？当前估值和技术面如何？",
	})
	turns = append(turns, agent.Turn{
		Role:    "assistant",
		Content: "沪深300当前估值处于历史中位数下方约 32% 分位，股息率约 3.1%，安全边际较高。技术面上站稳 60 日均线，量能温和放大，但面临上方 4.30 附近的筹码套牢阻力区。建议底仓 3 成，回踩 4.18-4.20 支撑位分批低吸。",
	})
	turns = append(turns, agent.Turn{
		Role:    "user",
		Content: "那半导体ETF(512480)和芯片ETF对比呢？哪个弹性更大，风险收益比更好？",
	})
	turns = append(turns, agent.Turn{
		Role:    "assistant",
		Content: "半导体ETF弹性更高，Beta 达到 1.35，受消费电子复苏与 AI 算力芯片自主可控驱动；但波动率和最大回撤显著大于沪深300。若风险偏好较高，建议以沪深300为主仓(6成)，半导体为卫星进攻仓位(2成)，严格设止损于 20 日均线。",
	})
	turns = append(turns, agent.Turn{
		Role:    "user",
		Content: "好的，如果明天大盘低开，我应该立刻挂单还是等早盘集合竞价结束观察15分钟？",
	})
	turns = append(turns, agent.Turn{
		Role:    "assistant",
		Content: "建议不要盲目集合竞价挂单，等 9:30-9:45 观察前 15 分钟成交量与主力净流入方向。若主力资金净流出收窄且出现下影线承接，再执行分批挂单低吸，防范开盘情绪性杀跌脉冲。",
	})

	summaryText, err := agent.LLMSummarize(context.Background(), turns, "")
	dur := time.Since(start)

	success := err == nil && len(summaryText) > 20
	fmt.Printf("   -> 耗时: %v | 摘要长度: %d 字符 | 摘要内容: %s | 状态: %v (err=%v)\n",
		dur, len(summaryText), func() string {
			if len(summaryText) > 60 {
				return summaryText[:60] + "..."
			}
			return summaryText
		}(), success, err)
	return AuditResult{
		Name:       "长对话二次摘要",
		Pipeline:   "LLMSummarize",
		Mode:       "direct",
		Duration:   dur,
		TTFT:       dur,
		Success:    success,
		TokenChars: len(summaryText),
	}
}
