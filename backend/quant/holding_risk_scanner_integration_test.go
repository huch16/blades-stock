//go:build integration

package quant

import (
	"blades-stock/backend/db"
	"strings"
	"testing"
)

// TestScanHoldingRiskStructuralInvariants 覆盖不依赖行情走势的结构性不变量。
// 这些断言在任意市场状态下都必须成立，可用于 CI 门禁。
//
// 但它调用 ScanHoldingRisk 拉取实时 K 线（见同名原测试文件的说明），
// 且 db.Init 依赖仓库根 data/stock.db，属于网络/DB 依赖测试，
// 按 AGENTS.md 约定打 integration 标签：默认 `go test -skip integration`
// 不跑，`make test-integration`（-tags=integration）才跑。断言一字未改。
// R-02 从 holding_risk_scanner_test.go 原样移入本文件。
// 跳过的是环境依赖，不是缺陷；在有网 CI 仍须跑。
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
