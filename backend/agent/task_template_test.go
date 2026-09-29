package agent

import (
	"strings"
	"testing"
)

// TestIsIndividualStockQuestion 验证个股/非个股问题的判别：
// ETF 推荐、板块/市场/资讯等问题不得误判为个股，避免套用个股深度分析模板。
func TestIsIndividualStockQuestion(t *testing.T) {
	cases := []struct {
		question string
		want     bool
	}{
		{"分析天目药业600671走势及题材演化", true},
		{"对正元地信(sh688509)进行全方位深度分析", true},
		{"600519最新行情怎么样", true},
		{"分析贵州茅台的投资价值", true},
		{"帮我分析个股后给出建议", true},
		{"推荐3个ETF基金", false},
		{"分析ETF鹏华589020走势", false},
		{"510300当前价多少", false},
		{"今天A股市场总结", false},
		{"半导体行业分析", false},
		{"对比分析贵州茅台和五粮液", false},
	}
	for _, c := range cases {
		if got := isIndividualStockQuestion(c.question); got != c.want {
			t.Errorf("isIndividualStockQuestion(%q) = %v, want %v", c.question, got, c.want)
		}
	}
}

// TestRenderTaskTemplate 验证 {{TASK_TEMPLATE}} 按问题类型注入不同模板：
// ETF 推荐问题注入通用框架（不含个股模板），个股问题注入个股深度分析模板。
func TestRenderTaskTemplate(t *testing.T) {
	tpl := "任务：{{TASK_TEMPLATE}}"

	out := renderTaskTemplate(tpl, "推荐3个ETF基金")
	if strings.Contains(out, "个股深度分析模板") || strings.Contains(out, "第一步：数据采集") {
		t.Errorf("ETF question must NOT get stock template: %q", out)
	}
	if !strings.Contains(out, "不是针对单只个股") {
		t.Errorf("ETF question should get generic framework: %q", out)
	}

	out = renderTaskTemplate(tpl, "分析天目药业600671走势及题材演化")
	if !strings.Contains(out, "第一步：数据采集") || !strings.Contains(out, "高级分析工具") {
		t.Errorf("stock question should get stock template: %q", out)
	}
	if strings.Contains(out, "不是针对单只个股") {
		t.Errorf("stock question must NOT get generic framework: %q", out)
	}

	// 无占位符时原样返回（兼容旧式 JSON 覆盖配置）。
	if got := renderTaskTemplate("无占位符", "推荐3个ETF基金"); got != "无占位符" {
		t.Errorf("no placeholder should be unchanged: %q", got)
	}
}
