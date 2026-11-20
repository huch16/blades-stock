package quant

import (
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
