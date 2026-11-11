package agent

import (
	"sync"
	"sync/atomic"
	"testing"
)

// withBroadcastHook 临时接管 SSE 广播钩子并返回收集到的载荷，测试结束自动还原，
// 避免污染全局单例（多个测试并行运行时会互相干扰）。
func withBroadcastHook(t *testing.T) (*[]map[string]any, *sync.Mutex) {
	t.Helper()
	var mu sync.Mutex
	var got []map[string]any
	prev := PriceAlertBroadcastHook
	PriceAlertBroadcastHook = func(event string, data any) {
		if event != "stockPriceAlertTriggered" {
			return
		}
		if m, ok := data.(map[string]any); ok {
			mu.Lock()
			got = append(got, m)
			mu.Unlock()
		}
	}
	t.Cleanup(func() { PriceAlertBroadcastHook = prev })
	return &got, &mu
}

// TestPriceAlertBroadcastIsModelIndependent 是本轮修复的核心回归护栏。
//
// 背景（线上缺陷）：价格预警只在三个桌面 shell 里启动，cmd/server 从未启动，
// 导致 Web/Docker 部署下「设了预警价但永远不提醒」；而前端其实早已
// EventsOn('stockPriceAlertTriggered') 并备好了通知 UI。
//
// 同时修复了第二个失效模式：原先 triggerAgentAnalysis 在
// 「无 AI 配置 / 模型无效 / Agent 构建失败 / 并发超限」时直接 return，
// 于是**确定性的告警**被 LLM 可用性绑架 —— 没配好模型就一条告警都收不到。
// 现在确定性字段先发、AI 归因仅作附加。
//
// 本测试锁定：广播载荷**不经过任何模型**即可完整产生。
func TestPriceAlertBroadcastIsModelIndependent(t *testing.T) {
	got, mu := withBroadcastHook(t)

	w := &PriceAlertAgentWatcher{} // 刻意不初始化 wailsCtx：模拟服务端上下文
	const reason = "价格突破目标价 0.01（当前现价: 1402.30）"
	w.broadcastPriceAlert("sh600519", "贵州茅台", reason, 1402.3, 2.5, "", nil)

	mu.Lock()
	defer mu.Unlock()
	if len(*got) != 1 {
		t.Fatalf("应广播 1 条告警，实际 %d 条", len(*got))
	}
	p := (*got)[0]

	// 确定性字段必须齐全且正确（这些不依赖模型，是告警的价值主体）
	if p["code"] != "sh600519" {
		t.Errorf("code = %v, want sh600519", p["code"])
	}
	if p["name"] != "贵州茅台" {
		t.Errorf("name = %v, want 贵州茅台", p["name"])
	}
	if p["reason"] != reason {
		t.Errorf("reason = %v, want %q", p["reason"], reason)
	}
	if p["price"] != 1402.3 {
		t.Errorf("price = %v, want 1402.3", p["price"])
	}
	if p["changePercent"] != 2.5 {
		t.Errorf("changePercent = %v, want 2.5", p["changePercent"])
	}

	// AI 归因为空时也必须照样发出（而不是整条告警被吞掉）
	if ai, ok := p["aiAnalysis"]; !ok || ai != "" {
		t.Errorf("aiAnalysis 字段应存在且为空，实际 %v (存在=%v)", ai, ok)
	}

	// 前端 App.vue 的告警处理依赖这两个键；缺失会导致通知渲染异常
	for _, k := range []string{"time", "actions"} {
		if _, ok := p[k]; !ok {
			t.Errorf("载荷缺少前端依赖字段 %q", k)
		}
	}
}

// TestPriceAlertBroadcastToleratesNilHook 确保未接 SSE 钩子时不 panic
// （例如桌面端只走 Wails 通道的形态）。
func TestPriceAlertBroadcastToleratesNilHook(t *testing.T) {
	prev := PriceAlertBroadcastHook
	PriceAlertBroadcastHook = nil
	t.Cleanup(func() { PriceAlertBroadcastHook = prev })

	w := &PriceAlertAgentWatcher{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("钩子为 nil 时不应 panic: %v", r)
		}
	}()
	w.broadcastPriceAlert("sh000001", "上证指数", "测试", 3000, 1.0, "", nil)
}

// TestAnalyzeAnomalyConcurrencyGateDoesNotBlockAlert 验证并发闸门在超限时
// 只放弃 AI 归因（返回空串），而不会 panic 或泄漏计数器 —— 计数泄漏会让
// 后续所有归因永久失效（闸门再也放不进来）。
func TestAnalyzeAnomalyConcurrencyGateDoesNotBlockAlert(t *testing.T) {
	// 人为把并发计数顶到上限以上
	prev := atomic.SwapInt32(&watcherRunningAgentCount, 99)
	t.Cleanup(func() { atomic.StoreInt32(&watcherRunningAgentCount, prev) })

	w := &PriceAlertAgentWatcher{}
	analysis, actions := w.analyzeAnomaly("sh600519", "贵州茅台", "测试", 1402.3, 2.5)
	if analysis != "" || actions != nil {
		t.Errorf("并发超限时应返回空归因，实际 analysis=%q actions=%v", analysis, actions)
	}
	// 闸门必须在返回时把计数还回去（本次调用 +1 后判定超限，-1 还原）
	if n := atomic.LoadInt32(&watcherRunningAgentCount); n != 99 {
		t.Errorf("并发计数泄漏：%d, want 99（闸门必须在所有返回路径上归还计数）", n)
	}
}
