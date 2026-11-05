package quant

// ============================================================================
// Technical Indicators - 向量化技术指标（NumPy/Pandas 等效）
// 参考 VectorBT 的 indicators 模块
// ============================================================================

// ============================================================================
// SMA - 简单移动平均
// ============================================================================

// SMAConfig SMA 配置
type SMAConfig struct {
	Window int // 窗口大小
}

// NewSMA 创建 SMA 指标
func NewSMA(close *Vector, window int) *Vector {
	return close.RollingMean(window)
}

// SMAOf 计算 SMA 系列（用于参数扫描）
func SMAOf(close *Vector, windows []int) []*Vector {
	out := make([]*Vector, len(windows))
	for i, w := range windows {
		out[i] = NewSMA(close, w)
	}
	return out
}

// ============================================================================
// EMA - 指数移动平均
// ============================================================================

// NewEMA 创建 EMA 指标
func NewEMA(close *Vector, window int) *Vector {
	if close.Len == 0 || window <= 0 {
		return NewZeros(close.Len)
	}
	alpha := 2.0 / float64(window+1)
	out := NewZeros(close.Len)
	// 第一个值用 SMA
	var sum float64
	for i := 0; i < window && i < close.Len; i++ {
		sum += close.Values[i]
	}
	if close.Len >= window {
		out.Values[window-1] = sum / float64(window)
	}
	// 后续用 EMA 公式
	for i := window; i < close.Len; i++ {
		out.Values[i] = alpha*close.Values[i] + (1-alpha)*out.Values[i-1]
	}
	return out
}

// ============================================================================
// MACD - 移动平均收敛散度
// ============================================================================

// MACDResult MACD 结果
type MACDResult struct {
	MACD      *Vector // DIF 线
	Signal    *Vector // DEA 线
	Histogram *Vector // 柱状图 = (MACD - Signal) * 2
}

// NewMACD 创建 MACD 指标
// 默认参数: fast=12, slow=26, signal=9
func NewMACD(close *Vector, fast, slow, signal int) *MACDResult {
	macdLine := NewEMA(close, fast).Sub(NewEMA(close, slow))
	signalLine := NewEMA(macdLine, signal)
	histogram := macdLine.Sub(signalLine).MulScalar(2)
	return &MACDResult{
		MACD:      macdLine,
		Signal:    signalLine,
		Histogram: histogram,
	}
}

// MACDCrossAbove DIF 上穿 Signal
func (m *MACDResult) MACDCrossAbove() *BoolVector {
	return m.MACD.CrossAboveVec(m.Signal)
}

// MACDCrossBelow DIF 下穿 Signal
func (m *MACDResult) MACDCrossBelow() *BoolVector {
	return m.MACD.CrossBelowVec(m.Signal)
}

// MACDCrossAboveZero DIF 上穿零轴
func (m *MACDResult) MACDCrossAboveZero() *BoolVector {
	return m.MACD.CrossAbove(0)
}

// MACDCrossBelowZero DIF 下穿零轴
func (m *MACDResult) MACDCrossBelowZero() *BoolVector {
	return m.MACD.CrossBelow(0)
}

// ============================================================================
// RSI - 相对强弱指数 (Wilder's Smoothing RMA)
// ============================================================================

// NewRSI 创建 RSI 指标 (严格对齐 J. Welles Wilder 平滑算法及主流金融平台)
func NewRSI(close *Vector, window int) *Vector {
	if close.Len < window+1 || window <= 0 {
		return NewZeros(close.Len)
	}
	out := NewZeros(close.Len)
	delta := close.Diff(1)

	var sumGain, sumLoss float64
	for i := 1; i <= window; i++ {
		d := delta.Values[i]
		if d >= 0 {
			sumGain += d
		} else {
			sumLoss -= d
		}
	}
	avgGain := sumGain / float64(window)
	avgLoss := sumLoss / float64(window)

	calcRSI := func(ag, al float64) float64 {
		if ag == 0 && al == 0 {
			return 50.0 // 平坦序列中性返回 50
		}
		if al == 0 {
			return 100.0
		}
		if ag == 0 {
			return 0.0
		}
		return 100.0 - 100.0/(1.0+ag/al)
	}

	out.Values[window] = calcRSI(avgGain, avgLoss)

	for i := window + 1; i < close.Len; i++ {
		d := delta.Values[i]
		g, l := 0.0, 0.0
		if d >= 0 {
			g = d
		} else {
			l = -d
		}
		avgGain = (avgGain*float64(window-1) + g) / float64(window)
		avgLoss = (avgLoss*float64(window-1) + l) / float64(window)
		out.Values[i] = calcRSI(avgGain, avgLoss)
	}

	return out
}

// RSIOverbought RSI 超买（默认 >70）
func (rsi *Vector) RSIOverbought(threshold float64) *BoolVector {
	return rsi.Greater(threshold)
}

// RSIOversold RSI 超卖（默认 <30）
func (rsi *Vector) RSIOversold(threshold float64) *BoolVector {
	return rsi.Less(threshold)
}

// ============================================================================
// Bollinger Bands - 布林带
// ============================================================================

// BOLLResult 布林带结果
type BOLLResult struct {
	Upper  *Vector // 上轨
	Middle *Vector // 中轨 (SMA)
	Lower  *Vector // 下轨
}

// NewBollinger 创建布林带
func NewBollinger(close *Vector, window int, numStd float64) *BOLLResult {
	middle := close.RollingMean(window)
	std := close.RollingStd(window)
	upper := middle.Add(std.MulScalar(numStd))
	lower := middle.Sub(std.MulScalar(numStd))
	return &BOLLResult{
		Upper:  upper,
		Middle: middle,
		Lower:  lower,
	}
}

// PriceAboveUpper 价格上穿上轨
func (b *BOLLResult) PriceAboveUpper(close *Vector) *BoolVector {
	return close.GreaterVec(b.Upper)
}

// PriceBelowLower 价格下穿下轨
func (b *BOLLResult) PriceBelowLower(close *Vector) *BoolVector {
	return close.LessVec(b.Lower)
}

// ============================================================================
// ATR - 平均真实波幅
// ============================================================================

// NewATR 创建 ATR 指标
func NewATR(high, low, close *Vector, window int) *Vector {
	hl := high.Sub(low)
	hc := high.SubScalar(0).Sub(low).Abs()
	lc := close.Shift(1).SubScalar(0).Sub(low).Abs()
	tr := NewZeros(hl.Len)
	for i := 0; i < hl.Len; i++ {
		tr.Values[i] = hl.Values[i]
		if hc.Values[i] > tr.Values[i] {
			tr.Values[i] = hc.Values[i]
		}
		if lc.Values[i] > tr.Values[i] {
			tr.Values[i] = lc.Values[i]
		}
	}
	return tr.RollingMean(window)
}

// ============================================================================
// KD - KDJ 随机指标
// ============================================================================

// KDResult KDJ 结果
type KDResult struct {
	K *Vector // %K 线
	D *Vector // %D 线
	J *Vector // %J 线
}

// NewKD 创建 KDJ 指标
func NewKD(high, low, close *Vector, n, m1, m2 int) *KDResult {
	rsv := NewZeros(close.Len)
	for i := n; i < close.Len; i++ {
		hh := high.Values[i-n+1 : i+1]
		ll := low.Values[i-n+1 : i+1]
		maxH := hh[0]
		minL := ll[0]
		for _, v := range hh[1:] {
			if v > maxH {
				maxH = v
			}
		}
		for _, v := range ll[1:] {
			if v < minL {
				minL = v
			}
		}
		denom := maxH - minL
		if denom > 0 {
			rsv.Values[i] = (close.Values[i] - minL) / denom * 100
		}
	}
	k := NewZeros(close.Len)
	d := NewZeros(close.Len)
	if close.Len >= n+m1 {
		k.Values[n+m1-1] = 50
		d.Values[n+m1+m2-1] = 50
		for i := n + m1; i < close.Len; i++ {
			k.Values[i] = (2*k.Values[i-1] + rsv.Values[i]) / 3
		}
		for i := n + m1 + m2; i < close.Len; i++ {
			d.Values[i] = (2*d.Values[i-1] + k.Values[i]) / 3
		}
	}
	j := k.MulScalar(3).Sub(d.MulScalar(2))
	return &KDResult{K: k, D: d, J: j}
}

// KCrossAboveD K 上穿 D
func (kd *KDResult) KCrossAboveD() *BoolVector {
	return kd.K.CrossAboveVec(kd.D)
}

// KCrossBelowD K 下穿 D
func (kd *KDResult) KCrossBelowD() *BoolVector {
	return kd.K.CrossBelowVec(kd.D)
}

// ============================================================================
// OBV - 能量潮
// ============================================================================

// NewOBV 创建 OBV 指标
func NewOBV(close *Vector, volume *Vector) *Vector {
	n := min(close.Len, volume.Len)
	out := NewZeros(n)
	if n == 0 {
		return out
	}
	out.Values[0] = volume.Values[0]
	for i := 1; i < n; i++ {
		if close.Values[i] > close.Values[i-1] {
			out.Values[i] = out.Values[i-1] + volume.Values[i]
		} else if close.Values[i] < close.Values[i-1] {
			out.Values[i] = out.Values[i-1] - volume.Values[i]
		} else {
			out.Values[i] = out.Values[i-1]
		}
	}
	return out
}

// ============================================================================
// Volume Ratio - 量比
// ============================================================================

// NewVolumeRatio 创建量比（当日成交量 / 5日均量）
func NewVolumeRatio(volume *Vector, window int) *Vector {
	avgVol := volume.RollingMean(window)
	ratio := NewZeros(volume.Len)
	for i := 0; i < volume.Len; i++ {
		if avgVol.Values[i] > 0 {
			ratio.Values[i] = volume.Values[i] / avgVol.Values[i]
		}
	}
	return ratio
}

// VolumeExplosion 量比大于阈值
func (vr *Vector) VolumeExplosion(threshold float64) *BoolVector {
	return vr.Greater(threshold)
}

// ============================================================================
// VWAP - 成交量加权平均价
// ============================================================================

// NewVWAP 创建 VWAP
func NewVWAP(high, low, close *Vector, volume *Vector) *Vector {
	n := min(min(high.Len, low.Len), min(close.Len, volume.Len))
	typical := NewZeros(n)
	for i := 0; i < n; i++ {
		typical.Values[i] = (high.Values[i] + low.Values[i] + close.Values[i]) / 3.0
	}
	pv := typical.Mul(volume)
	cumPV := pv.CumSum()
	cumVol := volume.CumSum()
	return cumPV.Div(cumVol.NanToNum(1))
}

// ============================================================================
// 多周期信号组合
// ============================================================================

// CrossSignal 交叉信号
type CrossSignal struct {
	Entries *BoolVector // 金叉买入
	Exits   *BoolVector // 死叉卖出
}

// GoldenCross 黄金交叉（短期均线上穿长期均线）
func GoldenCross(close *Vector, fastWindow, slowWindow int) *CrossSignal {
	fastMA := NewSMA(close, fastWindow)
	slowMA := NewSMA(close, slowWindow)
	return &CrossSignal{
		Entries: fastMA.CrossAboveVec(slowMA),
		Exits:   fastMA.CrossBelowVec(slowMA),
	}
}

// DeathCross 死亡交叉（短期均线下穿长期均线）
func DeathCross(close *Vector, fastWindow, slowWindow int) *CrossSignal {
	fastMA := NewSMA(close, fastWindow)
	slowMA := NewSMA(close, slowWindow)
	return &CrossSignal{
		Entries: fastMA.CrossBelowVec(slowMA),
		Exits:   fastMA.CrossAboveVec(slowMA),
	}
}

// MACDSignal MACD 信号
func MACDSignal(close *Vector, fast, slow, signal int) *CrossSignal {
	macd := NewMACD(close, fast, slow, signal)
	return &CrossSignal{
		Entries: macd.MACDCrossAbove(),
		Exits:   macd.MACDCrossBelow(),
	}
}

// RSISignal RSI 信号
func RSISignal(close *Vector, window int, overbought, oversold float64) *CrossSignal {
	rsi := NewRSI(close, window)
	return &CrossSignal{
		Entries: rsi.RSIOversold(oversold),
		Exits:   rsi.RSIOverbought(overbought),
	}
}

// ============================================================================
// Signal Filters - 信号过滤器
// ============================================================================

// TrendFilter 趋势过滤（只在趋势方向操作）
func TrendFilter(entries, exits *BoolVector, trend *Vector, requireUptrend bool) (*BoolVector, *BoolVector) {
	if trend.Len == 0 {
		return entries, exits
	}
	trendAbove := trend.Greater(0)
	if !requireUptrend {
		trendAbove = trend.Less(0)
	}
	return entries.And(trendAbove), exits
}

// VolatilityFilter 波动率过滤
func VolatilityFilter(entries, exits *BoolVector, atr *Vector, minAtr, maxAtr float64) (*BoolVector, *BoolVector) {
	if atr.Len == 0 {
		return entries, exits
	}
	volOK := atr.Greater(minAtr).And(atr.Less(maxAtr))
	return entries.And(volOK), exits.And(volOK)
}

// ============================================================================
// Parameter Grid Builder - 参数网格构建器
// ============================================================================

// ParamGrid 参数网格
type ParamGrid struct {
	FastWindow []int
	SlowWindow []int
	SignalWin  []int
}

// NewParamGrid 创建参数网格
func NewParamGrid(fastWindows, slowWindows, signalWindows []int) *ParamGrid {
	total := len(fastWindows) * len(slowWindows) * len(signalWindows)
	return &ParamGrid{
		FastWindow: make([]int, 0, total),
		SlowWindow: make([]int, 0, total),
		SignalWin:  make([]int, 0, total),
	}
}

// Add 添加参数组合
func (pg *ParamGrid) Add(fast, slow, signal int) {
	pg.FastWindow = append(pg.FastWindow, fast)
	pg.SlowWindow = append(pg.SlowWindow, slow)
	pg.SignalWin = append(pg.SignalWin, signal)
}

// Count 参数组合数量
func (pg *ParamGrid) Count() int {
	return len(pg.FastWindow)
}

// BuildMACDParams 构建 MACD 参数网格
func BuildMACDParams(fastRange, slowRange, signalRange []int) *ParamGrid {
	pg := NewParamGrid(fastRange, slowRange, signalRange)
	for _, f := range fastRange {
		for _, s := range slowRange {
			if f >= s {
				continue // fast 必须小于 slow
			}
			for _, sig := range signalRange {
				pg.Add(f, s, sig)
			}
		}
	}
	return pg
}

// BuildSMAParams 构建 SMA 参数网格
func BuildSMAParams(fastRange, slowRange []int) *ParamGrid {
	pg := NewParamGrid(fastRange, slowRange, []int{0})
	for _, f := range fastRange {
		for _, s := range slowRange {
			if f >= s {
				continue
			}
			pg.Add(f, s, 0)
		}
	}
	return pg
}
