package quant

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

var (
	portfolioMu sync.Mutex
)

// PaperPortfolioManager 负责智能体专属虚拟模拟基金的撮合、头寸管理与动态净值问责
type PaperPortfolioManager struct{}

func NewPaperPortfolioManager() *PaperPortfolioManager {
	return &PaperPortfolioManager{}
}

// GetOrCreateDefaultPortfolio 获取或初始化默认模拟基金（财智1号买方量化先锋）
func (m *PaperPortfolioManager) GetOrCreateDefaultPortfolio() (*models.PaperPortfolio, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	portfolioMu.Lock()
	defer portfolioMu.Unlock()

	var p models.PaperPortfolio
	err := db.Dao.Where("name = ?", "财智1号买方量化先锋").First(&p).Error
	if err == nil {
		return &p, nil
	}

	// 初始化 1000 万模拟本金
	initialCapital := 10000000.0
	p = models.PaperPortfolio{
		Name:            "财智1号买方量化先锋",
		InitialCash:     initialCapital,
		CashBalance:     initialCapital,
		TotalAssetValue: initialCapital,
		CurrentNAV:      1.0000,
		PeakNAV:         1.0000,
		MaxDrawdown:     0.0,
		TotalReturnRate: 0.0,
		SharpeRatio:     0.0,
		WinRate:         0.0,
		TotalTrades:     0,
		WinningTrades:   0,
		LosingTrades:    0,
	}

	if err := db.Dao.Create(&p).Error; err != nil {
		return nil, fmt.Errorf("create paper portfolio failed: %w", err)
	}

	logger.SugaredLogger.Infof("[PaperPortfolio] 成功初始化智能体专属模拟盘账户: %s (初始本金: %.2f 万元)", p.Name, p.InitialCash/10000.0)
	return &p, nil
}

// ExecuteVirtualBuy 执行虚拟建仓/加仓买入指令
func (m *PaperPortfolioManager) ExecuteVirtualBuy(stockCode, stockName string, price, stopLoss, takeProfit, targetWeight float64, reason string) (*models.PaperTradeOrder, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if price <= 0 {
		return nil, fmt.Errorf("invalid buy price: %.2f", price)
	}
	if reason == "" {
		reason = "AI_RECOMMEND"
	}

	p, err := m.GetOrCreateDefaultPortfolio()
	if err != nil {
		return nil, err
	}

	portfolioMu.Lock()
	defer portfolioMu.Unlock()

	// 1. 单票风控限制：最高单仓 15% (150万)，默认 10%
	if targetWeight <= 0.01 || targetWeight > 0.15 {
		targetWeight = 0.10
	}
	maxAllocAmount := p.TotalAssetValue * targetWeight
	if maxAllocAmount > p.CashBalance {
		maxAllocAmount = p.CashBalance * 0.95 // 留 5% 现金缓冲
	}
	if maxAllocAmount < 10000.0 {
		return nil, fmt.Errorf("可用现金不足 (仅剩余 %.2f 元)，无法完成风控合规建仓", p.CashBalance)
	}

	// 2. 模拟滑点与佣金 (买入滑点 0.1%, 佣金万三, 最低5元)
	slippageRate := 0.001
	filledPrice := math.Round(price*(1.0+slippageRate)*100.0) / 100.0
	sharesFloat := maxAllocAmount / filledPrice
	shares := int(sharesFloat/100.0) * 100 // 必须为 100 股整数倍
	if shares < 100 {
		return nil, fmt.Errorf("目标金额 %.2f 不足以买入 1 手 %s (每股 %.2f 元)", maxAllocAmount, stockCode, filledPrice)
	}

	amount := float64(shares) * filledPrice
	commission := amount * 0.0003
	if commission < 5.0 {
		commission = 5.0
	}
	totalCost := amount + commission

	if totalCost > p.CashBalance {
		return nil, fmt.Errorf("扣除佣金后现金不足 (需 %.2f, 实际可用 %.2f)", totalCost, p.CashBalance)
	}

	// 3. 扣除现金并更新/新增持仓
	p.CashBalance -= totalCost

	var pos models.PaperPosition
	findErr := db.Dao.Where("portfolio_id = ? AND stock_code = ?", p.ID, stockCode).First(&pos).Error
	now := time.Now()
	if findErr != nil {
		// 新建仓位
		pos = models.PaperPosition{
			PortfolioID:     p.ID,
			StockCode:       stockCode,
			StockName:       stockName,
			Shares:          shares,
			AvgCostPrice:    filledPrice,
			CurrentPrice:    filledPrice,
			MarketValue:     amount,
			FloatingPnL:     -commission,
			FloatingPnLRate: (-commission / amount) * 100.0,
			StopLossPrice:   stopLoss,
			TakeProfitPrice: takeProfit,
			Weight:          (amount / p.TotalAssetValue) * 100.0,
			InitialBuyTime:  now,
		}
		if err := db.Dao.Create(&pos).Error; err != nil {
			return nil, err
		}
	} else {
		// 加仓合并均价
		newTotalShares := pos.Shares + shares
		newAvgCost := (float64(pos.Shares)*pos.AvgCostPrice + amount) / float64(newTotalShares)
		pos.Shares = newTotalShares
		pos.AvgCostPrice = math.Round(newAvgCost*100.0) / 100.0
		pos.CurrentPrice = filledPrice
		pos.MarketValue = float64(newTotalShares) * filledPrice
		pos.FloatingPnL = pos.MarketValue - float64(newTotalShares)*pos.AvgCostPrice
		pos.FloatingPnLRate = (pos.FloatingPnL / (float64(newTotalShares) * pos.AvgCostPrice)) * 100.0
		if stopLoss > 0 {
			pos.StopLossPrice = stopLoss
		}
		if takeProfit > 0 {
			pos.TakeProfitPrice = takeProfit
		}
		pos.Weight = (pos.MarketValue / p.TotalAssetValue) * 100.0
		pos.UpdatedAt = now
		if err := db.Dao.Save(&pos).Error; err != nil {
			return nil, err
		}
	}

	// 4. 生成成交订单记录
	order := models.PaperTradeOrder{
		PortfolioID: p.ID,
		StockCode:   stockCode,
		StockName:   stockName,
		Side:        "BUY",
		OrderPrice:  price,
		FilledPrice: filledPrice,
		Slippage:    slippageRate,
		Shares:      shares,
		Amount:      amount,
		Fee:         commission,
		RealizedPnL: 0.0,
		Reason:      reason,
		OrderTime:   now,
	}
	_ = db.Dao.Create(&order).Error

	// 5. 更新账户总市值与资金
	db.Dao.Model(p).Updates(map[string]interface{}{
		"cash_balance": p.CashBalance,
	})

	logger.SugaredLogger.Infof("[PaperPortfolio] 虚拟买入成交: %s(%s) %d股, 成交价: %.2f (滑点: %.1f%%), 动因: %s",
		stockName, stockCode, shares, filledPrice, slippageRate*100, reason)
	return &order, nil
}

// ExecuteVirtualSell 执行虚拟平仓/减仓卖出指令
func (m *PaperPortfolioManager) ExecuteVirtualSell(stockCode string, price float64, shares int, reason string) (*models.PaperTradeOrder, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if price <= 0 {
		return nil, fmt.Errorf("invalid sell price: %.2f", price)
	}

	p, err := m.GetOrCreateDefaultPortfolio()
	if err != nil {
		return nil, err
	}

	portfolioMu.Lock()
	defer portfolioMu.Unlock()

	var pos models.PaperPosition
	if err := db.Dao.Where("portfolio_id = ? AND stock_code = ?", p.ID, stockCode).First(&pos).Error; err != nil {
		return nil, fmt.Errorf("当前持仓中未找到标的 %s", stockCode)
	}

	if shares <= 0 || shares >= pos.Shares {
		shares = pos.Shares
	} else {
		shares = int(shares/100) * 100
		if shares < 100 {
			shares = 100
		}
	}

	// 卖出模拟滑点 0.1%, 佣金万三, 印花税万分之五
	slippageRate := 0.001
	filledPrice := math.Round(price*(1.0-slippageRate)*100.0) / 100.0
	grossAmount := float64(shares) * filledPrice
	commission := grossAmount * 0.0003
	if commission < 5.0 {
		commission = 5.0
	}
	stampDuty := grossAmount * 0.0005
	totalFee := commission + stampDuty
	netCash := grossAmount - totalFee

	realizedPnL := (filledPrice-pos.AvgCostPrice)*float64(shares) - totalFee

	// 更新现金
	p.CashBalance += netCash
	p.TotalTrades++
	if realizedPnL > 0 {
		p.WinningTrades++
	} else {
		p.LosingTrades++
	}
	if p.TotalTrades > 0 {
		p.WinRate = math.Round((float64(p.WinningTrades)/float64(p.TotalTrades)*100.0)*100.0) / 100.0
	}

	now := time.Now()
	order := models.PaperTradeOrder{
		PortfolioID: p.ID,
		StockCode:   stockCode,
		StockName:   pos.StockName,
		Side:        "SELL",
		OrderPrice:  price,
		FilledPrice: filledPrice,
		Slippage:    slippageRate,
		Shares:      shares,
		Amount:      grossAmount,
		Fee:         totalFee,
		RealizedPnL: realizedPnL,
		Reason:      reason,
		OrderTime:   now,
	}
	_ = db.Dao.Create(&order).Error

	// 更新或删除持仓
	if pos.Shares <= shares {
		db.Dao.Delete(&pos)
	} else {
		pos.Shares -= shares
		pos.MarketValue = float64(pos.Shares) * filledPrice
		pos.FloatingPnL = pos.MarketValue - float64(pos.Shares)*pos.AvgCostPrice
		pos.FloatingPnLRate = (pos.FloatingPnL / (float64(pos.Shares) * pos.AvgCostPrice)) * 100.0
		pos.UpdatedAt = now
		db.Dao.Save(&pos)
	}

	db.Dao.Model(p).Updates(map[string]interface{}{
		"cash_balance":   p.CashBalance,
		"total_trades":   p.TotalTrades,
		"winning_trades": p.WinningTrades,
		"losing_trades":  p.LosingTrades,
		"win_rate":       p.WinRate,
	})

	logger.SugaredLogger.Infof("[PaperPortfolio] 虚拟卖出平仓: %s(%s) %d股, 成交价: %.2f, 实现盈亏: %.2f 元, 动因: %s",
		pos.StockName, stockCode, shares, filledPrice, realizedPnL, reason)
	return &order, nil
}

// UpdateDailyNAV 每日收盘后对账，刷新最新行情并计算组合动态净值、最大回撤与夏普比率
func (m *PaperPortfolioManager) UpdateDailyNAV() (*models.PaperDailyNAV, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	p, err := m.GetOrCreateDefaultPortfolio()
	if err != nil {
		return nil, err
	}

	portfolioMu.Lock()
	defer portfolioMu.Unlock()

	var positions []models.PaperPosition
	db.Dao.Where("portfolio_id = ?", p.ID).Find(&positions)

	totalStockValue := 0.0

	// 刷新每个持仓的最新市价
	for i := range positions {
		pos := &positions[i]
		latestPrice := fetchLatestStockPrice(pos.StockCode)
		if latestPrice <= 0 {
			latestPrice = pos.CurrentPrice
		}
		if latestPrice > 0 {
			pos.CurrentPrice = latestPrice
			pos.MarketValue = float64(pos.Shares) * latestPrice
			pos.FloatingPnL = pos.MarketValue - float64(pos.Shares)*pos.AvgCostPrice
			if pos.AvgCostPrice > 0 {
				pos.FloatingPnLRate = (pos.FloatingPnL / (float64(pos.Shares) * pos.AvgCostPrice)) * 100.0
			}
			pos.UpdatedAt = time.Now()
			db.Dao.Save(pos)
		}
		totalStockValue += pos.MarketValue
	}

	// 计算组合总资产与最新单位净值
	p.TotalAssetValue = p.CashBalance + totalStockValue
	if p.InitialCash > 0 {
		p.CurrentNAV = math.Round((p.TotalAssetValue/p.InitialCash)*10000.0) / 10000.0
		p.TotalReturnRate = math.Round(((p.TotalAssetValue-p.InitialCash)/p.InitialCash*100.0)*100.0) / 100.0
	}

	// 历史峰值与动态最大回撤计算
	if p.CurrentNAV > p.PeakNAV {
		p.PeakNAV = p.CurrentNAV
	}
	currentDD := 0.0
	if p.PeakNAV > 0 {
		currentDD = (p.PeakNAV - p.CurrentNAV) / p.PeakNAV * 100.0
	}
	if currentDD > p.MaxDrawdown {
		p.MaxDrawdown = math.Round(currentDD*100.0) / 100.0
	}

	today := time.Now().Format("2006-01-02")
	var prevNAV models.PaperDailyNAV
	_ = db.Dao.Where("portfolio_id = ? AND trade_date < ?", p.ID, today).Order("trade_date desc").First(&prevNAV).Error
	dailyReturn := 0.0
	if prevNAV.NAV > 0 {
		dailyReturn = ((p.CurrentNAV - prevNAV.NAV) / prevNAV.NAV) * 100.0
	}

	// 保存每日净值快照
	dailySnapshot := models.PaperDailyNAV{
		PortfolioID:     p.ID,
		TradeDate:       today,
		NAV:             p.CurrentNAV,
		TotalAsset:      p.TotalAssetValue,
		Cash:            p.CashBalance,
		PositionValue:   totalStockValue,
		DailyReturnRate: math.Round(dailyReturn*100.0) / 100.0,
		DrawdownRate:    math.Round(currentDD*100.0) / 100.0,
	}

	var existingSnap models.PaperDailyNAV
	if err := db.Dao.Where("portfolio_id = ? AND trade_date = ?", p.ID, today).First(&existingSnap).Error; err == nil {
		dailySnapshot.ID = existingSnap.ID
		db.Dao.Save(&dailySnapshot)
	} else {
		db.Dao.Create(&dailySnapshot)
	}

	// 计算年化夏普比率 (基于每日净值收益率序列)
	p.SharpeRatio = m.calculateSharpeRatio(p.ID)

	db.Dao.Model(p).Updates(map[string]interface{}{
		"total_asset_value": p.TotalAssetValue,
		"current_nav":       p.CurrentNAV,
		"peak_nav":          p.PeakNAV,
		"max_drawdown":      p.MaxDrawdown,
		"total_return_rate": p.TotalReturnRate,
		"sharpe_ratio":      p.SharpeRatio,
	})

	logger.SugaredLogger.Infof("[PaperPortfolio] 每日净值对账完成: NAV=%.4f (累积收益: %.2f%%, 最大回撤: %.2f%%, 夏普比率: %.2f)",
		p.CurrentNAV, p.TotalReturnRate, p.MaxDrawdown, p.SharpeRatio)
	return &dailySnapshot, nil
}

// CheckAutoExitSignals 检查止损/止盈主动风控平仓信号
func (m *PaperPortfolioManager) CheckAutoExitSignals() ([]models.PaperTradeOrder, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	p, err := m.GetOrCreateDefaultPortfolio()
	if err != nil {
		return nil, err
	}

	var positions []models.PaperPosition
	db.Dao.Where("portfolio_id = ?", p.ID).Find(&positions)

	var executedOrders []models.PaperTradeOrder

	for _, pos := range positions {
		latestPrice := fetchLatestStockPrice(pos.StockCode)
		if latestPrice <= 0 {
			continue
		}

		// 1. 击穿止损线风控平仓
		if pos.StopLossPrice > 0 && latestPrice <= pos.StopLossPrice {
			order, err := m.ExecuteVirtualSell(pos.StockCode, latestPrice, pos.Shares, "STOP_LOSS")
			if err == nil && order != nil {
				executedOrders = append(executedOrders, *order)
				logger.SugaredLogger.Warnf("[PaperPortfolio] 触发纪律止损强平: %s 现价 %.2f <= 止损线 %.2f",
					pos.StockName, latestPrice, pos.StopLossPrice)
			}
			continue
		}

		// 2. 达到目标止盈线止盈平仓
		if pos.TakeProfitPrice > 0 && latestPrice >= pos.TakeProfitPrice {
			order, err := m.ExecuteVirtualSell(pos.StockCode, latestPrice, pos.Shares, "TAKE_PROFIT")
			if err == nil && order != nil {
				executedOrders = append(executedOrders, *order)
				logger.SugaredLogger.Infof("[PaperPortfolio] 达成目标收益止盈平仓: %s 现价 %.2f >= 目标价 %.2f",
					pos.StockName, latestPrice, pos.TakeProfitPrice)
			}
		}
	}

	return executedOrders, nil
}

// GetPortfolioStats 获取综合报表
func (m *PaperPortfolioManager) GetPortfolioStats() (*models.PaperPortfolioStats, error) {
	p, err := m.GetOrCreateDefaultPortfolio()
	if err != nil {
		return nil, err
	}

	var positions []models.PaperPosition
	db.Dao.Where("portfolio_id = ?", p.ID).Order("market_value desc").Find(&positions)

	var orders []models.PaperTradeOrder
	db.Dao.Where("portfolio_id = ?", p.ID).Order("order_time desc").Limit(30).Find(&orders)

	var navHistory []models.PaperDailyNAV
	db.Dao.Where("portfolio_id = ?", p.ID).Order("trade_date asc").Limit(180).Find(&navHistory)

	return &models.PaperPortfolioStats{
		Portfolio:    *p,
		Positions:    positions,
		RecentOrders: orders,
		NAVHistory:   navHistory,
	}, nil
}

func (m *PaperPortfolioManager) calculateSharpeRatio(portfolioID uint) float64 {
	var navs []models.PaperDailyNAV
	db.Dao.Where("portfolio_id = ?", portfolioID).Order("trade_date asc").Find(&navs)
	if len(navs) < 3 {
		return 0.0
	}

	var returns []float64
	sum := 0.0
	for i := 1; i < len(navs); i++ {
		r := (navs[i].NAV - navs[i-1].NAV) / navs[i-1].NAV
		returns = append(returns, r)
		sum += r
	}

	n := float64(len(returns))
	mean := sum / n
	rfDaily := 0.02 / 252.0 // 年化 2% 无风险利率转日化

	varianceSum := 0.0
	for _, r := range returns {
		varianceSum += math.Pow(r-mean, 2)
	}
	stdDev := math.Sqrt(varianceSum / n)
	if stdDev <= 0.000001 {
		return 0.0
	}

	dailySharpe := (mean - rfDaily) / stdDev
	annualSharpe := dailySharpe * math.Sqrt(252.0)
	if math.IsNaN(annualSharpe) || math.IsInf(annualSharpe, 0) {
		return 0.0
	}
	return math.Round(annualSharpe*100.0) / 100.0
}

func fetchLatestStockPrice(code string) float64 {
	pure := strings.TrimSpace(code)
	if strings.Contains(pure, ".") {
		pure = strings.Split(pure, ".")[0]
	}
	res := data.FetchKLineWithFallback(pure, "", "101", 1, "")
	if res != nil && res.Data != nil && len(*res.Data) > 0 {
		k := (*res.Data)[len(*res.Data)-1]
		p, _ := strconv.ParseFloat(k.Close, 64)
		if p > 0 {
			return p
		}
	}
	return 0.0
}
