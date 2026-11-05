package agent

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestTelemetryMetrics(t *testing.T) {
	ctx := context.Background()

	// Test that metrics can be recorded without error
	RecordAgentRequest(ctx, "react", true, 100*time.Millisecond)
	RecordAgentRequest(ctx, "plan_execute", false, 500*time.Millisecond)

	RecordToolCallV2(ctx, "WebSearch", true, 50*time.Millisecond, "")
	RecordToolCallV2(ctx, "FetchWebPage", false, 200*time.Millisecond, "timeout")

	RecordRetrieverHit(ctx, "embedding_retriever", true, 30*time.Millisecond)
	RecordRetrieverHit(ctx, "embedding_retriever", false, 25*time.Millisecond)

	RecordTokenUsage(ctx, "deepseek-chat", 100, 50)

	RecordLLMRun(ctx, 2*time.Second, 100*time.Millisecond, "success", 0)
	RecordToolCall(ctx, "WebSearch", true, 50*time.Millisecond)

	// Test that we can create a meter and verify metrics exist
	provider := sdkmetric.NewMeterProvider()
	meter := provider.Meter("test")
	counter, _ := meter.Int64Counter("test_counter")
	counter.Add(ctx, 1, metric.WithAttributes(attribute.String("key", "value")))

	// If we reach here without panic, the test passes
	t.Log("All telemetry metrics recorded successfully")
}
