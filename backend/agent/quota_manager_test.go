package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	bladesTools "github.com/go-kratos/blades/tools"
)

func TestToolQuotaManager_RunBudget(t *testing.T) {
	qm := GetToolQuotaManager()
	qm.ResetDailyQuota()

	// 注入单次运行上限为 2 次
	ctx := WithRunQuota(context.Background(), 2)

	cost1, err1 := qm.CheckAndConsume(ctx, "GetStockPriceInfo")
	if err1 != nil {
		t.Fatalf("首次调用不应超限: %v", err1)
	}
	if cost1 <= 0 {
		t.Errorf("调用成本应大于0, 实际: %f", cost1)
	}

	cost2, err2 := qm.CheckAndConsume(ctx, "WebSearch")
	if err2 != nil {
		t.Fatalf("第二次调用不应超限: %v", err2)
	}
	if cost2 <= 0 {
		t.Errorf("调用成本应大于0, 实际: %f", cost2)
	}

	// 第三次调用应触发单次运行预算超限
	_, err3 := qm.CheckAndConsume(ctx, "FetchWebPage")
	if err3 == nil {
		t.Fatalf("第三次调用应该触发单次预算超限错误")
	}
	if !strings.Contains(err3.Error(), "单次运行工具调用预算已达上限") {
		t.Errorf("错误信息不符合预期: %v", err3)
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

	// 设置单次运行预算为 1
	ctx := WithRunQuota(context.Background(), 1)

	// 第一次调用：成功
	resp1, err1 := wrapped.Handle(ctx, "arg1")
	if err1 != nil {
		t.Fatalf("wrapped tool execute error: %v", err1)
	}
	if !strings.Contains(resp1, "mock-success-result:arg1") {
		t.Errorf("首次调用结果不符: %s", resp1)
	}

	// 第二次调用：应当被优雅拦截为 JSON 结构，且返回的 Go error 为 nil（对大模型透明）
	resp2, err2 := wrapped.Handle(ctx, "arg2")
	if err2 != nil {
		t.Fatalf("wrapped tool 超限拦截时不应返回 Go error: %v", err2)
	}

	var jsonMap map[string]string
	if err := json.Unmarshal([]byte(resp2), &jsonMap); err != nil {
		t.Fatalf("超限返回结果必须是有效 JSON: %s, err: %v", resp2, err)
	}
	if jsonMap["code"] != "QUOTA_EXCEEDED" {
		t.Errorf("预期 code 为 QUOTA_EXCEEDED, 实际: %s", jsonMap["code"])
	}
	if jsonMap["tool"] != "MockTool" {
		t.Errorf("预期 tool 为 MockTool, 实际: %s", jsonMap["tool"])
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
			_, _ = qm.CheckAndConsume(ctx, "GetStockPriceInfo")
		}()
	}

	wg.Wait()
	status := qm.GetQuotaStatus()
	if status.DailyCalls != int64(workers) {
		t.Errorf("并发测试期望累计 %d 次, 实际: %d", workers, status.DailyCalls)
	}
}
