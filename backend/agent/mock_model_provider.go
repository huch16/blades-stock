package agent

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/go-kratos/blades"
)

// MockModelProvider 是一个用于测试的模型提供者，返回固定的响应，不依赖外部 API。
type MockModelProvider struct {
	name     string
	response string
}

// NewMockModelProvider 创建一个新的 MockModelProvider。
func NewMockModelProvider(name, response string) *MockModelProvider {
	return &MockModelProvider{
		name:     name,
		response: response,
	}
}

// Name 返回模型提供者的名称。
func (m *MockModelProvider) Name() string {
	return m.name
}

// Generate 返回固定的模型响应。
func (m *MockModelProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	// Create a blades.Message with a TextPart containing our response text.
	msg := &blades.Message{
		Role: blades.RoleAssistant,
		Parts: []blades.Part{
			blades.TextPart{Text: m.response},
		},
	}
	return &blades.ModelResponse{
		Message: msg,
	}, nil
}

// NewStreaming 返回一个固定的流式生成器，只产生一次完整响应。
func (m *MockModelProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		// Create a blades.Message with a TextPart containing our response text.
		msg := &blades.Message{
			Role: blades.RoleAssistant,
			Parts: []blades.Part{
				blades.TextPart{Text: m.response},
			},
		}
		resp := &blades.ModelResponse{
			Message: msg,
		}
		// Yield the response once.
		if !yield(resp, nil) {
			return
		}
	}
}

// ── 脚本化流式 Mock（fallback 降级链回归测试用）─────────────────────────
//
// 真实网关的流远比“一次完整响应”复杂：首包 role chunk、reasoning-only
// chunk 都是零 part 的空消息；流可能在任意位置被重置；也可能静默挂起。
// ScriptedStreamProvider 按步骤精确复现这些形态，且不碰网络/真实 AI。

// MockStreamStepKind 是流式脚本中一个步骤的类型。
type MockStreamStepKind string

const (
	// MockStreamEmpty 空消息（零 part）：模拟网关首包 role chunk，或 blades
	// 的 OpenAI 适配层把 reasoning_content delta 转成的零 part 消息。
	MockStreamEmpty MockStreamStepKind = "empty"
	// MockStreamText 正文 TextPart。
	MockStreamText MockStreamStepKind = "text"
	// MockStreamThink 包在 <think>...</think> 内的思考文本（以 TextPart 形式
	// 到达，如 MiniMax-M3 风格；上层 thinkFilter 会实时剥离）。
	MockStreamThink MockStreamStepKind = "think"
	// MockStreamTool 工具调用 ToolPart。
	MockStreamTool MockStreamStepKind = "tool"
	// MockStreamError 流错误：yield (nil, err) 后停止，如 HTTP/2 流重置。
	MockStreamError MockStreamStepKind = "error"
)

// MockStreamStep 是流式脚本中的一个步骤。
type MockStreamStep struct {
	Kind MockStreamStepKind
	Text string
	Name string // 工具调用名（Kind 为 tool 时）
	Err  error  // 流错误（Kind 为 error 时）
}

// ScriptedStreamProvider 按给定步骤顺序产出流式事件的 Mock provider。
type ScriptedStreamProvider struct {
	name  string
	steps []MockStreamStep
	calls int32
}

// NewScriptedStreamProvider 创建一个脚本化流式 Mock provider。
func NewScriptedStreamProvider(name string, steps []MockStreamStep) *ScriptedStreamProvider {
	return &ScriptedStreamProvider{name: name, steps: steps}
}

// Name 返回模型提供者的名称。
func (m *ScriptedStreamProvider) Name() string { return m.name }

// Calls 返回流式/非流式调用的总次数。
func (m *ScriptedStreamProvider) Calls() int32 { return atomic.LoadInt32(&m.calls) }

// Generate 返回各步骤正文拼接后的固定响应（不模拟流错误；流式场景请用 NewStreaming）。
func (m *ScriptedStreamProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	atomic.AddInt32(&m.calls, 1)
	var sb strings.Builder
	for _, st := range m.steps {
		if st.Kind == MockStreamText || st.Kind == MockStreamThink {
			sb.WriteString(st.Text)
		}
		if st.Kind == MockStreamError && st.Err != nil {
			return nil, st.Err
		}
	}
	msg := &blades.Message{
		Role:  blades.RoleAssistant,
		Parts: []blades.Part{blades.TextPart{Text: sb.String()}},
	}
	return &blades.ModelResponse{Message: msg}, nil
}

// NewStreaming 按步骤顺序 yield 流式事件。
func (m *ScriptedStreamProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		atomic.AddInt32(&m.calls, 1)
		for _, st := range m.steps {
			switch st.Kind {
			case MockStreamError:
				yield(nil, st.Err)
				return
			case MockStreamEmpty:
				msg := &blades.Message{Role: blades.RoleAssistant}
				if !yield(&blades.ModelResponse{Message: msg}, nil) {
					return
				}
			case MockStreamText:
				msg := &blades.Message{
					Role:  blades.RoleAssistant,
					Parts: []blades.Part{blades.TextPart{Text: st.Text}},
				}
				if !yield(&blades.ModelResponse{Message: msg}, nil) {
					return
				}
			case MockStreamThink:
				msg := &blades.Message{
					Role:  blades.RoleAssistant,
					Parts: []blades.Part{blades.TextPart{Text: "<think>" + st.Text + "</think>"}},
				}
				if !yield(&blades.ModelResponse{Message: msg}, nil) {
					return
				}
			case MockStreamTool:
				msg := &blades.Message{
					Role:  blades.RoleTool,
					Parts: []blades.Part{blades.ToolPart{Name: st.Name, Request: "{}", Response: "ok"}},
				}
				if !yield(&blades.ModelResponse{Message: msg}, nil) {
					return
				}
			}
		}
	}
}

// StallStreamProvider 模拟“流静默挂起”：连接不断、一个 chunk 都不发，
// 直到 ctx 被取消才以 ctx 错误结束。用于验证停滞看门狗能否主动取消并降级。
type StallStreamProvider struct {
	name  string
	calls int32
}

// NewStallStreamProvider 创建一个静默挂起的 Mock provider。
func NewStallStreamProvider(name string) *StallStreamProvider {
	return &StallStreamProvider{name: name}
}

// Name 返回模型提供者的名称。
func (m *StallStreamProvider) Name() string { return m.name }

// Calls 返回调用次数。
func (m *StallStreamProvider) Calls() int32 { return atomic.LoadInt32(&m.calls) }

// Generate 直接等待 ctx 取消（与流式行为一致：挂起直到超时/取消）。
func (m *StallStreamProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	atomic.AddInt32(&m.calls, 1)
	<-ctx.Done()
	return nil, ctx.Err()
}

// NewStreaming 等待 ctx 被取消后 yield ctx 错误。
func (m *StallStreamProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		atomic.AddInt32(&m.calls, 1)
		<-ctx.Done()
		yield(nil, ctx.Err())
	}
}
