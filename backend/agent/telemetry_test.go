package agent

import (
	"context"
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
}
