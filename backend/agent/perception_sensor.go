package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
)

// SentimentTrafficLight 情绪红绿灯
type SentimentTrafficLight string

const (
	LightGreenAttack   SentimentTrafficLight = "GREEN_ATTACK"      // 🟢 进攻
	LightYellowCaution SentimentTrafficLight = "YELLOW_DIVERGENCE" // 🟡 分歧防守
	LightRedRetreat    SentimentTrafficLight = "RED_RETREAT"       // 🔴 退潮空仓
)

// PortfolioRiskItem 单个持仓风险条目
type PortfolioRiskItem struct {
	StockCode  string  `json:"stockCode"`
	StockName  string  `json:"stockName"`
	CostPrice  float64 `json:"costPrice"`
	CurrPrice  float64 `json:"currPrice"`
	ProfitRate float64 `json:"profitRate"`
	IsWarning  bool    `json:"isWarning"`
	Reason     string  `json:"reason"`
}

// SystemStateSnapshot 系统的全局感知态势快照
type SystemStateSnapshot struct {
	Timestamp      string                `json:"timestamp"`
	ClockPhase     MarketPhase           `json:"clockPhase"`
	PhaseDesc      string                `json:"phaseDesc"`
	TrafficLight   SentimentTrafficLight `json:"trafficLight"`
	TrafficDesc    string                `json:"trafficDesc"`
	UpCount        int                   `json:"upCount"`
	DownCount      int                   `json:"downCount"`
	LimitUpCount   int                   `json:"limitUpCount"`
	LimitDownCount int                   `json:"limitDownCount"`
	HoldingsCount  int                   `json:"holdingsCount"`
	ProfitingCount int                   `json:"profitingCount"`
	LosingCount    int                   `json:"losingCount"`
	RiskHoldings   []PortfolioRiskItem   `json:"riskHoldings"`
	SummaryPrompt  string                `json:"summaryPrompt"`
}

// GlobalMarketSensor 全局市场感知器单例
type GlobalMarketSensor struct {
	mu           sync.RWMutex
	lastSnapshot *SystemStateSnapshot
	lastFetch    time.Time
}

var (
	sensorInstance *GlobalMarketSensor
	sensorOnce     sync.Once
)

// GetGlobalMarketSensor 获取单例
func GetGlobalMarketSensor() *GlobalMarketSensor {
	sensorOnce.Do(func() {
		sensorInstance = &GlobalMarketSensor{}
	})
	return sensorInstance
}

// CalculateClockPhase 计算当前交易时钟节律
func CalculateClockPhase(now time.Time) (MarketPhase, string) {
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return PhaseClosed, "周末休市中，适合离场复盘与系统优化"
	}

	hour := now.Hour()
	minute := now.Minute()
	timeVal := hour*100 + minute

	switch {
	case timeVal < 915:
		return PhasePreMarket, "盘前前瞻时段 (09:15前)，聚焦隔夜要闻与今日应对预案"
	case timeVal < 930:
		return PhaseCallAuction, "集合竞价时段 (09:15-09:30)，观察爆量抢筹与异动方向"
	case timeVal < 1130:
		return PhaseMorningTrading, "早盘主攻时段 (09:30-11:30)，主力真金白银表态，短线定方向"
	case timeVal < 1300:
		return PhaseNoonBreak, "午间休市时段 (11:30-13:00)，消息消化与午后标的梳理"
	case timeVal <= 1500:
		return PhaseAfternoonTrading, "盘中下午盘 (13:00-15:00)，观察尾盘抢筹或跳水，防范脉冲诱多"
	default:
		return PhasePostMarket, "收盘复盘时段 (15:00后)，全天量价资金结账，进行知行合一复盘"
	}
}

// GetSnapshot 获取最新态势感知快照（带短缓存避免频繁击穿）
func (s *GlobalMarketSensor) GetSnapshot() *SystemStateSnapshot {
	s.mu.RLock()
	if s.lastSnapshot != nil && time.Since(s.lastFetch) < 10*time.Second {
		defer s.mu.RUnlock()
		return s.lastSnapshot
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 二次检查
	if s.lastSnapshot != nil && time.Since(s.lastFetch) < 10*time.Second {
		return s.lastSnapshot
	}

	now := time.Now()
	phase, phaseDesc := CalculateClockPhase(now)

	snapshot := &SystemStateSnapshot{
		Timestamp:    now.Format("2006-01-02 15:04:05"),
		ClockPhase:   phase,
		PhaseDesc:    phaseDesc,
		TrafficLight: LightYellowCaution,
		TrafficDesc:  "🟡 市场处于分歧震荡阶段，建议严控仓位，拒绝追高",
		RiskHoldings: make([]PortfolioRiskItem, 0),
	}

	// 1. 抓取市场大盘与情绪数据
	marketStatAPI := data.NewMarketStatisticApi()
	todayStats := marketStatAPI.GetTodayData()
	if len(todayStats) > 0 {
		latest := todayStats[len(todayStats)-1]
		snapshot.UpCount = latest.UpCount
		snapshot.DownCount = latest.DownCount
		snapshot.LimitUpCount = latest.LimitUp
		snapshot.LimitDownCount = latest.LimitDown

		total := latest.UpCount + latest.DownCount
		if total > 0 {
			upRatio := float64(latest.UpCount) / float64(total)
			if upRatio > 0.65 && latest.LimitUp > 30 {
				snapshot.TrafficLight = LightGreenAttack
				snapshot.TrafficDesc = "🟢 市场多头合力充沛，投机情绪火热，可积极参与主线接力"
			} else if upRatio < 0.35 || latest.LimitDown > 15 {
				snapshot.TrafficLight = LightRedRetreat
				snapshot.TrafficDesc = "🔴 市场处于退潮主跌/冰点期，亏钱效应显著，坚决防守空仓"
			}
		}
	}

	// 2. 扫描持仓风控雷达
	if db.Dao != nil {
		var followedStocks []data.FollowedStock
		err := db.Dao.Model(&data.FollowedStock{}).Where("cost_price > 0 AND volume > 0").Find(&followedStocks).Error
		if err == nil && len(followedStocks) > 0 {
			snapshot.HoldingsCount = len(followedStocks)
			for _, stock := range followedStocks {
				var profitRate float64
				if stock.CostPrice > 0 {
					profitRate = ((stock.Price - stock.CostPrice) / stock.CostPrice) * 100
				}
				if profitRate >= 0 {
					snapshot.ProfitingCount++
				} else {
					snapshot.LosingCount++
				}

				// 判定风险告警条件：亏损 > 5% 或 跌破止损位
				isWarning := false
				var reasons []string
				if profitRate <= -5.0 {
					isWarning = true
					reasons = append(reasons, fmt.Sprintf("单票浮亏达 %.2f%%，逼近常规止损线", profitRate))
				}
				if stock.StopLossPrice > 0 && stock.Price > 0 && stock.Price <= stock.StopLossPrice {
					isWarning = true
					reasons = append(reasons, fmt.Sprintf("现价已触碰预设止损价(%.2f)", stock.StopLossPrice))
				}

				if isWarning {
					snapshot.RiskHoldings = append(snapshot.RiskHoldings, PortfolioRiskItem{
						StockCode:  stock.StockCode,
						StockName:  stock.Name,
						CostPrice:  stock.CostPrice,
						CurrPrice:  stock.Price,
						ProfitRate: profitRate,
						IsWarning:  true,
						Reason:     strings.Join(reasons, "；"),
					})
				}
			}
		}
	}

	// 3. 构建注入 Agent 的全局感知上下文 Prompt
	snapshot.SummaryPrompt = s.formatPrompt(snapshot)

	s.lastSnapshot = snapshot
	s.lastFetch = now
	return snapshot
}

func (s *GlobalMarketSensor) formatPrompt(snap *SystemStateSnapshot) string {
	var sb strings.Builder
	sb.WriteString("【全局环境感知状态 (System Context)】\n")
	sb.WriteString(fmt.Sprintf("• 当前时钟节律：%s (%s)\n", snap.ClockPhase, snap.PhaseDesc))
	sb.WriteString(fmt.Sprintf("• 市场投机情绪：%s\n", snap.TrafficDesc))
	if snap.UpCount > 0 || snap.DownCount > 0 {
		sb.WriteString(fmt.Sprintf("• 全市场涨跌比：%d 家上涨 vs %d 家下跌 | 涨停 %d 家 | 跌停 %d 家\n",
			snap.UpCount, snap.DownCount, snap.LimitUpCount, snap.LimitDownCount))
	}
	if snap.HoldingsCount > 0 {
		sb.WriteString(fmt.Sprintf("• 个人实盘持仓：%d 只标的 (盈利 %d 只 / 浮亏 %d 只)\n",
			snap.HoldingsCount, snap.ProfitingCount, snap.LosingCount))
		if len(snap.RiskHoldings) > 0 {
			sb.WriteString(fmt.Sprintf("⚠️ 重点风控告警标的 (%d只)：\n", len(snap.RiskHoldings)))
			for _, r := range snap.RiskHoldings {
				sb.WriteString(fmt.Sprintf("  - %s(%s): 现价 %.2f / 成本 %.2f (浮动 %.2f%%) | 风险: %s\n",
					r.StockName, r.StockCode, r.CurrPrice, r.CostPrice, r.ProfitRate, r.Reason))
			}
		}
	} else {
		sb.WriteString("• 个人实盘持仓：当前空仓或尚未录入持仓成本\n")
	}
	return sb.String()
}
