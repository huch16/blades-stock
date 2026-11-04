package quant

import (
	"blades-stock/backend/db"
	"strings"
	"testing"
)

func TestScanHoldingRisk7Stocks(t *testing.T) {
	db.Init("../../data/stock.db")

	testPrompt := `【盘前持仓风控扫描与防守边界审计】
自选分组：短线观察（共 7 只标的）
标的与持仓详情：
贵州茅台(600519): 现价1469.27元, 涨幅-0.72%
半导体ETF(512480): 现价0.730元, 涨幅+3.85%
比亚迪(002594): 现价85.77元, 涨幅+0.97%
TCL科技(000100): 现价4.65元, 涨幅+2.20%
沪深300ETF(510300): 现价4.12元, 涨幅+0.81%
华阳集团(002906): 现价24.46元, 涨幅+1.70%
宁德时代(300750): 现价253.75元, 涨幅+1.78%`

	items := ParseHoldingScanItemsFromPrompt(testPrompt)
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

	itemMap := make(map[string]HoldingRiskItem)
	for _, it := range res.Items {
		itemMap[it.StockCode] = it
	}

	// 1. 验证 贵州茅台 (600519) 必须为 🟢 多头持有 (禁止误判为 🔴 止损)
	maotai, ok := itemMap["600519"]
	if !ok {
		t.Fatalf("missing 600519 in results")
	}
	if maotai.RiskTier != "GREEN" {
		t.Errorf("贵州茅台 (600519) 预期为 GREEN (多头排列且站上MA20)，实际得到: %s (%s)", maotai.RiskTier, maotai.TierTitle)
	}
	if maotai.Alignment != "多头排列" {
		t.Errorf("贵州茅台 (600519) 均线预期为多头排列，实际得到: %s (MA5=%.2f, MA10=%.2f, MA20=%.2f)",
			maotai.Alignment, maotai.MA5, maotai.MA10, maotai.MA20)
	}
	if maotai.MA20 <= 0 || maotai.DefensePrice <= 0 {
		t.Errorf("贵州茅台 (600519) 防守价位缺失: %.2f", maotai.DefensePrice)
	}

	// 2. 验证 半导体ETF (512480) 必须为 🔴 禁止新增仓/破位 (禁止误判为 🟢 进攻跟踪)
	semi, ok := itemMap["512480"]
	if !ok {
		t.Fatalf("missing 512480 in results")
	}
	if semi.RiskTier != "RED" {
		t.Errorf("半导体ETF (512480) 预期为 RED (空头排列/破位/冷却)，实际得到: %s (%s)", semi.RiskTier, semi.TierTitle)
	}
	if strings.Contains(semi.ActionAdvice, "进攻") || strings.Contains(semi.ActionAdvice, "依托MA5持有") {
		t.Errorf("半导体ETF (512480) 策略严禁包含进攻或依托MA5，实际得到: %s", semi.ActionAdvice)
	}

	// 3. 验证 比亚迪 (002594) 必须为 🟢 多头趋势/持有 (禁止误判为 🟡 MA20下方)
	byd, ok := itemMap["002594"]
	if !ok {
		t.Fatalf("missing 002594 in results")
	}
	if byd.RiskTier != "GREEN" {
		t.Errorf("比亚迪 (002594) 预期为 GREEN (多头排列且高于MA20)，实际得到: %s (%s)", byd.RiskTier, byd.TierTitle)
	}
	if byd.DistanceToMA20 <= 0 {
		t.Errorf("比亚迪 (002594) 价格应在 MA20 上方，实际偏离度: %.2f%%", byd.DistanceToMA20)
	}

	// 4. 验证 TCL科技 (000100) 必须为 🟡 待放量确认 (禁止误判为 🟢 进攻)
	tcl, ok := itemMap["000100"]
	if !ok {
		t.Fatalf("missing 000100 in results")
	}
	if tcl.RiskTier != "YELLOW" {
		t.Errorf("TCL科技 (000100) 预期为 YELLOW (MA10<MA20纠缠/量比未放量)，实际得到: %s (%s)", tcl.RiskTier, tcl.TierTitle)
	}

	// 5. 验证 沪深300ETF (510300) 防守点位必须实时动态计算，不可搬运过期的 4.02
	etf300, ok := itemMap["510300"]
	if !ok {
		t.Fatalf("missing 510300 in results")
	}
	if etf300.MA20 < 4.15 || etf300.MA20 > 4.30 {
		t.Errorf("沪深300ETF (510300) MA20 计算偏差较大: %.3f (预期约 4.20)", etf300.MA20)
	}

	// 6. 验证 华阳集团 (002906) 为 🔴 空头禁抄底 (空头排列且破MA20)
	hy, ok := itemMap["002906"]
	if !ok {
		t.Fatalf("missing 002906 in results")
	}
	if hy.RiskTier != "RED" {
		t.Errorf("华阳集团 (002906) 预期为 RED，实际得到: %s (%s)", hy.RiskTier, hy.TierTitle)
	}

	// 7. 验证 宁德时代 (300750) 为 🟡 MA20下方
	catl, ok := itemMap["300750"]
	if !ok {
		t.Fatalf("missing 300750 in results")
	}
	if catl.RiskTier != "YELLOW" {
		t.Errorf("宁德时代 (300750) 预期为 YELLOW，实际得到: %s (%s)", catl.RiskTier, catl.TierTitle)
	}

	t.Logf("ScanHoldingRisk Report Output:\n%s", res.MarkdownReport)

	// 验证 CJK 字符数在合理精炼范围内
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
}
