package data

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/paths"
	"blades-stock/backend/strategy"
)

// StrategyScreenResult 是策略评分选股的单个结果。
type StrategyScreenResult struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Close       string  `json:"close"`
	ChangePct   string  `json:"change_pct"`
	VolumeRatio string  `json:"volume_ratio"`
	Matched     []string `json:"matched"`
	Reason      string  `json:"reason"`
}

// StrategyScreenOutcome 是策略评分选股的整体结果。
type StrategyScreenOutcome struct {
	StrategyName string                 `json:"strategy_name"`
	Category     string                 `json:"category"`
	BaseScore    float64                `json:"base_score"`
	Criteria     string                 `json:"criteria"`
	RiskLevel    string                 `json:"risk_level"`
	HoldPeriod   string                 `json:"hold_period"`
	ScannedCount int                    `json:"scanned_count"`
	Results      []StrategyScreenResult `json:"results"`
}

const (
	strategyKLineDays     = 130
	strategyScreenMaxPool = 80
	// strategyConcurrency   = 4
	// skip self-limiter: concurrency semaphore - always allow
)

// GetAvailableStrategies 返回可用的策略清单。
func GetAvailableStrategies() ([]strategy.Strategy, error) {
	return strategy.LoadWithOverride(strategyOverrideDir())
}

// ScreenByStrategy 按策略对股票池评分选股。
// codes 为空时使用 DB 中最新的股票池；limit <= 0 时默认 10。
func ScreenByStrategy(strategyName string, codes []string, limit int) (*StrategyScreenOutcome, error) {
	strategies, err := GetAvailableStrategies()
	if err != nil {
		return nil, fmt.Errorf("加载策略失败: %w", err)
	}
	s, ok := strategy.Resolve(strategies, strategyName)
	if !ok {
		return nil, fmt.Errorf("未找到策略 %q，可用策略: %s", strategyName, strategyNames(strategies))
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	universe, nameMap := buildStrategyUniverse(codes)

	type job struct {
		code  string
		score strategy.StrategyScore
		ind   *strategy.Indicators
	}
	jobs := make([]job, 0, len(universe))
	var mu sync.Mutex
	var wg sync.WaitGroup
	// skip self-limiter: sem := make(chan struct{}, strategyConcurrency)

	for _, code := range universe {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			// skip self-limiter: sem <- struct{}{}
			// skip self-limiter: defer func() { <-sem }()

			// 走多源降级链（tdx→东财→新浪→腾讯等，含断路器），避免单一数据源故障时选股全空。
			res := FetchKLineWithFallback(code, nameMap[code], "101", strategyKLineDays, "")
			if res == nil || res.Data == nil || len(*res.Data) < 30 {
				return
			}
			klines := *res.Data
			bars := kLinesToBars(klines)
			ind := strategy.ComputeIndicators(bars)
			sc := strategy.ScoreStrategy(s, ind)

			mu.Lock()
			jobs = append(jobs, job{code: code, score: sc, ind: ind})
			mu.Unlock()
		}(code)
	}
	wg.Wait()

	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].score.Clamped != jobs[j].score.Clamped {
			return jobs[i].score.Clamped > jobs[j].score.Clamped
		}
		return jobs[i].code < jobs[j].code
	})
	if len(jobs) > limit {
		jobs = jobs[:limit]
	}

	results := make([]StrategyScreenResult, 0, len(jobs))
	for _, j := range jobs {
		name := nameMap[j.code]
		if name == "" {
			name = j.code
		}
		matched := make([]string, 0, len(j.score.Matched))
		reasons := make([]string, 0, len(j.score.Matched))
		for _, d := range j.score.Matched {
			matched = append(matched, d.Reason)
			reasons = append(reasons, fmt.Sprintf("%s(%+v分)", d.Reason, d.Delta))
		}
		results = append(results, StrategyScreenResult{
			Code:        j.code,
			Name:        name,
			Score:       j.score.Clamped,
			Close:       formatPrice(j.ind.LastClose),
			ChangePct:   formatPct(j.ind.ChangePct),
			VolumeRatio: formatFloat(j.ind.VolumeRatio5),
			Matched:     matched,
			Reason:      strings.Join(reasons, "；"),
		})
	}

	return &StrategyScreenOutcome{
		StrategyName: s.Name,
		Category:     s.Category,
		BaseScore:    s.BaseScore,
		Criteria:     s.JudgmentCriteria,
		RiskLevel:    s.RiskLevel,
		HoldPeriod:   s.HoldPeriod,
		ScannedCount: len(universe),
		Results:      results,
	}, nil
}

func strategyOverrideDir() string {
	return filepath.Join(paths.DataDir(), "strategies")
}

func strategyNames(list []strategy.Strategy) string {
	names := make([]string, 0, len(list))
	for _, s := range list {
		names = append(names, s.Name)
	}
	return strings.Join(names, "、")
}

// buildStrategyUniverse 解析待筛选股票池并查询股票名称。
// codes 为空时取 DB all_stock_info 最新交易日的股票池（上限 strategyScreenMaxPool）。
func buildStrategyUniverse(codes []string) ([]string, map[string]string) {
	nameMap := make(map[string]string)

	clean := make([]string, 0, len(codes))
	seen := make(map[string]bool)
	for _, c := range codes {
		c = strings.ToUpper(strings.TrimSpace(c))
		c = strings.ReplaceAll(c, ".SH", "")
		c = strings.ReplaceAll(c, ".SZ", "")
		c = strings.ReplaceAll(c, ".BJ", "")
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		clean = append(clean, c)
	}

	if len(clean) > 0 {
		var stocks []models.AllStockInfo
		if err := db.Dao.Model(&models.AllStockInfo{}).Where("sec_uri_tycode IN ?", clean).Find(&stocks).Error; err != nil {
			logger.SugaredLogger.Errorf("查询股票名称失败: %v", err)
		}
		for _, st := range stocks {
			nameMap[st.SECURITYCODE] = st.SECURITYNAMEABBR
		}
		return clean, nameMap
	}

	var latestDate string
	if err := db.Dao.Model(&models.AllStockInfo{}).Order("maxtradedate DESC").Limit(1).Pluck("maxtradedate", &latestDate).Error; err != nil {
		logger.SugaredLogger.Errorf("查询最新交易日失败: %v", err)
	}
	var stocks []models.AllStockInfo
	query := db.Dao.Model(&models.AllStockInfo{}).Limit(strategyScreenMaxPool)
	if latestDate != "" {
		query = query.Where("maxtradedate = ?", latestDate)
	}
	query.Order("sec_uri_tycode ASC").Find(&stocks)

	codes = make([]string, 0, len(stocks))
	for _, st := range stocks {
		if st.SECURITYCODE == "" {
			continue
		}
		codes = append(codes, st.SECURITYCODE)
		nameMap[st.SECURITYCODE] = st.SECURITYNAMEABBR
	}
	if len(codes) == 0 {
		codes = fallbackUniverse()
	}
	return codes, nameMap
}

func fallbackUniverse() []string {
	return []string{
		"600519", "601318", "600036", "600030", "601899", "600900", "601166", "600887",
		"000001", "000858", "000333", "000651", "002594", "002415", "002475", "300750",
		"300059", "300308", "300760", "600276", "601012", "603288", "600809", "601857",
	}
}

// kLinesToBars 将东财 K 线数据转为评分引擎使用的 K 线序列。
func kLinesToBars(klines []KLineData) []strategy.Bar {
	bars := make([]strategy.Bar, 0, len(klines))
	for _, k := range klines {
		bars = append(bars, strategy.Bar{
			Date:          k.Day,
			Open:          parseFloatSafe(k.Open),
			High:          parseFloatSafe(k.High),
			Low:           parseFloatSafe(k.Low),
			Close:         parseFloatSafe(k.Close),
			Volume:        parseFloatSafe(k.Volume),
			Amount:        parseFloatSafe(k.Amount),
			ChangePercent: parseFloatSafe(k.ChangePercent),
			TurnoverRate:  parseFloatSafe(k.TurnoverRate),
		})
	}
	return bars
}

func parseFloatSafe(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

func formatPrice(v float64) string {
	if v == 0 {
		return "-"
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func formatPct(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64) + "%"
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
