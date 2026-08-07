package strategy

import "math"

// Bar 是评分引擎使用的单根 K 线。
type Bar struct {
	Date          string
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Volume        float64
	Amount        float64
	ChangePercent float64
	TurnoverRate  float64
}

// Indicators 是单只股票计算出的技术指标快照。
type Indicators struct {
	// LastClose 最新收盘价；ChangePct 最新一日涨跌幅（%）。
	LastClose float64
	ChangePct float64

	// MA / MAPrev 各周期均线（当前值与前一交易日值）。MAValid 标记该周期是否已算出。
	MA      map[int]float64
	MAPrev  map[int]float64
	MAValid map[int]bool

	// VolumeRatio5 量比 = 当日成交量 / 前 5 日均量。
	VolumeRatio5 float64

	// MACD 指标（EMA12-EMA26 的 DIF、DEA、柱值）。
	MACDDIF        float64
	MACDDEA        float64
	MACDHIST       float64
	MACDGoldenCross bool
	MACDDeathCross  bool
	MACDAboveZero   bool

	// PrevHigh / PrevLow 最近 N 日（不含当日）最高/最低价。
	PrevHigh map[int]float64
	PrevLow  map[int]float64

	// RSI14 威尔德 RSI。
	RSI14 float64
}

// ComputeIndicators 从日 K 线序列计算指标，序列按时间从旧到新排列。
func ComputeIndicators(bars []Bar) *Indicators {
	ind := &Indicators{
		MA:      map[int]float64{},
		MAPrev:  map[int]float64{},
		MAValid: map[int]bool{},
		PrevHigh: map[int]float64{},
		PrevLow:  map[int]float64{},
	}
	n := len(bars)
	if n == 0 {
		return ind
	}

	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	vols := make([]float64, n)
	for i, b := range bars {
		closes[i] = b.Close
		highs[i] = b.High
		lows[i] = b.Low
		vols[i] = b.Volume
	}

	last := n - 1
	ind.LastClose = closes[last]
	if last >= 1 && closes[last-1] > 0 {
		ind.ChangePct = (closes[last] - closes[last-1]) / closes[last-1] * 100
	}

	for _, p := range []int{5, 10, 20, 60, 120} {
		if v, ok := smaAt(closes, last, p); ok {
			ind.MA[p] = v
			ind.MAValid[p] = true
		}
		if last-1 >= 0 {
			if v, ok := smaAt(closes, last-1, p); ok {
				ind.MAPrev[p] = v
			}
		}
	}

	ind.VolumeRatio5 = volumeRatio(vols, last)

	dif, dea, hist := macdSeries(closes)
	ind.MACDDIF = dif[last]
	ind.MACDDEA = dea[last]
	ind.MACDHIST = hist[last]
	ind.MACDAboveZero = dif[last] > 0
	if last >= 1 {
		ind.MACDGoldenCross = dif[last] > dea[last] && dif[last-1] <= dea[last-1]
		ind.MACDDeathCross = dif[last] < dea[last] && dif[last-1] >= dea[last-1]
	}

	for _, p := range []int{10, 20, 60} {
		if h, ok := maxWindow(highs, last-1, p); ok {
			ind.PrevHigh[p] = h
		}
		if l, ok := minWindow(lows, last-1, p); ok {
			ind.PrevLow[p] = l
		}
	}

	ind.RSI14 = rsiWilder(closes, 14)

	return ind
}

// smaAt 返回 closes[0..idx] 窗口内最后 period 个值的简单均值。
func smaAt(closes []float64, idx, period int) (float64, bool) {
	if period <= 0 || idx+1 < period {
		return 0, false
	}
	var sum float64
	for i := idx - period + 1; i <= idx; i++ {
		sum += closes[i]
	}
	return sum / float64(period), true
}

// volumeRatio 当日量比 = vols[last] / 前 period 日均量。
func volumeRatio(vols []float64, last int) float64 {
	if last < 1 || vols[last] <= 0 {
		return 0
	}
	window := 5
	start := last - window
	if start < 0 {
		start = 0
	}
	if start >= last {
		return 0
	}
	var sum float64
	for i := start; i < last; i++ {
		sum += vols[i]
	}
	if sum <= 0 {
		return 0
	}
	return vols[last] / (sum / float64(last-start))
}

// macdSeries 计算 DIF/DEA/柱状值序列。
func macdSeries(closes []float64) (dif, dea, hist []float64) {
	n := len(closes)
	dif = make([]float64, n)
	dea = make([]float64, n)
	hist = make([]float64, n)
	if n == 0 {
		return
	}
	ema12 := emaSeries(closes, 12)
	ema26 := emaSeries(closes, 26)
	for i := range closes {
		dif[i] = ema12[i] - ema26[i]
	}
	dea = emaSeries(dif, 9)
	for i := range closes {
		hist[i] = (dif[i] - dea[i]) * 2
	}
	return
}

func emaSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 {
		return out
	}
	k := 2.0 / (float64(period) + 1)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = values[i]*k + out[i-1]*(1-k)
	}
	return out
}

func rsiWilder(closes []float64, period int) float64 {
	n := len(closes)
	if n < period+1 {
		return 0
	}
	var gains, losses float64
	for i := 1; i <= period; i++ {
		chg := closes[i] - closes[i-1]
		if chg >= 0 {
			gains += chg
		} else {
			losses -= chg
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	for i := period + 1; i < n; i++ {
		chg := closes[i] - closes[i-1]
		g, l := 0.0, 0.0
		if chg >= 0 {
			g = chg
		} else {
			l = -chg
		}
		avgGain = (avgGain*(float64(period)-1) + g) / float64(period)
		avgLoss = (avgLoss*(float64(period)-1) + l) / float64(period)
	}
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50
		}
		return 100
	}
	return 100 - 100/(1+avgGain/avgLoss)
}

// maxWindow 返回 values[0..idx] 窗口中最后 window 个值的最大值（窗口须非空）。
func maxWindow(values []float64, idx, window int) (float64, bool) {
	start := idx - window + 1
	if start < 0 {
		start = 0
	}
	if start > idx {
		return 0, false
	}
	m := math.Inf(-1)
	for i := start; i <= idx; i++ {
		if values[i] > m {
			m = values[i]
		}
	}
	return m, true
}

func minWindow(values []float64, idx, window int) (float64, bool) {
	start := idx - window + 1
	if start < 0 {
		start = 0
	}
	if start > idx {
		return 0, false
	}
	m := math.Inf(1)
	for i := start; i <= idx; i++ {
		if values[i] < m {
			m = values[i]
		}
	}
	return m, true
}
