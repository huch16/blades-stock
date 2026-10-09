package quant

import (
	"testing"
)

func TestCalculateETFMomentum(t *testing.T) {
	// 构造一个平稳单边上涨的 21 天收盘价序列: 1.0, 1.01, 1.02 ... 1.20
	closes := make([]float64, 25)
	for i := 0; i < 25; i++ {
		closes[i] = 1.0 + float64(i)*0.01
	}

	res, err := CalculateETFMomentum("510300.SH", "沪深300ETF", closes)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	if res.Score <= 0 {
		t.Errorf("稳步上涨序列的 Score 应该大于 0，实际: %f", res.Score)
	}
	if res.R2 < 0.95 {
		t.Errorf("完美线性上涨的 R2 应该接近 1.0，实际: %f", res.R2)
	}
	if res.CooldownGate {
		t.Errorf("单边上涨不应触发回撤冷却")
	}

	// 构造一个大跌触发回撤冷却的序列：前20天上涨至2.0，第21天下跌至1.8 (跌幅 10% 破 MA5)
	dropCloses := make([]float64, 25)
	for i := 0; i < 24; i++ {
		dropCloses[i] = 1.5 + float64(i)*0.02
	}
	dropCloses[24] = 1.70 // 暴跌
	resDrop, err := CalculateETFMomentum("159915.SZ", "创业板ETF", dropCloses)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}
	if !resDrop.CooldownGate {
		t.Errorf("暴跌跌破MA5应该触发回撤冷却拦截")
	}
}
