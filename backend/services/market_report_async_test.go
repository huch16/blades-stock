package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"blades-stock/backend/agent"
)

// 固化 2026-11-10 修复：市场研报不再堵死 HTTP 响应；连通性测试有 TCP 预检。
//
// 事故链（生产实测）：
//   入口约 20s 硬切断 → 研报生成 40~90s 全部 502
//   → 后台虽继续，但模型调用挂到 301s 才超时（dur=301762ms）
//   → 失败后 UI 回退到 11-07 的旧研报，用户以为"新的生成了"。
//   → 连通性测试同理 502@20.4s，用户看到误导性的"本系统网关错误"。

// 契约一：后台推进时，同步等待必须在软上限内返回带 InBackground 标记的占位报告。
func TestAwaitReportReturnsBackgroundPlaceholder(t *testing.T) {
	blocker := make(chan struct{})
	task := &reportTask{done: make(chan struct{}), kind: "pre_market", start: time.Now()}
	go func() {
		<-blocker
		task.finish(nil, nil)
	}()
	defer close(blocker)

	rep, err, ready := awaitReport(task, 50*time.Millisecond)
	if ready {
		t.Fatal("软上限内未完成时必须 ready=false")
	}
	if err != nil {
		t.Fatalf("未完成不是失败，不得返回 error（否则前端会直接报生成失败）：%v", err)
	}
	if rep == nil || !rep.InBackground {
		t.Fatal("必须返回带 InBackground 标记的占位报告，供前端转入轮询")
	}
	if rep.Type != "pre_market_briefing" {
		t.Errorf("占位报告的 Type 必须还原为研报类型，实际 %q", rep.Type)
	}
	if rep.StatusText == "" {
		t.Error("占位报告必须带面向用户的进度说明")
	}
}

// 契约二：软上限内完成时，必须如实返回结果（含失败原因）。
func TestAwaitReportReturnsResultWithinBudget(t *testing.T) {
	task := &reportTask{done: make(chan struct{}), kind: "post_market", start: time.Now()}
	task.finish(nil, errors.New("boom"))

	_, err, ready := awaitReport(task, 2*time.Second)
	if !ready {
		t.Fatal("已完成时必须 ready=true")
	}
	if err == nil || err.Error() != "boom" {
		t.Fatalf("必须如实返回失败原因，实际 %v", err)
	}
}

// 契约三：同类型任务在跑时复用，不重复生成（连点两次不烧两遍 token）。
func TestStartReportTaskDedupesInFlight(t *testing.T) {
	reportTasksMu.Lock()
	reportTasks = map[string]*reportTask{}
	reportTasksMu.Unlock()

	var calls int
	release := make(chan struct{})
	start := func() *reportTask {
		return startReportTask("dedupe_test", func() (*agent.MarketIntelligenceReport, error) {
			calls++
			<-release
			return nil, nil
		})
	}
	t1 := start()
	t2 := start()
	if t1 != t2 {
		t.Fatal("同类型任务在执行中必须复用同一实例")
	}
	close(release)
	<-t1.done
	if calls != 1 {
		t.Errorf("复用时不得重复执行，实际调用 %d 次", calls)
	}
}

// 契约四：MarketReportTaskStatus 能反映运行中/已完成。
func TestMarketReportTaskStatus(t *testing.T) {
	reportTasksMu.Lock()
	reportTasks = map[string]*reportTask{}
	reportTasksMu.Unlock()

	svc := &AgentService{}
	rel := make(chan struct{})
	startReportTask("status_test", func() (*agent.MarketIntelligenceReport, error) {
		<-rel
		return nil, errors.New("boom")
	})
	running := svc.MarketReportTaskStatus("status_test")
	if running["running"] != true {
		t.Errorf("运行中应报 running=true，实际 %v", running)
	}
	close(rel)
	time.Sleep(100 * time.Millisecond)
	done := svc.MarketReportTaskStatus("status_test")
	if done["running"] != false || done["ok"] != false {
		t.Errorf("完成后应报 running=false ok=false，实际 %v", done)
	}
	if done["error"] != "boom" {
		t.Errorf("必须回传失败原因，实际 %v", done["error"])
	}
}

// 契约五：preflightDial 必须对不可解析的地址立即给出可读原因，
// 且对黑洞地址在 4 秒内失败（而不是挂满整个测试预算）。
func TestPreflightDialFailsFast(t *testing.T) {
	if msg := preflightDial("not a url"); msg == "" {
		t.Error("无法解析的地址必须返回原因")
	}
	start := time.Now()
	msg := preflightDial("http://192.0.2.1:8080/v1") // TEST-NET-1，不可路由
	elapsed := time.Since(start)
	if msg == "" {
		t.Error("黑洞地址必须返回失败原因")
	}
	if elapsed > 6*time.Second {
		t.Errorf("预检必须在 4s 级完成，实际 %v", elapsed)
	}
	if !strings.Contains(msg, "超时") && !strings.Contains(msg, "无法连接") {
		t.Errorf("原因应明确指出超时或无法连接，实际 %q", msg)
	}
}

// 契约六：占位报告的类型还原必须覆盖全部研报类型，否则前端轮询会查错 key。
func TestBackgroundPlaceholderTypeMapping(t *testing.T) {
	cases := map[string]string{
		"pre_market":         "pre_market_briefing",
		"post_market":        "post_market_review",
		"etf_rotation":       "etf_rotation_decision",
		"serenity":           "serenity_industry_analysis",
		"serenity:512480.SH": "serenity_industry_analysis",
	}
	for kind, want := range cases {
		got := backgroundPlaceholder(kind)
		if got.Type != want {
			t.Errorf("任务 %q 的占位报告 Type 应为 %q，实际 %q", kind, want, got.Type)
		}
		if !got.InBackground || got.Content != "" {
			t.Errorf("任务 %q 的占位报告必须是空内容 + InBackground 标记", kind)
		}
	}
}
