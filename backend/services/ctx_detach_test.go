package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"blades-stock/backend/agent"
)

// 固化 2026-11-10 修复：后台研报任务必须**脱离 HTTP 请求上下文**。
//
// 事故：
//
//	cmd/server/dispatcher.go 把 `r.Context()` 注入 RPC 方法的 ctx 参数；
//	而 HTTP handler 一旦返回，r.Context() 立刻被取消。
//	我的异步包装却把同一个 ctx 传给了后台 goroutine ——
//	于是"12 秒返回 inBackground 占位"的瞬间，后台生成就被取消了。
//	生产运行记录里那条 `dur=1.1s err=context canceled` 就是它的签名：
//	前端拿到"后台生成中"、然后永远轮询不到结果 —— 即用户反复反馈的
//	"盘前/盘后/ETF/Serenity 无结果"。

// 契约一：detachForBackground 必须剥离取消信号。
func TestDetachForBackgroundDropsCancellation(t *testing.T) {
	reqCtx, cancel := context.WithCancel(context.Background())
	cancel() // 模拟"HTTP handler 已返回"

	bg := detachForBackground(reqCtx)
	if bg == nil {
		t.Fatal("不得返回 nil")
	}
	if err := bg.Err(); err != nil {
		t.Fatalf("后台上下文不得继承取消信号，实际 Err()=%v", err)
	}
	// 关键：等到"请求早已结束"之后仍必须可用
	select {
	case <-bg.Done():
		t.Fatal("后台上下文在请求结束后被取消了 —— 后台任务会被当场杀死")
	case <-time.After(120 * time.Millisecond):
	}
}

// 契约二：nil ctx 也要安全（部分内部调用会传 nil）。
func TestDetachForBackgroundHandlesNil(t *testing.T) {
	bg := detachForBackground(nil)
	if bg == nil || bg.Err() != nil {
		t.Fatalf("nil 应退化为可用上下文，实际 %v", bg)
	}
}

// 契约三：端到端语义 —— 请求 ctx 被取消后，后台任务仍能跑完并把结果写回。
func TestBackgroundReportTaskSurvivesRequestCancel(t *testing.T) {
	reportTasksMu.Lock()
	reportTasks = map[string]*reportTask{}
	reportTasksMu.Unlock()

	reqCtx, cancelReq := context.WithCancel(context.Background())
	bgCtx := detachForBackground(reqCtx)

	finished := make(chan error, 1)
	svc := &AgentService{}
	_ = svc

	task := startReportTask("ctx_detach_test", func() (*agent.MarketIntelligenceReport, error) {
		select {
		case <-bgCtx.Done():
			finished <- bgCtx.Err()
			return nil, bgCtx.Err()
		case <-time.After(400 * time.Millisecond):
			finished <- nil
			return nil, nil
		}
	})

	// 关键时序：先让"HTTP handler 返回"（取消请求 ctx），再等后台跑完
	cancelReq()

	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("后台任务被请求上下文杀死: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("后台任务未在预期时间内完成")
	}
	<-task.done
	if task.err != nil {
		t.Fatalf("后台任务应成功，实际 %v", task.err)
	}
}

// 契约四（反证）：**不**做脱离时，请求 ctx 被取消会当场杀死后台任务。
// 这条用例是"为什么必须 detachForBackground"的证据，
// 而不是"以防万一"的猜测 —— 它精确复现了生产日志里的
// `dur=1.1s err=context canceled`。
func TestWithoutDetachBackgroundTaskIsKilled(t *testing.T) {
	reportTasksMu.Lock()
	reportTasks = map[string]*reportTask{}
	reportTasksMu.Unlock()

	reqCtx, cancelReq := context.WithCancel(context.Background())

	killed := make(chan error, 1)
	task := startReportTask("no_detach_test", func() (*agent.MarketIntelligenceReport, error) {
		// 旧实现：直接用请求 ctx
		select {
		case <-reqCtx.Done():
			killed <- reqCtx.Err()
			return nil, reqCtx.Err()
		case <-time.After(400 * time.Millisecond):
			killed <- nil
			return nil, nil
		}
	})

	// HTTP handler 返回 → 请求 ctx 取消
	cancelReq()

	select {
	case err := <-killed:
		if err == nil {
			t.Fatal("反证失败：未脱离时本应被取消 —— 说明这条用例没有真实复现缺陷")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("应为 context.Canceled，实际 %v", err)
		}
		t.Logf("反证成立：未脱离的请求 ctx 被取消后，后台任务以 %v 终止（与生产日志 dur=1.1s err=context canceled 一致）", err)
	case <-time.After(3 * time.Second):
		t.Fatal("任务既未完成也未取消，行为异常")
	}
	<-task.done
	if task.err == nil {
		t.Error("未脱离时任务应记录失败")
	}
}
