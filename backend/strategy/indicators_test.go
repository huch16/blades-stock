package strategy

import (
	"fmt"
	"math"
	"testing"
)

// makeBars 构造 K 线序列，closes 为收盘价，volumes 可选（缺省 100）。
func makeBars(closes []float64, volumes []float64) []Bar {
	bars := make([]Bar, len(closes))
	for i, c := range closes {
		bars[i] = Bar{
			Date:   fmt.Sprintf("%04d", i+1),
			Open:   c,
			High:   c + 0.1,
			Low:    c - 0.1,
			Close:  c,
			Volume: 100,
		}
		if i < len(volumes) {
			bars[i].Volume = volumes[i]
		}
	}
	return bars
}

func TestComputeIndicatorsRising(t *testing.T) {
	closes := make([]float64, 70)
	for i := range closes {
		closes[i] = 10 + float64(i)*0.1
	}
	ind := ComputeIndicators(makeBars(closes, nil))

	if ind.LastClose != 16.9 {
		t.Fatalf("LastClose = %v, want 16.9", ind.LastClose)
	}
	if ind.LastClose != 16.9 {
		t.Fatalf("LastClose = %v, want 16.9", ind.LastClose)
	}
	if !ind.MAValid[5] || !ind.MAValid[60] {
		t.Fatal("MA5/MA60 应有效")
	}
	// 单调上升: MA5 > MA10 > MA20 > MA60
	if !(ind.MA[5] > ind.MA[10] && ind.MA[10] > ind.MA[20] && ind.MA[20] > ind.MA[60]) {
		t.Fatalf("多头排列未成立: MA=%+v", ind.MA)
	}
	if math.Abs(ind.MA[5]-16.7) > 1e-9 {
		t.Fatalf("MA5 = %v, want 16.7", ind.MA[5])
	}
	if ind.ChangePct <= 0 {
		t.Fatalf("ChangePct 应为正: %v", ind.ChangePct)
	}
	if !ind.MACDAboveZero || ind.MACDDIF <= 0 {
		t.Fatalf("上升趋势 DIF 应大于0: DIF=%v DEA=%v", ind.MACDDIF, ind.MACDDEA)
	}
	if ind.RSI14 <= 50 {
		t.Fatalf("上升趋势 RSI 应大于50: %v", ind.RSI14)
	}
	if ind.VolumeRatio5 == 0 {
		t.Fatal("VolumeRatio5 不应为 0")
	}
}

func TestComputeIndicatorsDeclining(t *testing.T) {
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 50 - float64(i)
	}
	ind := ComputeIndicators(makeBars(closes, nil))

	if !(ind.MA[5] < ind.MA[10] && ind.MA[10] < ind.MA[20]) {
		t.Fatalf("空头排列未成立: MA=%+v", ind.MA)
	}
	if ind.ChangePct >= 0 {
		t.Fatalf("ChangePct 应为负: %v", ind.ChangePct)
	}
	if ind.MACDAboveZero || ind.MACDDIF >= 0 {
		t.Fatalf("下跌趋势 DIF 应小于0: DIF=%v", ind.MACDDIF)
	}
	if ind.RSI14 >= 30 {
		t.Fatalf("持续下跌 RSI 应小于30: %v", ind.RSI14)
	}
}

func TestVolumeRatio(t *testing.T) {
	closes := make([]float64, 10)
	for i := range closes {
		closes[i] = 10
	}
	vols := make([]float64, 10)
	for i := range vols {
		vols[i] = 100
	}
	vols[9] = 250
	ind := ComputeIndicators(makeBars(closes, vols))
	if math.Abs(ind.VolumeRatio5-2.5) > 1e-9 {
		t.Fatalf("VolumeRatio5 = %v, want 2.5", ind.VolumeRatio5)
	}
}

func TestMACDGoldenCross(t *testing.T) {
	// 前 16 根 20，接着 9 根 10，最后一根 40 → MA5 上穿 MA10
	closes := []float64{}
	for i := 0; i < 16; i++ {
		closes = append(closes, 20)
	}
	for i := 0; i < 9; i++ {
		closes = append(closes, 10)
	}
	closes = append(closes, 40)

	ind := ComputeIndicators(makeBars(closes, nil))
	if !ind.MACDGoldenCross {
		t.Fatalf("预期 MACD 金叉: DIF=%v DEA=%v", ind.MACDDIF, ind.MACDDEA)
	}
	if ind.MACDDeathCross {
		t.Fatal("不应同时为死叉")
	}
}

func TestMACDDeathCross(t *testing.T) {
	// 对称的下跌突刺 → MA5 下穿 MA10
	closes := []float64{}
	for i := 0; i < 16; i++ {
		closes = append(closes, 10)
	}
	for i := 0; i < 9; i++ {
		closes = append(closes, 20)
	}
	closes = append(closes, 5)

	ind := ComputeIndicators(makeBars(closes, nil))
	if !ind.MACDDeathCross {
		t.Fatalf("预期 MACD 死叉: DIF=%v DEA=%v", ind.MACDDIF, ind.MACDDEA)
	}
}

func TestBreakoutHighLow(t *testing.T) {
	closes := make([]float64, 26)
	for i := 0; i < 25; i++ {
		closes[i] = 10
	}
	closes[25] = 11
	ind := ComputeIndicators(makeBars(closes, nil))
	if h := ind.PrevHigh[20]; math.Abs(h-10.1) > 1e-9 {
		t.Fatalf("PrevHigh20 = %v, want 10.1", h)
	}
	if !(ind.LastClose > ind.PrevHigh[20]) {
		t.Fatal("应突破20日新高")
	}

	closes2 := make([]float64, 26)
	for i := 0; i < 25; i++ {
		closes2[i] = 10
	}
	closes2[25] = 8
	ind2 := ComputeIndicators(makeBars(closes2, nil))
	if !(ind2.LastClose < ind2.PrevLow[20]) {
		t.Fatal("应跌破20日新低")
	}
}

func TestPullbackBelowMA(t *testing.T) {
	// 均线 10，当前价 9.7 → 回踩偏离 3%
	closes := make([]float64, 30)
	for i := 0; i < 30; i++ {
		closes[i] = 10
	}
	closes[29] = 9.7
	ind := ComputeIndicators(makeBars(closes, nil))
	if !evalCondition("pullback_below_ma(20,3)", ind) {
		t.Fatal("回踩3%应命中 pullback_below_ma(20,3)")
	}
	if evalCondition("pullback_below_ma(20,1)", ind) {
		t.Fatal("偏离3%不应命中 pullback_below_ma(20,1)")
	}
}
