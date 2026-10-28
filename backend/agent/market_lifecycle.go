package agent

import (
	"context"
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
	PhasePreMarket        MarketPhase = "pre_market"         // 盘前前瞻 (08:30 - 09:15)
	PhaseCallAuction      MarketPhase = "call_auction"       // 开盘集合竞价 (09:15 - 09:30)
	PhaseMorningTrading   MarketPhase = "morning_trading"    // 盘中早盘 (09:30 - 11:30)
	PhaseNoonBreak        MarketPhase = "noon_break"         // 午间休市 (11:30 - 13:00)
	PhaseAfternoonTrading MarketPhase = "afternoon_trading"  // 盘中下午盘 (13:00 - 15:00)
	PhasePostMarket       MarketPhase = "post_market"        // 盘后复盘 (15:00 - 18:00)
	PhaseClosed           MarketPhase = "closed"             // 夜间/非交易日休市
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
	Type        string    `json:"type"`        // "pre_market_briefing" 或 "post_market_review"
	Title       string    `json:"title"`       // 标题
	Summary     string    `json:"summary"`     // 核心要点
	Content     string              `json:"content"`     // 完整Markdown内容
	Actions     []types.AgentAction `json:"actions,omitempty"` // 报告伴随行动项指令
	GeneratedAt time.Time           `json:"generatedAt"` // 生成时间
	TradingDate string    `json:"tradingDate"` // 归属交易日
	Source      string    `json:"source"`      // 模型来源
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
func isValidMarketReport(content string) (bool, string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false, "模型未返回任何有效内容，请检查大模型配置或网络"
	}
	// 任何位置包含 ❌ 或“Agent 调用失败”均视为异常中断，严禁存库与呈现
	if strings.Contains(trimmed, "❌") || strings.Contains(trimmed, "Agent 调用失败") {
		return false, trimmed
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
		strings.Contains(lower, "antigravity") {
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
}

func NewMarketLifecycleService(ctx context.Context) *MarketLifecycleService {
	return &MarketLifecycleService{ctx: ctx}
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

	var cleanReasoningLines []string
	if reasoning != "" {
		for _, line := range strings.Split(reasoning, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[STATUS]") || strings.Contains(trimmed, "[STATUS]") {
				continue
			}
			cleanReasoningLines = append(cleanReasoningLines, line)
		}
	}
	cleanReasoning := strings.TrimSpace(strings.Join(cleanReasoningLines, "\n"))

	if cleanReasoning != "" {
		return fmt.Sprintf("<details style=\"margin-bottom:16px;padding:10px 14px;background:rgba(128,128,128,0.08);border-radius:8px;border-left:3px solid #18a058;\">\n<summary style=\"cursor:pointer;font-weight:600;color:inherit;opacity:0.85;\">💭 展开查看思考过程与推理链条</summary>\n\n%s\n</details>\n\n%s", cleanReasoning, cleanedContent), actions
	}
	return cleanedContent, actions
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

	// 采集主流行业与宽基 ETF 实时/日线行情
	etfCodes := []string{"510300.SH", "159915.SZ", "512480.SH", "512660.SH", "512010.SH", "512800.SH", "510880.SH"}
	var etfSummary strings.Builder
	etfSummary.WriteString("核心关注ETF池（沪深300、创业板、半导体、军工、医药、证券、红利）：\n")
	stockAPI := data.NewStockDataApi()
	for _, code := range etfCodes {
		quotes, err := stockAPI.GetStockCodeRealTimeData(code)
		if err == nil && quotes != nil && len(*quotes) > 0 {
			q := (*quotes)[0]
			etfSummary.WriteString(fmt.Sprintf("- %s(%s): 现价 %v, 今日/昨日涨跌幅 %v%%\n", q.Name, code, q.Price, q.ChangePercent))
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

	// 2. 组装盘前前瞻 + ETF轮动 Prompt (包含 Skill 7 范式)
	prompt := fmt.Sprintf(`你现在是顶级券商首席策略分析师兼量化宏观专家“财智”。今天是 %s。
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

### 🔍 三、核心ETF池六路并发因子诊断（Skill 7 范式）
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

【数据注入说明】：隔夜外盘行情、核心关注ETF池最新状态、纯数学动量矩阵（21日加权对数回归）以及隔夜宏观核心要闻已在上方详尽提供，无需额外调用外部工具，请直接基于上述一手真实数据进行深度量化推演与结构化研报生成。
全文请使用专业券商内参语气，数据引用请保持客观，避免过度解读。`, todayStr, globalBrief, etfSummary.String(), momentumTable, newsText.String(), crystalPrompt)

	sysPrompt := "你现在是顶级券商首席策略分析师兼量化宏观专家“财智”。所有核心客观数据（隔夜外盘、ETF行情与纯数学动量矩阵、隔夜财经要闻）已在用户输入中详尽提供，无需额外调用外部工具。请直接基于输入数据，按照要求的五维体系输出深度结构化的专业 Markdown 研报。"
	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	var streamErr string

	// 使用带合理超时 (10分钟) 的独立生成上下文，解耦 HTTP 请求临时断连，确保后台研报稳定生成与持久化
	genCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ch := NewStockAiAgentApi().ChatWithContext(genCtx, prompt, aiConfigId, nil, false, 0, false, string(AgentModeDirect), sysPrompt)
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			reasoningBuilder.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" && msg.Content != "agent-DONE" {
			// 若流中捕获到底层抛出的错误消息，绝不混入正文内容
			if strings.HasPrefix(msg.Content, "❌") || strings.Contains(msg.Content, "Agent 调用失败") {
				streamErr = msg.Content
				continue
			}
			contentBuilder.WriteString(msg.Content)
		}
	}

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

	// 持久化到 ai_response_result 归档，标明 code 为 ETF 组合
	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult("510300.SH", "ETF轮动盘前内参", finalReport, "pre_market_etf_"+todayStr, prompt)
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
		if finMd := stockAPI.GetStockLatestFinanceToMarkdown(targetStockCode); finMd != "" && !strings.HasPrefix(finMd, "获取") {
			realDataCtx.WriteString("【最新财务指标】\n" + finMd + "\n\n")
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
				realDataCtx.WriteString("【相关产业链要闻脉冲】:\n" + strings.Join(matchedNews, "\n") + "\n\n")
			}
		}
	}

	prompt := fmt.Sprintf(`你是专注于产业链瓶颈与卡脖子技术的研究专家“财智”。今天是 %s。
目标研判赛道/标的：【%s (%s)】
触发动因：%s。

%s
请严格遵守【Serenity式产业分析框架】（Skill 4）：
1. 【产业链全景拆解】：梳理%s在产业链中的位置（上中下游与核心参与者）；
2. 【稀缺层识别】：识别其中技术壁垒最高、国产替代率<30%%或CR5>60%%的绝对稀缺环节；
3. 【优先级打分】：按稀缺性、可投资性、确定性三维度打分并排序；
4. 【证据链验证】：列出营收增速、毛利率或重要客户支撑；
5. 【证伪条件】：列出结论失效的触发条件。

【数据注入说明】：请直接基于上述一手真实数据及专业产业认知进行深度量化推演，无需额外调用外部工具。
输出结构清晰、带有深度洞见的 Markdown 产业链深度拆解报告。`, todayStr, sectorOrStockName, code, reason, realDataCtx.String(), sectorOrStockName)

	sysPrompt := "你现在是专注于产业链瓶颈与卡脖子技术的研究专家“财智”。请直接根据输入中详实的一手异动与赛道信息，按照 Serenity 式产业研究框架输出深度结构化的 Markdown 产业链拆解研报。"
	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	var streamErr string

	genCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ch := NewStockAiAgentApi().ChatWithContext(genCtx, prompt, aiConfigId, nil, false, 0, false, string(AgentModeDirect), sysPrompt)
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			reasoningBuilder.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" && msg.Content != "agent-DONE" {
			if strings.HasPrefix(msg.Content, "❌") || strings.Contains(msg.Content, "Agent 调用失败") {
				streamErr = msg.Content
				continue
			}
			contentBuilder.WriteString(msg.Content)
		}
	}

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
	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult(saveCode, saveName, finalReport, "serenity_"+todayStr+"_"+saveCode, prompt)
	go ExtractAndSaveRecommendationsFromReport(report)

	cacheReport(report)

	safeRuntimeEventsEmit(s.ctx, "marketIntelligenceReport", report)

	return report, nil
}

// GeneratePostMarketReview 生成收盘盘后复盘与量化战绩 (15:00收盘后触发)
// 包含：全天涨跌分布、行业资金主力、连板梯队热度、AI预测回测胜率验证及明日推演
func (s *MarketLifecycleService) GeneratePostMarketReview(ctx context.Context, aiConfigId int) (*MarketIntelligenceReport, error) {
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
	prompt := fmt.Sprintf(`你现在是拥有20年实战经验的顶级A股量化私募投资总监“财智”。今天是 %s 收盘后。
请基于今日一手盘面全景数据与量化指标，直接撰写【A股全景收盘复盘与次日推演报告】：

【全天多空分布与市场情绪】：
%s

【涨停连板与题材热度概要】：
%s

【量化研报跟踪】：
%s
%s
请按以下结构直接输出结构化 Markdown 报告：
### 📊 一、全天行情概览与多空情绪定调
- 涨跌家数比、多空力量对比与广谱赚钱效应评估。

### 🚀 二、核心主线与资金筹码博弈
- 今日连板梯队高度、最强题材热点与领涨龙头持续性推演。

### 🧪 三、量化启示与策略认知
- 基于今日盘面特征的量化策略启示与认知纠偏。

### 🔮 四、明日走势推演与操作锦囊
- 明日开盘剧本推演与持仓风控金句。

【输出要求】：请直接开始输出 Markdown 研报正文，思考过程简短克制，严禁过度思考，确保正文完整输出。`, todayStr, statSummary.String(), uplimitSummary, btSummary, crystalPrompt)

	sysPrompt := "你现在是顶级A股投资总监“财智”。所有核心盘面客观数据（涨跌家数比、连板梯队题材、多空情绪指标）已在输入中提供。请直接开始输出 Markdown 研报正文，严禁在思考过程中冗余推导，优先确保报告正文完整输出。"
	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	var streamErr string

	genCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ch := NewStockAiAgentApi().ChatWithContext(genCtx, prompt, aiConfigId, nil, false, 0, false, string(AgentModeDirect), sysPrompt)
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			reasoningBuilder.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" && msg.Content != "agent-DONE" {
			if strings.HasPrefix(msg.Content, "❌") || strings.Contains(msg.Content, "Agent 调用失败") {
				streamErr = msg.Content
				continue
			}
			contentBuilder.WriteString(msg.Content)
		}
	}

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

	// 持久化到 ai_response_result
	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult("收盘复盘", "盘后复盘", finalReport, "post_market_"+todayStr, prompt)
	go ExtractAndSaveRecommendationsFromReport(report)

	cacheReport(report)

	// 桌面事件推送
	safeRuntimeEventsEmit(s.ctx, "marketIntelligenceReport", report)

	return report, nil
}

// GenerateETFRotationReport 手动或一键触发ETF轮动量化多Agent分析
func (s *MarketLifecycleService) GenerateETFRotationReport(ctx context.Context, aiConfigId int) (*MarketIntelligenceReport, error) {
	logger.SugaredLogger.Info("【MarketLifecycle】手动一键触发【ETF轮动多Agent量化决策分析】...")
	todayStr := time.Now().Format("2006-01-02")

	etfCodes := []string{"510300.SH", "159915.SZ", "512480.SH", "512660.SH", "512010.SH", "512800.SH", "510880.SH", "159949.SZ"}
	var etfSummary strings.Builder
	etfSummary.WriteString("【全市场核心ETF池实时监测】（沪深300、创业板、半导体、军工、医药、证券、红利、创业板50）：\n")
	stockAPI := data.NewStockDataApi()
	for _, code := range etfCodes {
		quotes, err := stockAPI.GetStockCodeRealTimeData(code)
		if err == nil && quotes != nil && len(*quotes) > 0 {
			q := (*quotes)[0]
			etfSummary.WriteString(fmt.Sprintf("- %s(%s): 现价 %v, 涨跌幅 %v%%\n", q.Name, code, q.Price, q.ChangePercent))
		}
	}

	prompt := fmt.Sprintf(`你是专注于量化对冲与ETF配置的首席基金经理“财智”。今天是 %s。
请严格执行【ETF轮动多Agent分析】（Skill 7）全套决策流程：

%s

### 请输出完整的ETF轮动量化决策报告：
1. 【动量池概览与筛选】：基于21日加权动量对上述ETF进行排序，并剔除近期回撤过深或盈亏比不佳的弱势标的；
2. 【6路因子并发分析】：对前4只优选ETF，从技术面、主力资金流、消息舆情、外围联动、宏观行业周期、溢价率多维度进行诊断；
3. 【投委会5视角加权评分表】：制作投委会打分矩阵（趋势30%%、风控20%%、价值15%%、筹码20%%、情绪15%%）；
4. 【最终交易决策】：给出明确的仓位建议（买入/持有/观望/减仓），并制定防守止损位。

【数据注入说明】：核心关注ETF池最新状态已在上方详尽提供，无需额外调用外部工具，请直接基于上述一手真实数据进行深度量化推演与结构化研报生成。`, todayStr, etfSummary.String())

	sysPrompt := "你现在是专注于量化对冲与ETF配置的首席基金经理“财智”。所有核心关注ETF池行情已在用户输入中提供，无需额外调用外部工具。请直接基于输入数据，按照 ETF 轮动决策流程输出完整的量化研报。"
	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	var streamErr string

	genCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ch := NewStockAiAgentApi().ChatWithContext(genCtx, prompt, aiConfigId, nil, false, 0, false, string(AgentModeDirect), sysPrompt)
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			reasoningBuilder.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" && msg.Content != "agent-DONE" {
			if strings.HasPrefix(msg.Content, "❌") || strings.Contains(msg.Content, "Agent 调用失败") {
				streamErr = msg.Content
				continue
			}
			contentBuilder.WriteString(msg.Content)
		}
	}

	if streamErr != "" {
		return nil, fmt.Errorf("生成ETF轮动分析报告中断: %s", streamErr)
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	rawContent := strings.TrimSpace(contentBuilder.String())
	if valid, errMsg := isValidMarketReport(rawContent); !valid {
		return nil, fmt.Errorf("生成ETF轮动分析报告失败: %s", errMsg)
	}

	title := fmt.Sprintf("📈 ETF轮动多Agent决策与投委会打分表 (%s)", todayStr)
	summary := "核心ETF池21日加权动量、6路因子并发扫描与投委会5视角加权评分表。"

	report := &MarketIntelligenceReport{
		Type:        "etf_rotation_decision",
		Title:       title,
		Summary:     summary,
		Content:     finalReport,
		Actions:     actions,
		GeneratedAt: time.Now(),
		TradingDate: todayStr,
		Source:      "ETF轮动决策引擎",
	}

	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult("510300.SH", "ETF轮动量化决策", finalReport, "etf_rotation_"+todayStr, prompt)
	go ExtractAndSaveRecommendationsFromReport(report)

	cacheReport(report)

	safeRuntimeEventsEmit(s.ctx, "marketIntelligenceReport", report)

	return report, nil
}

// GenerateSerenityResearchReport 手动一键触发任意板块/股票的 Serenity 产业链深度拆解
func (s *MarketLifecycleService) GenerateSerenityResearchReport(ctx context.Context, targetName string, code string, aiConfigId int) (*MarketIntelligenceReport, error) {
	return s.TriggerSectorAnomalySerenityAnalysis(ctx, targetName, code, "用户主动发起的Serenity深度产业链调研", aiConfigId)
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

	var items []models.AIResponseResult
	var query *gorm.DB
	switch reportType {
	case "pre_market_briefing":
		query = db.Dao.Where("chat_id LIKE ? OR stock_name LIKE ?", "%pre_market%", "%盘前%")
	case "post_market_review":
		query = db.Dao.Where("chat_id LIKE ? OR stock_name LIKE ? OR stock_code = ?", "%post_market%", "%收盘复盘%", "收盘复盘")
	case "etf_rotation_decision":
		query = db.Dao.Where("chat_id LIKE ? OR stock_name LIKE ?", "%etf_rotation%", "%ETF轮动%")
	case "serenity_industry_analysis":
		query = db.Dao.Where("chat_id LIKE ? OR stock_name LIKE ?", "%serenity%", "%Serenity%")
	default:
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
