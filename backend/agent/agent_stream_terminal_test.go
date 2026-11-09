package agent

import (
	"context"
	"testing"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
)

// 固化 v3.0.0 修复：Agent 流必须在**任何**返回路径上发送终态消息。
//
// 事故背景（第三方审计线索 + 源码核实）：
//
//	ChatWithContext 的 goroutine 里有 5 处 close(ch)，其中只有 1 处发送了
//	FinishReason；错误路径甚至写明 "don't send status/DONE signals — just close"。
//	而前端只在收到带 FinishReason 的终态消息时才 finalizeStream()。
//	后果：只要 AI 调用失败（限流 / 超时 / thinking 与工具调用不兼容 / 网关 429）
//	或提前返回，前端 isStreaming 就永远为真 ——
//	界面卡在「思考中…」、输入框持续禁用、停止按钮一直转，用户只能刷新页面。
//
// 本测试用「不可达的模型端点」强制走错误路径，并断言：
//  1. 流**一定会关闭**（看门狗超时即失败，而不是永久挂起）；
//  2. 最后一条消息**必须**带终态 FinishReason。

// unreachableConfig 写入一张指向本地未监听端口的模型卡片，返回其 ID。
func unreachableConfig(t *testing.T) uint {
	t.Helper()
	if err := db.Dao.Exec("DELETE FROM ai_config").Error; err != nil {
		t.Fatalf("清理 ai_config 失败: %v", err)
	}
	if err := db.Dao.Exec("DELETE FROM settings").Error; err != nil {
		t.Fatalf("清理 settings 失败: %v", err)
	}
	// 127.0.0.1:1 属于保留端口，必然连接被拒 → 强制走错误路径
	cfg := data.AIConfig{
		Name:      "不可达模型",
		BaseUrl:   "http://127.0.0.1:1/v1",
		ApiKey:    "sk-unreachable-test-key",
		ModelName: "no-such-model",
		MaxTokens: 512,
		TimeOut:   10,
	}
	cfg.SetDefaults()
	if err := db.Dao.Create(&cfg).Error; err != nil {
		t.Fatalf("写入 ai_config 失败: %v", err)
	}
	if err := db.Dao.Create(&data.Settings{
		OpenAiEnable: true,
		AiConfigID:   cfg.ID,
		Prompt:       "test",
	}).Error; err != nil {
		t.Fatalf("写入 settings 失败: %v", err)
	}
	return cfg.ID
}

// drainUntilClosed 收集流中的所有消息；若超时未关闭则判定为「卡死」。
func drainUntilClosed(t *testing.T, ch <-chan *StreamMessage, limit time.Duration) ([]*StreamMessage, bool) {
	t.Helper()
	var received []*StreamMessage
	deadline := time.After(limit)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return received, true
			}
			received = append(received, msg)
		case <-deadline:
			return received, false
		}
	}
}

// 契约一：模型端点不可达（真实错误路径）时，流必须关闭且最后一条消息带终态。
func TestChatWithContextErrorPathSendsTerminal(t *testing.T) {
	cfgID := unreachableConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 用 direct 模式规避工具编排，让请求尽快失败
	ch := NewStockAiAgentApi().ChatWithContext(ctx, "ping", int(cfgID), nil, false, 0, false, "direct")

	received, closed := drainUntilClosed(t, ch, 50*time.Second)
	if !closed {
		t.Fatalf("流在 50s 内未关闭 —— 后端没有发送终态就挂住了（前端会永久卡在“思考中”）。已收到 %d 条消息", len(received))
	}
	if len(received) == 0 {
		t.Fatal("未收到任何消息：错误场景下连错误正文都没给出")
	}
	last := received[len(received)-1]
	if last.FinishReason == "" {
		t.Fatalf("最后一条消息缺少终态 FinishReason —— 前端不会 finalizeStream()。"+
			"最后一条内容=%q", last.Content)
	}
	if last.FinishReason != "stop" {
		t.Fatalf("终态 FinishReason 应为 stop，实际=%q", last.FinishReason)
	}
}

// 契约二：AI 配置不存在（提前返回路径）时，同样必须发送终态。
// 这条路径以前是 `ch <- "❌ AI 配置不存在或无效"` 紧跟 `close(ch)`，没有终态。
func TestChatWithContextMissingConfigSendsTerminal(t *testing.T) {
	// 清空配置，强制走「无可用配置」分支
	if err := db.Dao.Exec("DELETE FROM ai_config").Error; err != nil {
		t.Fatalf("清理 ai_config 失败: %v", err)
	}
	if err := db.Dao.Exec("DELETE FROM settings").Error; err != nil {
		t.Fatalf("清理 settings 失败: %v", err)
	}
	if err := db.Dao.Create(&data.Settings{OpenAiEnable: true, Prompt: "test"}).Error; err != nil {
		t.Fatalf("写入 settings 失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := NewStockAiAgentApi().ChatWithContext(ctx, "ping", 99999, nil, false, 0, false, "direct")

	received, closed := drainUntilClosed(t, ch, 20*time.Second)
	if !closed {
		t.Fatalf("流未关闭（应为即时返回），已收到 %d 条", len(received))
	}
	if len(received) == 0 {
		t.Fatal("未收到任何消息：应至少给出「AI 配置不存在或无效」")
	}
	last := received[len(received)-1]
	if last.FinishReason != "stop" {
		t.Fatalf("无可用配置时也必须发送终态，实际最后一条: content=%q finish_reason=%q",
			last.Content, last.FinishReason)
	}
}

// 契约三：无论走哪条路径，终态消息**只允许出现一次**，
// 且出现终态之后通道必须立即关闭（不允许再发后续消息）。
func TestChatWithContextTerminalIsSentExactlyOnce(t *testing.T) {
	cfgID := unreachableConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ch := NewStockAiAgentApi().ChatWithContext(ctx, "ping", int(cfgID), nil, false, 0, false, "direct")
	received, closed := drainUntilClosed(t, ch, 50*time.Second)
	if !closed {
		t.Fatal("流未关闭")
	}

	terminalIdx := -1
	count := 0
	for i, m := range received {
		if m != nil && m.FinishReason != "" {
			count++
			if terminalIdx == -1 {
				terminalIdx = i
			}
		}
	}
	if count != 1 {
		t.Fatalf("终态消息应恰好 1 条，实际 %d 条", count)
	}
	if terminalIdx != len(received)-1 {
		t.Fatalf("终态必须是最后一条（其后不得再有消息），终态下标=%d 总条数=%d",
			terminalIdx, len(received))
	}
}

// 契约四：终态消息内容必须固定为 agent-DONE 哨兵或空，
// 不得把用户的正文误标为终态（否则前端会把正文当结束信号丢弃渲染）。
func TestTerminalMessageDoesNotSwallowContent(t *testing.T) {
	cfgID := unreachableConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ch := NewStockAiAgentApi().ChatWithContext(ctx, "ping", int(cfgID), nil, false, 0, false, "direct")
	received, closed := drainUntilClosed(t, ch, 50*time.Second)
	if !closed {
		t.Fatal("流未关闭")
	}
	last := received[len(received)-1]
	if last.Content != "agent-DONE" && last.Content != "" {
		t.Fatalf("终态消息的 Content 应为哨兵 agent-DONE（前端据此触发收尾），实际=%q", last.Content)
	}
	// 错误正文必须在其之前独立发出，不能被终态吞掉
	var sawError bool
	for _, m := range received {
		if m != nil && m.FinishReason == "" && len(m.Content) > 0 {
			sawError = true
			break
		}
	}
	if !sawError {
		t.Error("错误正文应与终态分离，且独立成条（不能被终态吞掉）")
	}
}
