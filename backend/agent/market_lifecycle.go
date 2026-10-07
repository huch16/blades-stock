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
	Content     string    `json:"content"`     // 完整Markdown内容
	GeneratedAt time.Time `json:"generatedAt"` // 生成时间
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

// GeneratePreMarketBriefing 生成盘前前瞻/晨报 (09:00前或手动触发)
// 包含：隔夜外盘/中概股动态、宏观政策快讯、今日财经日历/申购、重点板块关注与策略指引
func (s *MarketLifecycleService) GeneratePreMarketBriefing(ctx context.Context, aiConfigId int) (*MarketIntelligenceReport, error) {
	logger.SugaredLogger.Info("【MarketLifecycle】开始生成智能盘前前瞻晨报...")
	todayStr := time.Now().Format("2006-01-02")

	// 1. 采集盘前核心数据：全球指数、财联社快讯、市场统计
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

	// 2. 组装盘前前瞻 Prompt
	prompt := fmt.Sprintf(`你现在是顶级券商首席策略分析师兼量化宏观专家“财智”。今天是 %s。
请结合以下一手收集的盘前全球指数与要闻资讯，输出一份高水准、干货满满的【A股盘前智能前瞻与晨会内参】：

【隔夜及全球主要指数行情】：
%s

【最新宏观与隔夜核心要闻】：
%s

请严格按以下四维框架输出 Markdown 格式的盘前内参：
### 🌐 一、隔夜外盘与全球资产联动
- 欧美股市、中概金龙指数表现及对A股开盘情绪的影响；
- 汇率、大宗商品（黄金/原油）及主要风险偏好定调。

### 🧭 二、今日宏观与核心主线研判
- 核心要闻提炼（政策支持、产业新动向、重大突发事件）；
- 潜在收益板块与可能受压制方向。

### 🎯 三、集合竞价与开盘观察指引
- 关键指数点位防守区间；
- 开盘关注的重点风向标（如半导体/算力/新能源/高股息等）。

### 🛡️ 四、今日操作策略与风控纪律
- 仓位管理与节奏建议（严控追高、高抛低吸还是分批建仓）；
- 一句话点睛警示。`, todayStr, globalBrief, newsText.String())

	contentBuilder := strings.Builder{}
	ch := NewStockAiAgentApi().ChatWithContext(ctx, prompt, aiConfigId, nil, false, 0, false, "")
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			contentBuilder.WriteString(msg.ReasoningContent)
		}
		contentBuilder.WriteString(msg.Content)
	}

	finalReport := contentBuilder.String()
	if strings.TrimSpace(finalReport) == "" {
		return nil, fmt.Errorf("AI 生成盘前晨报内容为空")
	}

	title := fmt.Sprintf("🌅 A股盘前智能前瞻与晨会内参 (%s)", todayStr)
	summary := "隔夜全球资产联动分析、核心要闻发酵解读与今日开盘风向标策略指引。"

	report := &MarketIntelligenceReport{
		Type:        "pre_market_briefing",
		Title:       title,
		Summary:     summary,
		Content:     finalReport,
		GeneratedAt: time.Now(),
		TradingDate: todayStr,
		Source:      "Agent盘前引擎",
	}

	// 持久化到 ai_response_result 以便用户在历史分析中随时查阅
	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult("盘前前瞻", "盘前内参", finalReport, "pre_market_"+todayStr, prompt)

	lifecycleReportCacheMu.Lock()
	lifecycleLatestReport = report
	lifecycleReportCacheMu.Unlock()

	// 桌面事件与 Windows 通知提醒
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

	contentBuilder := strings.Builder{}
	ch := NewStockAiAgentApi().ChatWithContext(ctx, prompt, aiConfigId, nil, false, 0, false, "")
	for msg := range ch {
		if msg == nil {
			continue
		}
		if msg.ReasoningContent != "" {
			contentBuilder.WriteString(msg.ReasoningContent)
		}
		contentBuilder.WriteString(msg.Content)
	}

	finalReport := contentBuilder.String()
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
		GeneratedAt: time.Now(),
		TradingDate: todayStr,
		Source:      "Agent复盘引擎",
	}

	// 持久化到 ai_response_result
	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult("收盘复盘", "盘后复盘", finalReport, "post_market_"+todayStr, prompt)

	lifecycleReportCacheMu.Lock()
	lifecycleLatestReport = report
	lifecycleReportCacheMu.Unlock()

	// 桌面事件推送
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "marketIntelligenceReport", report)
	}

	return report, nil
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
