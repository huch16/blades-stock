package agent

import (
	"context"
	"testing"

	"blades-stock/backend/agent/types"
)

func TestExtractAndSaveRecommendationsFromReport(t *testing.T) {
	report := &MarketIntelligenceReport{
		Title:   "测试盘前研报",
		Source:  "TestAgent",
		Content: "建议重点关注 512480 半导体ETF 与 600519 贵州茅台 的反弹机会。",
		Actions: []types.AgentAction{
			{
				Type: types.ActionOpenDiagnosis,
				Payload: map[string]interface{}{
					"code":   "600519",
					"name":   "贵州茅台",
					"reason": "业绩支撑强劲，估值位于历史低位",
				},
			},
		},
	}

	recs, err := ExtractAndSaveRecommendationsFromReport(report)
	if err != nil {
		t.Fatalf("提取失败: %v", err)
	}
	t.Logf("成功提取并处理推荐条数: %d", len(recs))
}

func TestSyncRecommendStocksFromAllIntelligence(t *testing.T) {
	ctx := context.Background()
	count, err := SyncRecommendStocksFromAllIntelligence(ctx)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	t.Logf("同步成功新增条数: %d", count)
}
