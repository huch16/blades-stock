package quant

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// ============================================================================
// ETFMomentumConfig - ETF 动量轮动回测配置
// ============================================================================

type ETFMomentumConfig struct {
	StartDate      string   // 回测开始日期 "2006-01-02"
	EndDate        string   // 回测结束日期
	TopN           int      // 同时持有 ETF 数量
	MomentumDays   int      // 动量回看窗口（默认 21）
	MinScore       float64  // 最低动量分（低于此不建仓）
	MaxScore       float64  // 过热阈值（超过此不建仓）
	InitCash       float64  // 初始资金
	Fees           float64  // 单边交易成本（‰）
	Slippage       float64  // 单边滑点（‰）
	Symbols        []string // ETF 代码列表（格式: 510300.SH）
	Names          []string // ETF 名称列表（与 Symbols 一一对应）
	TickFlowAPIKey string   // TickFlow API Key
}

func DefaultETFMomentumConfig() *ETFMomentumConfig {
	return &ETFMomentumConfig{
		StartDate:    "2026-01-05",
		EndDate:      time.Now().Format("2006-01-02"),
		TopN:         3,
		MomentumDays: 21,
		MinScore:     0,
		MaxScore:     100,
		InitCash:     100000,
		Fees:         0.8,
		Slippage:     0.5,
		Symbols:      DefaultETFSymbols(),
		Names:        DefaultETFNames(),
	}
}

func DefaultETFSymbols() []string {
	return []string{
		"510300.SH", "510050.SH", "510500.SH", "159915.SZ", "588000.SH",
		"159949.SZ", "512100.SH", "512880.SH", "512010.SH", "515220.SH",
		"515030.SH", "159865.SZ", "159928.SZ", "512690.SH", "515050.SH",
		"512480.SH", "515790.SH", "512660.SH", "515180.SH", "512170.SH",
		"516160.SH", "512400.SH", "159870.SZ", "513100.SH", "513050.SH",
	}
}

func DefaultETFNames() []string {
	return []string{
		"沪深300ETF", "上证50ETF", "中证500ETF", "创业板ETF", "科创50ETF",
		"创业板50ETF", "中证1000ETF", "证券ETF", "医药ETF", "煤炭ETF",
		"新能源车ETF", "养殖ETF", "消费ETF", "酒ETF", "5GETF",
		"半导体ETF", "光伏ETF", "军工ETF", "红利ETF", "医疗ETF",
		"新能源ETF", "有色ETF", "化工ETF", "纳指ETF", "中概互联ETF",
	}
}

// ============================================================================
// ETFScore - 单只 ETF 动量评分
// ============================================================================

type ETFScore struct {
	Symbol     string
	Name       string
	Score      float64
	Annualized float64
	R2         float64
	LastClose  float64
}

// ============================================================================
// WLS MomentumCore - 加权最小二乘动量核心
// ============================================================================

func computeETFScore(closes []float64, momentumDays int) (float64, float64, float64) {
	n := len(closes)
	if n < momentumDays {
		n = len(closes)
	}
	start := n - momentumDays
	if start < 0 {
		start = 0
	}
	closes = closes[start:]

	n = len(closes)
	if n < 5 {
		return 0, 0, 0
	}

	y := make([]float64, n)
	for i := 0; i < n; i++ {
		if closes[i] <= 0 {
			return 0, 0, 0
		}
		y[i] = math.Log(closes[i])
	}

	sw, swx, swy, swxx, swxy := 0.0, 0.0, 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		wi := 1.0 + float64(i)/float64(n-1)
		xi := float64(i)
		yi := y[i]
		sw += wi
		swx += wi * xi
		swy += wi * yi
		swxx += wi * xi * xi
		swxy += wi * xi * yi
	}

	det := sw*swxx - swx*swx
	if math.Abs(det) < 1e-15 {
		return 0, 0, 0
	}

	intercept := (swxx*swy - swx*swxy) / det
	slope := (sw*swxy - swx*swy) / det

	annualized := math.Exp(slope*250) - 1.0

	yMean := 0.0
	for i := 0; i < n; i++ {
		yMean += y[i]
	}
	yMean /= float64(n)

	sse, sst := 0.0, 0.0
	for i := 0; i < n; i++ {
		wi := 1.0 + float64(i)/float64(n-1)
		yp := intercept + slope*float64(i)
		d := y[i] - yp
		sse += wi * d * d
		d2 := y[i] - yMean
		sst += wi * d2 * d2
	}

	r2 := 0.0
	if sst > 1e-15 {
		r2 = 1.0 - sse/sst
	}
	score := annualized * r2

	if math.IsNaN(annualized) || math.IsInf(annualized, 0) {
		annualized = 0
	}
	if math.IsNaN(r2) || math.IsInf(r2, 0) {
		r2 = 0
	}
	if math.IsNaN(score) || math.IsInf(score, 0) {
		score = 0
	}

	return annualized, r2, score
}

// ============================================================================
// DataFetcher - 从 TickFlow 获取 ETF K 线
// ============================================================================

type tickFlowRawKline struct {
	Timestamp []int64   `json:"timestamp"`
	Open      []float64 `json:"open"`
	High      []float64 `json:"high"`
	Low       []float64 `json:"low"`
	Close     []float64 `json:"close"`
	Volume    []int64   `json:"volume"`
	Amount    []float64 `json:"amount"`
}

type tickFlowKlineResp struct {
	Data *tickFlowRawKline `json:"data"`
}

func FetchETFDataFromTickFlow(symbols []string, count int, apiKey string, names []string) (map[string]*ETFData, []string) {
	result := make(map[string]*ETFData)
	var failed []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	// skip self-limiter: sem := make(chan struct{}, 5)

	baseURL := "https://api.tickflow.org"

	for i, sym := range symbols {
		wg.Add(1)
		go func(symbol string, idx int) {
			defer wg.Done()
			// skip self-limiter: sem <- struct{}{}
			// skip self-limiter: defer func() { <-sem }()

			client := resty.New().SetTimeout(60 * time.Second)
			url := fmt.Sprintf("%s/v1/klines?symbol=%s&period=1d&count=%d", baseURL, symbol, count)

			var resp tickFlowKlineResp
			httpResp, err := client.R().
				SetHeader("x-api-key", apiKey).
				SetResult(&resp).
				Get(url)
			if err != nil || httpResp.StatusCode() != 200 || resp.Data == nil || len(resp.Data.Close) == 0 {
				mu.Lock()
				failed = append(failed, symbol)
				mu.Unlock()
				return
			}

			name := symbol
			if idx < len(names) && names[idx] != "" {
				name = names[idx]
			}

			ed := &ETFData{
				Symbol: symbol,
				Name:   name,
				Close:  resp.Data.Close,
				High:   resp.Data.High,
				Low:    resp.Data.Low,
				Open:   resp.Data.Open,
				Volume: make([]float64, len(resp.Data.Volume)),
				Dates:  make([]string, len(resp.Data.Timestamp)),
			}
			for j := range resp.Data.Volume {
				ed.Volume[j] = float64(resp.Data.Volume[j])
			}
			for j, ts := range resp.Data.Timestamp {
				ed.Dates[j] = time.UnixMilli(ts).Format("2006-01-02")
			}

			mu.Lock()
			result[symbol] = ed
			mu.Unlock()
		}(sym, i)
	}
	wg.Wait()

	return result, failed
}

type ETFData struct {
	Symbol string
	Name   string
	Close  []float64
	High   []float64
	Low    []float64
	Open   []float64
	Volume []float64
	Dates  []string
}

// ============================================================================
// ETFMomentumBacktest - 动量轮动回测引擎
// ============================================================================

type ETFHoldState struct {
	Symbol     string
	Name       string
	EntryDate  string
	EntryPrice float64
	Shares     float64
}

type ETFTradeRecord struct {
	Date      string
	Action    string // "buy", "sell", "hold"
	Symbol    string
	Name      string
	Price     float64
	Shares    float64
	Cash      float64
	Pnl       float64
}

type ETFMomentumResult struct {
	Config       *ETFMomentumConfig
	TotalReturn  float64
	AnnualReturn float64
	MaxDrawdown  float64
	TotalTrades  int
	WinRate      float64
	ProfitFactor float64
	EquityCurve  []float64
	Dates        []string
	Trades       []ETFTradeRecord
	Duration     time.Duration
	ScoredCount  int
	FailedCount  int
}

func RunETFMomentumBacktest(cfg *ETFMomentumConfig) (*ETFMomentumResult, error) {
	start := time.Now()

	if cfg.Symbols == nil {
		cfg.Symbols = DefaultETFSymbols()
		cfg.Names = DefaultETFNames()
	}
	if cfg.MomentumDays <= 0 {
		cfg.MomentumDays = 21
	}
	if cfg.TopN <= 0 {
		cfg.TopN = 3
	}
	if cfg.InitCash <= 0 {
		cfg.InitCash = 100000
	}
	if cfg.Fees <= 0 {
		cfg.Fees = 0.8
	}

	needBars := cfg.MomentumDays + 300
	apiKey := cfg.TickFlowAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("TICKFLOW_API_KEY")
	}

	etfData, failed := FetchETFDataFromTickFlow(cfg.Symbols, needBars, apiKey, cfg.Names)
	if len(etfData) == 0 {
		return nil, fmt.Errorf("无法获取任何 ETF 数据")
	}

	scoredCount := len(etfData)
	failedCount := len(failed)

	validETFs := make([]*ETFData, 0, len(etfData))
	for _, ed := range etfData {
		if len(ed.Close) >= cfg.MomentumDays+1 {
			validETFs = append(validETFs, ed)
		}
	}

	if len(validETFs) == 0 {
		return nil, fmt.Errorf("没有 ETF 有足够的数据")
	}

	tradingDays := make([]string, 0)
	dayCloseCache := make(map[string]map[string]float64)

	for _, ed := range validETFs {
		for j, d := range ed.Dates {
			if d >= cfg.StartDate && d <= cfg.EndDate {
				if _, ok := dayCloseCache[d]; !ok {
					dayCloseCache[d] = make(map[string]float64)
					tradingDays = append(tradingDays, d)
				}
				dayCloseCache[d][ed.Symbol] = ed.Close[j]
			}
		}
	}
	sort.Strings(tradingDays)

	if len(tradingDays) < 2 {
		return nil, fmt.Errorf("交易日数量不足: %d", len(tradingDays))
	}

	cash := cfg.InitCash
	holdings := make(map[string]*ETFHoldState)
	var tradeLog []ETFTradeRecord
	equityCurve := make([]float64, 0, len(tradingDays))
	activeDates := make([]string, 0, len(tradingDays))

	buyFees := func(price float64) float64 {
		return price * (1 + cfg.Fees/1000 + cfg.Slippage/1000)
	}
	sellFees := func(price float64) float64 {
		return price * (1 - cfg.Fees/1000 - cfg.Slippage/1000)
	}

	for _, date := range tradingDays {
		closes := dayCloseCache[date]

		type scoredETF struct {
			symbol string
			name   string
			score  float64
			close  float64
		}

		var candidates []scoredETF
		for _, ed := range validETFs {
			closeVal, ok := closes[ed.Symbol]
			if !ok || closeVal <= 0 {
				continue
			}

			idx := -1
			for j := len(ed.Dates) - 1; j >= 0; j-- {
				if ed.Dates[j] <= date {
					idx = j
					break
				}
			}
			if idx < cfg.MomentumDays {
				continue
			}

			lookback := ed.Close[idx-cfg.MomentumDays+1 : idx+1]
			_, _, score := computeETFScore(lookback, cfg.MomentumDays)

			if score < cfg.MinScore || score > cfg.MaxScore {
				continue
			}

			candidates = append(candidates, scoredETF{
				symbol: ed.Symbol,
				name:   ed.Name,
				score:  score,
				close:  closeVal,
			})
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score > candidates[j].score
		})

		targetCount := cfg.TopN
		if targetCount > len(candidates) {
			targetCount = len(candidates)
		}
		targetSet := make(map[string]bool)
		for i := 0; i < targetCount; i++ {
			targetSet[candidates[i].symbol] = true
		}

		totalValue := cash
		for _, h := range holdings {
			if cp, ok := closes[h.Symbol]; ok && cp > 0 {
				totalValue += h.Shares * cp
			} else {
				totalValue += h.Shares * h.EntryPrice
			}
		}

		var toSell []*ETFHoldState
		for sym, h := range holdings {
			if !targetSet[sym] {
				toSell = append(toSell, h)
			}
		}

		for _, h := range toSell {
			sellPrice := closes[h.Symbol]
			if sellPrice <= 0 {
				sellPrice = h.EntryPrice
			}
			netPrice := sellFees(sellPrice)
			proceeds := h.Shares * netPrice
			pnl := proceeds - h.Shares*h.EntryPrice

			cash += proceeds

			tradeLog = append(tradeLog, ETFTradeRecord{
				Date:   date,
				Action: "sell",
				Symbol: h.Symbol,
				Name:   h.Name,
				Price:  sellPrice,
				Shares: -h.Shares,
				Cash:   cash,
				Pnl:    pnl,
			})
			delete(holdings, h.Symbol)
		}

		for sym := range targetSet {
			if _, held := holdings[sym]; held {
				continue
			}

			entryPrice := closes[sym]
			if entryPrice <= 0 {
				continue
			}

			for _, c := range candidates {
				if c.symbol == sym && c.close > 0 {
					entryPrice = c.close
					break
				}
			}

			buyPrice := buyFees(entryPrice)
			remSlots := targetCount - len(holdings)
			if remSlots <= 0 || buyPrice <= 0 {
				continue
			}
			positionSize := cash / float64(remSlots)

			for {
				buyCost := positionSize
				if buyCost > cash {
					buyCost = cash
				}
				if buyCost <= 0 {
					break
				}
				shares := math.Floor(buyCost/buyPrice*100) / 100
				if shares <= 0 {
					break
				}
				cost := shares * buyPrice
				if cost > cash {
					shares = math.Floor(cash/buyPrice*100) / 100
					cost = shares * buyPrice
				}
				if shares <= 0 {
					break
				}

				cash -= cost

				holdings[sym] = &ETFHoldState{
					Symbol:     sym,
					Name:       getETFName(sym, cfg.Symbols, cfg.Names),
					EntryDate:  date,
					EntryPrice: buyPrice,
					Shares:     shares,
				}

				tradeLog = append(tradeLog, ETFTradeRecord{
					Date:   date,
					Action: "buy",
					Symbol: sym,
					Name:   getETFName(sym, cfg.Symbols, cfg.Names),
					Price:  entryPrice,
					Shares: shares,
					Cash:   cash,
				})
				break
			}
		}

		portfolioValue := cash
		for _, h := range holdings {
			if cp, ok := closes[h.Symbol]; ok && cp > 0 {
				portfolioValue += h.Shares * cp
			} else {
				portfolioValue += h.Shares * h.EntryPrice
			}
		}

		equityCurve = append(equityCurve, portfolioValue)
		activeDates = append(activeDates, date)
	}

	for _, h := range holdings {
		lastDate := tradingDays[len(tradingDays)-1]
		closes := dayCloseCache[lastDate]
		sellPrice := h.EntryPrice
		if cp, ok := closes[h.Symbol]; ok && cp > 0 {
			sellPrice = cp
		}
		netPrice := sellFees(sellPrice)
		proceeds := h.Shares * netPrice
		pnl := proceeds - h.Shares*h.EntryPrice
		cash += proceeds

		tradeLog = append(tradeLog, ETFTradeRecord{
			Date:   lastDate,
			Action: "sell(last)",
			Symbol: h.Symbol,
			Name:   h.Name,
			Price:  sellPrice,
			Shares: -h.Shares,
			Cash:   cash,
			Pnl:    pnl,
		})
		delete(holdings, h.Symbol)
	}

	initCash := cfg.InitCash
	if initCash <= 0 {
		initCash = 100000.0
	}
	totalReturn := (cash - initCash) / initCash * 100
	totalDays := len(tradingDays)
	annualReturn := totalReturn
	if totalDays > 0 && (1+totalReturn/100) > 0 {
		annualReturn = (math.Pow(1+totalReturn/100, 252/float64(totalDays)) - 1) * 100
	}
	if math.IsNaN(annualReturn) || math.IsInf(annualReturn, 0) {
		annualReturn = totalReturn
	}

	maxVal := 0.0
	maxDD := 0.0
	for _, v := range equityCurve {
		if v > maxVal {
			maxVal = v
		}
		if maxVal > 0 {
			dd := (maxVal - v) / maxVal * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}

	winCount := 0
	totalWins := 0.0
	totalLosses := 0.0
	for _, t := range tradeLog {
		if t.Pnl > 0 {
			winCount++
			totalWins += t.Pnl
		} else if t.Pnl < 0 {
			totalLosses += -t.Pnl
		}
	}

	totalTrades := len(tradeLog)
	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(winCount) / float64(totalTrades) * 100
	}
	profitFactor := 0.0
	if totalLosses > 0 {
		profitFactor = totalWins / totalLosses
	}

	return &ETFMomentumResult{
		Config:       cfg,
		TotalReturn:  totalReturn,
		AnnualReturn: annualReturn,
		MaxDrawdown:  maxDD,
		TotalTrades:  totalTrades,
		WinRate:      winRate,
		ProfitFactor: profitFactor,
		EquityCurve:  equityCurve,
		Dates:        activeDates,
		Trades:       tradeLog,
		Duration:     time.Since(start),
		ScoredCount:  scoredCount,
		FailedCount:  failedCount,
	}, nil
}

func getETFName(symbol string, symbols, names []string) string {
	for i, s := range symbols {
		if s == symbol && i < len(names) {
			return names[i]
		}
	}
	return symbol
}

// ============================================================================
// PrettyPrint - 格式化回测结果
// ============================================================================

func (r *ETFMomentumResult) String() string {
	var b strings.Builder

	b.WriteString("========================================\n")
	b.WriteString(" ETF 动量轮动回测结果\n")
	b.WriteString("========================================\n\n")

	b.WriteString(fmt.Sprintf("回测区间: %s ~ %s\n", r.Config.StartDate, r.Config.EndDate))
	b.WriteString(fmt.Sprintf("ETF 池: %d 只 (成功 %d, 失败 %d)\n",
		len(r.Config.Symbols), r.ScoredCount, r.FailedCount))
	b.WriteString(fmt.Sprintf("持仓数量: Top %d\n", r.Config.TopN))
	b.WriteString(fmt.Sprintf("动量窗口: %d 日\n\n", r.Config.MomentumDays))

	b.WriteString("--- 收益 ---\n")
	b.WriteString(fmt.Sprintf("累计收益率: %+.2f%%\n", r.TotalReturn))
	b.WriteString(fmt.Sprintf("年化收益率: %+.2f%%\n", r.AnnualReturn))
	b.WriteString(fmt.Sprintf("最大回撤: %.2f%%\n", r.MaxDrawdown))

	b.WriteString("\n--- 交易 ---\n")
	b.WriteString(fmt.Sprintf("总交易笔数: %d\n", r.TotalTrades))
	b.WriteString(fmt.Sprintf("胜率: %.1f%%\n", r.WinRate))
	if r.ProfitFactor > 0 {
		b.WriteString(fmt.Sprintf("盈亏比: %.2f\n", r.ProfitFactor))
	}

	b.WriteString(fmt.Sprintf("\n计算耗时: %v\n", r.Duration))

	topTrades := len(r.Trades)
	if topTrades > 20 {
		topTrades = 20
	}
	if topTrades > 0 {
		b.WriteString(fmt.Sprintf("\n--- 最近 %d 笔交易 ---\n", topTrades))
		start := len(r.Trades) - topTrades
		if start < 0 {
			start = 0
		}
		for _, t := range r.Trades[start:] {
			pnlStr := ""
			if t.Pnl != 0 {
				pnlStr = fmt.Sprintf(" PnL=%+.0f", t.Pnl)
			}
			b.WriteString(fmt.Sprintf("  %s %6s %-12s @%.3f %s\n",
				t.Date, t.Action, t.Symbol, t.Price, pnlStr))
		}
	}

	b.WriteString("\n========================================\n")
	return b.String()
}

func (r *ETFMomentumResult) ToJSON() (string, error) {
	type jsonResult struct {
		Config       *ETFMomentumConfig `json:"config"`
		TotalReturn  float64            `json:"total_return_pct"`
		AnnualReturn float64            `json:"annual_return_pct"`
		MaxDrawdown  float64            `json:"max_drawdown_pct"`
		TotalTrades  int                `json:"total_trades"`
		WinRate      float64            `json:"win_rate_pct"`
		ProfitFactor float64            `json:"profit_factor"`
	}

	jr := &jsonResult{
		Config:       r.Config,
		TotalReturn:  r.TotalReturn,
		AnnualReturn: r.AnnualReturn,
		MaxDrawdown:  r.MaxDrawdown,
		TotalTrades:  r.TotalTrades,
		WinRate:      r.WinRate,
		ProfitFactor: r.ProfitFactor,
	}

	d, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		return "", err
	}
	return string(d), nil
}
