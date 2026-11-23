package agent

import (
	"errors"
	"testing"
)

var errFakeLLM = errors.New("fake llm failure")

// TestScoreResultStructured 锁定确定性反思评分的四维度计算契约。
//
// ScoreResultStructured 是 LLM 反思不可用时的兜底评分，纯关键词启发式，
// 不依赖任何外部服务，应当完全确定可复现。
func TestScoreResultStructured(t *testing.T) {
	e := NewEvaluator()

	t.Run("empty string returns zero", func(t *testing.T) {
		got := e.ScoreResultStructured("")
		if got.Score != 0 || got.Accuracy != 0 || got.Relevance != 0 ||
			got.ToolUse != 0 || got.Completeness != 0 {
			t.Errorf("empty input should return all zeros, got %+v", got)
		}
	})

	t.Run("positive keywords raise score and completeness", func(t *testing.T) {
		got := e.ScoreResultStructured("获取到数据，查询成功，输出结果如下")
		if got.Score <= 0 {
			t.Errorf("positive keywords should yield Score > 0, got %d", got.Score)
		}
		if got.Completeness <= 5 {
			t.Errorf("positive keywords should raise Completeness > 5, got %d", got.Completeness)
		}
	})

	t.Run("negative keywords lower score and accuracy", func(t *testing.T) {
		got := e.ScoreResultStructured("失败，错误，异常，无法获取，超时")
		if got.Score >= 0 {
			t.Errorf("negative keywords should yield Score <= 0, got %d", got.Score)
		}
		if got.Accuracy >= 5 {
			t.Errorf("negative keywords should lower Accuracy <= 5, got %d", got.Accuracy)
		}
	})

	t.Run("numeric indicators boost accuracy and completeness", func(t *testing.T) {
		base := e.ScoreResultStructured("分析完成")
		withNum := e.ScoreResultStructured("分析完成，涨幅 5.2%，市值 300 亿元")
		if withNum.Accuracy <= base.Accuracy {
			t.Errorf("numeric indicators should boost Accuracy: %d vs %d", withNum.Accuracy, base.Accuracy)
		}
		if withNum.Completeness <= base.Completeness {
			t.Errorf("numeric indicators should boost Completeness: %d vs %d", withNum.Completeness, base.Completeness)
		}
	})

	t.Run("time-related keywords boost toolUse and relevance", func(t *testing.T) {
		base := e.ScoreResultStructured("分析完成")
		withTime := e.ScoreResultStructured("截至今日，最新数据已获取，时间戳记录")
		if withTime.ToolUse <= base.ToolUse {
			t.Errorf("time keywords should boost ToolUse: %d vs %d", withTime.ToolUse, base.ToolUse)
		}
		if withTime.Relevance <= base.Relevance {
			t.Errorf("time keywords should boost Relevance: %d vs %d", withTime.Relevance, base.Relevance)
		}
	})

	t.Run("advice/risk keywords boost completeness", func(t *testing.T) {
		base := e.ScoreResultStructured("分析完成")
		withAdvice := e.ScoreResultStructured("建议观察，存在风险，观点如下，结论明确")
		if withAdvice.Completeness <= base.Completeness {
			t.Errorf("advice/risk keywords should boost Completeness: %d vs %d", withAdvice.Completeness, base.Completeness)
		}
	})

	t.Run("clamp bounds 0 to 10", func(t *testing.T) {
		// 极大量正面关键词不应超过 10
		got := e.ScoreResultStructured("成功 完成 获取到 查到了 如下 数据 结果 输出 返回 response 成功 完成 获取到 查到了 如下 数据 结果 输出 返回 response")
		if got.Accuracy > 10 || got.Relevance > 10 || got.ToolUse > 10 || got.Completeness > 10 {
			t.Errorf("values should be clamped to 10, got %+v", got)
		}
		if got.Score > 1 {
			t.Errorf("composite Score should be clamped to 1, got %d", got.Score)
		}

		// 极大量负面关键词不应低于 0
		got2 := e.ScoreResultStructured("失败 错误 异常 无法 超时 error failed timeout nil 空 失败 错误 异常 无法 超时 error failed timeout nil 空")
		if got2.Accuracy < 0 || got2.Relevance < 0 || got2.ToolUse < 0 || got2.Completeness < 0 {
			t.Errorf("values should be clamped to 0, got %+v", got2)
		}
		if got2.Score < -1 {
			t.Errorf("composite Score should be clamped to -1, got %d", got2.Score)
		}
	})

	t.Run("composite score mapping", func(t *testing.T) {
		// score > 0 -> composite = 1
		pos := e.ScoreResultStructured("成功")
		if pos.Score != 1 {
			t.Errorf("positive composite should be 1, got %d", pos.Score)
		}
		// score < 0 -> composite = -1
		neg := e.ScoreResultStructured("失败")
		if neg.Score != -1 {
			t.Errorf("negative composite should be -1, got %d", neg.Score)
		}
		// score == 0 -> composite = 0
		neu := e.ScoreResultStructured("股票代码 600519，公司名称 贵州茅台")
		if neu.Score != 0 {
			t.Errorf("neutral composite should be 0, got %d", neu.Score)
		}
	})
}

// TestScoreResultStructured_Deterministic 锁定确定性：同一输入多次调用结果相同。
func TestScoreResultStructured_Deterministic(t *testing.T) {
	e := NewEvaluator()
	input := "分析完成，涨幅 5.2%，建议观察，存在风险"
	first := e.ScoreResultStructured(input)
	second := e.ScoreResultStructured(input)
	if first != second {
		t.Errorf("ScoreResultStructured should be deterministic, got %+v vs %+v", first, second)
	}
}

// TestScoreResultStructured_BareKongNotNegative 回归：裸 "空" 曾是负面词，
// 会误伤"空间/空头/空白"等正常词，导致正常研报被打低分并误触发进化。
// 修复后只有"为空/空值/无数据"这类精确短语才扣分。
func TestScoreResultStructured_BareKongNotNegative(t *testing.T) {
	e := NewEvaluator()

	t.Run("normal words containing kong are neutral", func(t *testing.T) {
		withKong := e.ScoreResultStructured("分析完成，市场空间广阔，空头仓位下降，空白表格已填好")
		withoutKong := e.ScoreResultStructured("分析完成，市场广阔，仓位下降，表格已填好")
		if withKong != withoutKong {
			t.Errorf("words with 空 should score the same as without: %+v vs %+v", withKong, withoutKong)
		}
	})

	t.Run("precise empty-result phrases still penalize", func(t *testing.T) {
		// 注意：不要用"查询结果为空"这类输入——"结果/数据"本身是正面词，
		// 会抵消负面词，这是关键词启发式的固有噪声，不是本回归要锁的。
		got := e.ScoreResultStructured("查询为空，无数据")
		if got.Score >= 0 {
			t.Errorf("empty-result phrases should yield Score < 0, got %+v", got)
		}
		if got.Accuracy >= 5 {
			t.Errorf("empty-result phrases should lower Accuracy < 5, got %+v", got)
		}
	})
}

// TestGenerateReflection_LLMFailureFallsBack 回归：LLM 打分失败（或返回
// 非 JSON）时应静默无声地退回确定性评分之外——至少记一条日志，
// 且结果仍为确定性评分（本测试只锁定后者：错误不冒泡、不丢分）。
func TestGenerateReflection_LLMFailureFallsBack(t *testing.T) {
	e := NewEvaluator()
	failingLLM := func(systemPrompt, userMessage string) (string, error) {
		return "", errFakeLLM
	}
	garbageLLM := func(systemPrompt, userMessage string) (string, error) {
		return "not json at all", nil
	}
	answer := "获取到数据，查询成功，输出结果如下：涨幅 5.2%"
	for name, llm := range map[string]LLMReflectionFunc{
		"llm error":   failingLLM,
		"llm garbage": garbageLLM,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := e.GenerateReflection("test-session", "600519 怎么样", answer, 100, llm, "GetStockQuote")
			if err != nil {
				t.Fatalf("GenerateReflection should not fail on LLM error: %v", err)
			}
			want := e.ScoreResultStructured(answer)
			if got.Score != want.Score || got.Accuracy != want.Accuracy ||
				got.Relevance != want.Relevance || got.ToolUse != want.ToolUse ||
				got.Completeness != want.Completeness {
				t.Errorf("LLM failure should fall back to deterministic score %+v, got %+v", want, got)
			}
		})
	}
}
