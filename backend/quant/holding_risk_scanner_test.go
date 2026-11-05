package quant

import (
	"blades-stock/backend/db"
	"strings"
	"testing"
)

// 说明（v3.0.0 固化）：
//
// 本测试同时覆盖两类断言，边界必须清晰区分：
//
//  1. **确定性断言**（可安全用于 CI）——解析、条目数量、分级合法性、防守价有效性、
//     “分级与操作建议不得自相矛盾”、报告长度上限。这些与行情无关，永远不会误报。
//
//  2. **行情相关观测**（不可用于 CI 断言）——例如“TCL科技 必须为 YELLOW”，
//     它依赖当日实时 MA 排列。历史版本把它写成 t.Errorf，导致行情一变、CI 必红
//     （实测 2026-11-05 TCL科技 已从 YELLOW 转为 GREEN 而测试失败）。
//     现在只以 t.Logf 输出观测值，供人工核对，不再作为断言。
//
// 若需要在 CI 中锁死“分级矩阵”本身的行为，应对纯函数补充表驱动测试，
// 而不是对实时行情断言绝对分级。

const holdingScanTestPrompt = `【盘前持仓风控扫描与防守边界审计】
自选分组：短线观察（共 7 只标的）
标的与持仓详情：
贵州茅台(600519): 现价1469.27元, 涨幅-0.72%
半导体ETF(512480): 现价0.730元, 涨幅+3.85%
比亚迪(002594): 现价85.77元, 涨幅+0.97%
TCL科技(000100): 现价4.65元, 涨幅+2.20%
沪深300ETF(510300): 现价4.12元, 涨幅+0.81%
华阳集团(002906): 现价24.46元, 涨幅+1.70%
宁德时代(300750): 现价253.75元, 涨幅+1.78%`

var validRiskTiers = map[string]bool{"GREEN": true, "YELLOW": true, "RED": true}

// TestParseHoldingScanItemsFromPrompt 纯确定性：提示词解析必须稳定
func TestParseHoldingScanItemsFromPrompt(t *testing.T) {
	items := ParseHoldingScanItemsFromPrompt(holdingScanTestPrompt)
	if len(items) != 7 {
		t.Fatalf("expected 7 items, got %d", len(items))
	}
	codes := make(map[string]bool, len(items))
	for _, it := range items {
		if it.StockCode == "" {
			t.Errorf("解析出空股票代码: %+v", it)
		}
		codes[it.StockCode] = true
	}
	for _, want := range []string{"600519", "512480", "002594", "000100", "510300", "002906", "300750"} {
		if !codes[want] {
			t.Errorf("解析结果缺少标的 %s", want)
		}
	}
}

// TestScanHoldingRiskStructuralInvariants 覆盖不依赖行情走势的结构性不变量。
// 这些断言在任意市场状态下都必须成立，可用于 CI 门禁。
func TestScanHoldingRiskStructuralInvariants(t *testing.T) {
	db.Init("../../data/stock.db")

	items := ParseHoldingScanItemsFromPrompt(holdingScanTestPrompt)
	if len(items) != 7 {
		t.Fatalf("ParseHoldingScanItemsFromPrompt expected 7 items, got %d", len(items))
	}

	res, err := ScanHoldingRisk(items)
	if err != nil {
		t.Fatalf("ScanHoldingRisk returned error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil ScanHoldingRisk result")
	}
	if len(res.Items) != 7 {
		t.Fatalf("expected 7 items in result, got %d", len(res.Items))
	}

	itemMap := make(map[string]HoldingRiskItem, len(res.Items))
	for _, it := range res.Items {
		itemMap[it.StockCode] = it
	}

	for _, code := range []string{"600519", "512480", "002594", "000100", "510300", "002906", "300750"} {
		item, ok := itemMap[code]
		if !ok {
			t.Fatalf("结果缺少标的 %s", code)
		}

		// (1) 分级必须落在合法集合内（代码决定，禁止 LLM 篡改结论）
		if !validRiskTiers[item.RiskTier] {
			t.Errorf("[%s] 非法风险分级: %q", code, item.RiskTier)
		}
		// (2) 分级前缀与分级必须一致（防止模板错配）
		wantPrefix := map[string]string{"GREEN": "🟢", "YELLOW": "🟡", "RED": "🔴"}[item.RiskTier]
		if wantPrefix != "" && item.TierPrefix != wantPrefix {
			t.Errorf("[%s] 分级前缀与分级不符: tier=%s prefix=%q", code, item.RiskTier, item.TierPrefix)
		}
		// (3) 分级标题不得为空
		if strings.TrimSpace(item.TierTitle) == "" {
			t.Errorf("[%s] 分级标题为空", code)
		}
		// (4) 防守位必须为正（数据缺失时应走 fallback 并给出有效防守位）
		if item.DefensePrice <= 0 {
			t.Errorf("[%s] 防守位非正: %.4f", code, item.DefensePrice)
		}
		// (5) 均线纠缠态的判定必须与均线取值自洽（结构性约束，非行情断言）
		if item.Alignment == "多头排列" && !(item.MA5 >= item.MA10 && item.MA10 >= item.MA20) {
			t.Errorf("[%s] 标记为多头排列但 MA 不满足 MA5>=MA10>=MA20: %.3f/%.3f/%.3f",
				code, item.MA5, item.MA10, item.MA20)
		}
		if item.Alignment == "空头排列" && !(item.MA5 <= item.MA10 && item.MA10 <= item.MA20) {
			t.Errorf("[%s] 标记为空头排列但 MA 不满足 MA5<=MA10<=MA20: %.3f/%.3f/%.3f",
				code, item.MA5, item.MA10, item.MA20)
		}
		// (6) 硬风控锁定必须落在 RED 档（确定性硬覆盖）
		if item.HasRiskLock && item.RiskTier != "RED" {
			t.Errorf("[%s] 触发硬风控锁定却未进入 RED 档: tier=%s", code, item.RiskTier)
		}
		// (7) 严禁自相矛盾：🔴 禁止新增仓 / 空头禁抄底 的建议里不得出现进攻话术
		if item.RiskTier == "RED" {
			if strings.Contains(item.ActionAdvice, "进攻") || strings.Contains(item.ActionAdvice, "依托MA5持有") {
				t.Errorf("[%s] RED 档策略包含进攻话术，逻辑自相矛盾: %s", code, item.ActionAdvice)
			}
		}
	}

	// (8) 汇总统计必须与明细一致（防止计数漂移）
	total := res.GreenCount + res.YellowCount + res.RedCount
	if total != len(res.Items) {
		t.Errorf("分级计数之和 %d 与明细条数 %d 不一致 (G=%d Y=%d R=%d)",
			total, len(res.Items), res.GreenCount, res.YellowCount, res.RedCount)
	}

	// (9) 报告长度上限（精炼输出约束）
	cjkCount := 0
	for _, r := range res.MarkdownReport {
		if r >= 0x4e00 && r <= 0x9fa5 {
			cjkCount++
		}
	}
	t.Logf("Markdown Report CJK count: %d characters", cjkCount)
	if cjkCount > 350 {
		t.Errorf("Markdown Report 字符数过长: %d CJK characters (预期 <= 350)", cjkCount)
	}

	// 观测输出（仅供人工核对，不作为断言——分级依赖当日实时行情）
	t.Logf("ScanHoldingRisk Report Output:\n%s", res.MarkdownReport)
	for _, code := range []string{"600519", "512480", "002594", "000100", "510300", "002906", "300750"} {
		it := itemMap[code]
		t.Logf("  观测 %s %s -> %s %s (MA5=%.3f MA10=%.3f MA20=%.3f 价=%.3f 排列=%s 防守=%.2f)",
			code, it.StockName, it.RiskTier, it.TierTitle, it.MA5, it.MA10, it.MA20,
			it.CurrentPrice, it.Alignment, it.DefensePrice)
	}
}
