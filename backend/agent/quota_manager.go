package agent

import (
	"context"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/logger"
	bladesTools "github.com/go-kratos/blades/tools"
)

// 工具调用单次估算成本表 (USD)
var defaultToolCosts = map[string]float64{
	"WebSearch":             0.010,
	"FetchWebPage":          0.005,
	"RunVectorizedBacktest": 0.008,
	"VectorGridSearch":      0.015,
	"DiagnoseStock":         0.005,
	"GetStockPriceInfo":     0.001,
	"GetQueryStockNews":     0.002,
	"GetFinancialReports":   0.004,
	"GetGraphQuery":         0.003,
}

const defaultFallbackToolCost = 0.002

// RunQuota 上下文单次运行调用计数器
type runQuotaKeyType struct{}

var runQuotaKey = runQuotaKeyType{}

type RunQuota struct {
	mu           sync.Mutex
	currentCalls int64
}

// WithRunQuota 将单次运行的调用计数器注入 Context
func WithRunQuota(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	rq := &RunQuota{}
	return context.WithValue(ctx, runQuotaKey, rq)
}

// GetRunQuotaFromContext 从 Context 中提取当前单次调用的计数器对象
func GetRunQuotaFromContext(ctx context.Context) *RunQuota {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(runQuotaKey).(*RunQuota); ok {
		return v
	}
	return nil
}

// QuotaStatus 供前端或监控端查询的工具调用使用情况摘要
type QuotaStatus struct {
	Date         string  `json:"date"`
	DailyCalls   int64   `json:"dailyCalls"`
	DailyCostUSD float64 `json:"dailyCostUSD"`
}

// ToolQuotaManager 线程安全的全局工具调用使用情况统计器
type ToolQuotaManager struct {
	mu           sync.Mutex
	currentDate  string
	dailyCalls   int64
	dailyCostUSD float64
}

var (
	defaultQuotaManager *ToolQuotaManager
	quotaOnce           sync.Once
)

// GetToolQuotaManager 获取全局配额管理器单例
func GetToolQuotaManager() *ToolQuotaManager {
	quotaOnce.Do(func() {
		defaultQuotaManager = &ToolQuotaManager{
			currentDate: time.Now().Format("2006-01-02"),
		}
	})
	return defaultQuotaManager
}

// GetToolCost 获取指定工具的预估调用成本 (USD)
func (qm *ToolQuotaManager) GetToolCost(toolName string) float64 {
	if cost, ok := defaultToolCosts[toolName]; ok {
		return cost
	}
	return defaultFallbackToolCost
}

// CheckAndConsume 记录一次工具调用的使用情况并记账到可观测指标。
// 注意：本管理器只有计量，没有可执行的上限值（全仓没有配置任何工具
// 调用次数/成本上限），所以这里恢复的是「指标产生」，不是硬限流；
// 硬限流在 ModelProviderWrapper 一侧（卡片自带 RateLimitPerSecond /
// DailyRequestQuota / DailyCostQuota 配置）。
func (qm *ToolQuotaManager) CheckAndConsume(ctx context.Context, toolName string) (float64, error) {
	qm.mu.Lock()

	// 1. 跨天自动重置
	today := time.Now().Format("2006-01-02")
	if qm.currentDate != today {
		qm.currentDate = today
		qm.dailyCalls = 0
		qm.dailyCostUSD = 0
	}

	toolCost := qm.GetToolCost(toolName)

	// 2. 记录单次会话调用计数（不再设上限）
	if rq := GetRunQuotaFromContext(ctx); rq != nil {
		rq.mu.Lock()
		rq.currentCalls++
		rq.mu.Unlock()
	}

	// 3. 累加使用量
	qm.dailyCalls++
	qm.dailyCostUSD += toolCost
	qm.mu.Unlock()

	// 4. 记账到可观测指标（锁外：OTEL 记录不参与配额状态机）。
	//    此前只累加内存计数、从不调这两个函数，配额/成本指标永不产生。
	RecordQuotaUsage(ctx, toolName, 1)
	RecordToolCost(ctx, toolName, toolCost)

	return toolCost, nil
}

// GetQuotaStatus 查询当前工具调用累计使用情况
func (qm *ToolQuotaManager) GetQuotaStatus() QuotaStatus {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if qm.currentDate != today {
		qm.currentDate = today
		qm.dailyCalls = 0
		qm.dailyCostUSD = 0
	}

	return QuotaStatus{
		Date:         qm.currentDate,
		DailyCalls:   qm.dailyCalls,
		DailyCostUSD: qm.dailyCostUSD,
	}
}

// ResetDailyQuota 重置今日使用情况计数器
func (qm *ToolQuotaManager) ResetDailyQuota() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.currentDate = time.Now().Format("2006-01-02")
	qm.dailyCalls = 0
	qm.dailyCostUSD = 0
}

func WrapToolWithQuota(original bladesTools.Tool) bladesTools.Tool {
	toolName := original.Name()
	return bladesTools.NewTool(
		toolName,
		original.Description(),
		bladesTools.HandleFunc(func(ctx context.Context, input string) (string, error) {
			qm := GetToolQuotaManager()
			cost, _ := qm.CheckAndConsume(ctx, toolName)
			logger.SugaredLogger.Debugf("[ToolQuota] 工具 [%s] 使用情况已记录 (预估成本: $%.4f)", toolName, cost)
			start := time.Now()
			result, err := original.Handle(ctx, input)
			duration := time.Since(start)
			// V2（带错误类型）：失败时 agent_tool_errors_total 按
			// error_type 分桶计数；V1 仍由 agent_api.go 的流式工具
			// 观测点调用，这里只记一次，不重复计数。
			RecordToolCallV2(ctx, toolName, err == nil, duration, classifyToolErrorType(err))
			return result, err
		}),
	)
}

// classifyToolErrorType 把工具错误映射到 RecordToolCallV2 的粗粒度
// error_type 分桶（成功时返回 ""，V2 不计错误）。
func classifyToolErrorType(err error) string {
	if err == nil {
		return ""
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "timeout") || strings.Contains(s, "deadline exceeded"):
		return "timeout"
	case strings.Contains(s, "429") || strings.Contains(s, "rate limit") || strings.Contains(s, "rate_limit"):
		return "rate_limit"
	case strings.Contains(s, "401") || strings.Contains(s, "403") || strings.Contains(s, "unauthorized") || strings.Contains(s, "forbidden"):
		return "auth"
	default:
		return "unknown"
	}
}

// WrapToolsWithQuota 批量为工具列表注入配额与成本控制器
func WrapToolsWithQuota(toolList []bladesTools.Tool) []bladesTools.Tool {
	if len(toolList) == 0 {
		return toolList
	}
	wrapped := make([]bladesTools.Tool, len(toolList))
	for i, t := range toolList {
		wrapped[i] = WrapToolWithQuota(t)
	}
	return wrapped
}
