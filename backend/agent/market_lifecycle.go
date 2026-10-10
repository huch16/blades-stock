package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"
	"blades-stock/backend/agent/types"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	lifecycleReportCacheMu sync.RWMutex
	lifecycleLatestReport  *MarketIntelligenceReport
)

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

全文请使用专业券商内参语气，数据引用请保持客观，避免过度解读。`, todayStr, globalBrief, etfSummary.String(), momentumTable, newsText.String())

	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	ch := NewStockAiAgentApi().ChatWithContext(ctx, prompt, aiConfigId, nil, false, 0, false, "")
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			reasoningBuilder.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" {
			contentBuilder.WriteString(msg.Content)
		}
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	if strings.TrimSpace(finalReport) == "" {
		return nil, fmt.Errorf("AI 生成盘前晨报内容为空")
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

	lifecycleReportCacheMu.Lock()
	lifecycleLatestReport = report
	lifecycleReportCacheMu.Unlock()

	// 桌面事件与 Windows 通知提醒
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "marketIntelligenceReport", report)
	}

	return report, nil
}

// TriggerSectorAnomalySerenityAnalysis 当盘中出现资金大额流入或异动突发时，自动触发 Serenity 产业链深度拆解
func (s *MarketLifecycleService) TriggerSectorAnomalySerenityAnalysis(ctx context.Context, sectorOrStockName string, code string, reason string, aiConfigId int) (*MarketIntelligenceReport, error) {
	logger.SugaredLogger.Warnf("🔥 【Serenity异动拆解】检测到异动 %s(%s): %s，触发Serenity产业链分析", sectorOrStockName, code, reason)
	todayStr := time.Now().Format("2006-01-02")

	prompt := fmt.Sprintf(`你是专注于产业链瓶颈与卡脖子技术的研究专家“财智”。
盘中检测到板块/标的【%s (%s)】出现强异动：%s。

请严格遵守【Serenity式产业分析框架】（Skill 4）：
1. 【产业链全景拆解】：梳理%s在产业链中的位置（上中下游与核心参与者）；
2. 【稀缺层识别】：识别其中技术壁垒最高、国产替代率<30%%或CR5>60%%的绝对稀缺环节；
3. 【优先级打分】：按稀缺性、可投资性、确定性三维度打分并排序；
4. 【证据链验证】：列出营收增速、毛利率或重要客户支撑；
5. 【证伪条件】：列出结论失效的触发条件。

输出结构清晰的 Markdown 产业链深度拆解报告。`, sectorOrStockName, code, reason, sectorOrStockName)

	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	ch := NewStockAiAgentApi().ChatWithContext(ctx, prompt, aiConfigId, nil, false, 0, false, "")
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			reasoningBuilder.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" {
			contentBuilder.WriteString(msg.Content)
		}
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	if strings.TrimSpace(finalReport) == "" {
		return nil, fmt.Errorf("Serenity 产业链拆解生成失败")
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

	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult(code, "Serenity产业链分析", finalReport, "serenity_"+code, prompt)
	go ExtractAndSaveRecommendationsFromReport(report)

	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "marketIntelligenceReport", report)
	}

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

	// 涨停热点与连板板块
	uplimitHot := data.NewMarketNewsApi().GetUplimitHot(todayStr, 15)
	uplimitJSON, _ := data.JSONToMarkdownTable([]byte(fmt.Sprintf("%v", uplimitHot)))

	// 2. 触发 AI 历史报告自动回测闭环检验
	btMgr := quant.NewBacktestManager()
	btSummary := "回测引擎检验中..."
	if btRes, err := btMgr.RunAIReportBacktest(ctx, 50); err == nil && btRes.Processed > 0 {
		btSummary = fmt.Sprintf("今日盘后回测检验已处理 %d 份近期AI研报，预测准确率 %.1f%% (准确 %d 份)",
			btRes.Processed, btRes.AccuracyRate*100, btRes.Accurate)
	}

	// 3. 组装收盘复盘 Prompt
	prompt := fmt.Sprintf(`你现在是拥有20年实战经验的顶级A股量化私募投资总监“财智”。今天是 %s 收盘后。
请基于今日盘面全景数据与量化指标，撰写一份深刻、严谨且指导性强的【A股全景收盘复盘与次日推演报告】：

【全天市场涨跌与量能分布】：
%s

【涨停连板与题材热度概要】：
%s

【AI预测回测与量化跟踪】：
%s

请按以下结构撰写深度复盘 Markdown 报告：
### 📊 一、全天行情概览与多空情绪定调
- 大盘指数运行特征（放量/缩量、反弹/分歧、震荡区间）；
- 赚钱效应评估与多空力量对比。

### 🚀 二、核心主线与资金筹码博弈
- 今日最强热点板块、领涨龙头与板块持续性推演；
- 资金流向偏好（进攻型成长 vs 防御型红利/高股息）。

### 🧪 三、量化验证与AI预测胜率复盘
- 结合回测数据，客观评估近期策略与研报判断的胜率闭环；
- 提炼近期市场特征带来的启示与认知纠偏。

### 🔮 四、明日走势推演与操作锦囊
- 明日开盘剧本推演（高开冲高/低开探底/平开震荡）；
- 应对方案与持仓风控金句。`, todayStr, statSummary.String(), uplimitJSON, btSummary)

	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	ch := NewStockAiAgentApi().ChatWithContext(ctx, prompt, aiConfigId, nil, false, 0, false, "")
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			reasoningBuilder.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" {
			contentBuilder.WriteString(msg.Content)
		}
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	if strings.TrimSpace(finalReport) == "" {
		return nil, fmt.Errorf("AI 生成收盘复盘内容为空")
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

	lifecycleReportCacheMu.Lock()
	lifecycleLatestReport = report
	lifecycleReportCacheMu.Unlock()

	// 桌面事件推送
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "marketIntelligenceReport", report)
	}

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
4. 【最终交易决策】：给出明确的仓位建议（买入/持有/观望/减仓），并制定防守止损位。`, todayStr, etfSummary.String())

	reasoningBuilder := strings.Builder{}
	contentBuilder := strings.Builder{}
	ch := NewStockAiAgentApi().ChatWithContext(ctx, prompt, aiConfigId, nil, false, 0, false, "")
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			reasoningBuilder.WriteString(msg.ReasoningContent)
		}
		if msg.Content != "" {
			contentBuilder.WriteString(msg.Content)
		}
	}

	finalReport, actions := cleanAndAssembleReport(reasoningBuilder.String(), contentBuilder.String())
	if strings.TrimSpace(finalReport) == "" {
		return nil, fmt.Errorf("生成ETF轮动分析报告失败")
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

	lifecycleReportCacheMu.Lock()
	lifecycleLatestReport = report
	lifecycleReportCacheMu.Unlock()

	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "marketIntelligenceReport", report)
	}

	return report, nil
}

// GenerateSerenityResearchReport 手动一键触发任意板块/股票的 Serenity 产业链深度拆解
func (s *MarketLifecycleService) GenerateSerenityResearchReport(ctx context.Context, targetName string, code string, aiConfigId int) (*MarketIntelligenceReport, error) {
	return s.TriggerSectorAnomalySerenityAnalysis(ctx, targetName, code, "用户主动发起的Serenity深度产业链调研", aiConfigId)
}

// GetLatestReport 获取最新生成的智能研报（盘前或盘后）
func (s *MarketLifecycleService) GetLatestReport() *MarketIntelligenceReport {
	lifecycleReportCacheMu.RLock()
	defer lifecycleReportCacheMu.RUnlock()
	if lifecycleLatestReport != nil {
		return lifecycleLatestReport
	}

	// 若内存无缓存，尝试从 DB 读取最新一条盘前或盘后记录
	var item models.AIResponseResult
	err := db.Dao.Where("stock_code IN (?, ?)", "盘前前瞻", "收盘复盘").
		Order("created_at DESC").
		First(&item).Error
	if err == nil && item.ID > 0 {
		reportType := "pre_market_briefing"
		if item.StockCode == "收盘复盘" {
			reportType = "post_market_review"
		}
		return &MarketIntelligenceReport{
			Type:        reportType,
			Title:       fmt.Sprintf("%s (%s)", item.StockName, item.CreatedAt.Format("2006-01-02")),
			Summary:     "历史智能分析报告存档",
			Content:     item.Content,
			GeneratedAt: item.CreatedAt,
			TradingDate: item.CreatedAt.Format("2006-01-02"),
			Source:      item.ModelName,
		}
	}

	return nil
}
