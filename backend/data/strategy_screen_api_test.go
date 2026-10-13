package data

import (
	"testing"
)

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
