package data

import (
	"strings"
	"testing"
)

// 模拟行情：真实价格/市值（来自实时接口实际值）
var mockQuotesV2 = map[string]realQuote{
	"688509": {Price: 4.20, TotalMcap: 32.34, Name: "正元地信"},
	"002028": {Price: 156.30, TotalMcap: 1223.16, Name: "思源电气"},
	"002837": {Price: 48.29, TotalMcap: 615.38, Name: "英维克"},
	"601869": {Price: 275.10, TotalMcap: 2277.57, Name: "长飞光纤"},
	"600008": {Price: 2.94, TotalMcap: 215.08, Name: "首创环保"},
}

const v2ReportSample = `| 标的 | 权重建议 | 买入区间参考 | 止损线 |
|------|----------|--------------|--------|
| 正元地信 | 20% | 45-50 元 | 跌破 20 日线 |
| 思源电气 | 20% | 65-70 元 | 跌破 60 日线 |
| 英维克 | 20% | 55-60 元 | 跌破 20 日线 |
| 长飞光纤 | 20% | 18-20 元 | 跌破 60 日线 |
| 首创股份 | 20% | 5.5-6.0 元 | 跌破半年线 |

核心标的：**正元地信 688509** / ~110 亿 / 拥有自主 3D GIS 内核
**思源电气 002028** / ~320 亿 / GIS 全球前三
**英维克 002837** / ~210 亿 / 液冷 CDU 国产龙头
**长飞光纤 601869** / ~400 亿 / 预制棒自给率 100%
**首创股份 600008** / ~330 亿 / 京津冀水务龙头`

func TestCorrectReportContent_PriceAndMcap(t *testing.T) {
	rawCodes := extractSixDigitCodes(v2ReportSample)
	if len(rawCodes) != 5 {
		t.Fatalf("expected 5 codes, got %d: %v", len(rawCodes), rawCodes)
	}
	out := correctReportContent(v2ReportSample, rawCodes, mockQuotesV2)

	// 价格区间被校正为实时价（偏差>50% 且真实价在区间外）
	cases := []struct {
		code string
		want string
	}{
		{"688509", "45-50 元[已校正:实时价格约4.2元]"},
		{"002028", "65-70 元[已校正:实时价格约156.3元]"},
		// 英维克 55-60 vs 48.29：偏差 (60-48.29)/48.29≈24% <50%，不校正
		{"002837", "55-60 元"},
		{"601869", "18-20 元[已校正:实时价格约275.1元]"},
		{"600008", "5.5-6.0 元[已校正:实时价格约2.94元]"},
	}
	for _, c := range cases {
		if !strings.Contains(out, c.want) {
			t.Errorf("code %s: 期望包含 %q，实际输出:\n%s", c.code, c.want, out)
		}
	}

	// 市值被校正为实时值
	if !strings.Contains(out, "~110 亿[已校正:实时市值约32.34亿]") {
		t.Errorf("688509 市值未校正:\n%s", out)
	}
	if !strings.Contains(out, "~320 亿[已校正:实时市值约1223.16亿]") {
		t.Errorf("002028 市值未校正:\n%s", out)
	}
	if !strings.Contains(out, "~210 亿[已校正:实时市值约615.38亿]") {
		t.Errorf("002837 市值未校正:\n%s", out)
	}
	if !strings.Contains(out, "~400 亿[已校正:实时市值约2277.57亿]") {
		t.Errorf("601869 市值未校正:\n%s", out)
	}
	if !strings.Contains(out, "~330 亿[已校正:实时市值约215.08亿]") {
		t.Errorf("600008 市值未校正:\n%s", out)
	}

	// 名称配对：报告旧名"首创股份"→实时名"首创环保"，价格区间应校正
	if !strings.Contains(out, "| 首创股份 | 20% | 5.5-6.0 元[已校正:实时价格约2.94元]") {
		t.Errorf("首创股份(旧名)价格区间未通过名称匹配校正:\n%s", out)
	}
}

func TestCorrectReportContent_NoCode(t *testing.T) {
	out := correctReportContent("这是一段没有股票代码的文本", nil, nil)
	if out != "这是一段没有股票代码的文本" {
		t.Errorf("无代码文本不应被修改: %q", out)
	}
}

func TestCorrectReportContent_UnknownCode(t *testing.T) {
	in := "推荐标的 999999 值得关注"
	out := correctReportContent(in, []string{"999999"}, nil)
	if !strings.Contains(out, "999999[代码不存在]") {
		t.Errorf("未知代码应标注[代码不存在]: %q", out)
	}
}

func TestCorrectReportContent_SmallDeviationUnchanged(t *testing.T) {
	// 价格 25.72，区间 24.2~27.0 含真实值，不应被改
	in := "当前价区间 24.2-27.0 元"
	out := correctReportContent(in, []string{"002906"}, map[string]realQuote{
		"002906": {Price: 25.72, TotalMcap: 80},
	})
	if !strings.Contains(out, "24.2-27.0 元") {
		t.Errorf("小幅偏差不应被校正: %q", out)
	}
}

func TestExtractSixDigitCodes_Dedup(t *testing.T) {
	in := "688509 与 688509 重复，另有 002028"
	codes := extractSixDigitCodes(in)
	if len(codes) != 2 {
		t.Fatalf("expected 2 unique codes, got %v", codes)
	}
	if codes[0] != "688509" || codes[1] != "002028" {
		t.Errorf("order wrong: %v", codes)
	}
}

func TestRangeDeviates(t *testing.T) {
	// 区间 [45,50]，真实 4.2：真实在区间外，偏差 (45-4.2)/4.2≈9.7 >0.5
	if !rangeDeviates(45, 50, 4.2) {
		t.Error("45-50 vs 4.2 应判定偏离")
	}
	// 区间 [24,27]，真实 25.72：在区间内
	if rangeDeviates(24, 27, 25.72) {
		t.Error("24-27 vs 25.72 不应判定偏离")
	}
	// 区间 [55,60]，真实 48.29：偏差 (55-48.29)/48.29≈14% <50%
	if rangeDeviates(55, 60, 48.29) {
		t.Error("55-60 vs 48.29 偏差<50%，不应判定偏离")
	}
	// 区间 [5.5,6.0]，真实 2.94：偏差 (5.5-2.94)/2.94≈87% >50%
	if !rangeDeviates(5.5, 6.0, 2.94) {
		t.Error("5.5-6.0 vs 2.94 应判定偏离")
	}
}

func TestSingleDeviates(t *testing.T) {
	if !singleDeviates(110, 32.34) {
		t.Error("110 vs 32.34 应判定偏离")
	}
	if singleDeviates(32.34, 32.34) {
		t.Error("相等不应判定偏离")
	}
	if singleDeviates(50, 55) {
		t.Error("50 vs 55 偏差<50%，不应判定偏离")
	}
}
