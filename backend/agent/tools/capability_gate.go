package tools

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	bladesTools "github.com/go-kratos/blades/tools"
)

// ── 能力闸门（Capability Gate）────────────────────────────────────────────
//
// 目的：把「不要在这个流程里调用 X」从**提示词请求**升级为**代码级约束**。
//
// 提示词的问题是：模型可以遵守，也可以不遵守；同一个问题这次调了、下次没调。
// 闸门的做法更简单也更可靠——「这个流程不该用的工具，根本不出现在注入列表里」。
//
// 灰度策略：默认**影子模式**（只计算并记录差异，不实际裁剪）。
// 配置 BLADES_STOCK_GATE_ENFORCE=1 后才真正生效，便于先对比差异再打开。

// Gate 一次请求的能力约束。
type Gate struct {
	// ForbidGroups 整组不可见（比提示词"严禁"更强：根本不在列表里）
	ForbidGroups []ToolGroup `json:"forbidGroups,omitempty"`
	// ForbidTools 单个工具不可见
	ForbidTools []string `json:"forbidTools,omitempty"`
	// RequireGroups 必须可见（用于技能声明的 tool_groups）
	RequireGroups []ToolGroup `json:"requireGroups,omitempty"`
	// Deferred 表示该闸门默认只做影子对比，不实际裁剪。
	//
	// 适用于「迁移**已有**纪律」——那会改变既有行为，必须先看差异再打开。
	// 新建的 mode 闸门（如 pipeline）不设此位：它的行为差异就是迁移目标本身，
	// 且不影响任何既有调用方，因此直接生效。
	Deferred bool `json:"deferred,omitempty"`
}

// Active 该闸门当前是否应当实际裁剪（而非仅影子对比）。
func (g Gate) Active() bool {
	if g.IsZero() {
		return false
	}
	return !g.Deferred || GateEnforceEnabled()
}

// IsZero 是否是空闸门（不施加任何约束）。
func (g Gate) IsZero() bool {
	return len(g.ForbidGroups) == 0 && len(g.ForbidTools) == 0 && len(g.RequireGroups) == 0
}

// forbidGroupSet / forbidToolSet 便于快速判断。
func (g Gate) forbidGroupSet() map[ToolGroup]bool {
	m := make(map[ToolGroup]bool, len(g.ForbidGroups))
	for _, gr := range g.ForbidGroups {
		m[gr] = true
	}
	return m
}

func (g Gate) forbidToolSet() map[string]bool {
	m := make(map[string]bool, len(g.ForbidTools))
	for _, t := range g.ForbidTools {
		m[strings.TrimSpace(t)] = true
	}
	return m
}

// ApplyGate 按闸门裁剪工具列表，返回保留的工具与被移除的工具名。
//
// 保留顺序与原列表一致（reorderToolsByRank 之后还会重排，故此处无需关心顺序）。
func ApplyGate(in []bladesTools.Tool, g Gate) (kept []bladesTools.Tool, removed []string) {
	if g.IsZero() {
		return in, nil
	}
	fg := g.forbidGroupSet()
	ft := g.forbidToolSet()
	for _, t := range in {
		blocked := ""
		if ft[t.Name()] {
			blocked = "tool"
		} else if c, ok := CapabilityOf(t.Name()); ok && fg[c.Group] {
			blocked = "group:" + string(c.Group)
		}
		if blocked != "" {
			removed = append(removed, t.Name()+"("+blocked+")")
			continue
		}
		kept = append(kept, t)
	}
	return kept, removed
}

// AgentModePipeline 五阶流水线专用运行模式。
//
// 为什么需要它：前端此前对「五阶流水线」与「普通对话」都传空 mode
// （后端按 classifyComplexity 自动判定），导致**无法区分两者**，
// 于是流水线提示词里的「严禁调用 WebSearch 从外网抓取」只能靠提示词约束。
// 现在前端为流水线入口显式传 "pipeline"，后端即可**精确**配置闸门，
// 而不影响普通对话（它们没有这个闸门，仍可联网）。
//
// 行为差异（已核实，仅此一项）：流水线当前走 classifyComplexity →
// React/PlanExecute（其提示词不含 Direct 标记），本模式同样给全量工具，
// 只是移除外网搜索。故升级到本模式不会减少任何既有可用能力。
const AgentModePipeline = "pipeline"

// pipelineForbiddenExternalSearch 流水线禁用的**外网检索**工具。
//
// 只禁这两个，不禁整个 news_research 组 —— 组内的 QueryStockNewsTool /
// GetNewsListData / StockNotice 等是**一手内置**数据源，流水线仍应可用。
// 这正是提示词原文的口径：「严禁调用 WebSearch 从外网抓取过时新闻碎片」。
var pipelineForbiddenExternalSearch = []string{"WebSearch", "FetchWebPage"}

// GateForMode 内置默认策略：按 Agent 运行模式给出的能力约束。
//
// 注意：Direct 模式在 GetStockAiAgent 中本就是零工具，这里显式声明同一意图，
// 是为了让约束**可读、可测、可覆盖**，而不是散在调用点的 switch 里
// ——将来若有人给 Direct 加了工具，闸门仍能兜住。该闸门标为 Deferred：
// direct 的行为现状已满足约束，先影子观察差异，不做无谓的行为收紧。
func GateForMode(mode string) Gate {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	// 注意：这里用字符串字面量而非 agent.AgentMode ——
	// tools 不能被 agent 反向导入（会成环），而 agentMode 的取值由 agent 包定义
	// （direct / react / plan_execute / pipeline / 空=auto）。
	case "direct":
		return Gate{
			ForbidGroups: []ToolGroup{
				GroupNewsResearch, // 外部搜索/网页抓取
				GroupMCPServer,    // MCP 管理类（运维面）
			},
			Deferred: true,
		}
	case AgentModePipeline:
		// 五阶流水线：移除外网检索（提示词纪律 → 代码级约束）。
		// 非 Deferred：新 mode 只影响流水线自身，不改变任何既有调用方。
		return Gate{
			ForbidTools: pipelineForbiddenExternalSearch,
		}
	default:
		return Gate{}
	}
}

var (
	gateOverrideMu    sync.RWMutex
	gateOverrideRaw   string // 已解析的原始 env 值，用于判断是否需要重新解析
	gateOverride      Gate
	gateOverrideErr   error
	gateOverrideFirst = true // 首次解析时才记录错误，避免日志风暴
)

// loadGateOverride 从环境变量 BLADES_STOCK_GATE_OVERRIDE 读取额外闸门（JSON）。
// 例：{"forbidTools":["WebSearch"],"forbidGroups":["mcp_server"]}
//
// 按原始 env 值缓存解析结果：env 未变时零开销；env 变更时重新解析。
// （旧实现用 sync.Once，导致配置在整个进程生命周期内只读一次 ——
//
//	既不可测试，也会让「改了 env 但不重启」的场景静默失效。）
func loadGateOverride() Gate {
	raw := strings.TrimSpace(os.Getenv("BLADES_STOCK_GATE_OVERRIDE"))

	gateOverrideMu.RLock()
	if !gateOverrideFirst && raw == gateOverrideRaw {
		defer gateOverrideMu.RUnlock()
		return gateOverride
	}
	gateOverrideMu.RUnlock()

	gateOverrideMu.Lock()
	defer gateOverrideMu.Unlock()

	if !gateOverrideFirst && raw == gateOverrideRaw {
		return gateOverride
	}
	gateOverrideRaw = raw
	gateOverrideFirst = false
	if raw == "" {
		gateOverride = Gate{}
		gateOverrideErr = nil
		return gateOverride
	}
	var g Gate
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		// 解析失败只忽略，绝不让进程起不来；错误经 GateOverrideError() 可取
		gateOverrideErr = err
		gateOverride = Gate{}
		return gateOverride
	}
	gateOverrideErr = nil
	gateOverride = g
	return gateOverride
}

// GateOverrideError 返回闸门覆盖配置的解析错误（无则为 nil）。
func GateOverrideError() error { return gateOverrideErr }

// GateEnforceEnabled 闸门是否真正生效（默认 false = 影子模式）。
func GateEnforceEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("BLADES_STOCK_GATE_ENFORCE")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("BLADES_STOCK_GATE_ENFORCE")), "true")
}

// EffectiveGate 合并内置策略与覆盖配置，得到本次请求生效的闸门。
func EffectiveGate(mode string) Gate {
	g := GateForMode(mode)
	o := loadGateOverride()
	g.ForbidGroups = append(append([]ToolGroup{}, g.ForbidGroups...), o.ForbidGroups...)
	g.ForbidTools = append(append([]string{}, g.ForbidTools...), o.ForbidTools...)
	g.RequireGroups = append(append([]ToolGroup{}, g.RequireGroups...), o.RequireGroups...)
	// 覆盖配置一律视为 Deferred：它是运维旋钮，先看差异再决定是否真正收紧。
	if !o.IsZero() {
		g.Deferred = true
	}
	return g
}
