package quant

import (
	"math"
	"testing"
)

func TestNewVector(t *testing.T) {
	v := NewVector([]float64{1, 2, 3})
	if v.Len != 3 {
		t.Fatalf("expected Len=3, got %d", v.Len)
	}
	if v.Values[1] != 2 {
		t.Fatalf("expected Values[1]=2, got %f", v.Values[1])
	}
}

func TestNewVec(t *testing.T) {
	v := NewVec(4.0, 5.0, 6.0)
	if v.Len != 3 || v.Values[2] != 6 {
		t.Fatal("NewVec failed")
	}
}

func TestNewZeros(t *testing.T) {
	v := NewZeros(5)
	if v.Len != 5 {
		t.Fatal("NewZeros length mismatch")
	}
	for _, val := range v.Values {
		if val != 0 {
			t.Fatal("NewZeros should be all zero")
		}
	}
}

func TestNewOnes(t *testing.T) {
	v := NewOnes(4)
	if v.Len != 4 {
		t.Fatal("NewOnes length mismatch")
	}
	for _, val := range v.Values {
		if val != 1 {
			t.Fatal("NewOnes should be all one")
		}
	}
}

func TestClone(t *testing.T) {
	a := NewVec(1, 2, 3)
	b := a.Clone()
	b.Values[0] = 99
	if a.Values[0] != 1 {
		t.Fatal("Clone should be deep copy")
	}
}

func TestVectorAdd(t *testing.T) {
	a := NewVec(1, 2, 3)
	b := NewVec(4, 5, 6)
	c := a.Add(b)
	if c.Values[0] != 5 || c.Values[1] != 7 || c.Values[2] != 9 {
		t.Fatalf("Add got %v", c.Values)
	}
}

func TestVectorAddScalar(t *testing.T) {
	a := NewVec(1, 2, 3)
	b := a.AddScalar(10)
	if b.Values[0] != 11 || b.Values[2] != 13 {
		t.Fatal("AddScalar failed")
	}
}

func TestVectorSub(t *testing.T) {
	a := NewVec(10, 20, 30)
	b := NewVec(1, 2, 3)
	c := a.Sub(b)
	if c.Values[0] != 9 || c.Values[2] != 27 {
		t.Fatalf("Sub got %v", c.Values)
	}
}

func TestVectorSubScalar(t *testing.T) {
	a := NewVec(10, 20, 30)
	b := a.SubScalar(5)
	if b.Values[0] != 5 || b.Values[2] != 25 {
		t.Fatal("SubScalar failed")
	}
}

func TestVectorMul(t *testing.T) {
	a := NewVec(2, 3, 4)
	b := NewVec(5, 6, 7)
	c := a.Mul(b)
	if c.Values[0] != 10 || c.Values[1] != 18 || c.Values[2] != 28 {
		t.Fatalf("Mul got %v", c.Values)
	}
}

func TestVectorMulScalar(t *testing.T) {
	a := NewVec(1, 2, 3)
	b := a.MulScalar(3)
	if b.Values[0] != 3 || b.Values[2] != 9 {
		t.Fatal("MulScalar failed")
	}
}

func TestVectorDiv(t *testing.T) {
	a := NewVec(10, 20, 30)
	b := NewVec(2, 4, 5)
	c := a.Div(b)
	if c.Values[0] != 5 || c.Values[1] != 5 || c.Values[2] != 6 {
		t.Fatalf("Div got %v", c.Values)
	}
}

func TestVectorDivByZero(t *testing.T) {
	a := NewVec(10, 20)
	b := NewVec(0, 5)
	c := a.Div(b)
	if c.Values[0] != 0 {
		t.Fatal("Div by zero should return 0")
	}
	if c.Values[1] != 4 {
		t.Fatalf("Div got %f at index 1", c.Values[1])
	}
}

func TestVectorDivScalar(t *testing.T) {
	a := NewVec(10, 20, 30)
	b := a.DivScalar(10)
	if b.Values[0] != 1 || b.Values[1] != 2 || b.Values[2] != 3 {
		t.Fatal("DivScalar failed")
	}
}

func TestVectorNeg(t *testing.T) {
	a := NewVec(1, -2, 3)
	b := a.Neg()
	if b.Values[0] != -1 || b.Values[1] != 2 || b.Values[2] != -3 {
		t.Fatal("Neg failed")
	}
}

func TestVectorAbs(t *testing.T) {
	a := NewVec(-1, 2, -3)
	b := a.Abs()
	if b.Values[0] != 1 || b.Values[1] != 2 || b.Values[2] != 3 {
		t.Fatal("Abs failed")
	}
}

func TestVectorSqrt(t *testing.T) {
	a := NewVec(4, 9, 16)
	b := a.Sqrt()
	if b.Values[0] != 2 || b.Values[1] != 3 || b.Values[2] != 4 {
		t.Fatalf("Sqrt got %v", b.Values)
	}
}

func TestVectorSqrtNegative(t *testing.T) {
	a := NewVec(-1, 4)
	b := a.Sqrt()
	if b.Values[0] != 0 {
		t.Fatal("Sqrt of negative should be 0")
	}
	if b.Values[1] != 2 {
		t.Fatal("Sqrt of 4 should be 2")
	}
}

func TestVectorPow(t *testing.T) {
	a := NewVec(2, 3, 4)
	b := a.Pow(2)
	if b.Values[0] != 4 || b.Values[1] != 9 || b.Values[2] != 16 {
		t.Fatalf("Pow got %v", b.Values)
	}
}

func TestVectorLog(t *testing.T) {
	a := NewVec(1, math.E, math.E*math.E)
	b := a.Log()
	if math.Abs(b.Values[0]) > 1e-9 {
		t.Fatalf("Log(1) should be 0, got %f", b.Values[0])
	}
	if math.Abs(b.Values[1]-1) > 1e-9 {
		t.Fatalf("Log(e) should be 1, got %f", b.Values[1])
	}
	if math.Abs(b.Values[2]-2) > 1e-9 {
		t.Fatalf("Log(e^2) should be 2, got %f", b.Values[2])
	}
}

func TestVectorLogNonPositive(t *testing.T) {
	a := NewVec(-1, 0, 1)
	b := a.Log()
	if b.Values[0] != 0 || b.Values[1] != 0 {
		t.Fatal("Log of non-positive should be 0")
	}
}

func TestVectorExp(t *testing.T) {
	a := NewVec(0, 1, 2)
	b := a.Exp()
	if math.Abs(b.Values[0]-1) > 1e-9 {
		t.Fatalf("Exp(0) should be 1, got %f", b.Values[0])
	}
	if math.Abs(b.Values[1]-math.E) > 1e-9 {
		t.Fatalf("Exp(1) should be e, got %f", b.Values[1])
	}
}

func TestVectorMax(t *testing.T) {
	a := NewVec(3, 1, 4, 1, 5)
	if a.Max() != 5 {
		t.Fatalf("Max should be 5, got %f", a.Max())
	}
}

func TestVectorMaxEmpty(t *testing.T) {
	v := NewVector(nil)
	if !math.IsNaN(v.Max()) {
		t.Fatal("Max of empty should be NaN")
	}
}

func TestVectorMin(t *testing.T) {
	a := NewVec(3, 1, 4, 1, 5)
	if a.Min() != 1 {
		t.Fatalf("Min should be 1, got %f", a.Min())
	}
}

func TestVectorMinEmpty(t *testing.T) {
	v := NewVector(nil)
	if !math.IsNaN(v.Min()) {
		t.Fatal("Min of empty should be NaN")
	}
}

func TestVectorSum(t *testing.T) {
	a := NewVec(1, 2, 3, 4)
	if a.Sum() != 10 {
		t.Fatalf("Sum should be 10, got %f", a.Sum())
	}
}

func TestVectorMean(t *testing.T) {
	a := NewVec(1, 2, 3, 4)
	if a.Mean() != 2.5 {
		t.Fatalf("Mean should be 2.5, got %f", a.Mean())
	}
}

func TestVectorMeanEmpty(t *testing.T) {
	v := NewVector(nil)
	if v.Mean() != 0 {
		t.Fatal("Mean of empty should be 0")
	}
}

func TestVectorStd(t *testing.T) {
	a := NewVec(1, 1, 1, 1)
	if a.Std() != 0 {
		t.Fatalf("Std of constant should be 0, got %f", a.Std())
	}
	a = NewVec(2, 4, 4, 4, 5, 5, 7, 9)
	std := a.Std()
	expected := 2.0
	if math.Abs(std-expected) > 0.01 {
		t.Fatalf("Std should be ~2.0, got %f", std)
	}
}

func TestVectorVar(t *testing.T) {
	a := NewVec(2, 4)
	v := a.Var()
	if v <= 0 {
		t.Fatal("Var should be positive")
	}
}

func TestRollingSum(t *testing.T) {
	a := NewVec(1, 2, 3, 4, 5)
	r := a.RollingSum(3)
	if r.Values[2] != 6 || r.Values[3] != 9 || r.Values[4] != 12 {
		t.Fatalf("RollingSum got %v", r.Values)
	}
}

func TestRollingMean(t *testing.T) {
	a := NewVec(1, 2, 3, 4, 5)
	r := a.RollingMean(3)
	if r.Values[2] != 2 || r.Values[3] != 3 || r.Values[4] != 4 {
		t.Fatalf("RollingMean got %v", r.Values)
	}
}

func TestRollingMeanInvalidWindow(t *testing.T) {
	a := NewVec(1, 2, 3)
	r := a.RollingMean(0)
	if r.Len != 3 {
		t.Fatal("RollingMean with window 0 should return zeros of same length")
	}
}

func TestRollingStd(t *testing.T) {
	a := NewVec(1, 2, 3, 4, 5)
	r := a.RollingStd(3)
	if r.Values[2] != 0.816496580927726 {
		t.Fatalf("RollingStd[2] should be ~0.8165, got %f", r.Values[2])
	}
}

func TestRollingMax(t *testing.T) {
	a := NewVec(1, 3, 2, 5, 4)
	r := a.RollingMax(3)
	if r.Values[2] != 3 || r.Values[3] != 5 || r.Values[4] != 5 {
		t.Fatalf("RollingMax got %v", r.Values)
	}
}

func TestRollingMin(t *testing.T) {
	a := NewVec(3, 1, 4, 2, 5)
	r := a.RollingMin(3)
	if r.Values[2] != 1 || r.Values[3] != 1 || r.Values[4] != 2 {
		t.Fatalf("RollingMin got %v", r.Values)
	}
}

func TestShift(t *testing.T) {
	a := NewVec(1, 2, 3, 4, 5)
	r := a.Shift(1)
	if r.Values[0] != 0 || r.Values[1] != 1 || r.Values[4] != 4 {
		t.Fatalf("Shift(1) got %v", r.Values)
	}
	r = a.Shift(-1)
	if r.Values[0] != 2 || r.Values[3] != 5 || r.Values[4] != 0 {
		t.Fatalf("Shift(-1) got %v", r.Values)
	}
	r = a.Shift(0)
	for i := 0; i < 5; i++ {
		if r.Values[i] != a.Values[i] {
			t.Fatal("Shift(0) should be identity")
		}
	}
}

func TestDiff(t *testing.T) {
	a := NewVec(10, 12, 15, 20)
	r := a.Diff(1)
	if r.Values[0] != 0 || r.Values[1] != 2 || r.Values[2] != 3 || r.Values[3] != 5 {
		t.Fatalf("Diff(1) got %v", r.Values)
	}
	r = a.Diff(2)
	if r.Values[0] != 0 || r.Values[1] != 0 || r.Values[2] != 5 || r.Values[3] != 8 {
		t.Fatalf("Diff(2) got %v", r.Values)
	}
}

func TestPctChange(t *testing.T) {
	a := NewVec(100, 105, 110)
	r := a.PctChange(1)
	if r.Values[0] != 0 || math.Abs(r.Values[1]-0.05) > 1e-9 {
		t.Fatalf("PctChange(1) index 1 should be 0.05, got %f", r.Values[1])
	}
}

func TestCumSum(t *testing.T) {
	a := NewVec(1, 2, 3, 4)
	r := a.CumSum()
	if r.Values[0] != 1 || r.Values[1] != 3 || r.Values[2] != 6 || r.Values[3] != 10 {
		t.Fatalf("CumSum got %v", r.Values)
	}
}

func TestCumProd(t *testing.T) {
	a := NewVec(1, 2, 3, 4)
	r := a.CumProd()
	if r.Values[0] != 1 || r.Values[1] != 2 || r.Values[2] != 6 || r.Values[3] != 24 {
		t.Fatalf("CumProd got %v", r.Values)
	}
}

func TestCumMax(t *testing.T) {
	a := NewVec(1, 3, 2, 5, 4)
	r := a.CumMax()
	if r.Values[0] != 1 || r.Values[1] != 3 || r.Values[2] != 3 || r.Values[3] != 5 || r.Values[4] != 5 {
		t.Fatalf("CumMax got %v", r.Values)
	}
}

func TestCumMin(t *testing.T) {
	a := NewVec(3, 1, 4, 2, 0)
	r := a.CumMin()
	if r.Values[0] != 3 || r.Values[1] != 1 || r.Values[2] != 1 || r.Values[4] != 0 {
		t.Fatalf("CumMin got %v", r.Values)
	}
}

func TestGreater(t *testing.T) {
	a := NewVec(1, 5, 3, 7)
	r := a.Greater(4)
	if r.Values[0] || !r.Values[1] || !r.Values[3] {
		t.Fatalf("Greater got %v", r.Values)
	}
}

func TestGreaterVec(t *testing.T) {
	a := NewVec(1, 5, 3)
	b := NewVec(2, 4, 3)
	r := a.GreaterVec(b)
	if r.Values[0] || !r.Values[1] || r.Values[2] {
		t.Fatal("GreaterVec failed")
	}
}

func TestLess(t *testing.T) {
	a := NewVec(1, 5, 3)
	r := a.Less(3)
	if !r.Values[0] || r.Values[1] || r.Values[2] {
		t.Fatalf("Less got %v", r.Values)
	}
}

func TestLessVec(t *testing.T) {
	a := NewVec(1, 5, 3)
	b := NewVec(2, 4, 3)
	r := a.LessVec(b)
	if !r.Values[0] || r.Values[1] || r.Values[2] {
		t.Fatal("LessVec failed")
	}
}

func TestEqual(t *testing.T) {
	a := NewVec(1, 2, 3)
	r := a.Equal(2)
	if r.Values[0] || !r.Values[1] || r.Values[2] {
		t.Fatal("Equal failed")
	}
}

func TestCrossAbove(t *testing.T) {
	a := NewVec(1, 2, 4, 3, 5)
	r := a.CrossAbove(3)
	if r.Values[0] || r.Values[1] || !r.Values[2] || r.Values[3] || !r.Values[4] {
		t.Fatalf("CrossAbove got %v", r.Values)
	}
}

func TestCrossBelow(t *testing.T) {
	a := NewVec(4, 3, 1, 2, 5)
	r := a.CrossBelow(3)
	if r.Values[0] || r.Values[1] || !r.Values[2] || r.Values[4] {
		t.Fatalf("CrossBelow got %v", r.Values)
	}
}

func TestCrossAboveVec(t *testing.T) {
	a := NewVec(1, 2, 5, 3)
	b := NewVec(2, 3, 3, 4)
	r := a.CrossAboveVec(b)
	if r.Values[0] || r.Values[1] || !r.Values[2] || r.Values[3] {
		t.Fatal("CrossAboveVec failed")
	}
}

func TestCrossBelowVec(t *testing.T) {
	a := NewVec(5, 3, 1, 2)
	b := NewVec(2, 3, 3, 1)
	r := a.CrossBelowVec(b)
	if r.Values[0] || r.Values[1] || !r.Values[2] || r.Values[3] {
		t.Fatal("CrossBelowVec failed")
	}
}

func TestNanToNum(t *testing.T) {
	a := NewVec(math.NaN(), 1, math.Inf(1), 2)
	b := a.NanToNum(0)
	if b.Values[0] != 0 || b.Values[1] != 1 || b.Values[2] != 0 || b.Values[3] != 2 {
		t.Fatalf("NanToNum got %v", b.Values)
	}
}

func TestBoolVectorAnd(t *testing.T) {
	a := NewBoolVector([]bool{true, false, true})
	b := NewBoolVector([]bool{true, true, false})
	c := a.And(b)
	if !c.Values[0] || c.Values[1] || c.Values[2] {
		t.Fatal("And failed")
	}
}

func TestBoolVectorOr(t *testing.T) {
	a := NewBoolVector([]bool{true, false, false})
	b := NewBoolVector([]bool{false, true, false})
	c := a.Or(b)
	if !c.Values[0] || !c.Values[1] || c.Values[2] {
		t.Fatal("Or failed")
	}
}

func TestBoolVectorNot(t *testing.T) {
	a := NewBoolVector([]bool{true, false, true})
	b := a.Not()
	if b.Values[0] || !b.Values[1] || b.Values[2] {
		t.Fatal("Not failed")
	}
}

func TestBoolVectorSum(t *testing.T) {
	a := NewBoolVector([]bool{true, false, true, false, true})
	if a.Sum() != 3 {
		t.Fatalf("Sum should be 3, got %d", a.Sum())
	}
}

func TestBoolVectorAny(t *testing.T) {
	a := NewBoolVector([]bool{false, false})
	if a.Any() {
		t.Fatal("Any should be false")
	}
	b := NewBoolVector([]bool{false, true})
	if !b.Any() {
		t.Fatal("Any should be true")
	}
}

func TestBoolVectorAll(t *testing.T) {
	a := NewBoolVector([]bool{true, true})
	if !a.All() {
		t.Fatal("All should be true")
	}
	b := NewBoolVector([]bool{true, false})
	if b.All() {
		t.Fatal("All should be false")
	}
}

func TestNewMatrix(t *testing.T) {
	m := NewMatrix(3, 4)
	if m.Rows != 3 || m.Cols != 4 || len(m.Values) != 12 {
		t.Fatal("NewMatrix failed")
	}
}

func TestNewMatrixFromVecs(t *testing.T) {
	a := NewVec(1, 2, 3)
	b := NewVec(4, 5, 6)
	m := NewMatrixFromVecs(a, b)
	if m.Rows != 3 || m.Cols != 2 {
		t.Fatalf("expected 3x2, got %dx%d", m.Rows, m.Cols)
	}
	// row 0: [1, 4]
	// row 1: [2, 5]
	// row 2: [3, 6]
	if m.Values[0] != 1 || m.Values[1] != 4 {
		t.Fatalf("Row 0 failed, got %f %f", m.Values[0], m.Values[1])
	}
	if m.Values[2] != 2 || m.Values[3] != 5 {
		t.Fatalf("Row 1 failed, got %f %f", m.Values[2], m.Values[3])
	}
}

func TestMatrixRow(t *testing.T) {
	m := NewMatrixFromVecs(NewVec(1, 2), NewVec(3, 4))
	r := m.Row(1)
	if r.Values[0] != 2 || r.Values[1] != 4 {
		t.Fatalf("Row 1 got %v", r.Values)
	}
}

func TestMatrixCol(t *testing.T) {
	m := NewMatrixFromVecs(NewVec(1, 2), NewVec(3, 4))
	c := m.Col(1)
	if c.Values[0] != 3 || c.Values[1] != 4 {
		t.Fatalf("Col 1 got %v", c.Values)
	}
}

func TestQuantile(t *testing.T) {
	a := NewVec(1, 2, 3, 4, 5)
	q := a.Quantile(0.5)
	if q != 3 {
		t.Fatalf("Median should be 3, got %f", q)
	}
	q = a.Quantile(0)
	if q != 1 {
		t.Fatalf("Q0 should be 1, got %f", q)
	}
	q = a.Quantile(1)
	if q != 5 {
		t.Fatalf("Q1 should be 5, got %f", q)
	}
}

func TestQuantileEmpty(t *testing.T) {
	v := NewVector(nil)
	if !math.IsNaN(v.Quantile(0.5)) {
		t.Fatal("Quantile of empty should be NaN")
	}
}

func TestPercentile(t *testing.T) {
	a := NewVec(10, 20, 30, 40, 50)
	p := a.Percentile(50)
	if p != 30 {
		t.Fatalf("50th percentile should be 30, got %f", p)
	}
}

func TestMedian(t *testing.T) {
	a := NewVec(1, 2, 3, 4, 5)
	m := a.Median()
	if m != 3 {
		t.Fatalf("Median should be 3, got %f", m)
	}
	a = NewVec(1, 2, 3, 4)
	m = a.Median()
	if m != 2.5 {
		t.Fatalf("Median of even should be 2.5, got %f", m)
	}
}
