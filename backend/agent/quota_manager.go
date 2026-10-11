package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"blades-stock/backend/agent/telemetry"
	bladesTools "github.com/go-kratos/blades/tools"
)

var (
	// ErrRunBudgetExceeded 单次推理工具调用配额超限错误
	ErrRunBudgetExceeded = errors.New("单次运行工具调用预算已达上限")
	// ErrDailyQuotaExceeded 今日全局工具调用总配额超限错误
	ErrDailyQuotaExceeded = errors.New("今日外部工具调用配额已耗尽")
	// ErrDailyCostExceeded 今日外部工具与API调用预估总成本超限错误
	ErrDailyCostExceeded = errors.New("今日外部API调用预估成本预算已超限")
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

// RunQuota 上下文单次运行预算追踪器
type runQuotaKeyType struct{}

var runQuotaKey = runQuotaKeyType{}

type RunQuota struct {
	mu           sync.Mutex
	maxCalls     int64
	currentCalls int64
}

// WithRunQuota 将单次执行的最大工具调用配额注入 Context
func WithRunQuota(ctx context.Context, maxCalls int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	rq := &RunQuota{
		maxCalls: maxCalls,
	}
	return context.WithValue(ctx, runQuotaKey, rq)
}

// GetRunQuotaFromContext 从 Context 中提取当前单次调用的预算对象
func GetRunQuotaFromContext(ctx context.Context) *RunQuota {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(runQuotaKey).(*RunQuota); ok {
		return v
	}
	return nil
}

// QuotaStatus 供前端或监控端查询的配额与预算统计摘要
type QuotaStatus struct {
	Date            string  `json:"date"`
	DailyCalls      int64   `json:"dailyCalls"`
	DailyCallsLimit int64   `json:"dailyCallsLimit"`
	DailyCostUSD    float64 `json:"dailyCostUSD"`
	DailyCostLimit  float64 `json:"dailyCostLimit"`
	RunBudgetLimit  int64   `json:"runBudgetLimit"`
	BlockedCalls    int64   `json:"blockedCalls"`
}

// ToolQuotaManager 线程安全的全局工具调用配额与成本管理器
type ToolQuotaManager struct {
	mu           sync.Mutex
	currentDate  string
	dailyCalls   int64
	dailyCostUSD float64
	blockedCalls int64
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

// CheckAndConsume 检查并核销一次工具调用的配额与成本
// 若超限则返回对应的结构化错误并记录可观测指标
func (qm *ToolQuotaManager) CheckAndConsume(ctx context.Context, toolName string) (float64, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// 1. 跨天自动重置
	today := time.Now().Format("2006-01-02")
	if qm.currentDate != today {
		qm.currentDate = today
		qm.dailyCalls = 0
		qm.dailyCostUSD = 0
		qm.blockedCalls = 0
	}

	settingConfig := data.GetSettingConfig()
	runLimit := settingConfig.GetToolRunBudget()
	dailyCallsLimit := settingConfig.GetToolDailyBudget()
	dailyCostLimit := settingConfig.GetToolDailyCostUSD()
	toolCost := qm.GetToolCost(toolName)

	// 2. 检查单次会话运行配额
	if rq := GetRunQuotaFromContext(ctx); rq != nil {
		rq.mu.Lock()
		if rq.maxCalls > 0 && rq.currentCalls >= rq.maxCalls {
			rq.mu.Unlock()
			qm.blockedCalls++
			RecordQuotaExceeded(ctx, toolName, "run_calls")
			return 0, fmt.Errorf("%w: 单次推理已调用 %d 次 (上限 %d 次)", ErrRunBudgetExceeded, rq.currentCalls, rq.maxCalls)
		}
		rq.currentCalls++
		rq.mu.Unlock()
	}

	// 3. 检查全局每日调用次数配额
	if dailyCallsLimit > 0 && qm.dailyCalls >= dailyCallsLimit {
		qm.blockedCalls++
		RecordQuotaExceeded(ctx, toolName, "daily_calls")
		return 0, fmt.Errorf("%w: 今日已累计调用 %d 次 (上限 %d 次)", ErrDailyQuotaExceeded, qm.dailyCalls, dailyCallsLimit)
	}

	// 4. 检查全局每日 API 成本预算
	if dailyCostLimit > 0 && (qm.dailyCostUSD+toolCost) > dailyCostLimit {
		qm.blockedCalls++
		RecordQuotaExceeded(ctx, toolName, "daily_cost")
		return 0, fmt.Errorf("%w: 今日已累计产生成本 $%.4f (预计添加 $%.4f 后超过上限 $%.2f)", ErrDailyCostExceeded, qm.dailyCostUSD, toolCost, dailyCostLimit)
	}

	// 5. 校验通过，原子累计
	qm.dailyCalls++
	qm.dailyCostUSD += toolCost

	// 6. 上报 OTel 业务度量
	RecordToolCost(ctx, toolName, toolCost)
	RecordQuotaUsage(ctx, toolName, qm.dailyCalls)
	RecordQuotaLimits(ctx, runLimit, dailyCallsLimit, dailyCostLimit)

	return toolCost, nil
}

// GetQuotaStatus 查询当前配额与累计使用情况
func (qm *ToolQuotaManager) GetQuotaStatus() QuotaStatus {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if qm.currentDate != today {
		qm.currentDate = today
		qm.dailyCalls = 0
		qm.dailyCostUSD = 0
		qm.blockedCalls = 0
	}

	settingConfig := data.GetSettingConfig()
	return QuotaStatus{
		Date:            qm.currentDate,
		DailyCalls:      qm.dailyCalls,
		DailyCallsLimit: settingConfig.GetToolDailyBudget(),
		DailyCostUSD:    qm.dailyCostUSD,
		DailyCostLimit:  settingConfig.GetToolDailyCostUSD(),
		RunBudgetLimit:  settingConfig.GetToolRunBudget(),
		BlockedCalls:    qm.blockedCalls,
	}
}

// ResetDailyQuota 重置今日配额与成本计数器
func (qm *ToolQuotaManager) ResetDailyQuota() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.currentDate = time.Now().Format("2006-01-02")
	qm.dailyCalls = 0
	qm.dailyCostUSD = 0
	qm.blockedCalls = 0
}

func WrapToolWithQuota(original bladesTools.Tool) bladesTools.Tool {
	toolName := original.Name()
	return bladesTools.NewTool(
		toolName,
		original.Description(),
		bladesTools.HandleFunc(func(ctx context.Context, input string) (string, error) {
			qm := GetToolQuotaManager()
			cost, err := qm.CheckAndConsume(ctx, toolName)
			if err != nil {
				logger.SugaredLogger.Warnf("🛡️ [ToolQuota] 工具 [%s] 触发配额拦截: %v", toolName, err)
				resp := map[string]string{
					"error":      "TOOL_QUOTA_EXCEEDED",
					"code":       "QUOTA_EXCEEDED",
					"tool":       toolName,
					"message":    fmt.Sprintf("工具 [%s] 已触发配额或预算拦截: %v。请根据当前已有上下文与分析数据直接推导输出结论，不要再次重试调用受限工具。", toolName, err),
					"suggestion": "请直接利用目前已掌握的数据形成最终研报或交易策略总结。",
				}
				bytes, _ := json.Marshal(resp)
				return string(bytes), nil
			}
			logger.SugaredLogger.Debugf("[ToolQuota] 工具 [%s] 预算核销成功 (预估成本: $%.4f)", toolName, cost)
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
