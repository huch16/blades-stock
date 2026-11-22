package agent

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTelemetryMetricsRecording(t *testing.T) {
	ctx := context.Background()

	// 验证 RecordLLMRun 正常记录且无 panic
	RecordLLMRun(ctx, 1200*time.Millisecond, 300*time.Millisecond, "success", 1)

	// 验证 RecordToolCall 正常记录且无 panic
	RecordToolCall(ctx, "GetStockPriceInfo", true, 80*time.Millisecond)
	RecordToolCall(ctx, "RunVectorizedBacktest", false, 250*time.Millisecond)

	// 验证 RecordMCPConnect 正常记录且无 panic
	RecordMCPConnect(ctx, "local-mcp-server", true, 450*time.Millisecond)

	// 验证配额与成本相关度量上报
	RecordQuotaUsage(ctx, "WebSearch", 5)
	RecordQuotaExceeded(ctx, "WebSearch", "daily_calls")
	RecordToolCost(ctx, "WebSearch", 0.010)
	RecordQuotaLimits(ctx, 15, 200, 2.0)

	// 验证新增的回测准确率与 embedding 延迟度量（此前只有注释
	// 提及而无函数体，backtestAccuracyGauge 创建后从未被赋值）。
	SetBacktestAccuracy(ctx, 0.85)
	RecordEmbeddingLatency(ctx, "text-embedding-3-small", 120*time.Millisecond)
	// 失败的工具调用现在会计入 toolErrorCounter（此前失败分支
	// 从不计数，生产里 agent_tool_errors_total 恒为 0）。
	RecordToolCall(ctx, "GetStockPriceInfo", false, 50*time.Millisecond)
}

func TestStartMetricsExporter_Endpoints(t *testing.T) {
	// 设置环境变量启用 prometheus 导出器及测试端口
	os.Setenv("METRICS_EXPORTER", "prometheus")
	os.Setenv("METRICS_PROMETHEUS_ADDR", "127.0.0.1:19464")
	defer func() {
		os.Unsetenv("METRICS_EXPORTER")
		os.Unsetenv("METRICS_PROMETHEUS_ADDR")
	}()

	ctx := context.Background()
	shutdown, err := StartMetricsExporter(ctx)
	if err != nil {
		t.Fatalf("StartMetricsExporter 失败: %v", err)
	}
	if shutdown == nil {
		t.Fatalf("预期返回有效的 shutdown 函数")
	}
	defer func() {
		_ = shutdown(context.Background())
	}()

	// 稍作等待以确保 HTTP server 监听建立
	time.Sleep(150 * time.Millisecond)

	// 1. 测试 /healthz 端点
	resp, err := http.Get("http://127.0.0.1:19464/healthz")
	if err != nil {
		t.Fatalf("请求 /healthz 失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("预期状态码 200, 实际: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"UP"`) {
		t.Errorf("/healthz 响应内容不符合预期: %s", string(body))
	}

	// 2. 测试 /livez 端点
	respLive, err := http.Get("http://127.0.0.1:19464/livez")
	if err != nil {
		t.Fatalf("请求 /livez 失败: %v", err)
	}
	defer respLive.Body.Close()
	if respLive.StatusCode != http.StatusOK {
		t.Errorf("预期状态码 200, 实际: %d", respLive.StatusCode)
	}

	// 3. 测试 /metrics 端点
	respMetrics, err := http.Get("http://127.0.0.1:19464/metrics")
	if err != nil {
		t.Fatalf("请求 /metrics 失败: %v", err)
	}
	defer respMetrics.Body.Close()
	if respMetrics.StatusCode != http.StatusOK {
		t.Errorf("预期状态码 200, 实际: %d", respMetrics.StatusCode)
	}
	bodyMetrics, _ := io.ReadAll(respMetrics.Body)
	if len(bodyMetrics) == 0 {
		t.Errorf("/metrics 响应内容不应为空")
	}
}
