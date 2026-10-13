package agent

import (
	"context"
	"sync"
	"time"

	"blades-stock/backend/logger"
	bladesTools "github.com/go-kratos/blades/tools"
)

// 工具调用单次估算成本表 (USD)
var defaultToolCosts = map[string]float64{
	"WebSearch":              0.010,
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

// CheckAndConsume 记录一次工具调用的使用情况
// 限额检查已移除，始终允许调用，仅累加使用量供监控。
func (qm *ToolQuotaManager) CheckAndConsume(ctx context.Context, toolName string) (float64, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

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
			RecordToolCall(ctx, toolName, err == nil, duration)
			return result, err
		}),
	)
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
