package data

import (
	"testing"
)

func TestResolveStrategyScreenName(t *testing.T) {
	// 回归测试：nameMap 未命中时绝不能把代码本身当作名称
	// （曾导致 601318 以名称“601318”混入选股/分析结果）。
	nameMap := map[string]string{
		"600519": "贵州茅台",
		"000001": "平安银行",
		"601318": "",       // DB 行存在但名称空白
		"600036": "600036", // 名称本身就是代码（脏数据）
	}
	cases := []struct {
		code string
		want string
	}{
		{"600519", "贵州茅台"},
		{"000001", "平安银行"},
		{"601318", "未知"}, // 名称空白 -> 标记未知，不回退代码
		{"600036", "未知"}, // 名称==代码 -> 标记未知
		{"999999", "未知"}, // 列表里没有的代码 -> 标记未知
		{"300750", "未知"}, // 兜底池里可能出现但 nameMap 为空的代码
	}
	for _, tc := range cases {
		if got := resolveStrategyScreenName(nameMap, tc.code); got != tc.want {
			t.Errorf("resolveStrategyScreenName(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
	// nil map 也不应 panic，同样标记未知。
	if got := resolveStrategyScreenName(nil, "601318"); got != "未知" {
		t.Errorf("resolveStrategyScreenName(nil, 601318) = %q, want 未知", got)
	}
	// 名称含空白时应 trim 后返回。
	padded := map[string]string{"601318": "  中国平安  "}
	if got := resolveStrategyScreenName(padded, "601318"); got != "中国平安" {
		t.Errorf("resolveStrategyScreenName(padded) = %q, want 中国平安", got)
	}
}

func TestFallbackScreenByIndicators(t *testing.T) {
	// 测试空输入
	if _, ok := FallbackScreenByIndicators(""); ok {
		t.Errorf("expected false for empty query, got true")
	}

	// 测试量化策略匹配意图（多头、突破、反弹等）
	res, ok := FallbackScreenByIndicators("请帮我筛选均线多头排列的股票")
	// 无论本地是否有足够K线数据计算，FallbackScreenByIndicators 都不应发生 panic
	t.Logf("FallbackScreenByIndicators (多头): ok=%v, res_len=%d", ok, len(res))

	res2, ok2 := FallbackScreenByIndicators("放量突破")
	t.Logf("FallbackScreenByIndicators (放量突破): ok=%v, res_len=%d", ok2, len(res2))

	res3, ok3 := FallbackScreenByIndicators("半导体")
	t.Logf("FallbackScreenByIndicators (半导体行业/概念): ok=%v, res_len=%d", ok3, len(res3))
}
