package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/agent"
)

// ── 市场智能研报：把长推演移出 HTTP 响应链路 ─────────────────────────────
//
// 事故背景（2026-11-10 用户反馈「盘前前瞻、盘后复盘等没有结果」）：
//
//	生产入口（Cloudflare/instabox）对单次 HTTP 请求有约 **20 秒**硬切断。
//	实测三个生成器全部 502@20.4s / 22.6s / 21.1s；
//	而研报生成本身要 40~90 秒（模型慢时更久）。
//
//	后果是三重叠加，用户看到的是"没有结果"：
//	  ① 响应被入口切断 → 前端收到 502；
//	  ② 后台生成虽继续（生成上下文是独立的 10 分钟 context），
//	     但模型调用会挂到约 300 秒（共享客户端超时）才失败 →
//	     运行记录实测 dur=301762ms err=context deadline exceeded；
//	  ③ 生成失败后，UI 从归档表回退到**几个月前的旧研报**
//	     （实测回退到 2026-11-07、来源 gemini-3.8-flash）
//	     —— 用户以为"新的生成了"，其实看到的是旧的。
//
// 本文件的策略：
//
//	**绝不让 HTTP 响应等待长推演。** 在 marketReportSoftWait 内能完成就正常返回；
//	超时则立即返回一个"仍在后台生成"的可识别错误，生成本身继续在后台跑完并
//	落库/入缓存。前端抽屉对这类错误的既有处理是：转入轮询等待
//	（startPollingForCompletion），完成后自动呈现。
//
//	这样无论模型多慢、入口多严，用户都能在 20 秒内得到确定反馈，
//	并最终拿到真正的结果，而不是旧研报冒充新研报。

// detachForBackground 让后台任务**脱离 HTTP 请求上下文**。
//
// 为什么必须这么做（踩过的坑，务必保留这段注释）：
//
//	cmd/server/dispatcher.go 把 `r.Context()` 注入 RPC 方法的 ctx 参数，
//	而 HTTP handler 一旦返回，r.Context() 立刻被取消。
//	异步包装若把同一个 ctx 传给后台 goroutine，那么"12 秒返回 inBackground
//	占位"的瞬间，后台生成就被取消了 —— 前端拿到"后台生成中"，
//	然后永远轮询不到结果。生产运行记录里那条
//	`dur=1.1s err=context canceled` 就是它的签名。
//
// context.WithoutCancel（Go 1.21+）保留 ctx 中的值（追踪 ID、运行类型等），
// 但剥离取消信号与截止时间 —— 后台任务因此能跑满它自己的 10 分钟上限。
func detachForBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

// marketReportSoftWait 同步等待研报的软上限。
//
// 必须显著小于入口的 20s 硬切断（留出协议与回传余量）。
// 生成本身不受此限制 —— 它跑在独立 context 上，会一直做到完成或 10 分钟上限。
const marketReportSoftWait = 12 * time.Second

// backgroundPlaceholder 是"生成仍在后台进行"的占位报告。
//
// 用**返回结构上的字段**（InBackground）而不是 error 传递这个状态，
// 原因是：三个研报 RPC 的既有签名只返回 *MarketIntelligenceReport
// （Wails 绑定契约），没有 error 通道；而新增字段对旧版前端只是被忽略的额外
// JSON 键，向后兼容。前端据此转入轮询等待后台完成。
func backgroundPlaceholder(kind string) *agent.MarketIntelligenceReport {
	return &agent.MarketIntelligenceReport{
		Type:         reportTaskTypeKey(kind),
		InBackground: true,
		StatusText:   "AI 深度推演仍在后台进行：响应窗口内未完成，系统将继续推演并在完成后自动呈现，请稍候。",
	}
}

// reportTaskTypeKey 把任务 key 还原为研报 type（serenity 带代码后缀）。
func reportTaskTypeKey(kind string) string {
	switch {
	case strings.HasPrefix(kind, "serenity"):
		return "serenity_industry_analysis"
	case kind == "etf_rotation":
		return "etf_rotation_decision"
	case kind == "pre_market":
		return "pre_market_briefing"
	case kind == "post_market":
		return "post_market_review"
	}
	return kind
}

// reportTask 一次研报生成的异步句柄。
type reportTask struct {
	done  chan struct{}
	rep   *agent.MarketIntelligenceReport
	err   error
	once  sync.Once
	kind  string
	start time.Time
}

func (t *reportTask) finish(rep *agent.MarketIntelligenceReport, err error) {
	t.once.Do(func() {
		t.rep, t.err = rep, err
		close(t.done)
	})
}

var (
	reportTasksMu sync.Mutex
	reportTasks   = map[string]*reportTask{}
)

// startReportTask 若同类型任务已在跑则复用，否则启动新任务。
// 复用可以防止用户连点两次把同一份研报生成两遍（浪费 token 且结果互相覆盖）。
func startReportTask(kind string, run func() (*agent.MarketIntelligenceReport, error)) *reportTask {
	reportTasksMu.Lock()
	if t, ok := reportTasks[kind]; ok {
		select {
		case <-t.done: // 已完成：允许重新生成
			delete(reportTasks, kind)
		default:
			reportTasksMu.Unlock()
			return t
		}
	}
	t := &reportTask{done: make(chan struct{}), kind: kind, start: time.Now()}
	reportTasks[kind] = t
	reportTasksMu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.finish(nil, fmt.Errorf("研报生成内部异常: %v", r))
			}
			// 完成后**不立即删除**：前端/运维需要能查到"这次生成结束了、结果是
			// 成功还是失败"。重复生成由 startReportTask 负责替换旧任务
			//（其复用逻辑见 `case <-t.done: delete(...)`），故不会泄漏。
		}()
		rep, err := run()
		t.finish(rep, err)
	}()
	return t
}

// awaitReport 等待任务完成，但最多只等 softWait。
// 返回 (report, err, ready)：
//
//	ready=true  → 已在软上限内完成（无论成败），可如实返回
//	ready=false → 仍在后台跑，应返回 errReportInBackground
func awaitReport(t *reportTask, softWait time.Duration) (*agent.MarketIntelligenceReport, error, bool) {
	select {
	case <-t.done:
		return t.rep, t.err, true
	case <-time.After(softWait):
		return backgroundPlaceholder(t.kind), nil, false
	}
}

// GeneratePreMarketBriefingAsync 盘前晨报：同步部分不超过 12 秒。
func (s *AgentService) GeneratePreMarketBriefingAsync(ctx context.Context, aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	// 关键：后台任务必须用**脱离请求**的 ctx，否则 handler 一返回就被取消
	bg := detachForBackground(ctx)
	svc := agent.NewMarketLifecycleService(bg)
	t := startReportTask("pre_market", func() (*agent.MarketIntelligenceReport, error) {
		return svc.GeneratePreMarketBriefing(bg, aiConfigId)
	})
	rep, err, ready := awaitReport(t, marketReportSoftWait)
	if !ready {
		// 同步窗口内未完成：占位报告带 InBackground 标记，前端据此转轮询。
		// 生成继续在后台跑（独立 context），完成后自动呈现。
		return rep, nil
	}
	if err != nil {
		return nil, err
	}
	return rep, nil
}

// GeneratePostMarketReviewAsync 收盘复盘：同步部分不超过 12 秒。
func (s *AgentService) GeneratePostMarketReviewAsync(ctx context.Context, aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	bg := detachForBackground(ctx)
	svc := agent.NewMarketLifecycleService(bg)
	t := startReportTask("post_market", func() (*agent.MarketIntelligenceReport, error) {
		return svc.GeneratePostMarketReview(bg, aiConfigId)
	})
	rep, err, ready := awaitReport(t, marketReportSoftWait)
	if !ready {
		// 同步窗口内未完成：占位报告带 InBackground 标记，前端据此转轮询。
		// 生成继续在后台跑（独立 context），完成后自动呈现。
		return rep, nil
	}
	if err != nil {
		return nil, err
	}
	return rep, nil
}

// GenerateETFRotationReportAsync ETF 轮动：同步部分不超过 12 秒。
func (s *AgentService) GenerateETFRotationReportAsync(ctx context.Context, aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	bg := detachForBackground(ctx)
	svc := agent.NewMarketLifecycleService(bg)
	t := startReportTask("etf_rotation", func() (*agent.MarketIntelligenceReport, error) {
		return svc.GenerateETFRotationReport(bg, aiConfigId)
	})
	rep, err, ready := awaitReport(t, marketReportSoftWait)
	if !ready {
		// 同步窗口内未完成：占位报告带 InBackground 标记，前端据此转轮询。
		// 生成继续在后台跑（独立 context），完成后自动呈现。
		return rep, nil
	}
	if err != nil {
		return nil, err
	}
	return rep, nil
}

// GenerateSerenityResearchReportAsync Serenity 产业链拆解：同步部分不超过 12 秒。
func (s *AgentService) GenerateSerenityResearchReportAsync(ctx context.Context, targetName, code string, aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	bg := detachForBackground(ctx)
	svc := agent.NewMarketLifecycleService(bg)
	kind := "serenity:" + code
	t := startReportTask(kind, func() (*agent.MarketIntelligenceReport, error) {
		return svc.GenerateSerenityResearchReport(bg, targetName, code, aiConfigId)
	})
	rep, err, ready := awaitReport(t, marketReportSoftWait)
	if !ready {
		// 同步窗口内未完成：占位报告带 InBackground 标记，前端据此转轮询。
		// 生成继续在后台跑（独立 context），完成后自动呈现。
		return rep, nil
	}
	if err != nil {
		return nil, err
	}
	return rep, nil
}

// MarketReportTaskStatus 供前端/运维查询某类型研报的后台生成状态。
func (s *AgentService) MarketReportTaskStatus(kind string) map[string]any {
	reportTasksMu.Lock()
	t, ok := reportTasks[kind]
	reportTasksMu.Unlock()
	if !ok {
		return map[string]any{"running": false}
	}
	select {
	case <-t.done:
		return map[string]any{
			"running": false,
			"ok":      t.err == nil,
			"error":   errString(t.err),
			"elapsed": time.Since(t.start).Milliseconds(),
		}
	default:
		return map[string]any{
			"running": true,
			"elapsed": time.Since(t.start).Milliseconds(),
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
