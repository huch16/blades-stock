package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

// MarketPhase 表示市场所处的时段
type MarketPhase string

const (
	PhasePreMarket        MarketPhase = "pre_market"        // 盘前前瞻 (08:30 - 09:15)
	PhaseCallAuction      MarketPhase = "call_auction"      // 开盘集合竞价 (09:15 - 09:30)
	PhaseMorningTrading   MarketPhase = "morning_trading"   // 盘中早盘 (09:30 - 11:30)
	PhaseNoonBreak        MarketPhase = "noon_break"        // 午间休市 (11:30 - 13:00)
	PhaseAfternoonTrading MarketPhase = "afternoon_trading" // 盘中下午盘 (13:00 - 15:00)
	PhasePostMarket       MarketPhase = "post_market"       // 盘后复盘 (15:00 - 18:00)
	PhaseClosed           MarketPhase = "closed"            // 夜间/非交易日休市
)

// MarketLifecycleStatus 市场生命周期状态信息
type MarketLifecycleStatus struct {
	Phase           MarketPhase `json:"phase"`
	PhaseName       string      `json:"phaseName"`
	IsTradingDay    bool        `json:"isTradingDay"`
	IsTradingTime   bool        `json:"isTradingTime"`
	CurrentTime     string      `json:"currentTime"`
	MarketStateDesc string      `json:"marketStateDesc"`
	NextEvent       string      `json:"nextEvent"`
	NextEventTime   string      `json:"nextEventTime"`
}

// MarketIntelligenceReport 智能化全时段研报产出
type MarketIntelligenceReport struct {
	Type        string              `json:"type"`              // "pre_market_briefing" 或 "post_market_review"
	Title       string              `json:"title"`             // 标题
	Summary     string              `json:"summary"`           // 核心要点
	Content     string              `json:"content"`           // 完整Markdown内容
	Actions     []types.AgentAction `json:"actions,omitempty"` // 报告伴随行动项指令
	GeneratedAt time.Time           `json:"generatedAt"`       // 生成时间
	TradingDate string              `json:"tradingDate"`       // 归属交易日
	Source      string              `json:"source"`            // 模型来源
	// InBackground 为 true 表示"生成仍在后台进行，本次未在响应窗口内完成"。
	//
	// 为什么需要它：生产入口（Cloudflare/instabox）对单请求约 20 秒硬切断，
	// 而研报推演常态 40~90 秒。同步等满是必然 502，用户看到"没有结果"。
	// 现在同步窗口只等 12 秒：未完成就返回这个带标记的占位报告，
	// 前端据此转入轮询；生成在独立 context 上继续，完成后自动呈现。
	InBackground bool   `json:"inBackground,omitempty"`
	StatusText   string `json:"statusText,omitempty"` // 面向用户的进度说明
}

var (
	lifecycleReportCacheMu    sync.RWMutex
	lifecycleLatestReport     *MarketIntelligenceReport
	lifecycleReportsByType    = make(map[string]*MarketIntelligenceReport)
	MarketReportBroadcastHook func(event string, data any)
)

func cacheReport(report *MarketIntelligenceReport) {
	if report == nil {
		return
	}
	lifecycleReportCacheMu.Lock()
	lifecycleLatestReport = report
	if lifecycleReportsByType == nil {
		lifecycleReportsByType = make(map[string]*MarketIntelligenceReport)
	}
	lifecycleReportsByType[report.Type] = report
	lifecycleReportCacheMu.Unlock()

	if MarketReportBroadcastHook != nil {
		MarketReportBroadcastHook("marketIntelligenceReport", report)
	}
}

func formatYuanAmount(raw string) string {
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return raw
	}
	prefix := ""
	if val > 0 {
		prefix = "+"
	}
	absVal := math.Abs(val)
	if absVal >= 1e8 {
		return fmt.Sprintf("%s%.2f 亿元", prefix, val/1e8)
	} else if absVal >= 1e4 {
		return fmt.Sprintf("%s%.2f 万元", prefix, val/1e4)
	}
	return fmt.Sprintf("%s%.2f 元", prefix, val)
}

// isValidMarketReport 严密校验模型返回内容，剔除上游网关报错文本、异常提示及无效短内容
// isTelemetryLine 判断一行是否为**遥测/状态**行，而非研报正文或推理内容。
//
// 事故背景（用户截图实证）：报告正文位置只剩一行
// “[STEP]🔄 容灾切换：主模型 [gemini-3.8-flash] 异常(…429 Token pool is empty…)”
// 而看不到任何实际研报。根因是清理逻辑只过滤了 [STATUS]，没过滤 [STEP]/[DONE] ——
// 这些原本是给前端展示“流程进度”的遥测信息，却混进了最终报告。
func isTelemetryLine(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return false
	}
	for _, p := range []string{"[STATUS]", "[STEP]", "[DONE]"} {
		if strings.HasPrefix(t, p) || strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// stripTelemetryLines 剔除所有遥测行（用于正文校验与报告组装）。
func stripTelemetryLines(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if isTelemetryLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func isValidMarketReport(content string) (bool, string) {
	// 先剔除遥测行：否则“只有一行容灾切换日志”会被当成有效研报展示、入库
	trimmed := stripTelemetryLines(content)
	if trimmed == "" {
		return false, "模型未返回任何有效内容，请检查大模型配置或网络"
	}
	// 任何位置包含 ❌ 或“Agent 调用失败”均视为异常中断，严禁存库与呈现。
	// 注意：错误原因里只放截断后的文本——旧实现把整篇研报原文
	// 原样塞进 error，调用方再原样打日志，一份几万字的报告
	// 会完整进日志文件。
	if strings.Contains(trimmed, "❌") || strings.Contains(trimmed, "Agent 调用失败") {
		return false, truncateText(trimmed, 200)
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "no longer available") ||
		strings.Contains(lower, "please switch to") ||
		strings.Contains(lower, "token_rejected") ||
		strings.Contains(lower, "404 not found") ||
		strings.Contains(lower, "502 bad gateway") ||
		(strings.Contains(lower, "502") && (strings.Contains(lower, "error") || strings.Contains(lower, "错误") || strings.Contains(lower, "bad gateway"))) ||
		strings.Contains(lower, "出现错误：502") ||
		strings.Contains(lower, "503 service unavailable") ||
		strings.Contains(lower, "504 gateway timeout") ||
		strings.Contains(lower, "service unavailable") ||
		strings.Contains(lower, "context canceled") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "bad gateway") ||
		strings.Contains(lower, "antigravity") ||
		// 网关/账号池限流与容灾降级提示：这类文本是链路诊断，不是研报正文
		strings.Contains(lower, "token pool is empty") ||
		strings.Contains(lower, "账号池") ||
		strings.Contains(lower, "额度耗尽") ||
		strings.Contains(lower, "网关限流") ||
		strings.Contains(lower, "容灾切换") {
		return false, fmt.Sprintf("模型上游网关返回异常提示：%s", trimmed)
	}
	if trimmed == "agent-DONE" || len([]rune(trimmed)) < 100 {
		return false, fmt.Sprintf("模型返回内容过短（仅 %d 字），非有效研报：%s", len([]rune(trimmed)), trimmed)
	}
	return true, ""
}

// FormatUplimitHotToMarkdown 将复杂的涨停题材原始数据提炼为高信息密度的 Markdown 摘要，杜绝大 JSON 提示词爆炸
func FormatUplimitHotToMarkdown(uplimitHot map[string]any) string {
	if uplimitHot == nil {
		return "暂无连板涨停数据"
	}
	dataMap, ok := uplimitHot["data"].(map[string]any)
	if !ok {
		return "涨停复盘数据整理中..."
	}
	var sb strings.Builder

	// 1. 连板高度与梯队结构 (ban_info)
	if maxCount, ok := dataMap["max_count"].(float64); ok && maxCount > 0 {
		sb.WriteString(fmt.Sprintf("- **全市场空间板高度**：%d 连板\n", int(maxCount)))
	}
	if banInfo, ok := dataMap["ban_info"].(map[string]any); ok {
		sb.WriteString("- **连板梯队分布**：")
		var ladders []string
		for i := 15; i >= 1; i-- {
			key := strconv.Itoa(i)
			if item, exists := banInfo[key]; exists {
				if m, ok := item.(map[string]any); ok {
					if cnt, ok := m["count"].(float64); ok && cnt > 0 {
						ladders = append(ladders, fmt.Sprintf("%d板(%d家)", i, int(cnt)))
					}
				}
			}
		}
		if len(ladders) > 0 {
			sb.WriteString(strings.Join(ladders, " > ") + "\n")
		} else {
			sb.WriteString("无连板梯队数据\n")
		}
	}

	// 2. 最强题材板块 TOP 5
	if plates, ok := dataMap["plate"].([]any); ok && len(plates) > 0 {
		sb.WriteString("- **核心主线题材热度 TOP 5**：\n")
		for i, p := range plates {
			if i >= 5 {
				break
			}
			if arr, ok := p.([]any); ok && len(arr) >= 3 {
				pName := fmt.Sprintf("%v", arr[0])
				pScore := arr[2]
				sb.WriteString(fmt.Sprintf("  %d. **%s** (资金聚焦热度分: %v)\n", i+1, pName, pScore))
			}
		}
	}

	// 3. 核心连板龙头标的 (plate_stocks)
	if plateStocks, ok := dataMap["plate_stocks"].(map[string]any); ok {
		type stockItem struct {
			code      string
			name      string
			desc      string
			keepTimes int
			time      string
		}
		var topStocks []stockItem
		seen := make(map[string]bool)
		for _, v := range plateStocks {
			if list, ok := v.([]any); ok {
				for _, s := range list {
					if sm, ok := s.(map[string]any); ok {
						code := fmt.Sprintf("%v", sm["stock_code"])
						if seen[code] {
							continue
						}
						seen[code] = true
						name := fmt.Sprintf("%v", sm["stock_name"])
						desc := fmt.Sprintf("%v", sm["up_limit_desc"])
						timeStr := fmt.Sprintf("%v", sm["up_limit_time"])
						keepTimes := 0
						if kt, ok := sm["up_limit_keep_times"].(float64); ok {
							keepTimes = int(kt)
						}
						topStocks = append(topStocks, stockItem{code, name, desc, keepTimes, timeStr})
					}
				}
			}
		}
		sort.Slice(topStocks, func(i, j int) bool {
			return topStocks[i].keepTimes > topStocks[j].keepTimes
		})
		if len(topStocks) > 0 {
			sb.WriteString("- **代表性高标与领涨标的**：\n")
			limit := 8
			if len(topStocks) < limit {
				limit = len(topStocks)
			}
			for i := 0; i < limit; i++ {
				stk := topStocks[i]
				sb.WriteString(fmt.Sprintf("  - %s (%s): %s (封单时间 %s)\n", stk.name, stk.code, stk.desc, stk.time))
			}
		}
	}

	res := sb.String()
	if strings.TrimSpace(res) == "" {
		return "涨停复盘数据整理中..."
	}
	return res
}

// safeRuntimeEventsEmit 安全推流 Wails 桌面事件，避免在 Web / Server 模式或非 Wails 上下文下
// 调用 runtime.EventsEmit 触发 Wails 内部 log.Fatalf("An invalid context was passed...") 导致进程被 os.Exit(1) 强制杀死。
func safeRuntimeEventsEmit(ctx context.Context, event string, optionalData ...any) {
	if ctx == nil || ctx.Value("events") == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Warnf("safeRuntimeEventsEmit recover: %v", r)
		}
	}()
	runtime.EventsEmit(ctx, event, optionalData...)
}

// MarketLifecycleService 市场全流程生命周期智能服务
type MarketLifecycleService struct {
	ctx context.Context
	// serenityChatRound runs one report LLM round and collects the stream.
	// serenityFallbackPicker selects the first available fallback card for
	// a report whole-round retry. Both are injectable seams: production
	// defaults are set by NewMarketLifecycleService, tests replace them.
	// A nil seam falls back to the production implementation, so a
	// zero-value service still works. (名字留自 Serenity 首接，四个研报
	// 生成器共用同一对 seam。)
	serenityChatRound      func(ctx context.Context, prompt, sysPrompt string, aiConfigId int) reportStreamResult
	serenityFallbackPicker func(ctx context.Context, primaryAIConfigID int) (fallbackID int, fallbackName string, ok bool)
}

func NewMarketLifecycleService(ctx context.Context) *MarketLifecycleService {
	return &MarketLifecycleService{
		ctx:                    ctx,
		serenityChatRound:      defaultReportChatRound,
		serenityFallbackPicker: defaultReportFallbackPicker,
	}
}

// defaultReportChatRound is the production report round: Direct mode,
// zero tools, stream collected by collectReportStream. Shared by all four
// report generators (pre-market, post-market, ETF rotation, Serenity).
func defaultReportChatRound(ctx context.Context, prompt, sysPrompt string, aiConfigId int) reportStreamResult {
	ch := NewStockAiAgentApi().ChatWithContext(ctx, prompt, aiConfigId, nil, false, 0, false, string(AgentModeDirect), sysPrompt)
	return collectReportStream(ch)
}

// defaultReportFallbackPicker resolves the card actually used by the
// first round and returns the first available fallback card from the
// fallback chain (registry order, same exclusions as
// BuildFallbackProvidersFromDB). It never hard-codes a model: whatever
// the registry lists first wins.
func defaultReportFallbackPicker(ctx context.Context, primaryAIConfigID int) (int, string, bool) {
	res, err := data.ResolveModel(primaryAIConfigID)
	if err != nil {
		return 0, "", false
	}
	if fb, ok := FirstFallbackAIConfig(res.Config); ok {
		return int(fb.ID), fb.ModelName, true
	}
	return 0, "", false
}

// runReportGeneration runs one report generation round. If the round is
// interrupted mid-stream (stream error / timeout), it retries the WHOLE
// round exactly once with the first available fallback card.
//
// 背景：fallback 层只在「无实质正文」时降级；已有实质正文后断流，错误会
// 原样上抛（两段模型输出绝不能无声拼成一段）。但研报失败时半份内容本来
// 就被丢弃、不落库，所以在 market_lifecycle 层换卡整轮重跑是安全的：
// 重试丢弃首轮的全部半份输出（reasoning 与 content 均不带入第二轮），
// 第二轮从空白 prompt 重新生成，最终结果只含第二轮的输出。
//
// 只重试一次；第二轮的结果（成功或失败）直接采用，不再重试。
// 截断（finish_reason=length/content_filter）不是流中断，不重试——
// 换卡也大概率在同样的 maxTokens 下再次截断。
//
// label 标识报告类型，出现在重试日志里（如 "Serenity"、"盘前晨报")。
// newGenCtx 为每一轮各建一个新的生成上下文：重试轮绝不能继承首轮已
// 消耗的超时——旧实现两轮共用一个 10 分钟 ctx，首轮跑 9 分钟后断流，
// 重试只剩约 1 分钟。两轮各拿一个完整的超时，互不挤占。
func (s *MarketLifecycleService) runReportGeneration(ctx context.Context, prompt, sysPrompt string, aiConfigId int, label string, newGenCtx func() (context.Context, context.CancelFunc)) reportStreamResult {
	chatRound := s.serenityChatRound
	if chatRound == nil {
		chatRound = defaultReportChatRound
	}
	genCtx, cancel := newGenCtx()
	defer cancel()
	res := chatRound(genCtx, prompt, sysPrompt, aiConfigId)
	if res.StreamErr == "" {
		return res
	}
	picker := s.serenityFallbackPicker
	if picker == nil {
		picker = defaultReportFallbackPicker
	}
	fbID, fbName, ok := picker(ctx, aiConfigId)
	if !ok {
		return res
	}
	logger.SugaredLogger.Warnf("🔄 【%s换卡整轮重试】首轮生成中断（%s），丢弃半份输出，改用备用卡片 [%s] 整轮重跑（仅重试一次）", label, res.StreamErr, fbName)
	retryCtx, retryCancel := newGenCtx()
	defer retryCancel()
	return chatRound(retryCtx, prompt, sysPrompt, fbID)
}

// GetCurrentMarketPhase 计算当前时间的市场阶段
func GetCurrentMarketPhase(t time.Time) (MarketPhase, string) {
	// 判断是否为交易日（从交易日历检查）
	isDay := data.IsTradingDay(t.Format("2006-01-02"))
	if !isDay {
		weekday := t.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			return PhaseClosed, "周末休市"
		}
		return PhaseClosed, "法定节假日休市"
	}

	hour, minute := t.Hour(), t.Minute()
	hm := hour*100 + minute

	switch {
	case hm >= 830 && hm < 915:
		return PhasePreMarket, "盘前前瞻期"
	case hm >= 915 && hm < 930:
		return PhaseCallAuction, "集合竞价期"
	case hm >= 930 && hm <= 1130:
		return PhaseMorningTrading, "早盘连续竞价"
	case hm > 1130 && hm < 1300:
		return PhaseNoonBreak, "午间休市期"
	case hm >= 1300 && hm <= 1500:
		return PhaseAfternoonTrading, "午后连续竞价"
	case hm > 1500 && hm < 1800:
		return PhasePostMarket, "盘后复盘期"
	default:
		return PhaseClosed, "夜间休市"
	}
}

// GetMarketLifecycleStatus 获取完整的市场生命周期状态
func (s *MarketLifecycleService) GetMarketLifecycleStatus() MarketLifecycleStatus {
	now := time.Now()
	phase, phaseName := GetCurrentMarketPhase(now)
	isDay := data.IsTradingDay(now.Format("2006-01-02"))

	hour, minute := now.Hour(), now.Minute()
	hm := hour*100 + minute

	isTrading := (isDay && ((hm >= 915 && hm <= 1130) || (hm >= 1300 && hm <= 1500)))

	var nextEvent, nextEventTime string
	if !isDay {
		nextEvent = "下一交易日早盘"
		nextEventTime = "09:15"
	} else {
		switch phase {
		case PhaseClosed:
			if hm < 830 {
				nextEvent = "盘前前瞻发布"
				nextEventTime = "08:30"
			} else {
				nextEvent = "明日盘前前瞻"
				nextEventTime = "08:30"
			}
		case PhasePreMarket:
			nextEvent = "开盘集合竞价"
			nextEventTime = "09:15"
		case PhaseCallAuction:
			nextEvent = "连续竞价开盘"
			nextEventTime = "09:30"
		case PhaseMorningTrading:
			nextEvent = "早盘收盘休市"
			nextEventTime = "11:30"
		case PhaseNoonBreak:
			nextEvent = "下午开盘"
			nextEventTime = "13:00"
		case PhaseAfternoonTrading:
			nextEvent = "全天收盘复盘"
			nextEventTime = "15:00"
		case PhasePostMarket:
			nextEvent = "夜间复盘总结"
			nextEventTime = "18:00"
		}
	}

	stateDesc := fmt.Sprintf("%s · %s", now.Format("15:04:05"), phaseName)

	return MarketLifecycleStatus{
		Phase:           phase,
		PhaseName:       phaseName,
		IsTradingDay:    isDay,
		IsTradingTime:   isTrading,
		CurrentTime:     now.Format("2006-01-02 15:04:05"),
		MarketStateDesc: stateDesc,
		NextEvent:       nextEvent,
		NextEventTime:   nextEventTime,
	}
}

func cleanAndAssembleReport(reasoning, content string) (string, []types.AgentAction) {
	reasoning = strings.TrimSpace(reasoning)
	content = strings.TrimSpace(content)

	cleanedContent, actions := types.ParseAndExtractActions(content)

	// 剔除全部遥测行（[STATUS] / [STEP] / [DONE]）。
	// 旧实现只剔除 [STATUS]，导致“[STEP]🔄 容灾切换…”被当成思考过程渲染进报告。
	cleanReasoning := stripTelemetryLines(reasoning)
	// 正文同样剔除（防御性：遥测绝不应出现在正文区）
	cleanedContent = stripTelemetryLines(cleanedContent)

	if cleanReasoning != "" {
		return fmt.Sprintf("<details style=\"margin-bottom:16px;padding:10px 14px;background:rgba(128,128,128,0.08);border-radius:8px;border-left:3px solid #18a058;\">\n<summary style=\"cursor:pointer;font-weight:600;color:inherit;opacity:0.85;\">💭 展开查看思考过程与推理链条</summary>\n\n%s\n</details>\n\n%s", cleanReasoning, cleanedContent), actions
	}
	return cleanedContent, actions
}

// reportStreamResult 汇总一次研报生成的流式结果。
type reportStreamResult struct {
	// Reasoning / Content 分别是思考链与正文（已剔除 ❌ 错误串）。
	Reasoning string
	Content   string
	// StreamErr 非空表示流中捕获到底层错误消息。
	StreamErr string
	// FinishReason 是模型**真实**的结束原因（stop/length/content_filter）。
	FinishReason string
	// Truncated 为 true 表示模型因 length/max_tokens/content_filter 结束，
	// 正文不完整。绝不是「正常生成完成」。
	Truncated bool
}

// collectReportStream 消费 ChatWithContext 的流，汇总正文/思考并识别截断。
//
// 存在意义（用户实证「AI盘后分析等经常运行一段，没有全部完成」）：
// 四个研报生成器原先各自内联同一个循环，且**完全不看 FinishReason**。
// 模型因 maxTokens 耗尽结束（finish_reason=length）时，循环正常退出、
// 半份正文被当成完整研报校验并落库展示 —— 用户看到「生成到一半就停了，
// 且没有任何说明」。现在统一在这里识别截断，交由调用方给出可行动错误。
func collectReportStream(ch chan *StreamMessage) reportStreamResult {
	var res reportStreamResult
	var reasoning, content strings.Builder
	for msg := range ch {
		if msg == nil {
			continue
		}
		// 终态消息携带模型真实的结束原因（agent_api.go 现已下发真实值）。
		if fr := strings.TrimSpace(msg.FinishReason); fr != "" && fr != "tool_calls" {
			res.FinishReason = fr
			switch strings.ToLower(fr) {
			case "length", "max_tokens", "content_filter":
				res.Truncated = true
			}
		}
		if msg.ReasoningContent != "" && !strings.HasPrefix(msg.ReasoningContent, "[STATUS]") {
			reasoning.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" && msg.Content != "agent-DONE" {
			if strings.HasPrefix(msg.Content, "❌") || strings.Contains(msg.Content, "Agent 调用失败") {
				res.StreamErr = msg.Content
				continue
			}
			content.WriteString(msg.Content)
		}
	}
	res.Reasoning = reasoning.String()
	res.Content = content.String()
	return res
}

// GeneratePreMarketBriefing 生成盘前前瞻/晨报 (09:10前或手动触发)
// 包含：隔夜外盘/中概股动态、核心行业ETF动量排名与5视角加权打分、宏观要闻与开盘策略
func (s *MarketLifecycleService) GeneratePreMarketBriefing(ctx context.Context, aiConfigId int) (*MarketIntelligenceReport, error) {
	logger.SugaredLogger.Info("【MarketLifecycle】开始生成智能盘前前瞻与ETF轮动晨报...")
	todayStr := time.Now().Format("2006-01-02")

	// 1. 采集盘前核心数据：全球指数、财联社快讯、核心行业ETF行情
	globalBrief := data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
	telegraphs := data.NewMarketNewsApi().TelegraphList(15)

	var newsText strings.Builder
	if telegraphs != nil {
		for i, item := range *telegraphs {
			if i >= 10 {
				break
			}
			newsText.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, item.Time, item.Content))
		}
	}

	// 采集主流行业与宽基 ETF 实时/日线行情。
	// 一次批量拉取：旧实现对 7 只 ETF 逐只调 GetStockCodeRealTimeData，
	// 每只都是一次独立 HTTP 请求；该接口本身支持变长批量参数。
	etfCodes := []string{"510300.SH", "159915.SZ", "512480.SH", "512660.SH", "512010.SH", "512800.SH", "510880.SH"}
	var etfSummary strings.Builder
	etfSummary.WriteString("核心关注ETF池（沪深300、创业板、半导体、军工、医药、证券、红利）：\n")
	stockAPI := data.NewStockDataApi()
	etfQuotes, err := stockAPI.GetStockCodeRealTimeData(etfCodes...)
	if err != nil {
		logger.SugaredLogger.Warnf("批量获取核心ETF行情失败: %v", err)
	} else if etfQuotes != nil {
		// 按 etfCodes 顺序输出（批量接口返回顺序不保证，用 clean code 建表映射）。
		quoteByCode := make(map[string]data.StockInfo, len(*etfQuotes))
		for _, q := range *etfQuotes {
			if clean := models.CleanStockCode(q.Code); clean != "" {
				quoteByCode[clean] = q
			}
		}
		for _, code := range etfCodes {
			if q, ok := quoteByCode[models.CleanStockCode(code)]; ok {
				etfSummary.WriteString(fmt.Sprintf("- %s(%s): 现价 %v, 今日/昨日涨跌幅 %v%%\n", q.Name, code, q.Price, q.ChangePercent))
			}
		}
	}

	// 计算纯 Go 21日加权动量与硬风控指标（注入硬核数学，避免 LLM 幻觉)
	momentumList, err := quant.BatchCalculateETFMomentum(etfCodes)
	if err != nil {
		logger.SugaredLogger.Warnf("计算ETF动量失败: %v", err)
		momentumList = []*quant.ETFMomentumScoreResult{}
	}
	momentumTable := quant.FormatETFMomentumSummary(momentumList)

	// 检索与盘前宏观和ETF轮动高度匹配的历史高胜率知识结晶 (语义 RAG 赋能)
	retrievedCrystals, _ := DefaultSemanticDistiller().SearchKnowledgeCrystals(ctx, "A股盘前 隔夜外盘 宏观政策 行业ETF轮动", "etf_rotation", 2)
	crystalPrompt := DefaultSemanticDistiller().FormatCrystalsForPrompt(retrievedCrystals)

	// 2. 组装盘前前瞻 + ETF轮动 Prompt (包含 ETF轮动多Agent分析范式)
	prompt := fmt.Sprintf(`你现在是买方宏观与大类资产配置委员会（L1 Macro Allocation Committee）主任委员。今天是 %s。
请严格按照【ETF轮动多Agent分析】实战范式，结合以下一手收集的盘前全球指数、核心ETF动态与要闻资讯，输出一份高水准【A股盘前智能前瞻与ETF轮动内参】：

【隔夜及全球主要指数行情】：
%s

【核心ETF池最新状态】：
%s

【核心ETF池纯数学动量矩阵（21日加权对数回归）】：
%s

【最新宏观与隔夜核心要闻】：
%s
%s
请按以下五维结构输出 Markdown 格式的盘前内参：
### 🌐 一、隔夜外盘与全球资产联动
- 欧美股市、中概金龙指数表现及对A股开盘情绪的影响；
- 国内宏观面：PMI、CPI、财政政策、汇率波动；
- 商品市场：原油、黄金、有色金属、农产品关联性；

### 📊 二、核心ETF池量化动态
- 上表已给出纯数学动量排名（21日加权对数回归得分 = 年化收益率 × R²），请直接使用此矩阵进行后续分析；
- 重点观察：动量排名变化、5日回撤情况、是否站上MA5、风控拦截状态；

### 🔍 三、核心ETF池六路并发因子诊断（ETF轮动多Agent分析范式）
- 技术面：60日均线系统、MACD、RSI、布林带位置、成交量能；
- 主力资金流：大单净流入、北向资金、基金申购赎回、融资融券变动；
- 消息舆情：利好利空新闻热度、互动易问答、机构调研频次；
- 外围联动：美股ETF期货、原油铜价、汇率人民币中间价、VIX恐慌指数；
- 宏观行业周期：PMI景气链、利率债收益率曲线、产业政策执行力度；
- 溢价率分析：场内价格与净值差异、基金规模变化、份额持有结构；

### 📋 四、投委会五视角加权评分表（趋势30%% 风控20%% 价值15%% 筹码20%% 情绪15%%）
- 请基于上述六路因子诊断结果，对前3只动量排名靠前的ETF（或未被风控拦截的）逐项打分；
- 最终输出一份详细的投委会打分矩阵与持仓建议；

### 🎯 五、最终交易决策
- 给出明确的仓位建议（买入/持有/观望/减仓），并制定防守止损位；
- 如遇风控拦截（回撤冷却或盈亏比不足），则明确给出观望或等待回调的建议；

【数据注入说明与边界铁律】：
1. 隔夜外盘行情与财经快讯均已在上方详尽提供，无需额外调用外部工具。
2. 提示：核心 ETF 池的 21 日加权对数动量与硬风控状态机已在上方给出纯数学计算结果，更深入的 ETF 轮动换仓决策由独立的量化模块执行。
3. 铁律：本盘前晨报仅作为全球宏观联动与客观隔夜事件参考，严禁给出任何具体个股的买入建议或预测当日大盘精准收盘点位！
全文请使用专业券商内参语气，数据引用请保持客观，避免过度解读。`, todayStr, globalBrief, etfSummary.String(), momentumTable, newsText.String(), crystalPrompt)

	sysPrompt := "你现在是买方宏观与大类资产配置委员会（L1 Macro Allocation Committee）主任委员。所有核心客观数据（隔夜外盘、ETF行情与纯数学动量矩阵、隔夜财经要闻）已在用户输入中详尽提供，无需额外调用外部工具。请直接基于输入数据，按照要求的五维体系输出深度结构化的专业 Markdown 研报。"
	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	var streamErr string

	// 每轮生成用独立的 10 分钟上下文（见 runReportGeneration）：重试轮
	// 不继承首轮已消耗的超时；标注运行类型便于区分"到底是谁在跑"。
	// 使用独立生成上下文，解耦 HTTP 请求临时断连，确保后台研报稳定生成与持久化。
	newGenCtx := func() (context.Context, context.CancelFunc) {
		genCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		return WithRunKind(genCtx, RunKindPreMarket), cancel
	}

	streamRes := s.runReportGeneration(ctx, prompt, sysPrompt, aiConfigId, "盘前晨报", newGenCtx)
	if streamRes.Truncated {
		return nil, fmt.Errorf("AI 研报生成被截断（finish_reason=%s，输出长度达到上限），当前内容不完整。"+
			"请到「设置 → AI模型注册表」展开该卡片的「⚙️ 高级调度参数」，调大「最大输出长度 (maxTokens)」后重试。", streamRes.FinishReason)
	}
	reasoningBuilder.WriteString(streamRes.Reasoning)
	contentBuilder.WriteString(streamRes.Content)
	streamErr = streamRes.StreamErr

	if streamErr != "" {
		return nil, fmt.Errorf("AI 盘前晨报生成中断: %s", streamErr)
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	rawContent := strings.TrimSpace(contentBuilder.String())
	if valid, errMsg := isValidMarketReport(rawContent); !valid {
		return nil, fmt.Errorf("AI 生成盘前晨报失败: %s", errMsg)
	}

	title := fmt.Sprintf("🌅 A股盘前智能前瞻与ETF轮动内参 (%s)", todayStr)
	summary := "隔夜全球资产联动、核心ETF 5视角加权评分与今日开盘风向标策略指引。"

	report := &MarketIntelligenceReport{
		Type:        "pre_market_briefing",
		Title:       title,
		Summary:     summary,
		Content:     finalReport,
		Actions:     actions,
		GeneratedAt: time.Now(),
		TradingDate: todayStr,
		Source:      "Agent盘前与ETF轮动引擎",
	}

	// 持久化到 ai_response_result 归档，标明 code 为 ETF 组合（使用独立 Background 上下文，防止客户端临时断连导致落库中断）
	data.NewDeepSeekOpenAi(context.Background(), aiConfigId).SaveAIResponseResult("510300.SH", "ETF轮动盘前内参", finalReport, "pre_market_etf_"+todayStr, prompt)
	go ExtractAndSaveRecommendationsFromReport(report)

	cacheReport(report)

	// 桌面事件与 Windows 通知提醒
	safeRuntimeEventsEmit(s.ctx, "marketIntelligenceReport", report)

	return report, nil
}

// TriggerSectorAnomalySerenityAnalysis 当盘中出现资金大额流入或异动突发时，自动触发 Serenity 产业链深度拆解
func (s *MarketLifecycleService) TriggerSectorAnomalySerenityAnalysis(ctx context.Context, sectorOrStockName string, code string, reason string, aiConfigId int) (*MarketIntelligenceReport, error) {
	sectorOrStockName = strings.TrimSpace(sectorOrStockName)
	if sectorOrStockName == "" {
		sectorOrStockName = "半导体核心赛道"
	}
	code = strings.TrimSpace(code)
	if reason == "" {
		reason = "用户主动发起的Serenity深度产业链调研"
	}
	logger.SugaredLogger.Warnf("🔥 【Serenity异动拆解】检测到异动 %s(%s): %s，触发Serenity产业链分析", sectorOrStockName, code, reason)
	todayStr := time.Now().Format("2006-01-02")

	if aiConfigId <= 0 {
		settingConfig := data.GetSettingConfig()
		if settingConfig != nil && settingConfig.AiConfigID > 0 {
			aiConfigId = int(settingConfig.AiConfigID)
		}
	}

	// 预提取个股或板块真实数据，消除模型幻觉并极大加速研报合成
	var realDataCtx strings.Builder
	var targetStockCode = code
	if targetStockCode == "" {
		if m := regexp.MustCompile(`[036]\d{5}|51\d{4}|58\d{4}|15\d{4}`).FindString(sectorOrStockName); m != "" {
			targetStockCode = m
		}
	}

	stockAPI := data.NewStockDataApi()
	if targetStockCode != "" {
		// 若为指定个股，注入其最新行情与财务估值
		if quotes, err := stockAPI.GetStockCodeRealTimeData(targetStockCode); err == nil && quotes != nil && len(*quotes) > 0 {
			q := (*quotes)[0]
			realDataCtx.WriteString(fmt.Sprintf("【目标标的真实盘面】%s(%s): 现价:%s 涨跌幅:%.2f%% 昨收:%s 最高:%s 最低:%s 成交额:%s\n",
				q.Name, q.Code, q.Price, q.ChangePercent, q.PreClose, q.High, q.Low, q.Amount))
		}
		// 注入第一级证据：上市公司真实法定公告 (最高权威)
		notices := data.NewMarketNewsApi().StockNotice(targetStockCode)
		if len(notices) > 0 {
			realDataCtx.WriteString("【第一级证据：上市公司法定公告 (StockNotice 官方披露，最高权威)】\n")
			count := 0
			for _, item := range notices {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				title, _ := m["title"].(string)
				date, _ := m["notice_date"].(string)
				if len(date) > 10 {
					date = date[:10]
				}
				cat := "临时公告"
				if cols, ok := m["columns"].([]any); ok && len(cols) > 0 {
					if colMap, ok := cols[0].(map[string]any); ok {
						if cName, ok := colMap["column_name"].(string); ok {
							cat = cName
						}
					}
				}
				realDataCtx.WriteString(fmt.Sprintf("- [%s] %s (%s)\n", date, title, cat))
				count++
				if count >= 10 {
					break
				}
			}
			realDataCtx.WriteString("\n")
		}
		// 注入第二级证据：定期报告财务与估值
		if finMd := stockAPI.GetStockLatestFinanceToMarkdown(targetStockCode); finMd != "" && !strings.HasPrefix(finMd, "获取") {
			realDataCtx.WriteString("【第二级证据：定期报告财务指标与研发/现金流】\n" + finMd + "\n\n")
		}
		if valMd := stockAPI.GetStockValuationPercentileToMarkdown(targetStockCode); valMd != "" && !strings.HasPrefix(valMd, "获取") {
			realDataCtx.WriteString("【估值分位参考】\n" + valMd + "\n\n")
		}
	} else {
		// 若为行业或赛道板块，注入行业板块资金流向以及相关宏观要闻
		industryRanks := data.NewMarketNewsApi().GetIndustryMoneyRankSina("0", "netamount")
		if len(industryRanks) > 0 {
			var rankLines []string
			for i, r := range industryRanks {
				if i >= 6 {
					break
				}
				name, _ := r["name"].(string)
				netAmount, _ := r["netamount"].(string)
				rankLines = append(rankLines, fmt.Sprintf("- %s: 主力净额 %s", name, formatYuanAmount(netAmount)))
			}
			if len(rankLines) > 0 {
				realDataCtx.WriteString("【今日行业板块主力资金流向参考】:\n" + strings.Join(rankLines, "\n") + "\n\n")
			}
		}

		telegraphs := data.NewMarketNewsApi().TelegraphList(15)
		if telegraphs != nil {
			var matchedNews []string
			for _, n := range *telegraphs {
				if strings.Contains(n.Content, sectorOrStockName) || strings.Contains(n.Title, sectorOrStockName) {
					matchedNews = append(matchedNews, fmt.Sprintf("- [%s] %s", n.Time, n.Content))
				}
			}
			if len(matchedNews) > 0 {
				realDataCtx.WriteString("【第三级证据：相关产业链要闻脉冲】:\n" + strings.Join(matchedNews, "\n") + "\n\n")
			}
		}
	}

	prompt := fmt.Sprintf(`你是产业中观与产业链稀缺性研究组（L2 Industry & Supply-Chain Bottleneck）资深研究员。今天是 %s。
目标研判赛道/标的：【%s (%s)】
触发动因：%s。

%s

【必须严格遵守的 Serenity 投研铁律 (Serenity 原作规范)】：
1. 【产业链全景拆解】：梳理%s在产业链8层（终端需求、系统集成、模块子系统、芯片器件、工艺封测、设备量测、材料耗材、物理基础设施）中的位置及主要玩家；
2. 【稀缺层识别】：识别其中技术壁垒最高、验证周期长、扩产约束最紧的绝对稀缺环节；
3. 【严禁综合打分，只输出定性优先级】：
   - 严禁计算综合得分（严禁出现"综合得分 9.0"、"稀缺性 9.5"等伪量化数学游戏）；
   - 用文字清晰解释相对差异，仅输出定性优先级：
     * P0（控制稀缺层，卡住关键瓶颈，公告证据强）
     * P1（供应稀缺层，核心供应商，逻辑成立）
     * P2（需求受益但控制力有限，定价权弱）
     * P3（有故事缺证据，背景参考）
4. 【证据天梯 (Evidence Ladder) 溯源】：
   - 区分证据等级：第一级（法定公告与申报）、第二级（定期报告财务）、第三级（行业快讯）；
   - 引用核心财务与经营数据时必须标明报告期或出处锚点（如 [来源: 2025年报]、[来源: 临时公告]）；
   - 严禁凭空捏造无事实来源的百分比（如虚构市占率 78.4%%、替代率等）；
5. 【必须坦承研究缺口 (Research Gaps)】：
   - 查不到公开披露或缺少公告验证的关键细节（如排他性长协协议、具体大客户采购份额），必须诚实列为【研究缺口 (Research Gaps)】，严禁用臆测填补；
6. 【严格区分商业化六阶段】：
   - 必须严格区分：研发中、送样中、产线验证中、量产试制、在手订单、已确认收入；
7. 【明确证伪条件 (Counterarguments)】：
   - 详细说明什么具体事实发生时（如关键零部件被替代、毛利率跌破阈值、二供份额超预期），本投资假说失效；
8. 【结构化 Thesis 输出】：
   在报告末尾必须输出以下合法 JSON 格式的代码块（用于直通存入买方认知账本）：
`+"```thesis\n"+`{
  "stockCode": "%s",
  "stockName": "%s",
  "thesis": "一句话核心投资假说与瓶颈卡点定论",
  "verdict": "BUY / OBSERVE / AVOID",
  "evidenceChain": "官方公告与定期报告证据摘要",
  "researchGaps": "未被公告证实的关键缺口",
  "counterarguments": "何时假说被证伪的触发条件",
  "trackingChecklist": "后续需跟踪的关键公告或节点"
}
`+"```\n\n"+`输出结构严密、文字克制、求真务实的 Markdown 产业链深度拆解报告。`, todayStr, sectorOrStockName, code, reason, realDataCtx.String(), sectorOrStockName, targetStockCode, sectorOrStockName)

	sysPrompt := "你现在是产业中观与产业链稀缺性研究组（L2 Industry & Supply-Chain Bottleneck）资深研究员。请直接根据输入中详实的一手异动与赛道信息，按照 Serenity 式产业研究框架输出深度结构化的 Markdown 产业链拆解研报。"
	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	var streamErr string

	// 每轮生成用独立的 10 分钟上下文（见 runReportGeneration）；注意：
	// 必须用 WithoutCancel 包住入参 ctx 再派生——直接用 context.Background()
	// 会丢掉调用方预先打上的 RunKindSerenity 标注（GenerateSerenityResearchReport
	// 先标 serenity，这里 WithRunKindIfUnset 本应保留它），导致手动 Serenity
	// 拆解在运行记录里被误记为 sector_anomaly；而直接用 ctx 派生又会把 HTTP
	// 请求的取消信号带进来，后台研报会被前端断连掐断。WithoutCancel 只剥离
	// 取消信号，保留全部 context 值，两全其美。
	newGenCtx := func() (context.Context, context.CancelFunc) {
		genCtx, cancel := context.WithTimeout(context.WithoutCancel(ctxOrBackground(ctx)), 10*time.Minute)
		return WithRunKindIfUnset(genCtx, RunKindSectorAnomaly), cancel
	}

	streamRes := s.runReportGeneration(ctx, prompt, sysPrompt, aiConfigId, "Serenity", newGenCtx)
	if streamRes.Truncated {
		return nil, fmt.Errorf("AI 研报生成被截断（finish_reason=%s，输出长度达到上限），当前内容不完整。"+
			"请到「设置 → AI模型注册表」展开该卡片的「⚙️ 高级调度参数」，调大「最大输出长度 (maxTokens)」后重试。", streamRes.FinishReason)
	}
	reasoningBuilder.WriteString(streamRes.Reasoning)
	contentBuilder.WriteString(streamRes.Content)
	streamErr = streamRes.StreamErr

	if streamErr != "" {
		return nil, fmt.Errorf("Serenity 产业链拆解生成中断: %s", streamErr)
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	rawContent := strings.TrimSpace(contentBuilder.String())
	if valid, errMsg := isValidMarketReport(rawContent); !valid {
		return nil, fmt.Errorf("Serenity 产业链拆解生成失败: %s", errMsg)
	}

	title := fmt.Sprintf("🔍 Serenity产业链深度拆解 — %s (%s)", sectorOrStockName, todayStr)
	summary := fmt.Sprintf("盘中异动触发 (%s)，上中下游稀缺卡口与证伪条件分析。", reason)

	report := &MarketIntelligenceReport{
		Type:        "serenity_industry_analysis",
		Title:       title,
		Summary:     summary,
		Content:     finalReport,
		Actions:     actions,
		GeneratedAt: time.Now(),
		TradingDate: todayStr,
		Source:      "Serenity产业链引擎",
	}

	saveCode := targetStockCode
	if saveCode == "" {
		saveCode = "serenity"
	}
	saveName := fmt.Sprintf("Serenity-%s", sectorOrStockName)
	data.NewDeepSeekOpenAi(context.Background(), aiConfigId).SaveAIResponseResult(saveCode, saveName, finalReport, "serenity_"+todayStr+"_"+saveCode, prompt)
	go ExtractAndSaveRecommendationsFromReport(report)

	// 异步尝试从 Serenity 研报中提炼结构化假说，自动沉淀至买方认知账本 (StockThesisLedger)
	go func(reportContent, defaultCode, defaultName string) {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("Serenity auto thesis save recover: %v", r)
			}
		}()
		re := regexp.MustCompile("(?s)```thesis\\s*\\{?\\s*(.*?)\\s*\\}?\\s*```")
		matches := re.FindStringSubmatch(reportContent)
		if len(matches) > 1 {
			rawJson := "{" + strings.TrimSpace(matches[1]) + "}"
			var parsed struct {
				StockCode         string `json:"stockCode"`
				StockName         string `json:"stockName"`
				Thesis            string `json:"thesis"`
				Verdict           string `json:"verdict"`
				EvidenceChain     string `json:"evidenceChain"`
				ResearchGaps      string `json:"researchGaps"`
				Counterarguments  string `json:"counterarguments"`
				TrackingChecklist string `json:"trackingChecklist"`
			}
			if err := json.Unmarshal([]byte(rawJson), &parsed); err == nil {
				c := parsed.StockCode
				if c == "" {
					c = defaultCode
				}
				n := parsed.StockName
				if n == "" {
					n = defaultName
				}
				if c != "" && c != "serenity" {
					v := models.VerdictNoChange
					vt := "观望"
					switch strings.ToUpper(strings.TrimSpace(parsed.Verdict)) {
					case "BUY", "强烈看多", "买入":
						v = models.VerdictThesisStrengthen
						vt = "买入/逻辑强化"
					case "AVOID", "规避", "离场":
						v = models.VerdictThesisBroken
						vt = "规避/逻辑证伪"
					default:
						v = models.VerdictNewThesis
						vt = "观望/新建档"
					}
					_ = data.NewStockThesisApi().SaveStockThesis(&models.StockThesisLedger{
						StockCode:    c,
						StockName:    n,
						Verdict:      v,
						VerdictText:  vt,
						CoreThesis:   parsed.Thesis,
						KeyCatalysts: fmt.Sprintf("【证据链】%s\n【跟踪项】%s", parsed.EvidenceChain, parsed.TrackingChecklist),
						RiskFactors:  fmt.Sprintf("【研究缺口】%s\n【证伪条件】%s", parsed.ResearchGaps, parsed.Counterarguments),
						ModelName:    "Serenity",
					})
				}
			}
		}
	}(finalReport, targetStockCode, sectorOrStockName)

	cacheReport(report)

	safeRuntimeEventsEmit(s.ctx, "marketIntelligenceReport", report)

	return report, nil
}

// GeneratePostMarketReview 生成收盘盘后复盘与量化战绩 (15:00收盘后触发)
// 包含：全天涨跌分布、行业资金主力、连板梯队热度、AI预测回测胜率验证及明日推演
func (s *MarketLifecycleService) GeneratePostMarketReview(ctx context.Context, aiConfigId int) (*MarketIntelligenceReport, error) {
	now := time.Now()
	// 交易日时段硬核约束：A 股 15:00 闭市。工作日盘前与盘中严禁预判生成尚未收盘交易日的“复盘”研报！
	if now.Weekday() >= time.Monday && now.Weekday() <= time.Friday && now.Hour() < 15 {
		return nil, fmt.Errorf("当前交易日尚未收盘（需等待 15:00 闭市归档），严禁在开盘前或盘中提前生成收盘复盘报告！请于 15:00 闭市数据归档后触发。")
	}

	logger.SugaredLogger.Info("【MarketLifecycle】开始生成智能盘后收盘复盘...")
	todayStr := time.Now().Format("2006-01-02")

	// 1. 采集全天收盘数据与量化统计
	marketStatAPI := data.NewMarketStatisticApi()
	todayStats := marketStatAPI.GetTodayData()
	var statSummary strings.Builder
	if len(todayStats) > 0 {
		latestStat := todayStats[len(todayStats)-1]
		statSummary.WriteString(fmt.Sprintf("上涨: %d家, 下跌: %d家, 涨停: %d家, 跌停: %d家, 市场情绪: %s\n",
			latestStat.UpCount, latestStat.DownCount, latestStat.LimitUp, latestStat.LimitDown, latestStat.SentimentDesc))
	} else {
		statSummary.WriteString("全天涨跌统计正在归档中...\n")
	}

	// 涨停热点与连板板块（转为高信息密度 Markdown 摘要，杜绝大 JSON 提示词爆炸）
	uplimitHot := data.NewMarketNewsApi().GetUplimitHot(todayStr, 15)
	uplimitSummary := FormatUplimitHotToMarkdown(uplimitHot)

	// 2. 异步触发近期研报回测闭环统计，避免阻塞当前研报生成响应
	btMgr := quant.NewBacktestManager()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("RunAIReportBacktest recover: %v", r)
			}
		}()
		_, _ = btMgr.RunAIReportBacktest(context.Background(), 50)
	}()
	btSummary := "历史研报量化回测样本库自动滚动校验中"

	// 检索与盘后大盘相关的经验结晶与避险案例（限定 category 为 market_review，避免误混入无关个股案例）
	retrievedCrystals, _ := DefaultSemanticDistiller().SearchKnowledgeCrystals(ctx, "A股大盘 收盘复盘 涨跌分布 龙虎榜 资金博弈", "market_review", 2)
	crystalPrompt := DefaultSemanticDistiller().FormatCrystalsForPrompt(retrievedCrystals)

	// 3. 组装收盘复盘 Prompt
	prompt := fmt.Sprintf(`你现在是买方投研盘面复盘与量化情绪策略组（L4/L5 Market Sentiment & Post-Market Review）首席分析师。今天是 %s 收盘后。
请基于今日一手盘面全景数据与客观量化统计，直接撰写【A股全景收盘复盘与次日推演报告】：

【全天多空分布与市场情绪】：
%s

【涨停连板与题材热度概要（连板高度梯队与核心标的）】：
%s

【量化研报跟踪】：
%s
%s
请按以下结构直接输出结构化 Markdown 报告：
### 📊 一、全天行情概览与多空情绪定调
- 涨跌家数比、多空力量对比与广谱赚钱效应客观评估；

### 🚀 二、核心主线与资金筹码博弈
- 今日连板梯队高度、最强题材热点与领涨龙头持续性推演；
- 重点指出是否存在"高标断层"、"中位股塌陷"等盘面结构性特征；

### 🧪 三、量化启示与策略认知
- 基于今日盘面特征的量化策略启示与认知纠偏；

### 🔮 四、明日走势推演与操作观察
- 明日开盘观察重点（如高标连板晋级率、中位股分歧承接力）；
- 【铁律】：严禁给出神棍式的概率预测（严禁出现"剧本 A 概率 60%% / 剧本 B 概率 40%%"等无数学根据的臆造数字），所有推演必须以盘面客观事实与确认信号为准；严禁编造未在榜单上的所谓主力席位。

【输出要求】：请直接开始输出 Markdown 研报正文，思考过程简短克制，严禁过度思考，确保正文完整输出。`, todayStr, statSummary.String(), uplimitSummary, btSummary, crystalPrompt)

	sysPrompt := "你现在是买方投研盘面复盘与量化情绪策略组（L4/L5 Market Sentiment & Post-Market Review）首席分析师。所有核心盘面客观数据（涨跌家数比、连板梯队题材、多空情绪指标）已在输入中提供。请直接开始输出 Markdown 研报正文，严禁在思考过程中冗余推导，优先确保报告正文完整输出。"
	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	var streamErr string

	// 每轮生成用独立的 10 分钟上下文（见 runReportGeneration）；
	// 标注本次运行类型（写入运行记录，便于区分"到底是谁在跑"）。
	newGenCtx := func() (context.Context, context.CancelFunc) {
		genCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		return WithRunKind(genCtx, RunKindPostMarket), cancel
	}

	streamRes := s.runReportGeneration(ctx, prompt, sysPrompt, aiConfigId, "盘后复盘", newGenCtx)
	if streamRes.Truncated {
		return nil, fmt.Errorf("AI 研报生成被截断（finish_reason=%s，输出长度达到上限），当前内容不完整。"+
			"请到「设置 → AI模型注册表」展开该卡片的「⚙️ 高级调度参数」，调大「最大输出长度 (maxTokens)」后重试。", streamRes.FinishReason)
	}
	reasoningBuilder.WriteString(streamRes.Reasoning)
	contentBuilder.WriteString(streamRes.Content)
	streamErr = streamRes.StreamErr

	if streamErr != "" {
		return nil, fmt.Errorf("AI 生成收盘复盘中断: %s", streamErr)
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	rawContent := strings.TrimSpace(contentBuilder.String())
	if valid, errMsg := isValidMarketReport(rawContent); !valid {
		return nil, fmt.Errorf("AI 生成收盘复盘失败: %s", errMsg)
	}

	title := fmt.Sprintf("🌆 A股全景收盘复盘与次日推演 (%s)", todayStr)
	summary := fmt.Sprintf("全天行情量价深度复盘、热点资金筹码博弈剖析与次日推演策略。(%s)", btSummary)

	report := &MarketIntelligenceReport{
		Type:        "post_market_review",
		Title:       title,
		Summary:     summary,
		Content:     finalReport,
		Actions:     actions,
		GeneratedAt: time.Now(),
		TradingDate: todayStr,
		Source:      "Agent复盘引擎",
	}

	// 持久化到 ai_response_result（使用独立 Background 上下文，防止客户端临时断连导致落库中断）
	data.NewDeepSeekOpenAi(context.Background(), aiConfigId).SaveAIResponseResult("收盘复盘", "盘后复盘", finalReport, "post_market_"+todayStr, prompt)
	go ExtractAndSaveRecommendationsFromReport(report)

	cacheReport(report)

	// 桌面事件推送
	safeRuntimeEventsEmit(s.ctx, "marketIntelligenceReport", report)

	return report, nil
}

// GenerateETFRotationReport 手动或一键触发ETF轮动量化多Agent分析
func (s *MarketLifecycleService) GenerateETFRotationReport(ctx context.Context, aiConfigId int) (*MarketIntelligenceReport, error) {
	logger.SugaredLogger.Info("【MarketLifecycle】手动一键触发【stevia0829 原生 ETF轮动量化决策分析】...")
	todayStr := time.Now().Format("2006-01-02")

	etfCodes := []string{"510300.SH", "159915.SZ", "512480.SH", "512660.SH", "512010.SH", "512800.SH", "510880.SH", "159949.SZ"}

	// 1. 纯 Go 确定性计算：21日加权对数动量回归斜率与 R² 排名 (0 Token 算力消耗，100% 确定性)
	momentumList, err := quant.BatchCalculateETFMomentum(etfCodes)
	if err != nil {
		logger.SugaredLogger.Warnf("计算ETF动量失败: %v", err)
		momentumList = []*quant.ETFMomentumScoreResult{}
	}
	momentumTable := quant.FormatETFMomentumSummary(momentumList)

	// 2. 纯 Go 确定性计算：510300 沪深300ETF 120日均线与 60日高点回撤硬风控状态机 (stevia0829 核心灵魂)
	regimeStatus, err := quant.CalculateETFRegimeStatus()
	if err != nil {
		logger.SugaredLogger.Warnf("计算大盘硬风控状态机失败: %v", err)
		regimeStatus = &quant.ETFRegimeStatus{
			BenchmarkCode:   "510300.SH",
			BenchmarkName:   "沪深300ETF",
			State:           "DEFENSE",
			StateDesc:       "🛡️ 绝对防守 (数据异常兜底)",
			Reason:          "风控数据拉取异常，安全兜底触发防守",
			SuggestedAction: "保持轻仓防守",
		}
	}

	// 3. 确定候选标的与开盘/盘后跳空复核
	topCandidateCode := "510300.SH"
	topCandidateName := "沪深300ETF"
	for _, m := range momentumList {
		if !m.CooldownGate && m.Score > 0 {
			topCandidateCode = m.Code
			topCandidateName = m.Name
			break
		}
	}
	preOpenCheck, _ := quant.CheckETFPreOpenGap(topCandidateCode)
	regimeSummary := quant.FormatETFRegimeSummary(regimeStatus, preOpenCheck)

	// 4. 组装零虚构、硬核数学注入的 System / User Prompt
	prompt := fmt.Sprintf(`你是由 blades-stock 原生量化引擎（集成 stevia0829 经典 ETF 动量轮动策略框架）赋能的买方量化配置总监。今天是 %s。
系统已在下方为你注入由本地纯 Go 量化引擎严格通过 21 日加权对数线性回归与 510300 沪深300ETF 硬风控状态机计算出的确定性数学结果。

%s

### 📊 核心关注 ETF 轮动池 21日纯数学加权动量矩阵 (代表性 8 标的跨风格/行业池 · stevia0829 对数回归与 R² 滤波)：
说明：本池由 8 只代表性宽基与核心行业 ETF 组成（沪深300、创业板、半导体、军工、医药、银行、红利、创业板50），用于捕捉跨风格与行业轮动，并非全市场 800+ 标的。

%s

【铁律与量化纪律说明】：
1. 【严禁虚增数据与编造打分】：上述动量排名、年化动量(%%)、线性拟合优度 R²、动量得分以及大盘硬风控状态全部由纯 Go 引擎精确计算得出，严禁大模型自行编造任何数字，严禁制作虚假打分矩阵！
2. 【严格服从硬风控状态机 (Regime Gate)】：
   - 当前 510300 大盘硬风控状态为：【%s】（状态枚举：OFFENSE 或 DEFENSE）；
   - 若状态为 DEFENSE（现价 < MA120 或 60日高点回撤 >= 8.0%%）：权益资产做多进攻建议【一票否决】！状态严格定义为【🛡️ 绝对防守避险 (DEFENSE)】，强制建议持币或配置货币 ETF (511880银华日利/511990华宝添益)，权益类资产建议仓位严格归零 (0%%)！严禁混用 AVOID、NO_CHANGE 等非标准状态名。
   - 【防守状态下的标的语义规范】：当处于 DEFENSE 状态时，当前策略的首选资产配置必须明确为【货币ETF (511880/511990) 或空仓持币】；池内动量排名第一的标的【%s (%s)】仅作为【动量领先跟踪观察标的】，严禁表述为即时买入的“首选标的”！必须明确向投资者说明“大盘硬风控熔断，即使个别行业动量尚存也严禁逆势开仓，权益仓位强制归零”。
   - 若状态为 OFFENSE：大盘处于安全右侧，允许将动量排名第一且未触发回撤冷却的领先标的【%s (%s)】列为【首选进攻标的】；
3. 【数学符号与排版规范禁忌】：
   - 所有拟合优度必须以标准 Markdown 纯文本 R² 或 R^2 输出，绝对禁止使用 HTML 尖括号 <R²> 或 <R^2> 包裹，防止被前端安全渲染组件剥离导致文本空洞！
   - 引用动量指标与拟合优度时，必须完整列出纯 Go 计算给出的具体数值（如 R²=%.3f、动量得分等），严禁遗漏数值或出现空白。
4. 【动态防守线与时段核查规范】：
   - 510300 动态跟踪防守位为 %.3f 元 (基于 MA20 %.3f 元动态浮动计算，严禁虚构或套用陈旧历史点位)；
   - 时段判定为【%s】，请客观引用开盘跳空幅度和累计涨跌幅，非集合竞价时段严禁误报为“集合竞价正在进行”。

请直接输出客观、严谨、零废话的 Markdown 格式【ETF轮动量化多Agent决策研报】：
### 🛡️ 一、大盘硬风控状态机判定 (stevia0829 核心)
- 详细解读 510300 现价与 MA120 均线位置、60日回撤幅度，明确当前多空防守边界与动态防守位(%.3f 元)；

### 📈 二、21日加权对数动量池全景解剖
- 基于纯数学动量矩阵，阐述池内表现相对领先标的（如 %s）的宏观流动性与产业催化背景；
- 明确指出哪些标的触发了 5 日回撤冷却拦截（Cooldown Gate），说明为何剔除；
- 【低波防御与红利资产特性提示】：银行 ETF (512800) 与红利 ETF (510880) 因天然低波动特性，年化回归斜率数值通常较小，但其 R² 拟合优度与抗跌韧性往往极高。在弱市或大盘处于 DEFENSE 周期时，客观解读此类低波资产的防御属性，不得仅依据绝对动量得分高低做单一维度的片面定性；

### 🎯 三、最终交易与仓位执行指令
- 明确给出仓位上限百分比（若 DEFENSE 则权益仓位为 0%%，持有货币ETF/持币）与具体代码；
- 制定精准的防守止损线与逻辑证伪底线。`,
		todayStr,
		regimeSummary,
		momentumTable,
		regimeStatus.StateDesc,
		topCandidateName, topCandidateCode,
		topCandidateName, topCandidateCode,
		0.859, // 示范格式
		regimeStatus.DynamicDefensePrice, regimeStatus.MA20,
		preOpenCheck.SessionName,
		regimeStatus.DynamicDefensePrice,
		topCandidateName)

	sysPrompt := "你现在是 blades-stock 原生量化投资总监（践行 stevia0829/multi-agents-etf-trade-strategy 经典策略框架）。所有动量得分、R²、回撤与大盘硬风控状态已由本地纯 Go 引擎确定性计算完毕，严禁虚增或篡改任何数学数据，交易决策必须严格服从 Go 引擎的硬风控状态机。"
	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	var streamErr string

	// 每轮生成用独立的 10 分钟上下文（见 runReportGeneration）；
	// 标注本次运行类型（写入运行记录，便于区分"到底是谁在跑"）。
	newGenCtx := func() (context.Context, context.CancelFunc) {
		genCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		return WithRunKind(genCtx, RunKindETFRotation), cancel
	}

	streamRes := s.runReportGeneration(ctx, prompt, sysPrompt, aiConfigId, "ETF轮动", newGenCtx)
	if streamRes.Truncated {
		return nil, fmt.Errorf("AI 研报生成被截断（finish_reason=%s，输出长度达到上限），当前内容不完整。"+
			"请到「设置 → AI模型注册表」展开该卡片的「⚙️ 高级调度参数」，调大「最大输出长度 (maxTokens)」后重试。", streamRes.FinishReason)
	}
	reasoningBuilder.WriteString(streamRes.Reasoning)
	contentBuilder.WriteString(streamRes.Content)
	streamErr = streamRes.StreamErr

	if streamErr != "" {
		return nil, fmt.Errorf("生成ETF轮动分析报告中断: %s", streamErr)
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	rawContent := strings.TrimSpace(contentBuilder.String())
	if valid, errMsg := isValidMarketReport(rawContent); !valid {
		return nil, fmt.Errorf("生成ETF轮动分析报告失败: %s", errMsg)
	}

	title := fmt.Sprintf("📈 ETF动量轮动与大盘硬风控研判 (%s)", todayStr)
	var summary string
	if regimeStatus.State == "DEFENSE" {
		summary = fmt.Sprintf("blades-stock量化(stevia0829)：510300处于【DEFENSE】绝对防守避险态，权益建议仓位【0%%】(建议持有货币ETF或持币)，池内动量领先观察【%s (%s)】",
			topCandidateName, topCandidateCode)
	} else {
		summary = fmt.Sprintf("blades-stock量化(stevia0829)：510300处于【OFFENSE】允许动量进攻态，首选进攻标的【%s (%s)】",
			topCandidateName, topCandidateCode)
	}

	report := &MarketIntelligenceReport{
		Type:        "etf_rotation_decision",
		Title:       title,
		Summary:     summary,
		Content:     finalReport,
		Actions:     actions,
		GeneratedAt: time.Now(),
		TradingDate: todayStr,
		Source:      "blades-stock-ETF动量轮动引擎(stevia0829)",
	}

	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult("510300.SH", "ETF轮动量化决策", finalReport, "etf_rotation_"+todayStr, prompt)
	go ExtractAndSaveRecommendationsFromReport(report)

	cacheReport(report)

	safeRuntimeEventsEmit(s.ctx, "marketIntelligenceReport", report)

	return report, nil
}

// GenerateSerenityResearchReport 手动一键触发任意板块/股票的 Serenity 产业链深度拆解
func (s *MarketLifecycleService) GenerateSerenityResearchReport(ctx context.Context, targetName string, code string, aiConfigId int) (*MarketIntelligenceReport, error) {
	// 先标上更精确的 kind：内部复用的通用入口用 WithRunKindIfUnset，不会把它覆盖
	return s.TriggerSectorAnomalySerenityAnalysis(
		WithRunKind(ctx, RunKindSerenity),
		targetName, code, "用户主动发起的Serenity深度产业链调研", aiConfigId)
}

// GetLatestReport 获取最新生成的智能研报（盘前或盘后）
func (s *MarketLifecycleService) GetLatestReport() *MarketIntelligenceReport {
	lifecycleReportCacheMu.RLock()
	latest := lifecycleLatestReport
	lifecycleReportCacheMu.RUnlock()
	if latest != nil {
		if valid, _ := isValidMarketReport(latest.Content); valid {
			return latest
		}
		// 内存缓存已污染，主动失效清除
		lifecycleReportCacheMu.Lock()
		lifecycleLatestReport = nil
		lifecycleReportCacheMu.Unlock()
	}
	return s.GetLatestReportByType("pre_market_briefing")
}

// GetLatestReportByType 获取指定类型（盘前/盘后/ETF轮动/Serenity）的最新一份智能研报
func (s *MarketLifecycleService) GetLatestReportByType(reportType string) *MarketIntelligenceReport {
	lifecycleReportCacheMu.RLock()
	if lifecycleReportsByType != nil {
		if r, ok := lifecycleReportsByType[reportType]; ok && r != nil {
			if valid, _ := isValidMarketReport(r.Content); valid {
				lifecycleReportCacheMu.RUnlock()
				return r
			}
			lifecycleReportCacheMu.RUnlock()
			lifecycleReportCacheMu.Lock()
			delete(lifecycleReportsByType, reportType)
			lifecycleReportCacheMu.Unlock()
		} else {
			lifecycleReportCacheMu.RUnlock()
		}
	} else {
		lifecycleReportCacheMu.RUnlock()
	}

	if db.Dao == nil {
		return nil
	}

	// 归档归属判定：**chat_id 前缀是唯一事实源**（由各 Generate* 方法写入）。
	//
	// 旧实现用「chat_id LIKE %x% OR stock_name LIKE %y%」双条件，造成两个真实故障：
	//  ① 跨类型串号：etf_rotation_decision 会因 stock_name LIKE '%ETF轮动%'
	//     把「ETF轮动盘前内参」(chat_id=pre_market_etf_…) 当成自己的报告返回 ——
	//     用户以为“新的分析没生成，看到的还是老报告”；
	//  ② 删不掉：ClearReportAndArchive 只按 chat_id 前缀删除，而读路径靠 stock_name
	//     又能把同一行捞回来。
	// 现在读取与删除**共用同一套前缀**，两者严格一致。
	var items []models.AIResponseResult
	var query *gorm.DB
	if prefixes, known := marketReportArchivePrefix[reportType]; known {
		query = db.Dao.Where("1 = 0")
		for _, p := range prefixes {
			query = query.Or("chat_id LIKE ?", p+"%")
		}
	} else {
		// 未知类型保留原有兜底行为
		query = db.Dao.Where("stock_code IN (?, ?) OR chat_id LIKE ?", "盘前前瞻", "收盘复盘", "%market%")
	}

	err := query.Order("created_at DESC").Limit(15).Find(&items).Error
	if err == nil && len(items) > 0 {
		for _, item := range items {
			if valid, errReason := isValidMarketReport(item.Content); !valid {
				logger.SugaredLogger.Warnf("[MarketLifecycle] 检测到历史污染或异常研报快照 ID=%d (原因: %s)，自动从数据库物理清除", item.ID, errReason)
				db.Dao.Delete(&item)
				continue
			}
			_, actions := types.ParseAndExtractActions(item.Content)
			title := item.StockName
			if title == "" {
				title = "智能研报"
			}
			if !strings.Contains(title, item.CreatedAt.Format("2006-01-02")) {
				title = fmt.Sprintf("%s (%s)", title, item.CreatedAt.Format("2006-01-02"))
			}
			rep := &MarketIntelligenceReport{
				Type:        reportType,
				Title:       title,
				Summary:     "历史智能分析报告存档",
				Content:     item.Content,
				Actions:     actions,
				GeneratedAt: item.CreatedAt,
				TradingDate: item.CreatedAt.Format("2006-01-02"),
				Source:      item.ModelName,
			}
			// 回填内存缓存加速后续读取
			cacheReport(rep)
			return rep
		}
	}

	return nil
}

// PurgeCorruptedReports 一键物理扫描并清理数据库与内存中所有因网络中断、超时、网关报错等产生的残损研报记录
// ClearReport 清除指定类型的市场智能研报（reportType 为空表示清除全部）。
//
// 为什么需要这个能力（用户反馈「删除旧的报告都没有办法」）：
//
//	研报只存在进程内存（lifecycleReportsByType）并通过 SSE 广播，
//	在此之前**既无 API 也无 UI** 可以清除它 —— 用户只能重启容器。
//	更糟的是抽屉在挂载/切页时会主动拉取“最新研报”，所以一份早该作废的
//	旧报告会被反复回显，看起来“删不掉也换不掉”。
//
// 返回值：实际清除的报告数量。清除后会广播 marketReportCleared，
// 让所有已打开页面立即移除，而不是刷新后才生效。
func ClearReport(reportType string) int {
	lifecycleReportCacheMu.Lock()
	cleared := 0
	if strings.TrimSpace(reportType) == "" {
		cleared = len(lifecycleReportsByType)
		lifecycleReportsByType = make(map[string]*MarketIntelligenceReport)
		lifecycleLatestReport = nil
	} else if _, ok := lifecycleReportsByType[reportType]; ok {
		delete(lifecycleReportsByType, reportType)
		cleared = 1
		if lifecycleLatestReport != nil && lifecycleLatestReport.Type == reportType {
			lifecycleLatestReport = nil
		}
	}
	lifecycleReportCacheMu.Unlock()

	if cleared > 0 {
		logger.SugaredLogger.Infof("[MarketLifecycle] 已清除研报缓存 type=%q，共 %d 份", reportType, cleared)
		if MarketReportBroadcastHook != nil {
			MarketReportBroadcastHook("marketReportCleared", map[string]any{"type": reportType})
		}
	}
	return cleared
}

// marketReportArchivePrefix 各类型研报在归档表 ai_response_result 中的 chat_id 前缀。
//
// **单一事实源**：读取兜底（GetLatestReportByType）与删除（ClearReportAndArchive）
// 必须共用它，否则就会出现“删掉了又被读回来”。
// 与各 Generate* 方法里 SaveAIResponseResult 的 chatId 参数必须保持一致。
var marketReportArchivePrefix = map[string][]string{
	"pre_market_briefing":        {"pre_market_etf_"},
	"post_market_review":         {"post_market_"},
	"etf_rotation_decision":      {"etf_rotation_"},
	"serenity_industry_analysis": {"serenity_"},
}

// ClearReportAndArchive 清除指定类型的研报：内存缓存 + 归档表历史记录。
// reportType 为空表示全部类型。返回面向用户的结论文案。
func ClearReportAndArchive(reportType string) string {
	reportType = strings.TrimSpace(reportType)
	cacheCleared := ClearReport(reportType)

	archiveDeleted := int64(0)
	if db.Dao != nil {
		prefixes := []string{}
		if reportType == "" {
			for _, ps := range marketReportArchivePrefix {
				prefixes = append(prefixes, ps...)
			}
		} else {
			prefixes = append(prefixes, marketReportArchivePrefix[reportType]...)
		}

		if len(prefixes) > 0 {
			q := db.Dao.Where("1 = 0")
			for _, p := range prefixes {
				q = q.Or("chat_id LIKE ?", p+"%")
			}
			res := db.Dao.Unscoped().Where(q).Delete(&models.AIResponseResult{})
			if res.Error != nil {
				logger.SugaredLogger.Errorf("[MarketLifecycle] 删除研报归档失败: %v", res.Error)
				return fmt.Sprintf("❌ 内存研报已清除 %d 份，但删除归档记录失败：%v", cacheCleared, res.Error)
			}
			archiveDeleted = res.RowsAffected
		}
	}

	if reportType == "" {
		return fmt.Sprintf("✅ 已清除全部市场智能研报：内存 %d 份，归档 %d 条", cacheCleared, archiveDeleted)
	}
	return fmt.Sprintf("✅ 已清除该类型研报：内存 %d 份，归档 %d 条", cacheCleared, archiveDeleted)
}

func PurgeCorruptedReports() int {
	lifecycleReportCacheMu.Lock()
	lifecycleLatestReport = nil
	lifecycleReportsByType = make(map[string]*MarketIntelligenceReport)
	lifecycleReportCacheMu.Unlock()

	if db.Dao == nil {
		return 0
	}
	var dirtyItems []models.AIResponseResult
	err := db.Dao.Where("content LIKE ? OR content LIKE ? OR content LIKE ? OR content LIKE ? OR content LIKE ? OR content LIKE ?",
		"%❌%", "%Agent 调用失败%", "%context canceled%", "%context deadline exceeded%", "%502 bad gateway%", "%出现错误：502%").
		Find(&dirtyItems).Error
	if err != nil || len(dirtyItems) == 0 {
		return 0
	}
	count := 0
	for _, item := range dirtyItems {
		db.Dao.Delete(&item)
		count++
	}
	logger.SugaredLogger.Infof("[MarketLifecycle] 已物理清洗 %d 条历史网络中断或网关超时产生的脏研报记录", count)
	return count
}

// ── 以下为 collectReportStream 的回归测试 ────────────────────────────────
