package agent

import (
	"context"

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
