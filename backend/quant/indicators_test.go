package quant

import (
	"math"
	"testing"
)

func approxEq(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

func vecApproxEq(t *testing.T, got *Vector, expected []float64, eps float64, name string) {
	t.Helper()
	if got.Len != len(expected) {
		t.Fatalf("%s: length mismatch: got %d, want %d", name, got.Len, len(expected))
	}
	for i := 0; i < got.Len; i++ {
		if !approxEq(got.Values[i], expected[i], eps) {
			t.Fatalf("%s[%d]: got %f, want %f", name, i, got.Values[i], expected[i])
		}
	}
}

func TestNewSMA(t *testing.T) {
	close := NewVec(1, 2, 3, 4, 5)
	sma := NewSMA(close, 3)
	if sma.Values[2] != 2 || sma.Values[3] != 3 || sma.Values[4] != 4 {
		t.Fatalf("SMA(3) got %v", sma.Values)
	}
}

func TestNewSMA_ShortData(t *testing.T) {
	close := NewVec(1, 2)
	sma := NewSMA(close, 3)
	if sma.Len != 2 {
		t.Fatal("SMA with short data should return same length")
	}
}

func TestSMAOf(t *testing.T) {
	close := NewVec(1, 2, 3, 4, 5, 6)
	results := SMAOf(close, []int{2, 3})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestNewEMA(t *testing.T) {
	close := NewVec(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	ema := NewEMA(close, 3)
	if ema.Values[2] != 2 {
		t.Fatalf("EMA first value should be SMA=2, got %f", ema.Values[2])
	}
	// EMA[3] = alpha*4 + (1-alpha)*2; alpha=2/4=0.5
	expected := 0.5*4 + 0.5*2
	if !approxEq(ema.Values[3], expected, 1e-9) {
		t.Fatalf("EMA[3] should be %f, got %f", expected, ema.Values[3])
	}
}

func TestNewEMA_Empty(t *testing.T) {
	close := NewVector(nil)
	ema := NewEMA(close, 10)
	if ema.Len != 0 {
		t.Fatal("EMA of empty should be empty")
	}
}

func TestNewMACD(t *testing.T) {
	close := NewVec(10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30)
	macd := NewMACD(close, 12, 26, 9)
	if macd.MACD == nil || macd.Signal == nil || macd.Histogram == nil {
		t.Fatal("MACD result should not be nil")
	}
	if macd.MACD.Len != close.Len {
		t.Fatalf("MACD line length mismatch: %d vs %d", macd.MACD.Len, close.Len)
	}
}

func TestMACDCrossAbove(t *testing.T) {
	close := NewVec(10, 10.5, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30)
	macd := NewMACD(close, 5, 10, 3)
	cross := macd.MACDCrossAbove()
	if cross.Len != close.Len {
		t.Fatalf("cross length mismatch: %d vs %d", cross.Len, close.Len)
	}
}

func TestMACDCrossBelow(t *testing.T) {
	close := NewVec(30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10)
	macd := NewMACD(close, 5, 10, 3)
	cross := macd.MACDCrossBelow()
	if cross.Len != close.Len {
		t.Fatalf("cross length mismatch: %d vs %d", cross.Len, close.Len)
	}
}

func TestMACDCrossAboveZero(t *testing.T) {
	close := NewVec(5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20)
	macd := NewMACD(close, 3, 8, 3)
	cross := macd.MACDCrossAboveZero()
	if cross.Len != close.Len {
		t.Fatal("cross length mismatch")
	}
}

func TestMACDCrossBelowZero(t *testing.T) {
	close := NewVec(20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6)
	macd := NewMACD(close, 3, 8, 3)
	cross := macd.MACDCrossBelowZero()
	if cross.Len != close.Len {
		t.Fatal("cross length mismatch")
	}
}

func TestNewRSI(t *testing.T) {
	close := NewVec(44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08,
		45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 45.64)
	rsi := NewRSI(close, 14)
	if rsi.Len != close.Len {
		t.Fatalf("RSI length mismatch: %d vs %d", rsi.Len, close.Len)
	}
	if rsi.Values[19] < 0 || rsi.Values[19] > 100 {
		t.Fatalf("RSI last value %f out of [0,100]", rsi.Values[19])
	}
}

func TestNewRSI_InsufficientData(t *testing.T) {
	close := NewVec(1, 2, 3)
	rsi := NewRSI(close, 14)
	if rsi.Len != 3 {
		t.Fatal("RSI with insufficient data should return same length")
	}
}

func TestRSIOverbought(t *testing.T) {
	rsi := NewVec(25, 50, 75, 80, 30)
	sig := rsi.RSIOverbought(70)
	if sig.Values[0] || sig.Values[1] || !sig.Values[2] || !sig.Values[3] || sig.Values[4] {
		t.Fatal("RSIOverbought failed")
	}
}

func TestRSIOversold(t *testing.T) {
	rsi := NewVec(25, 50, 75, 30, 20)
	sig := rsi.RSIOversold(30)
	if !sig.Values[0] || sig.Values[1] || sig.Values[2] || sig.Values[3] || !sig.Values[4] {
		t.Fatal("RSIOversold failed")
	}
}

func TestNewBollinger(t *testing.T) {
	close := NewVec(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	boll := NewBollinger(close, 5, 2)
	if boll.Upper.Len != close.Len || boll.Middle.Len != close.Len || boll.Lower.Len != close.Len {
		t.Fatal("Bollinger band length mismatch")
	}
	// Middle band should be SMA
	if !approxEq(boll.Middle.Values[9], 8, 0.1) {
		t.Fatalf("Middle band last: %f", boll.Middle.Values[9])
	}
	// Upper > Middle > Lower
	if boll.Upper.Values[9] <= boll.Middle.Values[9] || boll.Middle.Values[9] <= boll.Lower.Values[9] {
		t.Fatal("Bollinger band order wrong")
	}
}

func TestPriceAboveUpper(t *testing.T) {
	boll := &BOLLResult{
		Upper:  NewVec(5, 6, 7, 8, 9),
		Middle: NewVec(3, 4, 5, 6, 7),
		Lower:  NewVec(1, 2, 3, 4, 5),
	}
	close := NewVec(4, 7, 6, 9, 8)
	sig := boll.PriceAboveUpper(close)
	if sig.Values[0] || !sig.Values[1] || sig.Values[2] || !sig.Values[3] || sig.Values[4] {
		t.Fatal("PriceAboveUpper failed")
	}
}

func TestPriceBelowLower(t *testing.T) {
	boll := &BOLLResult{
		Upper:  NewVec(5, 6, 7, 8, 9),
		Middle: NewVec(3, 4, 5, 6, 7),
		Lower:  NewVec(1, 2, 3, 4, 5),
	}
	close := NewVec(2, 5, 0, 4, 6)
	sig := boll.PriceBelowLower(close)
	if sig.Values[0] || sig.Values[1] || !sig.Values[2] || sig.Values[3] || sig.Values[4] {
		t.Fatal("PriceBelowLower failed")
	}
}

func TestNewATR(t *testing.T) {
	high := NewVec(12, 13, 14, 13, 15)
	low := NewVec(10, 11, 10, 11, 12)
	close := NewVec(11, 12, 13, 12, 14)
	atr := NewATR(high, low, close, 3)
	if atr.Len != close.Len {
		t.Fatalf("ATR length mismatch: %d vs %d", atr.Len, close.Len)
	}
}

func TestNewKD(t *testing.T) {
	high := NewVec(10, 12, 14, 13, 15, 16, 14, 17, 18, 19, 20, 21)
	low := NewVec(8, 9, 10, 10, 11, 12, 11, 13, 14, 15, 16, 17)
	close := NewVec(9, 11, 13, 12, 14, 15, 13, 16, 17, 18, 19, 20)
	kd := NewKD(high, low, close, 5, 3, 3)
	if kd.K.Len != close.Len || kd.D.Len != close.Len || kd.J.Len != close.Len {
		t.Fatal("KD length mismatch")
	}
}

func TestKCrossAboveD(t *testing.T) {
	high := NewVec(10, 12, 14, 13, 15, 16, 14, 17, 18, 19, 20, 21, 22)
	low := NewVec(8, 9, 10, 10, 11, 12, 11, 13, 14, 15, 16, 17, 18)
	close := NewVec(9, 11, 13, 12, 14, 15, 13, 16, 17, 18, 19, 20, 21)
	kd := NewKD(high, low, close, 5, 3, 3)
	cross := kd.KCrossAboveD()
	if cross.Len != close.Len {
		t.Fatal("KCrossAboveD length mismatch")
	}
}

func TestKCrossBelowD(t *testing.T) {
	high := NewVec(20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8)
	low := NewVec(15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3)
	close := NewVec(18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6)
	kd := NewKD(high, low, close, 5, 3, 3)
	cross := kd.KCrossBelowD()
	if cross.Len != close.Len {
		t.Fatal("KCrossBelowD length mismatch")
	}
}

func TestNewOBV(t *testing.T) {
	close := NewVec(10, 11, 10, 12, 13)
	vol := NewVec(1000, 1500, 800, 2000, 1200)
	obv := NewOBV(close, vol)
	if obv.Len != close.Len {
		t.Fatalf("OBV length mismatch: %d vs %d", obv.Len, close.Len)
	}
	// Day 0: +1000
	if obv.Values[0] != 1000 {
		t.Fatalf("OBV[0] should be 1000, got %f", obv.Values[0])
	}
	// Day 1: close up, +1500 = 2500
	if obv.Values[1] != 2500 {
		t.Fatalf("OBV[1] should be 2500, got %f", obv.Values[1])
	}
	// Day 2: close down, -800 = 1700
	if obv.Values[2] != 1700 {
		t.Fatalf("OBV[2] should be 1700, got %f", obv.Values[2])
	}
}

func TestNewVolumeRatio(t *testing.T) {
	vol := NewVec(100, 200, 150, 300, 250)
	vr := NewVolumeRatio(vol, 3)
	if vr.Len != vol.Len {
		t.Fatal("VolumeRatio length mismatch")
	}
}

func TestVolumeExplosion(t *testing.T) {
	vr := NewVec(0.5, 1.0, 2.1, 3.0, 0.8)
	sig := vr.VolumeExplosion(2.0)
	if sig.Values[0] || sig.Values[1] || !sig.Values[2] || !sig.Values[3] || sig.Values[4] {
		t.Fatal("VolumeExplosion failed")
	}
}

func TestNewVWAP(t *testing.T) {
	high := NewVec(12, 13, 14, 13, 15)
	low := NewVec(10, 11, 10, 11, 12)
	close := NewVec(11, 12, 13, 12, 14)
	vol := NewVec(1000, 1500, 800, 2000, 1200)
	vwap := NewVWAP(high, low, close, vol)
	if vwap.Len != close.Len {
		t.Fatalf("VWAP length mismatch: %d vs %d", vwap.Len, close.Len)
	}
}

func TestGoldenCross(t *testing.T) {
	close := NewVec(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	sig := GoldenCross(close, 3, 5)
	if sig.Entries.Len != close.Len || sig.Exits.Len != close.Len {
		t.Fatal("GoldenCross length mismatch")
	}
}

func TestDeathCross(t *testing.T) {
	close := NewVec(10, 9, 8, 7, 6, 5, 4, 3, 2, 1)
	sig := DeathCross(close, 3, 5)
	if sig.Entries.Len != close.Len || sig.Exits.Len != close.Len {
		t.Fatal("DeathCross length mismatch")
	}
}

func TestMACDSignal(t *testing.T) {
	close := NewVec(10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20)
	sig := MACDSignal(close, 3, 8, 3)
	if sig.Entries.Len != close.Len || sig.Exits.Len != close.Len {
		t.Fatal("MACDSignal length mismatch")
	}
}

func TestRSISignal(t *testing.T) {
	close := NewVec(44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08,
		45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 45.64)
	sig := RSISignal(close, 14, 70, 30)
	if sig.Entries.Len != close.Len || sig.Exits.Len != close.Len {
		t.Fatal("RSISignal length mismatch")
	}
}

func TestTrendFilter(t *testing.T) {
	entries := NewBoolVector([]bool{true, true, true, true})
	trend := NewVec(-1, 1, 0, 1)
	e, _ := TrendFilter(entries, NewBoolVector([]bool{false, false, false, false}), trend, true)
	if e.Values[0] || !e.Values[1] || e.Values[2] || !e.Values[3] {
		t.Fatal("TrendFilter(requireUptrend) failed")
	}
	e, _ = TrendFilter(entries, NewBoolVector([]bool{false, false, false, false}), trend, false)
	if !e.Values[0] || e.Values[1] || e.Values[2] || e.Values[3] {
		t.Fatal("TrendFilter(!requireUptrend) failed")
	}
}

func TestVolatilityFilter(t *testing.T) {
	entries := NewBoolVector([]bool{true, true, true})
	exits := NewBoolVector([]bool{false, false, false})
	atr := NewVec(0.5, 2.0, 1.4)
	e, _ := VolatilityFilter(entries, exits, atr, 0.8, 1.5)
	if e.Values[0] || e.Values[1] || !e.Values[2] {
		t.Fatal("VolatilityFilter failed")
	}
}

func TestNewParamGrid(t *testing.T) {
	pg := NewParamGrid([]int{5, 10}, []int{20, 40}, []int{5, 9})
	if pg.Count() != 0 {
		t.Fatal("NewParamGrid should start empty")
	}
}

func TestBuildMACDParams(t *testing.T) {
	pg := BuildMACDParams([]int{5, 8, 12}, []int{18, 24, 30}, []int{5, 9})
	if pg.Count() == 0 {
		t.Fatal("BuildMACDParams should produce params")
	}
}

func TestBuildSMAParams(t *testing.T) {
	pg := BuildSMAParams([]int{5, 10, 20}, []int{30, 60})
	if pg.Count() == 0 {
		t.Fatal("BuildSMAParams should produce params")
	}
}
