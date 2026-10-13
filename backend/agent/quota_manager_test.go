package agent

import (
	"context"
	"strings"
	"sync"
	"testing"

	bladesTools "github.com/go-kratos/blades/tools"
)

func TestToolQuotaManager_RunBudget(t *testing.T) {
	qm := GetToolQuotaManager()
	qm.ResetDailyQuota()

	// 注入单次运行上限为 2 次，但现在 self-limiter 已移除 - 应该总是允许
	ctx := WithRunQuota(context.Background(), 2)

	cost1, err1 := qm.CheckAndConsume(ctx, "GetStockPriceInfo")
	if err1 != nil {
		t.Fatalf("首次调用不应报错 (self-limiter removed): %v", err1)
	}
	if cost1 <= 0 {
		t.Errorf("调用成本应大于0, 实际: %f", cost1)
	}

	cost2, err2 := qm.CheckAndConsume(ctx, "WebSearch")
	if err2 != nil {
		t.Fatalf("第二次调用不应报错 (self-limiter removed): %v", err2)
	}
	if cost2 <= 0 {
		t.Errorf("调用成本应大于0, 实际: %f", cost2)
	}

	// 第三次调用现在应该也允许 (self-limiter removed)
	cost3, err3 := qm.CheckAndConsume(ctx, "FetchWebPage")
	if err3 != nil {
		t.Fatalf("第三次调用现在应该允许 (self-limiter removed): %v", err3)
	}
	if cost3 <= 0 {
		t.Errorf("调用成本应大于0, 实际: %f", cost3)
	}
}

func TestToolQuotaManager_WrapToolWithQuota(t *testing.T) {
	qm := GetToolQuotaManager()
	qm.ResetDailyQuota()

	rawTool := bladesTools.NewTool(
		"MockTool",
		"测试工具",
		bladesTools.HandleFunc(func(ctx context.Context, input string) (string, error) {
			return "mock-success-result:" + input, nil
		}),
	)

	wrapped := WrapToolWithQuota(rawTool)

	// 设置单次运行预算为 1，但现在 self-limiter 已移除
	ctx := WithRunQuota(context.Background(), 1)

	// 第一次调用：成功
	resp1, err1 := wrapped.Handle(ctx, "arg1")
	if err1 != nil {
		t.Fatalf("wrapped tool execute error: %v", err1)
	}
	if !strings.Contains(resp1, "mock-success-result:arg1") {
		t.Errorf("首次调用结果不符: %s", resp1)
	}

	// 第二次调用：现在也应该成功 (self-limiter removed)
	resp2, err2 := wrapped.Handle(ctx, "arg2")
	if err2 != nil {
		t.Fatalf("wrapped tool 超限现在不应返回 Go error: %v", err2)
	}
	// Should return actual tool result, not quota exceeded JSON
	if !strings.Contains(resp2, "mock-success-result:arg2") {
		t.Errorf("第二次调用现在应该返回工具结果: %s", resp2)
	}
}

func TestToolQuotaManager_StatusAndReset(t *testing.T) {
	qm := GetToolQuotaManager()
	qm.ResetDailyQuota()

	ctx := context.Background()
	_, _ = qm.CheckAndConsume(ctx, "WebSearch")
	_, _ = qm.CheckAndConsume(ctx, "DiagnoseStock")

	status := qm.GetQuotaStatus()
	if status.DailyCalls != 2 {
		t.Errorf("预期 DailyCalls 为 2, 实际: %d", status.DailyCalls)
	}
	if status.DailyCostUSD <= 0 {
		t.Errorf("预期 DailyCostUSD > 0, 实际: %f", status.DailyCostUSD)
	}

	qm.ResetDailyQuota()
	resetStatus := qm.GetQuotaStatus()
	if resetStatus.DailyCalls != 0 || resetStatus.DailyCostUSD != 0 {
		t.Errorf("重置后计数应清零: %+v", resetStatus)
	}
}

func TestToolQuotaManager_Concurrency(t *testing.T) {
	qm := GetToolQuotaManager()
	qm.ResetDailyQuota()

	ctx := context.Background()
	var wg sync.WaitGroup
	workers := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, _ = qm.CheckAndConsume(ctx, "WebSearch")
			}
		}()
	}
	wg.Wait()

	status := qm.GetQuotaStatus()
	expectedCalls := int64(workers * 5)
	if status.DailyCalls != expectedCalls {
		t.Errorf("并发调用计数不一致: 期望 %d, 实际 %d", expectedCalls, status.DailyCalls)
	}
}