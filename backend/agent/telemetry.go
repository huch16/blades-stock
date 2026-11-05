package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelpromexp "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

var (
	meterInitOnce sync.Once

	// 指标定义 - LLM/Request metrics
	llmRequestDuration metric.Float64Histogram
	llmFirstChunkDelay metric.Float64Histogram
	requestCounter     metric.Int64Counter
	requestDuration    metric.Float64Histogram
	retryCounter       metric.Int64Counter

	// 工具调用指标
	toolCallCounter  metric.Int64Counter
	toolCallDuration metric.Float64Histogram
	toolErrorCounter metric.Int64Counter

	// MCP 指标
	mcpConnectDuration metric.Float64Histogram

	// 配额与成本指标
	toolQuotaLimitGauge      metric.Int64Gauge
	toolQuotaUsedGauge       metric.Int64Gauge
	toolQuotaExceededCounter metric.Int64Counter
	toolCostCounter          metric.Float64Counter
	modelCostCounter         metric.Float64Counter

	// 检索器指标
	retrieverHitsCounter   metric.Int64Counter
	retrieverMissesCounter metric.Int64Counter
	retrieverLatency       metric.Float64Histogram

	// Token 使用指标
	modelTokenUsageCounter metric.Int64Counter

	// 回测准确率指标
	backtestAccuracyGauge metric.Float64Gauge

	serverStartTime = time.Now()
)

// StartMetricsExporter 根据环境变量启动 Prometheus 导出器并暴露 HTTP 服务。
// - 如果 METRICS_EXPORTER != "prometheus"，不执行任何操作，返回 (nil, nil)。
// - 开启时在 METRICS_PROMETHEUS_ADDR（默认 ":9464"）启动 HTTP 服务：
//   - /metrics: 标准 Prometheus 抓取端点 (promhttp.Handler)
//   - /healthz: 运行健康检查 (状态、运行时间与时间戳)
//   - /livez: 存活探测端点
//   - /readyz: 就绪检查 (验证数据库连接等就绪态)
//
// 返回值：shutdownFunc, error。shutdownFunc 用于在应用退出时优雅关闭 HTTP server。
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

	addr := os.Getenv("METRICS_PROMETHEUS_ADDR")
	if addr == "" {
		addr = ":9464"
	}
	mux := http.NewServeMux()

	// 1. Prometheus 核心指标抓取接口
	mux.Handle("/metrics", promhttp.Handler())

	// 2. 健康探针端点 /healthz 与 /livez
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":         "UP",
			"uptime_seconds": time.Since(serverStartTime).Seconds(),
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ALIVE"}`))
	})

	// 3. 就绪探测端点 /readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dbReady := db.Dao != nil
		if dbReady {
			if sqlDB, err := db.Dao.DB(); err == nil {
				if err := sqlDB.Ping(); err != nil {
					dbReady = false
				}
			} else {
				dbReady = false
			}
		}

		if dbReady {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "READY",
				"db":     "connected",
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "NOT_READY",
				"db":     "disconnected",
			})
		}
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 启动 server（后台 goroutine）
	go func() {
		logger.SugaredLogger.Infof("Metrics & Probe HTTP server listening on %s (routes: /metrics, /healthz, /livez, /readyz)", addr)
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

		// Agent request metrics (mode: react|plan_execute)
		requestCounter, err = meter.Int64Counter(
			"agent_requests_total",
			metric.WithDescription("Agent 请求总次数"),
			metric.WithUnit("{request}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 requestCounter 失败: %v", err)
		}

		requestDuration, err = meter.Float64Histogram(
			"agent_request_duration_seconds",
			metric.WithDescription("Agent 请求端到端耗时(秒)"),
			metric.WithUnit("s"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 requestDuration 失败: %v", err)
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

		toolErrorCounter, err = meter.Int64Counter(
			"agent_tool_errors_total",
			metric.WithDescription("Agent 工具调用错误总次数"),
			metric.WithUnit("{error}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 toolErrorCounter 失败: %v", err)
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

		toolQuotaLimitGauge, err = meter.Int64Gauge(
			"agent_tool_quota_limit",
			metric.WithDescription("Agent 工具调用配额上限限制"),
			metric.WithUnit("{limit}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 toolQuotaLimitGauge 失败: %v", err)
		}

		toolQuotaUsedGauge, err = meter.Int64Gauge(
			"agent_tool_quota_used",
			metric.WithDescription("Agent 今日工具调用累计使用量"),
			metric.WithUnit("{call}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 toolQuotaUsedGauge 失败: %v", err)
		}

		toolQuotaExceededCounter, err = meter.Int64Counter(
			"agent_tool_quota_exceeded_total",
			metric.WithDescription("Agent 工具调用超出预算被拦截次数"),
			metric.WithUnit("{event}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 toolQuotaExceededCounter 失败: %v", err)
		}

		toolCostCounter, err = meter.Float64Counter(
			"agent_tool_cost_usd_total",
			metric.WithDescription("Agent 外部工具与模型调用累计估算成本(美元)"),
			metric.WithUnit("USD"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 toolCostCounter 失败: %v", err)
		}

		modelCostCounter, err = meter.Float64Counter(
			"agent_model_cost_usd_total",
			metric.WithDescription("Agent 模型调用累计估算成本(美元)"),
			metric.WithUnit("USD"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 modelCostCounter 失败: %v", err)
		}

		// Retriever metrics
		retrieverHitsCounter, err = meter.Int64Counter(
			"agent_retriever_hits_total",
			metric.WithDescription("Agent 检索器命中次数"),
			metric.WithUnit("{hit}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 retrieverHitsCounter 失败: %v", err)
		}

		retrieverMissesCounter, err = meter.Int64Counter(
			"agent_retriever_misses_total",
			metric.WithDescription("Agent 检索器未命中次数"),
			metric.WithUnit("{miss}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 retrieverMissesCounter 失败: %v", err)
		}

		retrieverLatency, err = meter.Float64Histogram(
			"agent_retriever_latency_ms",
			metric.WithDescription("Agent 检索器查询延迟(毫秒)"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 retrieverLatency 失败: %v", err)
		}

		// Token usage metrics
		modelTokenUsageCounter, err = meter.Int64Counter(
			"agent_model_token_usage_total",
			metric.WithDescription("Agent 模型 Token 使用总量"),
			metric.WithUnit("{token}"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 modelTokenUsageCounter 失败: %v", err)
		}

		backtestAccuracyGauge, err = meter.Float64Gauge(
			"agent_backtest_accuracy",
			metric.WithDescription("Agent 回测准确率"),
			metric.WithUnit("ratio"),
		)
		if err != nil {
			logger.SugaredLogger.Warnf("创建 backtestAccuracyGauge 失败: %v", err)
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

// RecordQuotaUsage 记录配额使用量变化
func RecordQuotaUsage(ctx context.Context, toolName string, amount int64) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("tool_name", toolName),
	}

	if toolQuotaUsedGauge != nil {
		toolQuotaUsedGauge.Record(ctx, amount, metric.WithAttributes(attrs...))
	}
}

// RecordQuotaExceeded 记录配额/成本超限拦截事件
func RecordQuotaExceeded(ctx context.Context, toolName string, quotaType string) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("tool_name", toolName),
		attribute.String("quota_type", quotaType),
	}

	if toolQuotaExceededCounter != nil {
		toolQuotaExceededCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordToolCost 记录工具调用估算成本累加 (美元)
func RecordToolCost(ctx context.Context, toolName string, costUSD float64) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("tool_name", toolName),
	}

	if toolCostCounter != nil && costUSD > 0 {
		toolCostCounter.Add(ctx, costUSD, metric.WithAttributes(attrs...))
	}
}

// RecordQuotaLimits 记录当前生效的各项配额上限值
func RecordQuotaLimits(ctx context.Context, runBudget, dailyBudget int64, dailyCostUSD float64) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	if toolQuotaLimitGauge != nil {
		toolQuotaLimitGauge.Record(ctx, runBudget, metric.WithAttributes(attribute.String("type", "run_budget")))
		toolQuotaLimitGauge.Record(ctx, dailyBudget, metric.WithAttributes(attribute.String("type", "daily_budget")))
	}
}

// SetBacktestAccuracy 设置回测准备率指标
func SetBacktestAccuracy(ctx context.Context, skillID uint, accuracy float64) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("skill_id", strconv.FormatUint(uint64(skillID), 10)),
	}

	if backtestAccuracyGauge != nil {
		backtestAccuracyGauge.Record(ctx, accuracy, metric.WithAttributes(attrs...))
	}
}

// RecordAgentRequest 记录 Agent 请求（包含模式：react|plan_execute）
func RecordAgentRequest(ctx context.Context, mode string, success bool, duration time.Duration) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("mode", mode),
		attribute.Bool("success", success),
	}

	if requestCounter != nil {
		requestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if requestDuration != nil {
		requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
}

// RecordToolCallV2 记录工具调用（扩展版本，支持错误类型）
func RecordToolCallV2(ctx context.Context, toolName string, success bool, duration time.Duration, errType string) {
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
	if !success && toolErrorCounter != nil && errType != "" {
		errAttrs := append(attrs, attribute.String("error_type", errType))
		toolErrorCounter.Add(ctx, 1, metric.WithAttributes(errAttrs...))
	}
}

// RecordRetrieverHit 记录检索器命中/未命中及延迟
func RecordRetrieverHit(ctx context.Context, retrieverName string, hit bool, latency time.Duration) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("retriever", retrieverName),
	}

	if hit {
		if retrieverHitsCounter != nil {
			retrieverHitsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
	} else {
		if retrieverMissesCounter != nil {
			retrieverMissesCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
	}
	if retrieverLatency != nil && latency > 0 {
		retrieverLatency.Record(ctx, float64(latency.Milliseconds()), metric.WithAttributes(attrs...))
	}
}

// RecordTokenUsage 记录模型 Token 使用量
func RecordTokenUsage(ctx context.Context, modelName string, promptTokens, completionTokens int64) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("model", modelName),
	}

	if modelTokenUsageCounter != nil {
		if promptTokens > 0 {
			modelTokenUsageCounter.Add(ctx, promptTokens, metric.WithAttributes(append(attrs, attribute.String("type", "prompt"))...))
		}
		if completionTokens > 0 {
			modelTokenUsageCounter.Add(ctx, completionTokens, metric.WithAttributes(append(attrs, attribute.String("type", "completion"))...))
		}
	}
}

// RecordEmbeddingLatency 记录嵌入模型延迟
func RecordEmbeddingLatency(ctx context.Context, modelName string, latency time.Duration) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("model", modelName),
	}
	if retrieverLatency != nil && latency > 0 {
		retrieverLatency.Record(ctx, float64(latency.Milliseconds()), metric.WithAttributes(attrs...))
	}
}
