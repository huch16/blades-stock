package agent

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// =============================================================================
// EncodeCount 基础测试
// =============================================================================

func TestEncodeCount_Empty(t *testing.T) {
	if got := EncodeCount(""); got != 0 {
		t.Errorf("empty string should be 0 tokens, got %d", got)
	}
}

func TestEncodeCount_English(t *testing.T) {
	// 经典 BPE 行为："Hello world" ≈ 2 tokens
	got := EncodeCount("Hello world")
	if got < 1 || got > 4 {
		t.Errorf("'Hello world' expected 1-4 tokens, got %d", got)
	}
}

func TestEncodeCount_Chinese(t *testing.T) {
	// 中文 "你好世界" 在 cl100k_base 下通常 4 tokens（一字约 1-2 tokens）
	got := EncodeCount("你好世界")
	if got < 2 || got > 8 {
		t.Errorf("'你好世界' expected 2-8 tokens, got %d", got)
	}
}

func TestEncodeCount_Numbers(t *testing.T) {
	// "12345" 在 BPE 下通常 1 token
	got := EncodeCount("12345")
	if got < 1 || got > 3 {
		t.Errorf("'12345' expected 1-3 tokens, got %d", got)
	}
}

func TestEncodeCount_Code(t *testing.T) {
	// 代码片段应得到合理的 token 数
	code := `func main() { fmt.Println("hello") }`
	got := EncodeCount(code)
	if got < 5 || got > 30 {
		t.Errorf("code snippet expected 5-30 tokens, got %d", got)
	}
}

func TestEncodeCount_Mixed(t *testing.T) {
	mixed := "分析 159915 这只 ETF 的 21 日加权动量排名"
	got := EncodeCount(mixed)
	if got < 5 || got > 40 {
		t.Errorf("mixed content expected 5-40 tokens, got %d", got)
	}
}

// =============================================================================
// EncodeTrimToTokens 测试
// =============================================================================

func TestEncodeTrimToTokens_NoTruncation(t *testing.T) {
	text := "Hello world"
	got := EncodeTrimToTokens(text, 100)
	if got != text {
		t.Errorf("short text should be unchanged: got %q", got)
	}
}

func TestEncodeTrimToTokens_TruncateEnglish(t *testing.T) {
	text := "This is a long sentence with many words that should definitely need truncation."
	trimmed := EncodeTrimToTokens(text, 5)
	if trimmed == text {
		t.Error("expected truncation")
	}
	if got := EncodeCount(trimmed); got > 5 {
		t.Errorf("trimmed text has %d tokens, exceeds budget 5", got)
	}
}

func TestEncodeTrimToTokens_TruncateChinese(t *testing.T) {
	text := "今天是个好日子，股市大涨，蓝筹股普涨，中小盘活跃，资金面宽松，技术面良好"
	trimmed := EncodeTrimToTokens(text, 10)
	if trimmed == text {
		t.Error("expected truncation")
	}
	if got := EncodeCount(trimmed); got > 10 {
		t.Errorf("trimmed chinese text has %d tokens, exceeds budget 10", got)
	}
}

func TestEncodeTrimToTokens_UTF8Safe(t *testing.T) {
	// 中文里每个字符都是 3 bytes 的 UTF-8，不能从中间切
	text := "中文内容绝对不能切到字符中间"
	trimmed := EncodeTrimToTokens(text, 3)
	if !utf8.ValidString(trimmed) {
		t.Errorf("trimmed text contains invalid UTF-8: %q", trimmed)
	}
}

func TestEncodeTrimToTokens_ZeroBudget(t *testing.T) {
	if got := EncodeTrimToTokens("hello", 0); got != "" {
		t.Errorf("zero budget should return empty, got %q", got)
	}
	if got := EncodeTrimToTokens("hello", -1); got != "" {
		t.Errorf("negative budget should return empty, got %q", got)
	}
}

// =============================================================================
// EncodeLinesSplit / EncodeLinesKeepTail 测试
// =============================================================================

func TestEncodeLinesSplit_AllFit(t *testing.T) {
	lines := []string{"a", "b", "c"}
	kept, omitted := EncodeLinesSplit(lines, 100)
	if omitted != 0 {
		t.Errorf("expected no omission, got %d", omitted)
	}
	if len(kept) != 3 {
		t.Errorf("expected all 3 lines, got %d", len(kept))
	}
}

func TestEncodeLinesSplit_Truncate(t *testing.T) {
	// 构造多行，超出预算
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "这是一行测试内容用于填充 token 预算检查"
	}
	kept, omitted := EncodeLinesSplit(lines, 10)
	if len(kept) >= len(lines) {
		t.Error("expected truncation")
	}
	if omitted == 0 {
		t.Error("expected omitted > 0")
	}
	if len(kept)+omitted != len(lines) {
		t.Errorf("kept(%d) + omitted(%d) should equal total(%d)", len(kept), omitted, len(lines))
	}
}

func TestEncodeLinesKeepTail_PreservesLast(t *testing.T) {
	lines := []string{"oldest", "middle1", "middle2", "recent"}
	kept, _ := EncodeLinesKeepTail(lines, 100)
	if len(kept) != 4 {
		t.Errorf("expected all 4 lines, got %d", len(kept))
	}
	if kept[len(kept)-1] != "recent" {
		t.Errorf("last line should be 'recent', got %q", kept[len(kept)-1])
	}
}

func TestEncodeLinesKeepTail_TruncatesOldest(t *testing.T) {
	// 大量 lines，预算只够保留尾部的几条
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line content with some chinese mixed"
	}
	kept, omitted := EncodeLinesKeepTail(lines, 15)
	if len(kept) >= 30 {
		t.Error("expected truncation")
	}
	if omitted == 0 {
		t.Error("expected omitted > 0")
	}
	// 验证尾部被完整保留（不是从头开始截）
	if len(kept) > 0 && kept[len(kept)-1] != lines[len(lines)-1] {
		t.Error("last line should be preserved")
	}
}

// =============================================================================
// 与旧 estimateTokens 对比
// =============================================================================

func TestEncodeCount_VsFallback_EnglishSimilar(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	realTokens := EncodeCount(text)
	heuristicTokens := estimateTokens(text) // 旧启发式
	// 差异不超过 2 倍即可接受
	if heuristicTokens == 0 || float64(realTokens)/float64(heuristicTokens) > 2.0 ||
		float64(heuristicTokens)/float64(realTokens) > 2.0 {
		t.Errorf("token estimates diverge too much: real=%d heuristic=%d", realTokens, heuristicTokens)
	}
}

func TestEncodeCount_VsFallback_ChineseSimilar(t *testing.T) {
	text := "股票市场分析与投资策略"
	realTokens := EncodeCount(text)
	heuristicTokens := estimateTokens(text)
	// 中文差异可能更大，但应在 3 倍内
	if heuristicTokens == 0 || float64(realTokens)/float64(heuristicTokens) > 3.0 ||
		float64(heuristicTokens)/float64(realTokens) > 3.0 {
		t.Errorf("chinese token estimates diverge: real=%d heuristic=%d", realTokens, heuristicTokens)
	}
}

// =============================================================================
// joinLinesAndOmitted
// =============================================================================

func TestJoinLinesAndOmitted_NoOmit(t *testing.T) {
	got := joinLinesAndOmitted([]string{"a", "b", "c"}, 0)
	if got != "a\nb\nc" {
		t.Errorf("got %q, want %q", got, "a\\nb\\nc")
	}
}

func TestJoinLinesAndOmitted_WithOmit(t *testing.T) {
	got := joinLinesAndOmitted([]string{"a", "b"}, 5)
	if !strings.Contains(got, "已省略") {
		t.Errorf("expected omit marker, got %q", got)
	}
	if !strings.Contains(got, "5 行") {
		t.Errorf("expected '5 行' in marker, got %q", got)
	}
}
