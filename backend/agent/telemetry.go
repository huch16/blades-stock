package agent

import (
	"context"
	"sync"
	"time"

	"blades-stock/backend/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	meterInitOnce sync.Once

	// 指标定义
	llmRequestDuration metric.Float64Histogram
	llmFirstChunkDelay metric.Float64Histogram
	toolCallCounter    metric.Int64Counter
	toolCallDuration   metric.Float64Histogram
	retryCounter       metric.Int64Counter
	mcpConnectDuration metric.Float64Histogram
)

func initMetrics() {
	meterInitOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("blades-stock/agent")

		var err error
		llmRequestDuration, err = meter.Float64Histogram(
			"agent_llm_request_duration_ms",
			metric.WithDescription("Agent LLM 请求端到端耗时(毫秒)"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 llmRequestDuration 失败: %v", err)
		}

		llmFirstChunkDelay, err = meter.Float64Histogram(
			"agent_llm_first_chunk_latency_ms",
			metric.WithDescription("Agent LLM 首包响应延迟(毫秒)"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 llmFirstChunkDelay 失败: %v", err)
		}

		toolCallCounter, err = meter.Int64Counter(
			"agent_tool_call_total",
			metric.WithDescription("Agent 工具调用总次数"),
			metric.WithUnit("{call}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 toolCallCounter 失败: %v", err)
		}

		toolCallDuration, err = meter.Float64Histogram(
			"agent_tool_call_duration_ms",
			metric.WithDescription("Agent 工具调用耗时(毫秒)"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 toolCallDuration 失败: %v", err)
		}

		retryCounter, err = meter.Int64Counter(
			"agent_retry_total",
			metric.WithDescription("Agent 遇到瞬态故障触发重试总次数"),
			metric.WithUnit("{retry}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 retryCounter 失败: %v", err)
		}

		mcpConnectDuration, err = meter.Float64Histogram(
			"agent_mcp_connect_duration_ms",
			metric.WithDescription("Agent 连接 MCP 服务耗时(毫秒)"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 mcpConnectDuration 失败: %v", err)
		}
	})
}

// RecordLLMRun 记录一次完整 LLM 对话调用的可观测指标
func RecordLLMRun(ctx context.Context, duration time.Duration, firstChunkLatency time.Duration, status string, retryCount int) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("status", status),
	}

	if llmRequestDuration != nil {
		llmRequestDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
	}
	if llmFirstChunkDelay != nil && firstChunkLatency > 0 {
		llmFirstChunkDelay.Record(ctx, float64(firstChunkLatency.Milliseconds()), metric.WithAttributes(attrs...))
	}
	if retryCounter != nil && retryCount > 0 {
		retryCounter.Add(ctx, int64(retryCount), metric.WithAttributes(attrs...))
	}
}

// RecordToolCall 记录一次工具调用的执行指标
func RecordToolCall(ctx context.Context, toolName string, success bool, duration time.Duration) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("tool_name", toolName),
		attribute.Bool("success", success),
	}

	if toolCallCounter != nil {
		toolCallCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if toolCallDuration != nil && duration > 0 {
		toolCallDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
	}
}

// RecordMCPConnect 记录 MCP 连接耗时与状态
func RecordMCPConnect(ctx context.Context, serverName string, success bool, duration time.Duration) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("server_name", serverName),
		attribute.Bool("success", success),
	}

	if mcpConnectDuration != nil {
		mcpConnectDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
	}
}
