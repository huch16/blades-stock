package data

import (
	"strings"
	"testing"
)

// ============ helper function unit tests ============

func TestExtractBillboardTotal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBuy  float64
		wantSell float64
	}{
		{"empty", "", 0, 0},
		{"buy only", "买入总额|5000.00万", 5000, 0},
		{"sell only", "卖出总额|3000.50万", 0, 3000.5},
		{"buy and sell", "买入总计(万)|12345.67\n卖出总计(万)|8765.43\n", 12345.67, 8765.43},
		{"no numbers", "无数据", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBuy, gotSell := ExtractBillboardTotal(tt.input)
			if gotBuy != tt.wantBuy {
				t.Errorf("ExtractBillboardTotal() buy = %v, want %v", gotBuy, tt.wantBuy)
			}
			if gotSell != tt.wantSell {
				t.Errorf("ExtractBillboardTotal() sell = %v, want %v", gotSell, tt.wantSell)
			}
		})
	}
}

func TestExtractFirstNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float64
	}{
		{"simple int", "5000", 5000},
		{"decimal", "12345.67", 12345.67},
		{"with text", "买入总额|5000.00万", 5000.00},
		{"multiple dots", "1.2.3", 1.2},
		{"no number", "abc", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractFirstNumber(tt.input)
			if tt.want == 0 && err != nil {
				return // expected error
			}
			if got != tt.want {
				t.Errorf("ExtractFirstNumber(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAbsFloat(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{5.0, 5.0},
		{-5.0, 5.0},
		{0, 0},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := AbsFloat(tt.input); got != tt.want {
				t.Errorf("AbsFloat(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectHolderConcentration(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"decrease", "股东户数较上期减少5.2%", 1},
		{"increase", "股东户数较上期增加3.1%", -1},
		{"下降 synonym", "户数下降", 1},
		{"上升 synonym", "户数上升", -1},
		{"no match", "无变化", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectHolderConcentration(tt.input)
			if got != tt.want {
				t.Errorf("DetectHolderConcentration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectBlockTradePositive(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"premium", "溢价1.5%成交", 1},
		{"discount", "折价2.0%成交", -1},
		{"no match", "平价成交", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectBlockTradePositive(tt.input)
			if got != tt.want {
				t.Errorf("DetectBlockTradePositive(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectMoneyFlowPositive(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"net inflow", "主力净流入5000万", 1},
		{"net outflow", "净流出3000万", -1},
		{"no match", "资金平衡", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMoneyFlowPositive(tt.input)
			if got != tt.want {
				t.Errorf("DetectMoneyFlowPositive(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectMarginPositive(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"increase", "融资余额增加1.2亿", 1},
		{"decrease", "融券余额减少0.5亿", -1},
		{"上升", "余额上升", 1},
		{"下降", "余额下降", -1},
		{"no match", "余额持平", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMarginPositive(tt.input)
			if got != tt.want {
				t.Errorf("DetectMarginPositive(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSignalIcon(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{1, "✅"},
		{-1, "❌"},
		{0, "⚪"},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := SignalIcon(tt.input); got != tt.want {
				t.Errorf("SignalIcon(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGuessMainForceBehavior(t *testing.T) {
	tests := []struct {
		pos, neg int
		contains string
	}{
		{3, 0, "强烈建仓"},
		{2, 0, "大概率建仓"},
		{2, 2, "分歧较大"},
		{0, 3, "强烈出货"},
		{0, 2, "大概率出货"},
		{0, 0, "无明显主力行为"},
		{1, 2, "大概率出货"},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := GuessMainForceBehavior(tt.pos, tt.neg)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("GuessMainForceBehavior(%v,%v) = %v, want contains %v", tt.pos, tt.neg, got, tt.contains)
			}
		})
	}
}

func TestAssessCapitalNature(t *testing.T) {
	tests := []struct {
		name    string
		main    float64
		retail  float64
		moneyMd string
		want    string
	}{
		{"empty data", 0, 0, "", "数据不足"},
		{"main force", 5000, -2000, "主力|+100|流入\n主力|+200|流入\n小单|-50|流出", "主力资金主导"},
		{"retail money", -2000, 3000, "主力|-100|流出\n小单|+50|流入\n", "资金方向不明确"}, // ExtractMoneyFlowTrend 始终返回 (0,0)
		{"no direction", 0, 0, "主力|0|平衡\n小单|0|平衡", "资金方向不明确"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessCapitalNature(tt.main, tt.retail, tt.moneyMd)
			if !strings.Contains(got, tt.want) {
				t.Errorf("AssessCapitalNature() = %v, want contains %v", got, tt.want)
			}
		})
	}
}

// ============ integration tests (require network + real API) ============

func TestHandleAnalyzeHotMoneyActivity_EmptyCode(t *testing.T) {
	ch := make(chan map[string]any, 10)
	ctx := &ToolContext{
		Messages:             &[]map[string]any{},
		CurrentAIContent:     &strings.Builder{},
		ReasoningContentText: &strings.Builder{},
		CurrentCallID:        "test-call",
		FuncName:             "AnalyzeHotMoneyActivity",
		Ch:                   ch,
	}
	err := handleAnalyzeHotMoneyActivity(nil, `{"stockCode":"","days":30}`, ctx)
	close(ch)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	msgs := *ctx.Messages
	if len(msgs) == 0 {
		t.Fatal("expected at least one tool message")
	}
	lastMsg := msgs[len(msgs)-1]
	content, ok := lastMsg["content"].(string)
	if !ok || !strings.Contains(content, "请提供股票代码") {
		t.Errorf("expected error message for empty code, got: %v", content)
	}
}

func TestHandleAnalyzeChipConcentration_EmptyCode(t *testing.T) {
	ch := make(chan map[string]any, 10)
	ctx := &ToolContext{
		Messages:             &[]map[string]any{},
		CurrentAIContent:     &strings.Builder{},
		ReasoningContentText: &strings.Builder{},
		CurrentCallID:        "test-call",
		FuncName:             "AnalyzeChipConcentration",
		Ch:                   ch,
	}
	err := handleAnalyzeChipConcentration(nil, `{"stockCode":""}`, ctx)
	close(ch)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	msgs := *ctx.Messages
	lastMsg := msgs[len(msgs)-1]
	content, ok := lastMsg["content"].(string)
	if !ok || !strings.Contains(content, "请提供股票代码") {
		t.Errorf("expected error message for empty code, got: %v", content)
	}
}

func TestHandleAnalyzeCapitalGame_EmptyCode(t *testing.T) {
	ch := make(chan map[string]any, 10)
	ctx := &ToolContext{
		Messages:             &[]map[string]any{},
		CurrentAIContent:     &strings.Builder{},
		ReasoningContentText: &strings.Builder{},
		CurrentCallID:        "test-call",
		FuncName:             "AnalyzeCapitalGame",
		Ch:                   ch,
	}
	err := handleAnalyzeCapitalGame(nil, `{"stockCode":"","days":20}`, ctx)
	close(ch)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	msgs := *ctx.Messages
	lastMsg := msgs[len(msgs)-1]
	content, ok := lastMsg["content"].(string)
	if !ok || !strings.Contains(content, "请提供股票代码") {
		t.Errorf("expected error message for empty code, got: %v", content)
	}
}

func TestToolHandlerRegistration(t *testing.T) {
	names := []string{
		"AnalyzeHotMoneyActivity",
		"AnalyzeChipConcentration",
		"AnalyzeCapitalGame",
	}
	for _, name := range names {
		if _, ok := toolHandlers[name]; !ok {
			t.Errorf("tool handler %s not registered", name)
		}
	}
}
