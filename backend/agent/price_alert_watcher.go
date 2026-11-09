package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"github.com/go-kratos/blades"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// PriceAlertAgentWatcher 价格告警与 Agent 联动监测器
type PriceAlertAgentWatcher struct {
	ctx           context.Context
	wailsCtx      context.Context
	stopChan      chan struct{}
	running       bool
	mu            sync.Mutex
	alertCooldown map[string]time.Time // 防频繁触发冷却
}

var globalWatcher *PriceAlertAgentWatcher
var watcherOnce sync.Once
var watcherRunningAgentCount int32 // 当前正在运行的 Agent 数量

// PriceAlertBroadcastHook 价格告警与 AI 异动分析跨端广播钩子（Web / SSE 兼容）
var PriceAlertBroadcastHook func(event string, data any)

// GetPriceAlertWatcher 获取单例
func GetPriceAlertWatcher() *PriceAlertAgentWatcher {
	watcherOnce.Do(func() {
		globalWatcher = &PriceAlertAgentWatcher{
			stopChan:      make(chan struct{}),
			alertCooldown: make(map[string]time.Time),
		}
	})
	return globalWatcher
}

// Start 启动后台价格巡检
func (w *PriceAlertAgentWatcher) Start(wailsCtx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.wailsCtx = wailsCtx
	w.ctx = context.Background()
	w.mu.Unlock()

	logger.SugaredLogger.Info("🚀 [PriceAlertAgentWatcher] 价格预警与 Agent 触发式分析引擎已启动")

	go w.watchLoop()
}

func (w *PriceAlertAgentWatcher) watchLoop() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			logger.SugaredLogger.Info("[PriceAlertAgentWatcher] 停止监测")
			return
		case <-ticker.C:
			w.checkAlerts()
		}
	}
}

// checkAlerts 检查自选股中的预警条件
func (w *PriceAlertAgentWatcher) checkAlerts() {
	stockApi := data.NewStockDataApi()
	followedPtr := stockApi.GetFollowList(0)
	if followedPtr == nil || len(*followedPtr) == 0 {
		return
	}
	followed := *followedPtr

	for _, s := range followed {
		// 如果没有设置目标预警价或涨跌幅，跳过
		if s.AlarmPrice <= 0 && s.AlarmChangePercent == 0 {
			continue
		}

		// 检查冷却时间（同一股票 15 分钟内不重复自动触发 Agent）
		w.mu.Lock()
		lastTime, exists := w.alertCooldown[s.StockCode]
		if exists && time.Since(lastTime) < 15*time.Minute {
			w.mu.Unlock()
			continue
		}
		w.mu.Unlock()

		// 获取最新实时行情
		quotes, err := stockApi.GetStockCodeRealTimeData(s.StockCode)
		if err != nil || quotes == nil || len(*quotes) == 0 {
			continue
		}
		curr := (*quotes)[0]
		price, _ := strconv.ParseFloat(curr.Price, 64)
		changePercent := curr.ChangePercent

		triggered := false
		reason := ""

		// 条件 1：突破/跌破预警价
		if s.AlarmPrice > 0 {
			if price >= s.AlarmPrice {
				triggered = true
				reason = fmt.Sprintf("价格突破目标价 %.2f（当前现价: %.2f）", s.AlarmPrice, price)
			}
		}

		// 条件 2：涨跌幅超阈值
		if !triggered && s.AlarmChangePercent != 0 {
			if s.AlarmChangePercent > 0 && changePercent >= s.AlarmChangePercent {
				triggered = true
				reason = fmt.Sprintf("涨幅达到 +%.2f%%（当前涨幅: +%.2f%%）", s.AlarmChangePercent, changePercent)
			} else if s.AlarmChangePercent < 0 && changePercent <= s.AlarmChangePercent {
				triggered = true
				reason = fmt.Sprintf("跌幅触发 %.2f%%（当前跌幅: %.2f%%）", s.AlarmChangePercent, changePercent)
			}
		}

		if triggered {
			w.mu.Lock()
			w.alertCooldown[s.StockCode] = time.Now()
			w.mu.Unlock()

			logger.SugaredLogger.Warnf("🔔 [价格预警触发] %s(%s): %s，正在唤醒 Agent 进行实时推理...", s.Name, s.StockCode, reason)

			// 异步唤醒 Agent 开展快速归因与行情推演
			go w.triggerAgentAnalysis(s.StockCode, s.Name, reason, price, changePercent)
		}
	}
}

// triggerAgentAnalysis 驱动 Agent 进行突发事件深度归因分析并向前端推送
func (w *PriceAlertAgentWatcher) triggerAgentAnalysis(code, name, reason string, price, changePercent float64) {
	// 全局闸门：并发 Agent 实例数超过上限时跳过，避免价格预警高峰期同时调起几十个 Agent
	// 推垮后端或造成内存压力（每个 Agent 会加载 100+ 工具及其描述，初始占用较大）。
	const maxConcurrentAgent = 2
	if cur := atomic.AddInt32(&watcherRunningAgentCount, 1); cur > maxConcurrentAgent {
		atomic.AddInt32(&watcherRunningAgentCount, -1)
		logger.SugaredLogger.Warnf("⚠️ 跳过价格预警 Agent 触发：当前并发 %d 超过上限 %d（股票=%s）", cur-1, maxConcurrentAgent, code)
		return
	}
	defer atomic.AddInt32(&watcherRunningAgentCount, -1)

	prompt := fmt.Sprintf("【突发价格预警】股票 %s(%s) 刚刚触发了价格告警：%s。当前最新价: %.2f，今日涨跌幅: %.2f%%。请调用相关工具，分析本次异动的主要驱动因素（最新突发消息、资金流入或盘口大单），并给出简明操作建议。",
		name, code, reason, price, changePercent)

	// 获取当前激活的 AI 配置（优先使用用户设置的当前活动配置，避免失效配置报错）
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil || len(settingConfig.AiConfigs) == 0 {
		return
	}
	// 统一走 ResolveModelForced（全局默认 → 回落），要求密钥有效。
	var mainConfig data.AIConfig
	if res, err := data.ResolveModelForced(0, true); err == nil {
		mainConfig = res.Config
	}
	if mainConfig.ID == 0 {
		for _, cfgPtr := range settingConfig.AiConfigs {
			if cfgPtr != nil && strings.TrimSpace(cfgPtr.ApiKey) != "" {
				mainConfig = *cfgPtr
				break
			}
		}
	}
	if mainConfig.ID == 0 {
		return
	}

	// 调起 Agent 生成简报
	agentInstance := GetStockAiAgent(w.ctx, mainConfig, prompt, string(AgentModeReact), nil)
	if agentInstance == nil {
		return
	}

	invocation := &blades.Invocation{
		Message:     blades.UserMessage(prompt),
		Instruction: blades.SystemMessage(personalityPrompt()),
		Streamable:  false,
	}

	var md strings.Builder
	for msg, err := range agentInstance.Agent.Run(w.ctx, invocation) {
		if err != nil {
			logger.SugaredLogger.Errorf("Agent 自动分析遇到错误: %v", err)
			break
		}
		if msg != nil && msg.Role == blades.RoleAssistant {
			if text := msg.Text(); text != "" {
				md.WriteString(text)
			}
		}
	}
	aiAnalysisText := md.String()

	// 解析告警分析中是否包含行动建议 (Agent-Action)
	cleanedAnalysis, actions := types.ParseAndExtractActions(aiAnalysisText)
	// ⚠️ 注意：不要在后台监控中直接 EventsEmit "agent:action"，否则会导致用户正在正常使用时
	// 界面不时突然弹出全屏 K线图或诊断抽屉！Action 应该放在 stockPriceAlertTriggered 载荷中供用户在通知里点击！

	alertPayload := map[string]any{
		"code":          code,
		"name":          name,
		"reason":        reason,
		"price":         price,
		"changePercent": changePercent,
		"time":          time.Now().Format("15:04:05"),
		"aiAnalysis":    cleanedAnalysis,
		"actions":       actions,
	}

	// 1. 将告警事件以及 Agent 推理结果广播给前端 UI（仅在 Wails 原生桌面上下文有效）
	if w.wailsCtx != nil && w.wailsCtx.Value("events") != nil {
		runtime.EventsEmit(w.wailsCtx, "stockPriceAlertTriggered", alertPayload)
	}

	// 2. 将告警事件广播至 Web SSE 客户端
	if PriceAlertBroadcastHook != nil {
		PriceAlertBroadcastHook("stockPriceAlertTriggered", alertPayload)
	}
}
