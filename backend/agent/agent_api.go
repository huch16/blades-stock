package agent

import (
	"blades-stock/backend/agent/tools"
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/blades"
	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/samber/lo"
)

// AgentRunMetrics 记录单次 Agent 执行的可观测性指标
type AgentRunMetrics struct {
	TotalDuration     time.Duration `json:"total_duration"`
	FirstChunkLatency time.Duration `json:"first_chunk_latency"`
	ToolCallCount     int           `json:"tool_call_count"`
	RetryCount        int           `json:"retry_count"`
	FinalStatus       string        `json:"final_status"` // success, retry_success, failed
}

// MarshalJSON 确保 time.Duration 序列化为前端易于消费的毫秒数 (ms)，彻底根除纳秒引发的“5.8年/75天”幽灵耗时惨案
func (m AgentRunMetrics) MarshalJSON() ([]byte, error) {
	type Alias AgentRunMetrics
	return json.Marshal(&struct {
		TotalDuration     int64 `json:"total_duration"`
		FirstChunkLatency int64 `json:"first_chunk_latency"`
		Alias
	}{
		TotalDuration:     m.TotalDuration.Milliseconds(),
		FirstChunkLatency: m.FirstChunkLatency.Milliseconds(),
		Alias:             (Alias)(m),
	})
}

type StreamMessage struct {
	Role             string              `json:"role"`
	Content          string              `json:"content"`
	ReasoningContent string              `json:"reasoning_content"`
	Actions          []types.AgentAction `json:"actions,omitempty"`
	Metrics          *AgentRunMetrics    `json:"metrics,omitempty"`
	ModelName        string              `json:"model_name,omitempty"`
	FinishReason     string              `json:"finish_reason,omitempty"`
	StatusText       string              `json:"status_text,omitempty"`
}

type StockAiAgent struct {
	instance   *AgentInstance
	sessionID  string
	aiConfigId int
	// resolution 本次实际使用的模型解析结果（写运行记录用）
	resolution data.ModelResolution
}

func NewStockAiAgentApi() *StockAiAgent {
	return &StockAiAgent{}
}

// newStockAiAgentResolved 用**已经解析好的**模型创建 Agent。
//
// 与旧实现的区别（同一类问题的根治）：
//
//	· 模型选择收敛到 data.ResolveModel 唯一入口，不再各链路各写一套；
//	· 传入的是配置**副本**，请求级 thinking 覆盖 WithThinking 作用在副本上，
//	  绝不会写回存储（旧代码 `aiConfig.Thinking = thinkingMode` 会污染配置）；
//	· 解析依据（Source/Reason/FallbackUsed）随返回值上抛，写进运行记录与日志。
func (receiver StockAiAgent) newStockAiAgentResolved(ctx *context.Context, res data.ModelResolution, thinkingMode bool, question string, agentMode string, matchedSkills []models.Skill, extraTools ...bladesTools.Tool) *StockAiAgent {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in newStockAiAgentResolved: %v", r)
		}
	}()

	if !res.Resolved() {
		logger.SugaredLogger.Errorf("no ai config available")
		return nil
	}

	// 请求级 thinking 只作用于副本
	aiConfig := res.WithThinking(thinkingMode).Config
	logger.SugaredLogger.Infof("【模型解析】%s | source=%s fallback=%v model=%q thinking=%v",
		res.Describe(), res.Source, res.FallbackUsed, aiConfig.ModelName, aiConfig.Thinking)

	sessionID := aiConfig.SessionId
	if sessionID == "" {
		sessionID = fmt.Sprintf("ai-config-%d", aiConfig.ID)
	}

	agentInstance := GetStockAiAgent(*ctx, aiConfig, question, agentMode, matchedSkills, extraTools...)
	if agentInstance == nil {
		logger.SugaredLogger.Errorf("failed to create agent for config id: %d", aiConfig.ID)
		return nil
	}

	return &StockAiAgent{
		instance:   agentInstance,
		sessionID:  sessionID,
		aiConfigId: int(aiConfig.ID),
		resolution: res,
	}
}

func (receiver StockAiAgent) Chat(question string, aiConfigId int, sysPromptId *int) chan *StreamMessage {
	return receiver.ChatWithContext(context.Background(), question, aiConfigId, sysPromptId, true, 20, false, "")
}

func (receiver StockAiAgent) ChatWithContext(ctx context.Context, question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, optsOverride ...string) chan *StreamMessage {
	ch := make(chan *StreamMessage, 1024)

	go func() {
		// ── 运行记录（可观测性）─────────────────────────────────────
		//
		// 用户反复遇到「到底哪个模型在跑」：界面显示的是**请求的**模型，而实际可能
		// 因“指定卡片已被删除 / 采用全局默认 / 回落第一张 / thinking 被请求覆盖”
		// 而在用另一个。这里把事实固化成一条可查询的记录（含解析依据与备用链）。
		//
		// 先声明再用：后面的错误分支与终态护栏都会往这个指针里填结果，
		// 而下面的 defer 最后注册、最后执行，能读到最终值。
		runStart := time.Now()
		// 运行类型：显式标注优先；pipeline 模式即使调用方未标注 ctx 也归为流水线
		runKind := RunKindFrom(ctx)
		if runKind == RunKindChat && AgentMode(agentMode) == AgentModePipeline {
			runKind = RunKindPipeline
		}
		runRecord := &models.AgentRunRecord{
			Kind:        runKind,
			RequestedID: aiConfigId,
		}
		// modelFinishReason 记录模型**真实**的结束原因（stop / length /
		// content_filter）。blades 的 openai provider 会把上游
		// choice.FinishReason 写到 Message 上并随 final response yield 出来。
		//
		// 为什么必须收集它（用户实证「盘后分析等经常运行一段，没有全部完成」）：
		// 旧实现从不读取该字段，终态一律硬编码 FinishReason="stop"。于是模型因
		// maxTokens 耗尽被截断（finish_reason=length）时，前后端都认为「正常
		// 结束」，半份研报被当成完整研报保存并展示 —— 用户看到的就是
		// 「生成到一半就停了，且没有任何说明」。
		// 声明必须早于下面的终态兜底 defer（它会读取该变量）。
		var modelFinishReason string
		// finalizeRun 幂等落库。
		//
		// 关键：必须在 sendTerminal **关闭通道之前**调用 ——
		// 否则消费者（如市场研报的 for msg := range ch）会在记录写入前就返回，
		// 出现“刚跑完却查不到记录”的竞态。
		runRecordDone := false
		finalizeRun := func() {
			if runRecordDone {
				return
			}
			runRecordDone = true
			runRecord.DurationMs = time.Since(runStart).Milliseconds()
			runRecord.Success = runRecord.ErrorMsg == "" && runRecord.FinishReason != ""
			if !runRecord.Success && runRecord.ErrorMsg == "" {
				runRecord.ErrorMsg = "运行未产生终态（可能被取消或内部异常）"
			}
			// 纯观测，绝不影响业务；RecordAgentRun 内部失败只记日志
			data.RecordAgentRun(runRecord)
		}
		// 兜底：任何未走到 sendTerminal 的路径（含 panic）也留下记录
		defer finalizeRun()

		// ── 终态收尾护栏（P0）─────────────────────────────────────
		//
		// 前端只在收到带 FinishReason 的终态消息时才 finalizeStream()。
		// 历史缺陷：本函数有 5 处 close(ch)，其中只有 1 处发送了 FinishReason，
		// 错误路径甚至**故意**只 close 不发终态（旧注释："just close"）。
		// 后果：只要 AI 调用失败（限流/超时/thinking 与工具调用不兼容/网关 429）
		// 或提前返回，前端 isStreaming 就永远为真 —— 界面卡在“思考中…”、
		// 输入框持续禁用、停止按钮一直转，用户只能刷新页面。
		//
		// 现在用 defer 保证：无论从哪条路径返回（正常/错误/提前返回/panic），
		// 都**一定且只会**发送一条终态消息，然后才关闭通道。
		terminalSent := false
		sendTerminal := func(m *StreamMessage) {
			if terminalSent {
				return
			}
			terminalSent = true
			if m != nil {
				runRecord.FinishReason = m.FinishReason
			}
			// 先落运行记录，再关闭通道：保证消费者拿到 close 时记录已可查询
			finalizeRun()
			// 带超时的发送：消费者若已退出也不会让本 goroutine 永久泄漏
			select {
			case ch <- m:
			case <-time.After(3 * time.Second):
				logger.SugaredLogger.Warnf("终态消息发送超时，消费者可能已退出")
			}
			close(ch)
		}
		defer func() {
			if terminalSent {
				return
			}
			// 兜底：任何忘了收尾的路径（含 panic）都在此补齐终态，杜绝前端卡死。
			// 用模型**真实**的结束原因，而不是一律写 "stop"：
			// 若模型是 length（maxTokens 耗尽）或 content_filter，
			// 前端需要据此提示「内容不完整」，而不是当成正常结束。
			fr := strings.TrimSpace(modelFinishReason)
			if fr == "" {
				fr = "stop"
			}
			sendTerminal(&StreamMessage{Role: "assistant", Content: "agent-DONE", FinishReason: fr})
		}()

		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in ChatWithContext: %v", r)
				runRecord.ErrorMsg = fmt.Sprintf("内部错误: %v", r)
				ch <- &StreamMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("❌ 内部错误: %v", r),
				}
				// 不再自行 close：交给终态护栏统一发送终态后关闭
			}
		}()

		var sessionIDOverride string
		var sysPromptOverride string
		if len(optsOverride) > 0 && optsOverride[0] != "" {
			sysPromptOverride = optsOverride[0]
		}
		if len(optsOverride) > 1 && optsOverride[1] != "" {
			sessionIDOverride = optsOverride[1]
		}

		settingConfig := data.GetSettingConfig()
		ctx = WithRunQuota(ctx)

		// 统一入口解析模型（不再各链路各写一套）
		// 把「本次注入了哪些能力」回写到运行记录（D 期可观测性）
		ctx = WithInjectedSink(ctx, &runRecord.ToolsInjected)

		resolved, resolveErr := data.ResolveModel(aiConfigId)
		if resolveErr != nil {
			logger.SugaredLogger.Errorf("模型解析失败: %v", resolveErr)
			runRecord.ErrorMsg = resolveErr.Error()
			ch <- &StreamMessage{
				Role:    "assistant",
				Content: "❌ " + resolveErr.Error(),
			}
			return
		}
		runRecord.ResolvedID = resolved.ResolvedID
		runRecord.Source = resolved.Source
		runRecord.FallbackUsed = resolved.FallbackUsed
		runRecord.Resolution = resolved.Describe()
		runRecord.ModelName = strings.TrimSpace(resolved.Config.ModelName)
		runRecord.BaseURL = strings.TrimSpace(resolved.Config.BaseUrl)
		runRecord.Thinking = resolved.Config.Thinking
		if resolved.FallbackUsed {
			logger.SugaredLogger.Warnf("【模型解析】发生回落: %s", resolved.Describe())
		}

		// 备用链（本 run 实际挂载了哪些备用模型）
		if _, fbNames := BuildFallbackProvidersFromDB(ctx, resolved.Config); len(fbNames) > 0 {
			runRecord.FallbackChain = strings.Join(fbNames, " -> ")
		}

		originalAiConfig := &resolved.Config
		embedService := ResolveEmbeddingService(settingConfig, originalAiConfig)

		// Match skills once, then reuse both for the skill prompt and for
		// force-enabling each skill's required tool groups in the agent.
		matchedSkills := matchSkills(question, embedService)
		if len(matchedSkills) > 0 {
			names := make([]string, 0, len(matchedSkills))
			for _, s := range matchedSkills {
				names = append(names, s.Name)
			}
			logger.SugaredLogger.Infof("skills matched: %v", names)
		}
		skillPrompt := BuildShortSkillPrompt(matchedSkills)

		// 零 Token 确定性持仓风控扫描拦截：
		// 严格践行零 Token 确定性法则，根治 LLM 幻觉生成虚假均线、虚假量比与反向投资评级。由纯 Go 量化引擎解析并计算！
		if strings.Contains(question, "【盘前持仓风控扫描与防守边界审计】") ||
			strings.Contains(question, "【盘前自选股与持仓综合排雷诊断清单】") ||
			strings.Contains(question, "【盘前自选股与持仓综合排雷诊断】") {
			scanItems := quant.ParseHoldingScanItemsFromPrompt(question)
			if len(scanItems) > 0 {
				ch <- &StreamMessage{
					Role:             "assistant",
					Content:          "",
					ReasoningContent: "[STATUS]🛡️ 正在执行纯 Go 确定性持仓风控与均线量比扫描...",
					StatusText:       "正在执行纯 Go 确定性持仓风控与均线量比扫描...",
				}
				scanRes, err := quant.ScanHoldingRisk(scanItems)
				if err == nil && scanRes != nil && scanRes.MarkdownReport != "" {
					ch <- &StreamMessage{
						Role:    "assistant",
						Content: scanRes.MarkdownReport,
					}
					ch <- &StreamMessage{
						Role:             "assistant",
						Content:          "",
						ReasoningContent: "[DONE]",
						StatusText:       "[DONE]",
					}
					// 异步记录审计日志与持久化对话记录
					go RecordAuditLog(models.AuditLog{
						ToolName:    "ScanHoldingRisk",
						RequestBody: fmt.Sprintf("扫描标的数: %d", len(scanItems)),
						Response:    scanRes.MarkdownReport,
						Success:     true,
					})
					if memoryMode && sessionIDOverride != "" {
						memSvc := NewChatMemoryService(sessionIDOverride, memoryCount)
						_ = memSvc.AddUserMessage(question)
						_ = memSvc.AddAssistantMessage(scanRes.MarkdownReport)
					}
					// 终态由 defer 护栏统一发送（此处直接 return）
					return
				}
			}
		}

		// 直接复用**已解析**的结果创建 Agent，避免二次解析出现不一致
		stockAiAgent := receiver.newStockAiAgentResolved(&ctx, resolved, thinkingMode, question, agentMode, matchedSkills)
		if stockAiAgent == nil {
			logger.SugaredLogger.Errorf("stockAiAgent is nil")
			runRecord.ErrorMsg = "AI 配置不存在或无效"
			ch <- &StreamMessage{
				Role:    "assistant",
				Content: "❌ AI 配置不存在或无效，请检查 AI 配置",
			}
			// 终态由 defer 护栏统一发送（此前只 close 不发终态 → 前端卡在“思考中”）
			return
		}

		if sessionIDOverride != "" {
			stockAiAgent.sessionID = sessionIDOverride
		}

		// Apply timeout from AI config to the context, so the LLM HTTP call
		// respects the configured timeout instead of hanging indefinitely.
		// Default to 300s (5 min) if not configured. Floor at 900s (15 min):
		// multi-step skill analysis (e.g. ETF轮动) with a thinking model plus
		// data-source fallbacks can legitimately exceed 10 minutes, and the
		// frontend streams via the Wails binding without a shorter cap.
		if stockAiAgent.instance != nil {
			timeOutSec := stockAiAgent.instance.AIConfig.TimeOut
			if timeOutSec <= 0 {
				timeOutSec = 300
			}
			if timeOutSec < 900 {
				timeOutSec = 900
			}
			toCtx, cancel := context.WithTimeout(ctx, time.Duration(timeOutSec)*time.Second)
			ctx = toCtx
			defer cancel()
		}

		var memoryService *ChatMemoryService
		var historyMessages []map[string]string
		if memoryMode && stockAiAgent.sessionID != "" {
			memoryService = NewChatMemoryService(stockAiAgent.sessionID, memoryCount)
			if embedService != nil {
				memoryService.SetEmbeddingService(embedService)
			}

			var err error
			historyMessages, err = memoryService.GetHistoryMessages(question)
			if err != nil {
				logger.SugaredLogger.Errorf("failed to get history messages: %v", err)
				historyMessages = nil
			}
		}

		sysPrompt := sysPromptOverride
		if sysPrompt == "" {
			if settingConfig.Prompt != "" {
				sysPrompt = settingConfig.Prompt + "\n\n"
			}
			if skillPrompt != "" {
				sysPrompt += skillPrompt + "\n\n"
			}
			if agentMode == string(AgentModeDirect) || len(stockAiAgent.instance.AllTools) == 0 {
				sysPrompt += "【研报数据使用纪律】：所有一手真实行情、财务指标与量化参考数据已在输入上下文中详尽注入，无需且不得尝试调用任何外部工具。请直接基于上述一手真实数据进行深度量化推演与结构化研报撰写，严禁在正文中生成任何 <tool_call> 等伪造标签。"
			} else {
				sysPrompt += GetPromptTemplates().SystemRules
			}
		}

		// 当日市场状态注入（TTL 缓存，复用 GetMarketData 数据源）：市场相关
		// 问题附带今日大盘概览，让 Agent 有"今天"的真实上下文。
		if brief := getDailyMarketBrief(question); brief != "" {
			sysPrompt += "\n\n" + brief
		}

		// 标的历史买方认知档案 (Stock Thesis Ledger) 前置注入 (实现认知连续演化，严禁割裂历史立论)
		reCode := regexp.MustCompile(`\b(00\d{4}|30\d{4}|60\d{4}|68\d{4}|51\d{4}|58\d{4}|15\d{4})\b`)
		targetStockCode := reCode.FindString(question)
		// 若当前提问未显式包含股票代码 (例如连续多轮追问 "它如果跌破止损位怎么办"), 逆序回溯历史消息查找最近提及的股票代码
		if targetStockCode == "" && len(historyMessages) > 0 {
			for i := len(historyMessages) - 1; i >= 0; i-- {
				if hmContent := historyMessages[i]["content"]; hmContent != "" {
					if hmCode := reCode.FindString(hmContent); hmCode != "" {
						targetStockCode = hmCode
						break
					}
				}
			}
		}

		if targetStockCode != "" {
			thesisApi := data.NewStockThesisApi()
			if thesis, err := thesisApi.GetStockThesis(targetStockCode); err == nil && thesis != nil && thesis.ID > 0 {
				sysPrompt += fmt.Sprintf("\n\n【标的历史买方认知档案（供连续性演化参考）】：\n标的代码: %s (%s)\n前期认知判定: 【%s】\n核心假说: %s\n重点跟踪催化: %s\n证伪条件/防守位: %s\n请结合最新量化事实与盘口资金，重点论证该买方逻辑是维持、强化还是证伪，严禁割裂前期认知。",
					thesis.StockCode, thesis.StockName, thesis.VerdictText, thesis.CoreThesis, thesis.KeyCatalysts, thesis.RiskFactors)
			}
		}

		// Direct 模式下前置召回语义向量经验结晶 (让买方研报直出同样具备平台历史智慧积累)
		if agentMode == string(AgentModeDirect) {
			crystals, _ := DefaultSemanticDistiller().SearchKnowledgeCrystals(ctx, question, "", 2)
			if len(crystals) > 0 {
				sysPrompt += "\n\n" + DefaultSemanticDistiller().FormatCrystalsForPrompt(crystals)
			}
		}

		aiConfig, _ := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
			return uint(aiConfigId) == item.ID
		})
		maxInputTokens := 0
		if aiConfig != nil {
			modeIsPlanExecute := agentMode == string(AgentModePlanExecute)
			maxInputTokens = getMaxInputTokens(aiConfig.ModelName, modeIsPlanExecute)
		}

		sysPromptTokens := estimateTokens(sysPrompt)
		questionTokens := estimateTokens(question)
		historyBudget := maxInputTokens - sysPromptTokens - questionTokens
		if historyBudget < 0 {
			historyBudget = 0
		}
		if len(historyMessages) > 0 && historyBudget > 0 {
			historyMessages = trimHistoryMessages(historyMessages, historyBudget)
		}

		// auto-compress history when too long
		if memoryService != nil && memoryService.NeedsCompress() {
			historyMessages, _ = memoryService.CompressHistory(historyMessages)
		}

		// 接入滑动上下文 SessionSliding
		sessionSliding := getSCManager().GetOrCreate(stockAiAgent.sessionID)
		if sessionSliding != nil && len(historyMessages) > 0 {
			for _, hm := range historyMessages {
				sessionSliding.Ctx.Add(Turn{
					Role:      hm["role"],
					Content:   hm["content"],
					Timestamp: time.Now(),
				})
			}
		}

		var bladeHistory []*blades.Message
		for _, hm := range historyMessages {
			role := hm["role"]
			content := hm["content"]
			switch role {
			case "user":
				bladeHistory = append(bladeHistory, blades.UserMessage(content))
			case "assistant":
				bladeHistory = append(bladeHistory, blades.AssistantMessage(content))
			case "system":
				bladeHistory = append(bladeHistory, blades.SystemMessage(content))
			}
		}

		// inject conversation summary if available
		if memoryService != nil {
			if summary, err := memoryService.GetSummary(); err == nil && summary != "" {
				bladeHistory = append([]*blades.Message{blades.SystemMessage("【对话摘要】" + summary)}, bladeHistory...)
			}
		}

		// inject historical analysis memory (relevant past AI reports)
		stockCode, stockName := extractStockInfo(question)
		analysisMem := NewAnalysisMemoryService().BuildAutoInjectContext(stockCode, stockName)
		if analysisMem != "" {
			bladeHistory = append([]*blades.Message{blades.SystemMessage(analysisMem)}, bladeHistory...)
		}

		// inject structured buy-side stock thesis ledger (买方认知档案与逻辑演进基准)
		if stockCode != "" {
			thesisApi := data.NewStockThesisApi()
			if thesis, _ := thesisApi.GetStockThesis(stockCode); thesis != nil && thesis.CoreThesis != "" {
				nameStr := thesis.StockName
				if nameStr == "" {
					nameStr = stockName
				}
				thesisPrompt := fmt.Sprintf("【当前买方认知档案 (Thesis Ledger)】\n- 标的：%s (%s)\n- 最新判定：%s (%s)\n- 核心投资假说：%s",
					nameStr, thesis.StockCode, thesis.VerdictText, thesis.Verdict, thesis.CoreThesis)
				if thesis.KeyCatalysts != "" {
					thesisPrompt += fmt.Sprintf("\n- 重点跟踪催化：%s", thesis.KeyCatalysts)
				}
				if thesis.RiskFactors != "" {
					thesisPrompt += fmt.Sprintf("\n- 关键证伪条件：%s", thesis.RiskFactors)
				}
				thesisPrompt += "\n👉 认知连续性提示：请以既有认知假说为基准展开增量推演。若分析发现该标的逻辑出现显著变化（强化/削弱/证伪/新假说），请在结论中清晰对照前期假说并输出【买方认知沉淀与假说检验】小节。"
				bladeHistory = append([]*blades.Message{blades.SystemMessage(thesisPrompt)}, bladeHistory...)
			}
		}

		// inject long-term facts + reflection lessons so the quality loop's
		// output actually reaches future conversations (write side only used to
		// store; this is the missing read-side wiring of MemoryPrompt).
		if memoryManager := DefaultMemoryManager(); memoryManager != nil {
			if memPrompt := memoryManager.MemoryPrompt(stockAiAgent.sessionID); memPrompt != "" {
				bladeHistory = append([]*blades.Message{blades.SystemMessage(memPrompt)}, bladeHistory...)
			}
		}

		// inject backtest track record
		btMgr := quant.NewBacktestManager()
		if stockCode != "" {
			if stats := btMgr.GetStockStats(stockCode); stats.VerifiedCount > 0 {
				sampleWarning := ""
				if stats.VerifiedCount < 5 {
					sampleWarning = " — ⚠️ 样本不足，结论仅供参考"
				}
				trackRecord := fmt.Sprintf("【回测记录】你在 %s 上的历史分析准确率: %.1f%% (已验证 %d 次%s)", stockCode, stats.AccuracyRate, stats.VerifiedCount, sampleWarning)
				sysPrompt += "\n\n" + trackRecord
			}
		} else if overall := btMgr.GetOverallStats(); overall.VerifiedCount > 0 {
			sampleWarning := ""
			if overall.VerifiedCount < 5 {
				sampleWarning = " — ⚠️ 样本不足，结论仅供参考"
			}
			trackRecord := fmt.Sprintf("【回测记录】你的历史分析总体准确率: %.1f%% (已验证 %d 次, 共 %d 份报告)%s", overall.AccuracyRate, overall.VerifiedCount, overall.TotalReports, sampleWarning)
			sysPrompt += "\n\n" + trackRecord
		}

		if memoryService != nil {
			if err := memoryService.AddUserMessage(question); err != nil {
				logger.SugaredLogger.Errorf("failed to save user message: %v", err)
			}
		}

		// parallel pre-fetch: 仅在非 direct 且包含实际工具时才进行子 Agent 并发探索。
		// direct 模式下数据已由 Go 原生毫秒级采集，且 tools=0，严禁启动子 Agent 避免 60 秒无意义等待！
		if agentMode != string(AgentModeDirect) && len(stockAiAgent.instance.AllTools) > 0 {
			if prefetched := preFetchInParallel(ctx, question, stockAiAgent.instance.AllTools, stockAiAgent.instance.AIConfig); prefetched != nil {
				parallelCtx := buildParallelContext(prefetched)
				if parallelCtx != "" {
					sysPrompt = parallelCtx + "\n\n" + sysPrompt
				}
			}
		}

		ch <- &StreamMessage{
			Role:             "assistant",
			Content:          "",
			ReasoningContent: "[STATUS]🤔 正在理解问题并制定分析计划...",
			StatusText:       "正在理解问题并制定分析计划...",
		}

		ctx = WithFallbackNotifier(ctx, func(curModel, nextModel string, err error) {
			reason := FormatFallbackReason(err)
			ch <- &StreamMessage{
				Role:             "assistant",
				Content:          "",
				ReasoningContent: fmt.Sprintf("[STATUS]⚠️ 主模型 [%s] 异常：%s，正在自动降级切换至备用模型 [%s] 继续分析...", curModel, reason, nextModel),
				StatusText:       fmt.Sprintf("主模型异常，正在自动降级切换至备用模型 [%s]...", nextModel),
				ModelName:        nextModel,
			}
			ch <- &StreamMessage{
				Role:             "assistant",
				Content:          "",
				ReasoningContent: fmt.Sprintf("[STEP]🔄 容灾切换：主模型 [%s] 异常(%s)，已无缝切换为备用模型 [%s]\n", curModel, reason, nextModel),
				ModelName:        nextModel,
			}
		})

		var fullResponse strings.Builder
		var toolNames []string
		var toolSuccess []bool
		var agentErr error

		runStartTime := time.Now()
		var firstChunkTime time.Time
		var retryCount int

		// runOnce executes one full agent pass (react loop + tool calls) and streams
		// to ch. A fresh invocation is built per attempt so a failed pass never
		// carries resume/session state into the retry.
		runOnce := func() error {
			// Stateful filter that drops <think>...</think> reasoning blocks the
			// model wrote into its content (MiniMax-M3 style) as the stream
			// arrives. Fresh per attempt so a retry never inherits a stuck state.
			var thinkFilter thinkStreamFilter
			// 【空答检测】这两个计数只属于**本次尝试**，绝不跨重试继承：
			// 若沿用外层共享变量，第 1 次尝试收了推理、第 2 次什么都没收，
			// 错误文案就会归因错误。
			firstChunkSeen := false
			// processChars = 模型 reasoning + 本系统发出的工具步骤文本。
			// 两者对用户都是「有过程、无结论」，因此合并计数（见 EmptyOutputError 注释）。
			processChars := 0
			invocation := &blades.Invocation{
				Message:     blades.UserMessage(question),
				Instruction: blades.SystemMessage(sysPrompt),
				History:     bladeHistory,
				Streamable:  true,
			}
			invocation.Message.Metadata = map[string]any{
				"agent_mode":      string(stockAiAgent.instance.Mode),
				"available_tools": toolNameList(stockAiAgent.instance.AllTools),
			}

			for msg, err := range stockAiAgent.instance.Agent.Run(ctx, invocation) {
				if err != nil {
					return err
				}

				if msg == nil {
					continue
				}

				if firstChunkTime.IsZero() {
					firstChunkTime = time.Now()
				}
				firstChunkSeen = true

				// 收集模型真实的结束原因。工具调用轮次的 finish_reason 是
				// "tool_calls"，不代表最终结束，因此只在**非 tool_calls**
				// 时记录，避免把中间轮次误当成终态。
				if fr := strings.TrimSpace(msg.FinishReason); fr != "" && fr != "tool_calls" {
					modelFinishReason = fr
					// 截断/内容拦截必须让用户立刻知道：这两种情况下正文
					// 是不完整的，静默保存会让用户拿到半份研报。
					if fr == "length" || fr == "content_filter" {
						logger.SugaredLogger.Warnf("⚠️ [Agent] 模型结束原因为 %q，输出可能不完整（finish_reason=%s）", fr, fr)
						ch <- &StreamMessage{
							Role:       "assistant",
							Content:    "",
							StatusText: fmt.Sprintf("⚠️ 模型输出因 %s 结束，内容可能不完整", finishReasonLabel(fr)),
						}
					}
				}

				if msg.Role == blades.RoleAssistant {
					text := msg.Text()
					if text != "" {
						// Strip <think> reasoning blocks the model wrote into the
						// content before streaming, so the live UI never shows them.
						text = thinkFilter.Clean(text)
						if text != "" {
							// 质量校验：模型输出 <unk> 乱码流时，拒绝写入并立即终止
							// 本次尝试，让外层重试逻辑用干净状态重新生成。
							if data.IsDegenerateGarbage(text) {
								logger.SugaredLogger.Warnf("agent stream degenerate <unk> garbage, aborting attempt (len=%d)", len(text))
								return data.DegenerateOutputError{}
							}
							// The model is now writing the final answer — announce it
							// before the first content delta so the status appears ahead
							// of the body instead of after it.
							if fullResponse.Len() == 0 {
								ch <- &StreamMessage{
									Role:             "assistant",
									Content:          "",
									ReasoningContent: "[STATUS]✍️ 正在根据数据生成回答...",
									StatusText:       "正在根据数据生成回答...",
								}
							}
							// Skip if this is the final accumulated message (openai stream
							// yields both delta chunks and the final full text — appending
							// the full text again causes duplication in the frontend).
							if fullResponse.Len() > 0 && text == fullResponse.String() {
								// final accumulated message, already streamed as deltas
							} else {
								fullResponse.WriteString(text)
								ch <- &StreamMessage{
									Role:    "assistant",
									Content: text,
								}
							}
						}
					}

					if msg.Metadata != nil {
						if rc, ok := msg.Metadata["reasoning_content"].(string); ok && rc != "" {
							processChars += len([]rune(rc))
							ch <- &StreamMessage{
								Role:             "assistant",
								Content:          "",
								ReasoningContent: rc,
							}
						}
					}
				}

				for _, part := range msg.Parts {
					if tp, ok := part.(*blades.ToolPart); ok {
						toolNames = append(toolNames, tp.Name)
						succeeded := toolCallSucceeded(tp.Response)
						toolSuccess = append(toolSuccess, succeeded)
						RecordToolCall(ctx, tp.Name, succeeded, 0)
						safeReq := tools.SanitizeSensitiveInfo(tp.Request)
						safeResp := tools.SanitizeSensitiveInfo(tp.Response)

						// Audit log for tool call
						auditEntry := models.AuditLog{
							ToolName:    tp.Name,
							RequestBody: tp.Request,
							Response:    tp.Response,
							Success:     succeeded,
						}
						if !succeeded && tp.Response != "" {
							auditEntry.ErrorMsg = tp.Response
						}
						go RecordAuditLog(auditEntry)
						logger.SugaredLogger.Infof("[ToolCall] %s(%s)", tp.Name, safeReq)
						toolStepText := fmt.Sprintf("[STEP]🔧 调用工具：%s(%s)\n", tp.Name, safeReq)
						processChars += len([]rune(toolStepText))
						ch <- &StreamMessage{
							Role:             "assistant",
							Content:          "",
							ReasoningContent: toolStepText,
						}
						if safeResp != "" {
							resultLen := len(safeResp)
							status := "✅"
							if resultLen == 0 || strings.Contains(strings.ToLower(safeResp), "error") || strings.Contains(safeResp, "失败") {
								status = "⚠️"
							}
							summary := safeResp
							if len(summary) > 80 {
								summary = summary[:80] + "..."
							}
							summary = strings.ReplaceAll(summary, "\n", " ")
							resultStepText := fmt.Sprintf("[STEP]%s 工具 %s 返回数据 (%d 字符): %s\n", status, tp.Name, resultLen, summary)
							processChars += len([]rune(resultStepText))
							ch <- &StreamMessage{
								Role:             "assistant",
								Content:          "",
								ReasoningContent: resultStepText,
							}
						}
					}
				}
			}
			// 【空答检测】模型正常结束本轮，但正文一个字都没有。
			//
			// 以前这里直接 `return nil`，上层会把本轮记为 ok=true / finish=stop，
			// 前端只能渲染「（空回复）」，用户得不到任何可行动的线索 ——
			// 实测生产环境 nemotron-3.5-super-vl-120b-a12b 出现过「同一卡片、
			// 同一问题，有时 2000+ 字符，有时跑满 55 秒却一字不给」。
			// 既然是间歇性的，就交给重试机制处理；重试仍失败时，错误文案
			// 会把真实原因说清楚（而非无信息量的「空回复」）。
			if fullResponse.Len() == 0 {
				return data.EmptyOutputError{
					ProcessChars:   processChars,
					FirstChunkSeen: firstChunkSeen,
				}
			}
			return nil
		}

		// Retry the whole pass only for transient server-side failures (rate limit /
		// overload / 5xx / stream cuts) or degenerate <unk> streams, that happen before any answer
		// text was streamed — a partial answer cannot be un-sent to the frontend,
		// so those surface as errors.
		const maxAttempts = 3
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			agentErr = runOnce()
			if agentErr == nil {
				break
			}
			if attempt == maxAttempts {
				break
			}
			_, isDegenerate := agentErr.(data.DegenerateOutputError)
			// 「静默空答」同样是可重试的瞬时故障（间歇性实测：同卡片同问题
			// 有时有正文、有时 55 秒空答），必须并入可重试集合，否则用户
			// 只是白等一轮就拿到空回复。
			isEmptyOutput := data.IsEmptyOutput(agentErr)
			if (!isTransientModelError(agentErr) && !isDegenerate && !isEmptyOutput) || fullResponse.Len() > 0 {
				break
			}
			retryCount++

			// 指数退避 (Exponential Backoff) + 随机抖动 (Jitter)
			baseSec := 1.0
			lowerErrStr := strings.ToLower(agentErr.Error())
			if strings.Contains(lowerErrStr, "429") || strings.Contains(lowerErrStr, "rate") {
				baseSec = 2.0 // 限流错误给予更大窗口冷却
			}
			backoffMs := baseSec * math.Pow(2, float64(attempt-1)) * 1000.0
			jitterMs := float64(rand.Intn(400) + 100) // 100ms ~ 500ms jitter
			sleepDur := time.Duration(backoffMs+jitterMs) * time.Millisecond

			logger.SugaredLogger.Warnf("⚠️ [Agent Retry] 瞬时故障 (尝试 %d/%d): %v, 退避等待 %v 后重试...", attempt, maxAttempts, agentErr, sleepDur)
			select {
			case <-ctx.Done():
				agentErr = ctx.Err()
			case <-time.After(sleepDur):
			}
			if ctx.Err() != nil {
				break
			}
		}

		// 结构化指标记录 (Observability Metrics)
		totalDur := time.Since(runStartTime)
		firstChunkLat := time.Duration(0)
		if !firstChunkTime.IsZero() {
			firstChunkLat = firstChunkTime.Sub(runStartTime)
		}
		finalStatus := "success"
		if agentErr != nil {
			finalStatus = "failed"
		} else if retryCount > 0 {
			finalStatus = "retry_success"
		}
		// 运行记录的 FinishReason 用模型真实值：
		// 「最近运行记录」面板据此才能看出「stop」与「length(截断)」的区别。
		if fr := strings.TrimSpace(modelFinishReason); fr != "" {
			runRecord.FinishReason = fr
		}
		metrics := AgentRunMetrics{
			TotalDuration:     totalDur,
			FirstChunkLatency: firstChunkLat,
			ToolCallCount:     len(toolNames),
			RetryCount:        retryCount,
			FinalStatus:       finalStatus,
		}
		logger.SugaredLogger.Infof("📊 [Agent Metrics] 总耗时: %v | 首包延迟: %v | 工具调用: %d次 | 重试: %d次 | 状态: %s",
			metrics.TotalDuration.Round(time.Millisecond),
			metrics.FirstChunkLatency.Round(time.Millisecond),
			metrics.ToolCallCount,
			metrics.RetryCount,
			metrics.FinalStatus,
		)
		// 记录业务级 OpenTelemetry 指标
		RecordLLMRun(ctx, totalDur, firstChunkLat, finalStatus, retryCount)

		// Audit log for LLM run
		isSuccess := agentErr == nil
		llmAuditEntry := models.AuditLog{
			ToolName:   "LLM_RUN",
			Success:    isSuccess,
			DurationMs: int64(totalDur.Milliseconds()),
		}
		if !isSuccess {
			llmAuditEntry.ErrorMsg = finalStatus
		}
		go RecordAuditLog(llmAuditEntry)

		if agentErr != nil {
			logger.SugaredLogger.Errorf("agent run error: %v", agentErr)
			// 写进运行记录：把“本次到底是哪张卡片、为什么报错”固化下来
			runRecord.ErrorMsg = agentErr.Error()

			errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", agentErr)
			lowerErr := strings.ToLower(agentErr.Error())
			if strings.Contains(lowerErr, "401") || strings.Contains(lowerErr, "403") || strings.Contains(lowerErr, "token_rejected") || strings.Contains(lowerErr, "invalid token") || strings.Contains(lowerErr, "unauthorized") || strings.Contains(lowerErr, "invalid_api_key") {
				errMsg = fmt.Sprintf("❌ AI服务鉴权失败 (HTTP 401/403)：API Key 无效或已过期。\n\n**可能原因**：当前所选大模型（配置ID：%d）的 API Key 未授权、余额不足或被平台拒绝 (token_rejected)。\n**解决方案**：请前往「系统设置 -> AI配置」检查并更新该模型的有效 API Key，或切换为可用模型（如 MiniMax-M3 / Nemotron 等）。\n\n(底层详情: %v)", aiConfigId, agentErr)
			} else if strings.Contains(lowerErr, "400") || strings.Contains(lowerErr, "bad request") {
				errMsg = fmt.Sprintf("❌ AI服务请求参数校验失败 (HTTP 400 Bad Request)。\n\n**可能原因**：网关或大模型服务端拒绝了请求参数（例如：参数重复定义、模型不支持特定思考模式或工具定义）。\n**解决方案**：系统已对常见网关参数自动执行容错过滤；若仍失败，请前往「系统设置 -> AI配置」检查该模型的配置（如检查 Base URL 格式、关闭 thinking 模式，或切换其他可用模型）。\n\n(底层详情: %v)", agentErr)
			} else if _, ok := agentErr.(data.DegenerateOutputError); ok {
				errMsg = "❌ 模型输出异常：检测到乱码(<unk>)流，已自动重试仍失败。\n\n**建议**：请重试一次；若反复出现，请切换 AI 配置中的模型。"
			} else if emptyErr, ok := agentErr.(data.EmptyOutputError); ok {
				errMsg = emptyOutputUserMessage(emptyErr, aiConfigId)
			} else if strings.Contains(lowerErr, "unexpected end of json input") || lowerErr == "eof" || strings.Contains(lowerErr, "unexpected eof") {
				errMsg = "❌ 模型服务响应中断：模型服务端返回了空响应或流式传输被意外切断 (unexpected end of JSON input)。\n\n**可能原因**：\n1. 当前大模型服务商（如 MiniMax/DeepSeek）瞬时过载、限流或网络握手重置\n2. 上下文或搜索内容较长触发了网关传输截断\n\n**解决方案**：\n1. 系统已尝试自动重试，请直接重新点击发送或换一个问题重试\n2. 建议在「AI配置」中配置备用模型，或开启 Fallback 多模型自动降级机制"
			} else if strings.Contains(lowerErr, "exceeds max iterations") {
				errMsg += "\n\n**可能原因**：模型在执行过程中进行了过多轮工具调用仍无法收敛，可能陷入了循环。\n\n**解决方案**：\n1. 尝试更精确地描述你的问题，减少模糊性\n2. 切换到支持更长上下文或更强推理能力的模型\n3. 简化查询条件"
			} else if strings.Contains(lowerErr, "reasoning") || strings.Contains(lowerErr, "thinking") {
				errMsg += "\n\n**可能原因**：当前模型开启了 thinking/reasoning 模式，但该模式与 Agent 工具调用不兼容。\n\n**解决方案**：请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型（如 deepseek-chat、gpt-4o 等）。"
			} else if isRequestTimeoutError(agentErr) {
				// ⚠️ 必须与"服务端繁忙"区分开：这条错误的真实原因是**我们自己配置的
				// 单次请求超时**（卡片「高级调度参数 → 请求超时」）。把它说成
				// "服务商限流/过载"会把用户引向完全错误的方向 —— 实测曾因此让用户
				// 反复切换模型，而真正该做的是调大这一项。
				errMsg += fmt.Sprintf(
					"\n\n**真实原因**：单次模型请求超过了当前配置的「请求超时」上限（%d 秒）。"+
						"这不是服务商故障，也不是限流。\n\n"+
						"**解决方案**：进入「设置 → AI模型注册表」，展开该卡片的"+
						"「⚙️ 高级调度参数」，把「请求超时 (秒)」调大（慢速推理模型建议 ≥ 300 秒；"+
						"实测本平台部分模型单次响应可达 200~400 秒）。\n\n"+
						"**其他可选**：改用响应更快的模型，或关闭该卡片的「深度思考」以减少单次推理耗时。",
					effectiveRequestTimeoutSec(aiConfigId))
			} else if isTransientModelError(agentErr) {
				errMsg += "\n\n**提示**：模型服务端暂时繁忙（限流/过载/网络中断），已自动重试仍失败，请稍后再试或切换其他 AI 配置。"
			}

			ch <- &StreamMessage{
				Role:    "assistant",
				Content: errMsg,
			}
		}

		// Agent 因错误提前退出：错误正文已在上方发出，这里只需 return，
		// 终态（FinishReason）由 defer 护栏统一补齐。
		// （旧实现注释写着 "don't send status/DONE signals — just close"，
		//   正是这句让错误场景下前端永久卡在流式状态。）
		if agentErr != nil {
			return
		}

		// Fold a duplicated final answer (MiniMax-M3 sometimes emits the whole
		// answer twice, once before and once after a re-confirmation think block)
		// so memory and post-processing persist a single copy. The frontend
		// performs the same collapse at finish time for its rendered copy.
		if collapsed := cleanFinalContent(fullResponse.String()); collapsed != fullResponse.String() {
			fullResponse.Reset()
			fullResponse.WriteString(collapsed)
		}

		if fullResponse.Len() != 0 && memoryService != nil {
			if err := memoryService.AddAssistantMessage(fullResponse.String()); err != nil {
				logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
			}
		}

		// fire-and-forget post-processing: memory, evaluation, tool ranking
		if fullResponse.Len() > 0 && stockAiAgent != nil && stockAiAgent.sessionID != "" {
			go postProcessResponse(stockAiAgent.sessionID, question, fullResponse.String(), toolNames, toolSuccess)
		}

		// 解析并在结束前下发伴随的全局控制动作指令
		if fullResponse.Len() > 0 {
			cleanedText, actions := types.ParseAndExtractActions(fullResponse.String())
			if strings.TrimSpace(cleanedText) != "" && cleanedText != fullResponse.String() {
				fullResponse.Reset()
				fullResponse.WriteString(cleanedText)
			}

			// Audit log for agent actions
			if len(actions) > 0 {
				for _, action := range actions {
					actionAuditEntry := models.AuditLog{
						ToolName: "AGENT_ACTION_" + string(action.Type),
						Success:  true, // Actions are parsed from successful LLM responses
					}
					// Serialize action details as JSON for the request/response fields
					actionJSON, _ := json.Marshal(action)
					actionAuditEntry.RequestBody = string(actionJSON)
					go RecordAuditLog(actionAuditEntry)
				}
			}
			if len(actions) > 0 {
				ch <- &StreamMessage{
					Role:    "assistant",
					Content: "",
					Actions: actions,
				}
			}
		}

		runRecord.ContentLen = len([]rune(fullResponse.String()))
		sendTerminal(&StreamMessage{
			Role:         "assistant",
			Content:      "agent-DONE",
			FinishReason: "stop",
			Metrics:      &metrics,
		})
	}()

	return ch
}

// postProcessResponse runs memory storage, evaluation, and tool ranking in
// parallel after the agent response is complete. It is fire-and-forget and
// must not block the response stream.
func postProcessResponse(sessionID, question, response string, toolNames []string, toolSuccess []bool) {
	recoverLog := func(label string) {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in postProcessResponse [%s]: %v", label, r)
		}
	}
	var wg sync.WaitGroup

	// 1. Store working memory
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer recoverLog("memory")
		DefaultMemoryManager().Add(sessionID, "user", question)
		DefaultMemoryManager().Add(sessionID, "assistant", response)
	}()

	// 2. Update tool ranking stats (success-aware, non-blocking).
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer recoverLog("tool_ranking")
		for i, name := range toolNames {
			ok := true
			if i < len(toolSuccess) {
				ok = toolSuccess[i]
			}
			DefaultEvolver().RecordToolUse(name, ok, 0)
		}
	}()

	// 4. 自动持久化个股分析报告（接入自反思与历史回测闭环）
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer recoverLog("ai_report_persistence")
		stockCode, stockName := extractStockInfo(question)
		cleanCode := models.CleanStockCode(stockCode)
		if cleanCode != "" && models.IsStockReportCode(cleanCode) && len(response) > 50 {
			if stockName == "" {
				var basic data.StockBasic
				if err := db.Dao.Where("ts_code LIKE ?", "%"+cleanCode+"%").First(&basic).Error; err == nil {
					stockName = basic.Name
				}
			}
			// 调用 SaveAIResponseResult 进行去重与落库
			data.NewDeepSeekOpenAi(context.Background(), 0).SaveAIResponseResult(cleanCode, stockName, response, sessionID, question)
		}
	}()

	wg.Wait()
	logger.SugaredLogger.Debugf("postProcessResponse completed for session %s", sessionID)
}

// isTransientModelError reports whether an agent/model error is a transient
// server-side condition (rate limit, overload, 5xx) that may succeed on retry.
// emptyOutputUserMessage 把「模型静默空答」翻译成用户**可执行**的操作指引。
//
// 存在意义：这种情况在旧实现下会被记为 ok=true/finish=stop，前端只渲染
// 无信息量的「（空回复）」，用户无从判断是软件坏了、网络不通、还是模型问题，
// 只能反复重试。这里把「到底发生了什么」和「该动哪个开关」讲清楚。
func emptyOutputUserMessage(e data.EmptyOutputError, aiConfigId int) string {
	var b strings.Builder
	b.WriteString("❌ 模型没有给出正文回答（本轮正常结束，但内容为空）。\n\n")
	b.WriteString("**本次实际发生了什么**：")
	switch {
	case e.ProcessChars > 0:
		fmt.Fprintf(&b, "模型产出了 %d 字符的思考/工具过程，但**始终没有写出最终答案**。", e.ProcessChars)
	case !e.FirstChunkSeen:
		b.WriteString("上游模型服务**没有返回任何内容**（连接建立后即结束）。")
	default:
		b.WriteString("上游返回了响应，但**正文为空**。")
	}
	b.WriteString("\n系统已自动重试，仍未获得正文。\n\n")

	b.WriteString("**可能原因与解决方案**：\n")
	if e.ProcessChars > 0 {
		b.WriteString("1. **思考模式吃掉了全部输出预算**（最常见）：到「设置 → AI模型注册表」关闭该卡片的「深度思考」，或把「最大输出长度 (maxTokens)」调大\n")
		b.WriteString("2. **模型执行了工具却未收敛出结论**：换用输出更稳定的模型（如 MiniMax-M3 / DeepSeek），或把问题问得更具体\n")
	} else {
		b.WriteString("1. **上游网关瞬时异常或配额不足**：稍后重试，或到「设置 → AI模型注册表」点该卡片的「检测连通性」\n")
		b.WriteString("2. **该模型当前不可用**：切换为其他 AI 配置，或配置备用模型启用自动降级\n")
	}
	fmt.Fprintf(&b, "\n(本次使用的 AI 配置 ID: %d)", aiConfigId)
	return b.String()
}

func isTransientModelError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	markers := []string{
		"429", "500", "502", "503", "504", "529",
		"rate limit", "rate_limit",
		"resourceexhausted", "resource exhausted",
		"internal_server_error", "internal server error",
		"overloaded", "too many requests", "request limit reached",
		"temporarily unavailable", "try again later", "server busy", "service busy",
		"over capacity",
		"unexpected end of json input",
		"unexpected eof",
		"eof",
		"connection reset by peer",
		"broken pipe",
		"wsarecv",
		"timeout awaiting response headers",
		"context deadline exceeded",
		"client.timeout exceeded",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// Precompiled regexes used across request hot paths (avoid MustCompile per call).
var (
	stockCodeRe = regexp.MustCompile(`(?:^|[^\d])((?:sh|SH|sz|SZ)?(?:[036849]\d{5}|51\d{4}|58\d{4}|15\d{4}))(?:[^\d]|$)`)
	stockNameRe = regexp.MustCompile(`([\p{Han}]{2,4})(?:股票|股价|行情|走势|代码)`)
)

// extractStockInfo 从用户问题中提取股票代码/名称（增强启发式 + 数据库对照匹配）
func extractStockInfo(question string) (stockCode, stockName string) {
	// 匹配股票代码: 独立 6 位数字（A股），或 sh/SH/sz/SZ 前缀
	if m := stockCodeRe.FindStringSubmatch(question); len(m) > 1 {
		stockCode = m[1]
	}
	// 匹配常见股票名称关键词（中文 2-4 字）
	if m := stockNameRe.FindStringSubmatch(question); len(m) > 1 {
		stockName = m[1]
	}

	// 互补数据库查询
	if stockCode != "" && stockName == "" && db.Dao != nil {
		clean := models.CleanStockCode(stockCode)
		var sb data.StockBasic
		if err := db.Dao.Where("ts_code LIKE ?", "%"+clean+"%").First(&sb).Error; err == nil && sb.Name != "" {
			stockName = sb.Name
		}
	} else if stockCode == "" && stockName != "" && db.Dao != nil {
		var sb data.StockBasic
		if err := db.Dao.Where("name = ?", stockName).First(&sb).Error; err == nil && sb.TsCode != "" {
			stockCode = sb.TsCode
		}
	}

	return
}

// isRequestTimeoutError 判断错误是否由**我们自己的单次请求超时**引起。
//
// 与 isTransientModelError 刻意分离：后者表示"上游真的繁忙"；
// 而 `context deadline exceeded` 在这里几乎总是卡片「请求超时」配置过低
// （chat_model_factory 用 aiConfig.TimeOut 作为每一次 LLM 请求的上限）。
// 混为一谈会把用户引向错误的排查方向。
func isRequestTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	for _, m := range []string{
		"context deadline exceeded",
		"request timeout",
		"timeout awaiting response headers",
		"client.timeout exceeded",
		"i/o timeout",
	} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// effectiveRequestTimeoutSec 返回某张卡片实际生效的"单次请求超时"秒数，
// 供错误提示如实告知用户该改哪一项。
// finishReasonLabel 把 OpenAI 的 finish_reason 翻译成用户能看懂的中文。
// 用于截断/拦截时给出一句人话，而不是把 "length" 原样丢给用户。
func finishReasonLabel(fr string) string {
	switch strings.ToLower(strings.TrimSpace(fr)) {
	case "length", "max_tokens":
		return "输出长度达到上限"
	case "content_filter":
		return "内容安全策略拦截"
	case "tool_calls", "function_call":
		return "工具调用"
	case "stop":
		return "正常结束"
	default:
		return fr
	}
}

func effectiveRequestTimeoutSec(aiConfigId int) int {
	cfg := data.GetSettingConfig()
	if cfg == nil {
		return 0
	}
	for _, c := range cfg.AiConfigs {
		if c != nil && int(c.ID) == aiConfigId {
			if c.TimeOut > 0 {
				return c.TimeOut
			}
			return 300
		}
	}
	return 0
}
