package quant

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"blades-stock/backend/data"
	"blades-stock/backend/models"
)

// ETFMomentumScoreResult ETF 21日加权动量计算结果 (纯数学动量矩阵)
type ETFMomentumScoreResult struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Close        float64 `json:"close"`
	Annualized   float64 `json:"annualized"`   // 年化收益率 (基于斜率)
	R2           float64 `json:"r2"`           // 线性回归拟合优度 R^2
	Score        float64 `json:"score"`        // 动量综合得分 = Annualized * R2
	Pullback5D   float64 `json:"pullback_5d"`  // 近5日高点回撤百分比
	AboveMA5     bool    `json:"above_ma5"`    // 现价是否收复 MA5
	CooldownGate bool    `json:"cooldown_gate"`// 是否触发回撤冷却拦截 (近5日回撤 >= 5% 且未收复 MA5)
	Rank         int     `json:"rank"`
}

// CalculateETFMomentum 计算单个标的的21日加权对数动量与回撤指标
// 严格对齐 multi-agents-etf-trade-strategy 的数学公式：
// y = log(close[-21:])
// weights = linspace(1, 2, 21)
// slope, intercept = polyfit(x, y, 1, w=weights)
// annualized = exp(slope * 250) - 1
// R^2 = 1 - sum(w * (y - y_hat)^2) / sum(w * (y - y_bar)^2)
// score = annualized * R^2
func CalculateETFMomentum(code, name string, closes []float64) (*ETFMomentumScoreResult, error) {
	n := len(closes)
	if n < 21 {
		return nil, fmt.Errorf("数据不足21天 (实际 %d 天)", n)
	}

	window := closes[n-21:]
	currentClose := window[20]

	// 1. 近 5 日高点回撤与 MA5
	max5D := window[16]
	sum5D := 0.0
	for i := 16; i <= 20; i++ {
		if window[i] > max5D {
			max5D = window[i]
		}
		sum5D += window[i]
	}
	ma5 := sum5D / 5.0
	pullback5D := 0.0
	if max5D > 0 {
		pullback5D = (max5D - currentClose) / max5D * 100.0
	}
	aboveMA5 := currentClose >= ma5
	// 回撤冷却闸门：5日回撤 >= 5% 且 未站上 MA5
	cooldownGate := pullback5D >= 5.0 && !aboveMA5

	// 2. 21日加权线性回归 (对数收盘价)
	y := make([]float64, 21)
	w := make([]float64, 21)
	sumW := 0.0
	sumWY := 0.0
	for i := 0; i < 21; i++ {
		if window[i] <= 0 {
			return nil, fmt.Errorf("收盘价存在非正数: %f", window[i])
		}
		y[i] = math.Log(window[i])
		// weights = linspace(1, 2, 21)
		w[i] = 1.0 + float64(i)*(1.0/20.0)
		sumW += w[i]
		sumWY += w[i] * y[i]
	}
	yBar := sumWY / sumW

	// 加权最小二乘法求斜率 slope
	// x = [0..20]
	sumWX := 0.0
	for i := 0; i < 21; i++ {
		sumWX += w[i] * float64(i)
	}
	xBar := sumWX / sumW

	var num, den float64
	for i := 0; i < 21; i++ {
		xi := float64(i)
		num += w[i] * (xi - xBar) * (y[i] - yBar)
		den += w[i] * math.Pow(xi-xBar, 2)
	}
	if den == 0 {
		return nil, fmt.Errorf("分母为0，回归计算失败")
	}
	slope := num / den
	intercept := yBar - slope*xBar

	// 年化收益率 = exp(slope * 250) - 1
	annualized := math.Exp(slope*250.0) - 1.0

	// 3. 计算加权 R^2
	var ssRes, ssTot float64
	for i := 0; i < 21; i++ {
		xi := float64(i)
		yHat := slope*xi + intercept
		ssRes += w[i] * math.Pow(y[i]-yHat, 2)
		ssTot += w[i] * math.Pow(y[i]-yBar, 2)
	}

	r2 := 0.0
	if ssTot > 1e-12 {
		r2 = 1.0 - (ssRes / ssTot)
	}
	if r2 < 0 {
		r2 = 0
	} else if r2 > 1.0 {
		r2 = 1.0
	}

	score := annualized * r2

	return &ETFMomentumScoreResult{
		Code:         code,
		Name:         name,
		Close:        math.Round(currentClose*1000) / 1000,
		Annualized:   math.Round(annualized*10000) / 100, // 百分比
		R2:           math.Round(r2*1000) / 1000,
		Score:        math.Round(score*1000) / 1000,
		Pullback5D:   math.Round(pullback5D*100) / 100,
		AboveMA5:     aboveMA5,
		CooldownGate: cooldownGate,
	}, nil
}

// BatchCalculateETFMomentum 批量拉取核心 ETF 池历史K线，计算纯数学动量排名与硬风控指标
func BatchCalculateETFMomentum(codes []string) ([]*ETFMomentumScoreResult, error) {
	loader := NewStockDataLoader()
	results := make([]*ETFMomentumScoreResult, 0, len(codes))

	for _, code := range codes {
		clean := models.CleanStockCode(code)
		closes, _, _, _, _, _, err := loader.LoadKLineData(clean, 40)
		if err != nil || len(closes) < 21 {
			continue
		}

		name := code
		quotes, err := data.NewStockDataApi().GetStockCodeRealTimeData(code)
		if err == nil && quotes != nil && len(*quotes) > 0 {
			name = (*quotes)[0].Name
		}

		res, err := CalculateETFMomentum(code, name, closes)
		if err == nil {
			results = append(results, res)
		}
	}

	// 按动量 Score 降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	for i := range results {
		results[i].Rank = i + 1
	}

	return results, nil
}

// FormatETFMomentumSummary 将纯 Go 计算出的真实动量矩阵格式化为硬核 Markdown 表格，注入 Agent
func FormatETFMomentumSummary(list []*ETFMomentumScoreResult) string {
	if len(list) == 0 {
		return "（未能获取到足够K线数据计算动量）\n"
	}

	var sb strings.Builder
	sb.WriteString("| 排名 | ETF名称 (代码) | 现价 | 年化动量(%) | 线性R² | 动量得分 | 5日回撤 | MA5状态 | 风控状态 |\n")
	sb.WriteString("| :---: | :--- | :---: | :---: | :---: | :---: | :---: | :---: | :--- |\n")
	for _, item := range list {
		status := "🟢 正常"
		if item.CooldownGate {
			status = "🚨 回撤冷却拦截(禁止新开仓)"
		} else if !item.AboveMA5 {
			status = "⚠️ MA5下方(警惕)"
		}
		ma5Str := "已站上"
		if !item.AboveMA5 {
			ma5Str = "失守"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s (%s) | %.3f | %+.2f%% | %.3f | **%.3f** | %.2f%% | %s | %s |\n",
			item.Rank, item.Name, item.Code, item.Close, item.Annualized, item.R2, item.Score, item.Pullback5D, ma5Str, status))
	}
	return sb.String()
}
