// Package quant - VectorBT-style vectorized backtesting engine for Go
// Core: Vector/Matrix operations inspired by NumPy
package quant

import (
	"math"
	"sort"
)

// ============================================================================
// Vector - 核心向量类型（NumPy ndarray 的 Go 实现）
// ============================================================================

// Vector 核心向量类型，表示一维浮点数数组
type Vector struct {
	Values []float64
	Len    int
}

// NewVector 从切片创建向量
func NewVector(values []float64) *Vector {
	return &Vector{Values: values, Len: len(values)}
}

// NewVec 便捷构造函数
func NewVec(values ...float64) *Vector {
	return &Vector{Values: values, Len: len(values)}
}

// NewZeros 创建零向量
func NewZeros(n int) *Vector {
	return &Vector{Values: make([]float64, n), Len: n}
}

// NewOnes 创建全1向量
func NewOnes(n int) *Vector {
	v := make([]float64, n)
	for i := range v {
		v[i] = 1
	}
	return &Vector{Values: v, Len: n}
}

// Clone 克隆向量
func (v *Vector) Clone() *Vector {
	cp := make([]float64, v.Len)
	copy(cp, v.Values)
	return &Vector{Values: cp, Len: v.Len}
}

// ============================================================================
// 向量运算（Element-wise Operations）
// ============================================================================

// Add 向量加法
func (v *Vector) Add(other *Vector) *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = v.Values[i] + other.Values[i]
	}
	return out
}

// AddScalar 向量 + 标量
func (v *Vector) AddScalar(s float64) *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = v.Values[i] + s
	}
	return out
}

// Sub 向量减法
func (v *Vector) Sub(other *Vector) *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = v.Values[i] - other.Values[i]
	}
	return out
}

// SubScalar 向量 - 标量
func (v *Vector) SubScalar(s float64) *Vector {
	return v.AddScalar(-s)
}

// Mul 向量乘法
func (v *Vector) Mul(other *Vector) *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = v.Values[i] * other.Values[i]
	}
	return out
}

// MulScalar 向量 × 标量
func (v *Vector) MulScalar(s float64) *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = v.Values[i] * s
	}
	return out
}

// Div 向量除法
func (v *Vector) Div(other *Vector) *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		if other.Values[i] != 0 {
			out.Values[i] = v.Values[i] / other.Values[i]
		}
	}
	return out
}

// DivScalar 向量 / 标量
func (v *Vector) DivScalar(s float64) *Vector {
	return v.MulScalar(1.0 / s)
}

// Neg 取负
func (v *Vector) Neg() *Vector {
	return v.MulScalar(-1)
}

// Abs 绝对值
func (v *Vector) Abs() *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = math.Abs(v.Values[i])
	}
	return out
}

// Sqrt 平方根
func (v *Vector) Sqrt() *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		if v.Values[i] > 0 {
			out.Values[i] = math.Sqrt(v.Values[i])
		}
	}
	return out
}

// Pow 幂运算
func (v *Vector) Pow(e float64) *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = math.Pow(v.Values[i], e)
	}
	return out
}

// Log 自然对数
func (v *Vector) Log() *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		if v.Values[i] > 0 {
			out.Values[i] = math.Log(v.Values[i])
		}
	}
	return out
}

// Exp 指数
func (v *Vector) Exp() *Vector {
	out := NewZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = math.Exp(v.Values[i])
	}
	return out
}

// Max 最大值
func (v *Vector) Max() float64 {
	if v.Len == 0 {
		return math.NaN()
	}
	m := v.Values[0]
	for i := 1; i < v.Len; i++ {
		if v.Values[i] > m {
			m = v.Values[i]
		}
	}
	return m
}

// Min 最小值
func (v *Vector) Min() float64 {
	if v.Len == 0 {
		return math.NaN()
	}
	m := v.Values[0]
	for i := 1; i < v.Len; i++ {
		if v.Values[i] < m {
			m = v.Values[i]
		}
	}
	return m
}

// Sum 求和
func (v *Vector) Sum() float64 {
	s := 0.0
	for _, val := range v.Values {
		s += val
	}
	return s
}

// Mean 均值
func (v *Vector) Mean() float64 {
	if v.Len == 0 {
		return 0
	}
	return v.Sum() / float64(v.Len)
}

// Std 标准差（总体）
func (v *Vector) Std() float64 {
	if v.Len == 0 {
		return 0
	}
	m := v.Mean()
	s := 0.0
	for _, val := range v.Values {
		d := val - m
		s += d * d
	}
	return math.Sqrt(s / float64(v.Len))
}

// Var 方差
func (v *Vector) Var() float64 {
	return v.Std() * v.Std()
}

// ============================================================================
// 滚动窗口运算（Rolling Window Operations）
// ============================================================================

// RollingSum 滚动求和
func (v *Vector) RollingSum(window int) *Vector {
	if window <= 0 || window > v.Len {
		return NewZeros(v.Len)
	}
	out := NewZeros(v.Len)
	s := 0.0
	for i := 0; i < window-1 && i < v.Len; i++ {
		s += v.Values[i]
	}
	for i := window - 1; i < v.Len; i++ {
		if i >= window {
			s -= v.Values[i-window]
		}
		s += v.Values[i]
		out.Values[i] = s
	}
	return out
}

// RollingMean 滚动均值
func (v *Vector) RollingMean(window int) *Vector {
	if window <= 0 || window > v.Len {
		return NewZeros(v.Len)
	}
	out := NewZeros(v.Len)
	s := 0.0
	for i := 0; i < window-1 && i < v.Len; i++ {
		s += v.Values[i]
	}
	for i := window - 1; i < v.Len; i++ {
		if i >= window {
			s -= v.Values[i-window]
		}
		s += v.Values[i]
		out.Values[i] = s / float64(window)
	}
	return out
}

// RollingStd 滚动标准差
func (v *Vector) RollingStd(window int) *Vector {
	if window <= 0 || window > v.Len {
		return NewZeros(v.Len)
	}
	out := NewZeros(v.Len)
	for i := window - 1; i < v.Len; i++ {
		windowVals := v.Values[i-window+1 : i+1]
		m := 0.0
		for _, val := range windowVals {
			m += val
		}
		m /= float64(window)
		var s float64
		for _, val := range windowVals {
			d := val - m
			s += d * d
		}
		out.Values[i] = math.Sqrt(s / float64(window))
	}
	return out
}

// RollingMax 滚动最大值
func (v *Vector) RollingMax(window int) *Vector {
	if window <= 0 || window > v.Len {
		return NewZeros(v.Len)
	}
	out := NewZeros(v.Len)
	for i := window - 1; i < v.Len; i++ {
		m := v.Values[i-window+1]
		for j := i - window + 2; j <= i; j++ {
			if v.Values[j] > m {
				m = v.Values[j]
			}
		}
		out.Values[i] = m
	}
	return out
}

// RollingMin 滚动最小值
func (v *Vector) RollingMin(window int) *Vector {
	if window <= 0 || window > v.Len {
		return NewZeros(v.Len)
	}
	out := NewZeros(v.Len)
	for i := window - 1; i < v.Len; i++ {
		m := v.Values[i-window+1]
		for j := i - window + 2; j <= i; j++ {
			if v.Values[j] < m {
				m = v.Values[j]
			}
		}
		out.Values[i] = m
	}
	return out
}

// Shift 位移（往前移，尾部补0）
func (v *Vector) Shift(n int) *Vector {
	out := NewZeros(v.Len)
	if n >= v.Len || n <= -v.Len {
		return out
	}
	if n > 0 {
		copy(out.Values[n:], v.Values[:v.Len-n])
	} else if n < 0 {
		copy(out.Values[:v.Len+n], v.Values[-n:])
	} else {
		copy(out.Values, v.Values)
	}
	return out
}

// Diff 差分
func (v *Vector) Diff(period int) *Vector {
	out := NewZeros(v.Len)
	for i := period; i < v.Len; i++ {
		out.Values[i] = v.Values[i] - v.Values[i-period]
	}
	return out
}

// PctChange 百分比变化
func (v *Vector) PctChange(period int) *Vector {
	out := NewZeros(v.Len)
	for i := period; i < v.Len; i++ {
		if v.Values[i-period] != 0 {
			out.Values[i] = (v.Values[i] - v.Values[i-period]) / v.Values[i-period]
		}
	}
	return out
}

// ============================================================================
// 累积运算（Cumulative Operations）
// ============================================================================

// CumSum 累积求和
func (v *Vector) CumSum() *Vector {
	out := NewZeros(v.Len)
	s := 0.0
	for i := 0; i < v.Len; i++ {
		s += v.Values[i]
		out.Values[i] = s
	}
	return out
}

// CumProd 累积乘积
func (v *Vector) CumProd() *Vector {
	out := NewZeros(v.Len)
	p := 1.0
	for i := 0; i < v.Len; i++ {
		p *= v.Values[i]
		out.Values[i] = p
	}
	return out
}

// CumMax 累积最大值
func (v *Vector) CumMax() *Vector {
	out := NewZeros(v.Len)
	if v.Len == 0 {
		return out
	}
	m := v.Values[0]
	out.Values[0] = m
	for i := 1; i < v.Len; i++ {
		if v.Values[i] > m {
			m = v.Values[i]
		}
		out.Values[i] = m
	}
	return out
}

// CumMin 累积最小值
func (v *Vector) CumMin() *Vector {
	out := NewZeros(v.Len)
	if v.Len == 0 {
		return out
	}
	m := v.Values[0]
	out.Values[0] = m
	for i := 1; i < v.Len; i++ {
		if v.Values[i] < m {
			m = v.Values[i]
		}
		out.Values[i] = m
	}
	return out
}

// ============================================================================
// 比较运算（Boolean Operations）
// ============================================================================

// Greater 大于
func (v *Vector) Greater(s float64) *BoolVector {
	out := NewBoolZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = v.Values[i] > s
	}
	return out
}

// GreaterVec 向量大于向量
func (v *Vector) GreaterVec(other *Vector) *BoolVector {
	n := min(v.Len, other.Len)
	out := NewBoolZeros(n)
	for i := 0; i < n; i++ {
		out.Values[i] = v.Values[i] > other.Values[i]
	}
	return out
}

// Less 小于
func (v *Vector) Less(s float64) *BoolVector {
	out := NewBoolZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = v.Values[i] < s
	}
	return out
}

// LessVec 向量小于向量
func (v *Vector) LessVec(other *Vector) *BoolVector {
	n := min(v.Len, other.Len)
	out := NewBoolZeros(n)
	for i := 0; i < n; i++ {
		out.Values[i] = v.Values[i] < other.Values[i]
	}
	return out
}

// Equal 等于
func (v *Vector) Equal(s float64) *BoolVector {
	out := NewBoolZeros(v.Len)
	for i := 0; i < v.Len; i++ {
		out.Values[i] = math.Abs(v.Values[i]-s) < 1e-9
	}
	return out
}

// CrossAbove 上穿（当前>阈值 且 上一<阈值）
func (v *Vector) CrossAbove(threshold float64) *BoolVector {
	out := NewBoolZeros(v.Len)
	for i := 1; i < v.Len; i++ {
		out.Values[i] = v.Values[i] > threshold && v.Values[i-1] <= threshold
	}
	return out
}

// CrossBelow 下穿（当前<阈值 且 上一>=阈值）
func (v *Vector) CrossBelow(threshold float64) *BoolVector {
	out := NewBoolZeros(v.Len)
	for i := 1; i < v.Len; i++ {
		out.Values[i] = v.Values[i] < threshold && v.Values[i-1] >= threshold
	}
	return out
}

// CrossAboveVec 向量上穿向量
func (v *Vector) CrossAboveVec(other *Vector) *BoolVector {
	n := min(v.Len, other.Len)
	out := NewBoolZeros(n)
	for i := 1; i < n; i++ {
		out.Values[i] = v.Values[i] > other.Values[i] && v.Values[i-1] <= other.Values[i-1]
	}
	return out
}

// CrossBelowVec 向量下穿向量
func (v *Vector) CrossBelowVec(other *Vector) *BoolVector {
	n := min(v.Len, other.Len)
	out := NewBoolZeros(n)
	for i := 1; i < n; i++ {
		out.Values[i] = v.Values[i] < other.Values[i] && v.Values[i-1] >= other.Values[i-1]
	}
	return out
}

// NanToNum 将 NaN 替换为指定值
func (v *Vector) NanToNum(replace float64) *Vector {
	out := v.Clone()
	for i := 0; i < v.Len; i++ {
		if math.IsNaN(out.Values[i]) || math.IsInf(out.Values[i], 0) {
			out.Values[i] = replace
		}
	}
	return out
}

// ============================================================================
// BoolVector - 布尔向量
// ============================================================================

type BoolVector struct {
	Values []bool
	Len    int
}

func NewBoolVector(values []bool) *BoolVector {
	return &BoolVector{Values: values, Len: len(values)}
}

func NewBoolZeros(n int) *BoolVector {
	return &BoolVector{Values: make([]bool, n), Len: n}
}

func NewBoolOnes(n int) *BoolVector {
	v := make([]bool, n)
	for i := range v {
		v[i] = true
	}
	return &BoolVector{Values: v, Len: n}
}

// And 逻辑与
func (a *BoolVector) And(b *BoolVector) *BoolVector {
	n := min(a.Len, b.Len)
	out := NewBoolZeros(n)
	for i := 0; i < n; i++ {
		out.Values[i] = a.Values[i] && b.Values[i]
	}
	return out
}

// Or 逻辑或
func (a *BoolVector) Or(b *BoolVector) *BoolVector {
	n := min(a.Len, b.Len)
	out := NewBoolZeros(n)
	for i := 0; i < n; i++ {
		out.Values[i] = a.Values[i] || b.Values[i]
	}
	return out
}

// Not 逻辑非
func (a *BoolVector) Not() *BoolVector {
	out := NewBoolZeros(a.Len)
	for i := 0; i < a.Len; i++ {
		out.Values[i] = !a.Values[i]
	}
	return out
}

// Sum True 的数量
func (a *BoolVector) Sum() int {
	n := 0
	for _, v := range a.Values {
		if v {
			n++
		}
	}
	return n
}

// Any 是否有任何 True
func (a *BoolVector) Any() bool {
	for _, v := range a.Values {
		if v {
			return true
		}
	}
	return false
}

// All 是否全为 True
func (a *BoolVector) All() bool {
	for _, v := range a.Values {
		if !v {
			return false
		}
	}
	return true
}

// ============================================================================
// Matrix - 矩阵类型
// ============================================================================

type Matrix struct {
	Rows    int
	Cols    int
	Values  []float64 // 行优先存储
}

func NewMatrix(rows, cols int) *Matrix {
	return &Matrix{
		Rows:   rows,
		Cols:   cols,
		Values: make([]float64, rows*cols),
	}
}

func NewMatrixFromVecs(vecs ...*Vector) *Matrix {
	if len(vecs) == 0 {
		return NewMatrix(0, 0)
	}
	rows := vecs[0].Len
	cols := len(vecs)
	m := NewMatrix(rows, cols)
	for j, vec := range vecs {
		for i := 0; i < rows && i < vec.Len; i++ {
			m.Values[i*cols+j] = vec.Values[i]
		}
	}
	return m
}

// Row 获取第 i 行
func (m *Matrix) Row(i int) *Vector {
	out := NewZeros(m.Cols)
	for j := 0; j < m.Cols; j++ {
		out.Values[j] = m.Values[i*m.Cols+j]
	}
	return out
}

// Col 获取第 j 列
func (m *Matrix) Col(j int) *Vector {
	out := NewZeros(m.Rows)
	for i := 0; i < m.Rows; i++ {
		out.Values[i] = m.Values[i*m.Cols+j]
	}
	return out
}

// ============================================================================
// 辅助函数
// ============================================================================

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Quantile 分位数
func (v *Vector) Quantile(q float64) float64 {
	if v.Len == 0 {
		return math.NaN()
	}
	sorted := make([]float64, v.Len)
	copy(sorted, v.Values)
	sort.Float64s(sorted)
	idx := q * float64(v.Len-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= v.Len {
		return sorted[v.Len-1]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// Percentile 百分位数
func (v *Vector) Percentile(p float64) float64 {
	return v.Quantile(p / 100.0)
}

// Median 中位数
func (v *Vector) Median() float64 {
	return v.Quantile(0.5)
}
