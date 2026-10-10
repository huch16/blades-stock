package agent

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"blades-stock/backend/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelpromexp "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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

// httpHandlerProvider is an interface that matches the otel prometheus exporter's
// HTTPHandler method if it exists.
type httpHandlerProvider interface {
	HTTPHandler() http.Handler
}

// StartMetricsExporter 根据环境变量启动相应的 exporter 并（可选）启动一个 HTTP server 暴露 /metrics。
// - 如果 METRICS_EXPORTER != "prometheus"，不执行任何操作，返回 (nil, nil)。
// - 如果使用 prometheus 导出器，会创建 otel prometheus exporter，
//   使用该 exporter 创建 MeterProvider 并设置为全局，并在 METRICS_PROMETHEUS_ADDR（默认 ":9464"）
//   启动 HTTP 服务暴露 /metrics（使用 exporter.HTTPHandler() 如果可用）。
// 返回值：shutdownFunc, error。shutdownFunc 用于在应用退出时关闭 HTTP server。
func StartMetricsExporter(ctx context.Context) (func(context.Context) error, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if os.Getenv("METRICS_EXPORTER") != "prometheus" {
		// 没有启用 prometheus 导出器
		return nil, nil
	}

	exp, err := otelpromexp.New()
	if err != nil {
		logger.SugaredLogger.Errorf("Failed to create prometheus exporter: %v", err)
		return nil, err
	}
	logger.SugaredLogger.Infof("Prometheus exporter created.")

	// 创建使用该 exporter 的 MeterProvider 并设置为全局。
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exp),
	)
	otel.SetMeterProvider(provider)
	logger.SugaredLogger.Infof("Prometheus exporter attached to global meter provider.")

	// 尝试使用 exporter 的 HTTPHandler 方法来启动 HTTP server。
	addr := os.Getenv("METRICS_PROMETHEUS_ADDR")
	if addr == "" {
		addr = ":9464"
	}
	mux := http.NewServeMux()
	var handler http.Handler
	if p, ok := interface{}(exp).(httpHandlerProvider); ok {
		handler = p.HTTPHandler()
		logger.SugaredLogger.Infof("Using exporter.HTTPHandler() for /metrics")
	} else {
		// 如果没有 HTTPHandler 方法，则不启动 HTTP server，仅记录警告。
		logger.SugaredLogger.Warnf("Prometheus exporter does not provide HTTPHandler; metrics will not be exposed via /metrics. Consider using an OTel collector to scrape metrics.")
		handler = nil
	}
	if handler != nil {
		mux.Handle("/metrics", handler)
		srv := &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		// 启动 server（后台 goroutine）
		go func() {
			logger.SugaredLogger.Infof("Metrics HTTP server listening on %s", addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.SugaredLogger.Errorf("Metrics HTTP server error: %v", err)
			}
			logger.SugaredLogger.Infof("Metrics HTTP server stopped")
		}()

		shutdown := func(shutdownCtx context.Context) error {
			if shutdownCtx == nil {
				shutdownCtx = context.Background()
			}
			logger.SugaredLogger.Infof("Shutting down metrics HTTP server")
			return srv.Shutdown(shutdownCtx)
		}
		return shutdown, nil
	}

	// 没有 HTTP server 可启动，返回 nil shutdown。
	return nil, nil
}

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