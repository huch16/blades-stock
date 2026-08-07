package strategy

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEmbedded(t *testing.T) {
	list, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if len(list) < 5 {
		t.Fatalf("内嵌策略数量异常: %d", len(list))
	}
	for _, s := range list {
		if strings.TrimSpace(s.Name) == "" {
			t.Fatalf("策略缺少名称: %+v", s)
		}
		if s.BaseScore < 0 {
			t.Fatalf("策略 %s 基础分为负: %v", s.Name, s.BaseScore)
		}
	}
}

func TestEmbeddedConditionsKnown(t *testing.T) {
	list, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	for _, s := range list {
		for _, r := range append(append([]Rule{}, s.Bonuses...), s.Penalties...) {
			name, _, err := ParseCondition(r.When)
			if err != nil {
				t.Fatalf("策略 %s 条件 %q 解析失败: %v", s.Name, r.When, err)
			}
			if !IsKnownCondition(name) {
				t.Fatalf("策略 %s 引用了未注册条件: %q", s.Name, r.When)
			}
		}
	}
}

func TestResolve(t *testing.T) {
	list, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	cases := []struct {
		input string
		want  string
		ok    bool
	}{
		{"趋势跟随", "趋势跟随", true},
		{"macd_golden_cross", "MACD金叉", true},
		{"macd金叉", "MACD金叉", true},
		{"放量", "放量突破", true},
		{"不存在的策略xyz", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := Resolve(list, c.input)
		if ok != c.ok {
			t.Fatalf("Resolve(%q) ok = %v, want %v", c.input, ok, c.ok)
		}
		if ok && got.Name != c.want {
			t.Fatalf("Resolve(%q) = %q, want %q", c.input, got.Name, c.want)
		}
	}
}

func TestLoadFromDirMissing(t *testing.T) {
	list, err := LoadFromDir(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("LoadFromDir 目录不存在应返回空: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("预期空列表, got %d", len(list))
	}
}

func TestParseYAML(t *testing.T) {
	data := []byte(`
name: 测试策略
alias:
  - test
base_score: 40
bonuses:
  - when: "above_ma(5)"
    score: 10
    desc: "站上5日线"
penalties:
  - when: "rsi_lt(30)"
    score: 20
    desc: "超卖"
`)
	s, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Name != "测试策略" || s.BaseScore != 40 || len(s.Bonuses) != 1 || len(s.Penalties) != 1 {
		t.Fatalf("解析结果异常: %+v", s)
	}
}

func TestParseMissingName(t *testing.T) {
	if _, err := Parse([]byte("base_score: 50\n")); err == nil {
		t.Fatal("缺少 name 应报错")
	}
}
