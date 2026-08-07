package strategy

import (
	"fmt"
	"strconv"
	"strings"
)

// conditionFunc 在指标快照上求值一个条件表达式。
type conditionFunc func(ind *Indicators, args []float64) bool

// conditionRegistry 注册所有可在策略 YAML 中使用的条件 DSL 名称。
// 支持带参数形式 name(p1,p2)，参数均为浮点数（周期类取整数部分）。
var conditionRegistry = map[string]conditionFunc{
	// 均线金叉/死叉：ma_golden_cross(5,10)
	"ma_golden_cross": func(ind *Indicators, a []float64) bool {
		s, l, ok := twoPeriods(ind, a)
		if !ok {
			return false
		}
		return ind.MA[s] > ind.MA[l] && ind.MAPrev[s] <= ind.MAPrev[l]
	},
	"ma_death_cross": func(ind *Indicators, a []float64) bool {
		s, l, ok := twoPeriods(ind, a)
		if !ok {
			return false
		}
		return ind.MA[s] < ind.MA[l] && ind.MAPrev[s] >= ind.MAPrev[l]
	},
	// 站上/跌破均线：above_ma(20) / below_ma(20)
	"above_ma": func(ind *Indicators, a []float64) bool {
		p, ok := onePeriod(ind, a)
		return ok && ind.LastClose > ind.MA[p]
	},
	"below_ma": func(ind *Indicators, a []float64) bool {
		p, ok := onePeriod(ind, a)
		return ok && ind.LastClose < ind.MA[p]
	},
	// 均线多头/空头排列（MA5>MA10>MA20>MA60）
	"ma_bullish_alignment": func(ind *Indicators, _ []float64) bool {
		return ind.MAValid[5] && ind.MAValid[10] && ind.MAValid[20] && ind.MAValid[60] &&
			ind.MA[5] > ind.MA[10] && ind.MA[10] > ind.MA[20] && ind.MA[20] > ind.MA[60]
	},
	"ma_bearish_alignment": func(ind *Indicators, _ []float64) bool {
		return ind.MAValid[5] && ind.MAValid[10] && ind.MAValid[20] && ind.MAValid[60] &&
			ind.MA[5] < ind.MA[10] && ind.MA[10] < ind.MA[20] && ind.MA[20] < ind.MA[60]
	},
	// 量比阈值：volume_ratio_gt(1.5) / volume_ratio_lt(0.8)
	"volume_ratio_gt": func(ind *Indicators, a []float64) bool {
		if len(a) < 1 {
			return false
		}
		return ind.VolumeRatio5 > a[0]
	},
	"volume_ratio_lt": func(ind *Indicators, a []float64) bool {
		if len(a) < 1 {
			return false
		}
		return ind.VolumeRatio5 > 0 && ind.VolumeRatio5 < a[0]
	},
	// 突破 N 日新高 / 跌破 N 日新低：breakout_high(20) / breakdown_low(20)
	"breakout_high": func(ind *Indicators, a []float64) bool {
		n, ok := onePeriod(ind, a)
		if !ok {
			return false
		}
		h, exists := ind.PrevHigh[n]
		return exists && ind.LastClose > h
	},
	"breakdown_low": func(ind *Indicators, a []float64) bool {
		n, ok := onePeriod(ind, a)
		if !ok {
			return false
		}
		l, exists := ind.PrevLow[n]
		return exists && ind.LastClose < l
	},
	// 涨跌幅阈值（%）：change_pct_gt(3) / change_pct_lt(-3)
	"change_pct_gt": func(ind *Indicators, a []float64) bool {
		if len(a) < 1 {
			return false
		}
		return ind.ChangePct > a[0]
	},
	"change_pct_lt": func(ind *Indicators, a []float64) bool {
		if len(a) < 1 {
			return false
		}
		return ind.ChangePct < a[0]
	},
	// MACD 状态
	"macd_golden_cross": func(ind *Indicators, _ []float64) bool { return ind.MACDGoldenCross },
	"macd_death_cross":  func(ind *Indicators, _ []float64) bool { return ind.MACDDeathCross },
	"macd_above_zero":   func(ind *Indicators, _ []float64) bool { return ind.MACDAboveZero },
	// 回踩均线未破 maxPct%：pullback_below_ma(20,3)
	"pullback_below_ma": func(ind *Indicators, a []float64) bool {
		if len(a) < 2 {
			return false
		}
		p := int(a[0])
		maxPct := a[1]
		if !ind.MAValid[p] || ind.LastClose >= ind.MA[p] {
			return false
		}
		return (ind.MA[p]-ind.LastClose)/ind.MA[p]*100 <= maxPct
	},
	// RSI 阈值：rsi_gt(70) / rsi_lt(30)
	"rsi_gt": func(ind *Indicators, a []float64) bool {
		if len(a) < 1 {
			return false
		}
		return ind.RSI14 > a[0]
	},
	"rsi_lt": func(ind *Indicators, a []float64) bool {
		if len(a) < 1 {
			return false
		}
		return ind.RSI14 > 0 && ind.RSI14 < a[0]
	},
}

// ParseCondition 解析 "name(p1,p2,...)" 形式的条件表达式，返回条件名与参数列表。
func ParseCondition(expr string) (name string, args []float64, err error) {
	expr = strings.TrimSpace(expr)
	open := strings.Index(expr, "(")
	if open == -1 {
		return expr, nil, nil
	}
	if !strings.HasSuffix(expr, ")") {
		return "", nil, fmt.Errorf("条件表达式缺少右括号: %s", expr)
	}
	name = strings.TrimSpace(expr[:open])
	inner := expr[open+1 : len(expr)-1]
	for _, p := range strings.Split(inner, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return "", nil, fmt.Errorf("条件参数 %q 非法: %w", p, err)
		}
		args = append(args, v)
	}
	return name, args, nil
}

// evalCondition 求值条件表达式；未知条件返回 false。
func evalCondition(expr string, ind *Indicators) bool {
	name, args, err := ParseCondition(expr)
	if err != nil || name == "" {
		return false
	}
	fn, ok := conditionRegistry[name]
	if !ok {
		return false
	}
	return fn(ind, args)
}

// IsKnownCondition 判断条件名是否已注册（供测试与加载校验使用）。
func IsKnownCondition(name string) bool {
	_, ok := conditionRegistry[name]
	return ok
}

func onePeriod(ind *Indicators, a []float64) (int, bool) {
	if len(a) < 1 {
		return 0, false
	}
	p := int(a[0])
	return p, ind.MAValid[p]
}

func twoPeriods(ind *Indicators, a []float64) (int, int, bool) {
	if len(a) < 2 {
		return 0, 0, false
	}
	s, l := int(a[0]), int(a[1])
	if s <= 0 || l <= 0 || s == l {
		return 0, 0, false
	}
	if s > l {
		s, l = l, s
	}
	return s, l, ind.MAValid[s] && ind.MAValid[l]
}
