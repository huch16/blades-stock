package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-kratos/blades"
)

// StreamMessage 补 tool_calls 字段（B 组任务二）回归测试。
//
// 背景：后端 StreamMessage 根本没有 tool_calls 字段，前端 agent-chat.vue
// 的 data.tool_calls 分支与整个 ToolTimeline 因此永远收不到数据，
// 工具调用只以 [STEP] 文本出现在思考区。
//
// 断言口径以「前端实际消费形状」为准（frontend/src/components/chat/types.ts
// 的 ToolCall）：tool_calls[].function.name/arguments、status、
// result、id、duration；序列化走 encoding/json，与 AgentMessageToFrontendMap
//（services 层对 StreamMessage 的 Marshal/Unmarshal 透传）同一机制。
//
// 注意：本文件只覆盖“转换 + 序列化”——端到端（mock 模型 → 工具执行 →
// 流里出现 tool_calls）见 stream_tool_calls_e2e_test.go，
// 接缝在 run_context.go WithMockAgent 与 mock_model_provider.go
// ToolCallStreamProvider。

// frontendToolCall 是前端 ToolCall 的 Go 侧镜像，只用于校验 JSON 形状兼容。
type frontendToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
	Status   string `json:"status"`
	Duration int64  `json:"duration"`
	Result   string `json:"result"`
}

func TestBuildStreamToolCallSuccess(t *testing.T) {
	tp := &blades.ToolPart{
		ID:       "call_001",
		Name:     "get_stock_data",
		Request:  `{"code":"000001"}`,
		Response: `{"price":12.22,"change":1.5}`,
	}
	got := buildStreamToolCall(tp)
	if got.ID != "call_001" {
		t.Errorf("ID 应原样透传，实际 %q", got.ID)
	}
	if got.Function.Name != "get_stock_data" {
		t.Errorf("function.name 错误，实际 %q", got.Function.Name)
	}
	if got.Function.Arguments != `{"code":"000001"}` {
		t.Errorf("function.arguments 错误，实际 %q", got.Function.Arguments)
	}
	if got.Status != "success" {
		t.Errorf("正常结果应为 success，实际 %q", got.Status)
	}
	if !strings.Contains(got.Result, "12.22") {
		t.Errorf("result 应保留结果摘要，实际 %q", got.Result)
	}
}

func TestBuildStreamToolCallError(t *testing.T) {
	// 含 error / 失败 / 异常等关键词的结果与 toolCallSucceeded 的判定一致，判 error。
	for _, resp := range []string{
		`{"error":"timeout"}`,
		"查询失败：接口返回空",
		"服务异常，请重试",
	} {
		got := buildStreamToolCall(&blades.ToolPart{Name: "search_web", Request: "{}", Response: resp})
		if got.Status != "error" {
			t.Errorf("结果 %q 应判 error，实际 %q", resp, got.Status)
		}
	}
}

func TestBuildStreamToolCallEmptyResponseIsError(t *testing.T) {
	// 空结果 = 工具没产出，与 toolCallSucceeded("")==false 一致，判 error，
	// 而不是把一条“成功但无结果”的调用画进时间线。
	got := buildStreamToolCall(&blades.ToolPart{Name: "get_kline", Request: "{}", Response: ""})
	if got.Status != "error" {
		t.Errorf("空结果应判 error，实际 %q", got.Status)
	}
	if got.Result != "" {
		t.Errorf("空结果的 result 应为空，实际 %q", got.Result)
	}
}

func TestBuildStreamToolCallTruncatesLongResult(t *testing.T) {
	long := strings.Repeat("数", streamToolCallResultMaxRunes+500)
	got := buildStreamToolCall(&blades.ToolPart{Name: "search_news", Request: "{}", Response: long})
	if len([]rune(got.Result)) > streamToolCallResultMaxRunes+20 {
		t.Fatalf("长结果未截断：%d runes（上限 %d）", len([]rune(got.Result)), streamToolCallResultMaxRunes)
	}
	if !strings.HasSuffix(got.Result, "…（结果过长，已截断）") {
		t.Errorf("截断后应带截断标记，实际尾部 %q", got.Result[len(got.Result)-30:])
	}
	if got.Status != "success" {
		t.Errorf("长结果本身无错误信号，不应判 error，实际 %q", got.Status)
	}
}

func TestBuildStreamToolCallSanitizesSecrets(t *testing.T) {
	// 参数/结果里的密钥必须脱敏后才进流，与 [STEP] 文本同一口径。
	got := buildStreamToolCall(&blades.ToolPart{
		Name:     "search_web",
		Request:  `{"api_key":"sk-abcdefghijklmnopqrstuv"}`,
		Response: `ok, auth=Bearer abcdefghijklmnopqrstuvwx`,
	})
	if strings.Contains(got.Function.Arguments, "sk-abcdefghijklmnopqrstuv") {
		t.Errorf("arguments 里的密钥未脱敏: %q", got.Function.Arguments)
	}
	if strings.Contains(got.Result, "abcdefghijklmnopqrstuvwx") {
		t.Errorf("result 里的令牌未脱敏: %q", got.Result)
	}
}

// TestStreamMessageToolCallsJSONShape 断言：一次带工具调用的流，
// StreamMessage 序列化后 tool_calls 字段出现且形状与前端 ToolCall 一致。
func TestStreamMessageToolCallsJSONShape(t *testing.T) {
	msg := &StreamMessage{
		Role: "assistant",
		ToolCalls: []StreamToolCall{buildStreamToolCall(&blades.ToolPart{
			ID:       "call_abc",
			Name:     "get_stock_data",
			Request:  `{"code":"600519"}`,
			Response: `{"price":1500.00}`,
		})},
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("StreamMessage 序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("序列化结果回读失败: %v", err)
	}
	rawCalls, ok := m["tool_calls"]
	if !ok {
		t.Fatalf("序列化后缺少 tool_calls 字段，实际 keys: %v\n原始 JSON: %s", keysOf(m), raw)
	}
	calls, ok := rawCalls.([]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("tool_calls 应为 1 条数组，实际: %#v", rawCalls)
	}
	// 用前端 ToolCall 的镜像结构回读：字段名/类型必须一一对上，
	// 否则 agent-chat.vue 的 targetMsg.toolCalls.push(...data.tool_calls)
	// 拿到的是缺字段的对象，ToolTimeline 渲染不出名字/参数/状态。
	var decoded struct {
		ToolCalls []frontendToolCall `json:"tool_calls"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("按前端 ToolCall 形状回读失败: %v\n原始 JSON: %s", err, raw)
	}
	c := decoded.ToolCalls[0]
	if c.ID != "call_abc" || c.Function.Name != "get_stock_data" ||
		c.Function.Arguments != `{"code":"600519"}` ||
		c.Status != "success" || !strings.Contains(c.Result, "1500") {
		t.Errorf("前端形状回读内容错误: %+v", c)
	}
}

// TestStreamMessageWithoutToolCallsOmitsField 断言：无工具调用的普通流，
// tool_calls 字段必须缺席（omitempty），前端分支不触发，不回归。
func TestStreamMessageWithoutToolCallsOmitsField(t *testing.T) {
	for _, msg := range []*StreamMessage{
		{Role: "assistant", Content: "普通正文"},
		{Role: "assistant", Content: "agent-DONE", FinishReason: "stop"},
		{Role: "assistant", ReasoningContent: "[STATUS]正在思考..."},
	} {
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if _, ok := m["tool_calls"]; ok {
			t.Errorf("无工具调用的消息不应带 tool_calls 字段，实际 JSON: %s", raw)
		}
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
