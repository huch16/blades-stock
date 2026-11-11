package data

import (
	"errors"
	"testing"
)

func TestIsDegenerateGarbage(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"normal chinese", "这是一段正常的股票分析回答，包含 PE、ROE 等指标。", false},
		{"normal with symbols", "买入区间 45-50 元，目标价 60 元。", false},
		{"single unk", "Now إن<unk>", true},
		{"repeated unk", "<unk><unk><unk><unk><unk>", true},
		{"mixed garbage", "Now إن<unk><unk> 持仓分析", true},
		{"case sensitive ok", "This is fine with <UNK> capital", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsDegenerateGarbage(c.in); got != c.want {
				t.Fatalf("IsDegenerateGarbage(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestDegenerateOutputError(t *testing.T) {
	e := DegenerateOutputError{}
	if e.Error() == "" {
		t.Fatal("DegenerateOutputError.Error() should not be empty")
	}
}

// TestEmptyOutputError 锁定「静默空答」的三种归因文案。
//
// 这是回归防线：旧实现把「正文为空」当成成功（ok=true/finish=stop），
// 前端只能渲染无信息量的「（空回复）」。这些文案是用户唯一的线索来源，
// 不能退化成同一句话。
func TestEmptyOutputError(t *testing.T) {
	cases := []struct {
		name string
		err  EmptyOutputError
		want string
	}{
		{
			name: "thinking only",
			err:  EmptyOutputError{ProcessChars: 1200, FirstChunkSeen: true},
			want: "模型未产出正文：只输出了思考/工具过程",
		},
		{
			name: "no response at all",
			err:  EmptyOutputError{ProcessChars: 0, FirstChunkSeen: false},
			want: "模型未产出正文：上游未返回任何响应内容",
		},
		{
			name: "responded but empty body",
			err:  EmptyOutputError{ProcessChars: 0, FirstChunkSeen: true},
			want: "模型未产出正文（空响应）",
		},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Fatalf("Error() = %q, want %q", got, c.want)
			}
			// 三种情况必须是**不同**的文案，否则用户无法区分原因。
			if seen[c.want] {
				t.Fatalf("重复文案 %q：三种空答归因必须互相可区分", c.want)
			}
			seen[c.want] = true
		})
	}
}

// TestIsEmptyOutput 锁定类型判定：必须只认 EmptyOutputError，
// 不能把 DegenerateOutputError（乱码）误判为空答 —— 两者文案与处置不同。
func TestIsEmptyOutput(t *testing.T) {
	if !IsEmptyOutput(EmptyOutputError{}) {
		t.Fatal("EmptyOutputError 应被 IsEmptyOutput 判定为 true")
	}
	if IsEmptyOutput(DegenerateOutputError{}) {
		t.Fatal("DegenerateOutputError（乱码）不应被判定为空答")
	}
	if IsEmptyOutput(nil) {
		t.Fatal("nil 不应被判定为空答")
	}
	if IsEmptyOutput(errors.New("some other error")) {
		t.Fatal("普通错误不应被判定为空答")
	}
}
