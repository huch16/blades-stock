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

// GateForMode 内置默认策略：按 Agent 运行模式给出的能力约束。
//
// 注意：Direct 模式在 GetStockAiAgent 中本就是零工具（agent.go 的 switch），
// 这里显式声明同一意图，是为了让约束**可读、可测、可覆盖**，
// 而不是散在调用点的 switch 里——将来若有人给 Direct 加了工具，闸门仍能兜住。
func GateForMode(mode string) Gate {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	// 注意：这里用字符串字面量而非 agent.AgentMode ——
	// tools 不能被 agent 反向导入（会成环），而 agentMode 的取值
	// 由 agent 包定义（direct / react / plan_execute / 空=auto）。
	case "direct":
		return Gate{
			ForbidGroups: []ToolGroup{
				GroupNewsResearch, // 外部搜索/网页抓取
				GroupMCPServer,    // MCP 管理类（运维面）
			},
		}
	default:
		return Gate{}
	}
}

var (
	gateOverrideOnce sync.Once
	gateOverride     Gate
)

// loadGateOverride 从环境变量 BLADES_STOCK_GATE_OVERRIDE 读取额外闸门（JSON）。
// 例：{"forbidTools":["WebSearch"],"forbidGroups":["mcp_server"]}
// 解析失败只记日志并忽略——绝不因此让进程起不来。
func loadGateOverride() Gate {
	gateOverrideOnce.Do(func() {
		raw := strings.TrimSpace(os.Getenv("BLADES_STOCK_GATE_OVERRIDE"))
		if raw == "" {
			return
		}
		var g Gate
		if err := json.Unmarshal([]byte(raw), &g); err != nil {
			// 该包不使用 logger（保持零依赖）；调用方可通过 GateOverrideError() 取错
			gateOverrideErr = err
			return
		}
		gateOverride = g
	})
	return gateOverride
}

var gateOverrideErr error

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
	return g
}
