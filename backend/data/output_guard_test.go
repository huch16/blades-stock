package data

import "testing"

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
