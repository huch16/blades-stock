//go:build integration

// Integration tests that drive the full agent loop and require real AI
// provider credentials. Run with: go test -tags=integration ./backend/agent/...

package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/duke-git/lancet/v2/fileutil"
	"github.com/go-kratos/blades"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

func TestGetStockAiAgent(t *testing.T) {
	ctx := context.Background()
	config := data.GetSettingConfig()
	if len(config.AiConfigs) == 0 {
		t.Skip("no AI config in DB; skipping live agent run")
	}
	agentInstance := GetStockAiAgent(ctx, *config.AiConfigs[0], "分析当前市场情绪和热点", "")

	if agentInstance == nil {
		t.Fatal("agent instance is nil")
	}

	t.Logf("Agent mode: %s", agentInstance.Mode)

	switch agentInstance.Mode {
	case AgentModePlanExecute:
		sysPrompt := config.Settings.Prompt
		invocation := &blades.Invocation{
			Message: blades.UserMessage(`结合以上提供的宏观经济数据/市场指数行情/国内外市场资讯/电报/会议/事件/投资者关注的问题，
结合宏观经济，事件驱动，政策支持，投资者关注的问题，分析当前市场情绪和热点 找出有潜力/优质的板块/行业/概念/标的/主题，
多因子深度分析计算上涨或下跌的逻辑和概率，
最后按风险和投资周期给出具体推荐标的操作建议`),
			Instruction: blades.SystemMessage(sysPrompt),
			Streamable:  true,
		}
		var md strings.Builder
		for msg, err := range agentInstance.Agent.Run(ctx, invocation) {
			if err != nil {
				t.Logf("agent error: %v", err)
				break
			}
			if msg == nil {
				continue
			}
			if msg.Role == blades.RoleAssistant {
				if text := msg.Text(); text != "" {
					md.WriteString(text)
				}
				if rc, ok := msg.Metadata["reasoning_content"].(string); ok && rc != "" {
					md.WriteString(rc)
				}
			}
			for _, part := range msg.Parts {
				if tp, ok := part.(*blades.ToolPart); ok {
					t.Logf("[ToolCall] %s(%s)", tp.Name, tp.Request)
				}
			}
		}
		logger.SugaredLogger.Info(md.String())

	default:
		var md strings.Builder
		ch := NewStockAiAgentApi().Chat("分析一下立讯精密", 2, nil)
		for message := range ch {
			logger.SugaredLogger.Infof("res:%s", message.Content)
			md.WriteString(message.Content)
		}
		logger.SugaredLogger.Info(md.String())
	}
}

func TestAgent(t *testing.T) {
	var md strings.Builder
	ch := NewStockAiAgentApi().Chat("分析一下立讯精密", 2, nil)
	for message := range ch {
		logger.SugaredLogger.Infof("res:%s", message.Content)
		md.WriteString(message.Content)
	}
	logger.SugaredLogger.Info(md.String())
	if err := fileutil.WriteStringToFile("../../data/result.md", md.String(), false); err != nil {
		t.Fatalf("write result: %v", err)
	}
}
