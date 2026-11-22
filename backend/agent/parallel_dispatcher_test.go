package agent

import (
	"strings"
	"testing"
)

// TestMergeParallelResults_DedupeAnnotatesOrigin 验证跨组重复的事实行
// 会被标注来源组而不是静默丢弃：旧实现 seen 表存的 origin 永远是
// 空串，`if origin != ""` 的标注分支是死代码，重复行直接消失，
// 主 Agent 看不到“多组交叉印证”这一信息。
func TestMergeParallelResults_DedupeAnnotatesOrigin(t *testing.T) {
	shared := "贵州茅台 600519 现价 1500.00"
	data := []PreFetchedData{
		{Group: "stock_analysis", Summary: shared + "\n技术面多头排列"},
		{Group: "market", Summary: shared + "\n市场情绪偏暖"},
	}
	merged := mergeParallelResults(data)
	if len(merged.sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(merged.sections))
	}
	second := merged.sections[1].Summary
	if !strings.Contains(second, "与个股/ETF分析组重复，已合并") {
		t.Errorf("repeated line should be annotated with its origin group, got:\n%s", second)
	}
	if !strings.Contains(second, shared) {
		t.Errorf("repeated line should be kept (annotated), not dropped, got:\n%s", second)
	}
	if !strings.Contains(merged.sections[0].Summary, shared) {
		t.Errorf("first occurrence should stay untouched, got:\n%s", merged.sections[0].Summary)
	}
}

// TestMergeParallelResults_UniqueLinesUntouched 验证无重复时各组内容原样保留。
func TestMergeParallelResults_UniqueLinesUntouched(t *testing.T) {
	data := []PreFetchedData{
		{Group: "stock_analysis", Summary: "贵州茅台 600519 现价 1500.00"},
		{Group: "market", Summary: "上证指数 3200 点"},
	}
	merged := mergeParallelResults(data)
	for _, s := range merged.sections {
		if strings.Contains(s.Summary, "已合并") {
			t.Errorf("unique lines must not be annotated: %q", s.Summary)
		}
	}
}
