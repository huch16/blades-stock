package agent

import "testing"

func TestContainsDecisionKeyword_Negative(t *testing.T) {
	cases := []string{
		"",                   // empty
		"hello",              // short non-chinese
		"今天的行情怎么样",           // just a question
		"查询一下茅台的代码",          // lookup
		"以下是 159915 的 K 线数据", // data report, no decision
		"茅台股价 1800 元",        // quote
	}
	for _, c := range cases {
		if got := containsDecisionKeyword(c); got {
			t.Errorf("containsDecisionKeyword(%q) = true, want false", c)
		}
	}
}

func TestContainsDecisionKeyword_Positive_Strong(t *testing.T) {
	cases := []string{
		"建议立即买入 159915",
		"建议卖出所有持仓",
		"今天加仓 5%",
		"明天减仓一半",
		"止损价 1750",
		"止盈目标 1900",
	}
	for _, c := range cases {
		if got := containsDecisionKeyword(c); !got {
			t.Errorf("containsDecisionKeyword(%q) = false, want true", c)
		}
	}
}

func TestContainsDecisionKeyword_Positive_Medium(t *testing.T) {
	cases := []string{
		"分析结论：今天适合建仓",
		"总结：长期持有",
		"最终建议：观望为主",
	}
	for _, c := range cases {
		if got := containsDecisionKeyword(c); !got {
			t.Errorf("containsDecisionKeyword(%q) = false, want true", c)
		}
	}
}

func TestContainsDecisionKeyword_Positive_Combo(t *testing.T) {
	cases := []string{
		"建议持有：估值合理",
		"建议买入：动量为正",
		"建议卖出：跌破支撑",
		"建议观望：等待突破",
	}
	for _, c := range cases {
		if got := containsDecisionKeyword(c); !got {
			t.Errorf("containsDecisionKeyword(%q) = false, want true", c)
		}
	}
}
