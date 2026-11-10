package tools

import (
	"encoding/json"
	"sort"
	"sync"

	bladesTools "github.com/go-kratos/blades/tools"
)

// ── 按 Token 预算裁剪（D 期：把「个数上限」升级为「成本上限」）────────────
//
// 为什么需要它（实测基线，全部 128 个内置工具）：
//
//	名称+描述合计 19,081 字符 ≈ 4,770 tokens（参数 schema 另计，实际更高）。
//	个数上限（groupToolCaps）只能压工具**数量**，压不掉成本：
//	  119 个候选 → 按个数 cap 过滤后 100 个，成本仍约 4,014 tokens。
//	也就是说，哪怕一个工具都不砍，只要换成长描述的工具，成本就会悄悄超支，
//	而个数上限对此毫无感知。
//
// 这里改为按**实测成本**预算：按 Tier 优先级（core > extended > niche）依次注入，
// 累计成本达到预算即停。个数上限仍保留为硬顶（先到者为准）。

// charsPerToken 折算 token 的经验系数。
//
// 对中英混排的技术文本取 3.6：纯英文约 4 字符/token，中文约 1.5~2 字符/token，
// 本仓库工具描述以中文为主、夹杂大量英文标识符。取**保守**值（偏小 → 高估成本），
// 宁可少注入一个工具，也不超预算。
// 注意：这里刻意用整数除法（成本高估方向），不用 float 再取整，
// 以免在 32 位平台上出现不必要的转换开销。见估算说明。
const charsPerTokenTenths = 36 // 3.6 字符/token，用「十分之一字符」的整数表示

var (
	schemaLenMu sync.RWMutex
	// schemaLenCache 按工具名缓存其 JSON schema 的字符数。
	// 工具在每次请求都会重新构造，若不缓存会对同一份 schema 反复做 JSON 序列化。
	schemaLenCache = map[string]int{}
)

// toolSchemaLen 返回工具输入 schema 的 JSON 字符数（按名缓存）。
func toolSchemaLen(t bladesTools.Tool) int {
	name := t.Name()
	schemaLenMu.RLock()
	n, ok := schemaLenCache[name]
	schemaLenMu.RUnlock()
	if ok {
		return n
	}
	n = 0
	if s := t.InputSchema(); s != nil {
		if b, err := json.Marshal(s); err == nil {
			n = len(b)
		}
	}
	schemaLenMu.Lock()
	schemaLenCache[name] = n
	schemaLenMu.Unlock()
	return n
}

// estimateToolTokens 估算单个工具定义的 token 成本。
// 计入名称、描述与输入 schema —— 这些都会随请求全量发给模型。
func estimateToolTokens(t bladesTools.Tool) int {
	cost := len(t.Name()) + len(t.Description()) + toolSchemaLen(t)
	if cost <= 0 {
		return 1
	}
	// cost / 3.6 === cost*10/36；用整数运算实现，结果略偏大（保守）
	return (cost*10 + charsPerTokenTenths/2) / charsPerTokenTenths
}

// EstimateTokens 估算整组工具的成本（导出供日志与测试使用）。
func EstimateTokens(ts []bladesTools.Tool) int {
	total := 0
	for _, t := range ts {
		total += estimateToolTokens(t)
	}
	return total
}

// groupTokenBudget 每组允许注入的 token 预算。未列出的组沿用个数上限或全量。
//
// **取值依据（实测全部 128 个内置工具，勿再拍脑袋）**：
//
//	组                工具数   总成本   core成本  core占比
//	base                12     1751        0      0%
//	stock_analysis      35     4179     3093     74%
//	market              12     2078      447     22%
//	screening            4     1050     1008     96%
//	money_flow           5      342      314     92%
//	news_research       20     1844     1089     59%
//	ai_analysis         19     2634     2373     90%
//	operations           4      413      329     80%
//	project              8     1093        0      0%
//	mcp_server           9      903        0      0%
//
// 预算口径 = core×1.15 + extended×0.35（取整到 50）：
//
//	· core 高占比组（screening/money_flow/ai_analysis）几乎全保 —— 它们是主力能力；
//	· core 低占比组（market/project）明显收紧 —— 那些组里大量低频工具，
//	  本来就不该每次请求都全量下发；
//	· base/mcp_server 无 core：预算仅容纳少量常用项。
//
// 教训：第一版预算拍脑袋定 420 给 screening，实测 FilterStocks **单个就 783**
// （描述极长），结果整组只剩它一个、其余全被裁 —— 预算比单工具成本还小，
// 「每个组至少保留一个」的兜底反而变成了「只保留最贵的那个」。
var groupTokenBudget = map[ToolGroup]int{
	GroupBase:          600,
	GroupStockAnalysis: 3900,
	GroupMarket:        1050,
	GroupScreening:     1150,
	GroupMoneyFlow:     350,
	GroupNewsResearch:  1500,
	GroupAIAnalysis:    2800,
	GroupOperations:    400,
	GroupProject:       350,
	GroupMCPServer:     350,
}

// FilterByTokenBudget 在**已按组过滤**的列表上，按 Tier 优先级与 token 预算裁剪。
//
// 行为约定：
//
//	· TierCore 优先保留；预算不足时先砍 niche，再砍 extended；
//	· 个数上限（groupToolCaps）继续作为硬顶，两者取先到者；
//	· 未映射到任何组的工具不受本函数约束（它们在分组过滤阶段已全量注入）；
//	· 每个组的**第一个**工具总是放行 —— 否则当预算小于单个工具成本时，
//	  该组会一个都不剩。
func FilterByTokenBudget(in []bladesTools.Tool, groups map[ToolGroup]bool) (kept []bladesTools.Tool, dropped []string) {
	if len(in) == 0 {
		return nil, nil
	}
	// 按 Tier 稳定排序：core 在前，extended 次之，niche 最后；同 Tier 保持原顺序。
	ordered := make([]bladesTools.Tool, len(in))
	copy(ordered, in)
	sort.SliceStable(ordered, func(i, j int) bool {
		ti, oki := CapabilityOf(ordered[i].Name())
		tj, okj := CapabilityOf(ordered[j].Name())
		if !oki || !okj {
			return false
		}
		return ti.Tier < tj.Tier
	})

	perGroupCount := map[ToolGroup]int{}
	perGroupTok := map[ToolGroup]int{}
	for _, t := range ordered {
		c, ok := CapabilityOf(t.Name())
		if !ok {
			kept = append(kept, t) // 未登记组：不占预算与个数（与分组过滤器语义一致）
			continue
		}
		if !groups[c.Group] {
			continue // 防御性：上游应已过滤
		}
		cost := estimateToolTokens(t)

		if cap, capped := groupToolCaps[c.Group]; capped && perGroupCount[c.Group] >= cap {
			dropped = append(dropped, t.Name()+"(count_cap:"+string(c.Group)+")")
			continue
		}
		if budget, has := groupTokenBudget[c.Group]; has {
			if perGroupTok[c.Group] > 0 && perGroupTok[c.Group]+cost > budget {
				dropped = append(dropped, t.Name()+"(token_budget:"+string(c.Group)+")")
				continue
			}
		}
		kept = append(kept, t)
		perGroupCount[c.Group]++
		perGroupTok[c.Group] += cost
	}
	return kept, dropped
}

// TokenBudgetReport 便于日志与测试观察的预算报告。
type TokenBudgetReport struct {
	ToolCount int
	TokenEst  int
	ByGroup   map[ToolGroup]int
	Budgets   map[ToolGroup]int
	Dropped   []string
}

// ReportTokenBudget 生成一份可读的预算报告（不改变输入）。
func ReportTokenBudget(in []bladesTools.Tool, groups map[ToolGroup]bool) TokenBudgetReport {
	kept, dropped := FilterByTokenBudget(in, groups)
	rep := TokenBudgetReport{
		ToolCount: len(kept),
		TokenEst:  EstimateTokens(kept),
		ByGroup:   map[ToolGroup]int{},
		Budgets:   map[ToolGroup]int{},
		Dropped:   dropped,
	}
	for _, t := range kept {
		if c, ok := CapabilityOf(t.Name()); ok {
			rep.ByGroup[c.Group] += estimateToolTokens(t)
		}
	}
	for g, b := range groupTokenBudget {
		rep.Budgets[g] = b
	}
	return rep
}

// ── 开关（与能力闸门共用，便于一起灰度）─────────────────────────────────

// TokenBudgetEnabled token 预算是否真正生效（默认 false = 影子模式）。
// 与 GateEnforceEnabled 使用同一个环境变量：两者都是「收紧模型能力」的改动，
// 应当一起对比差异、一起打开，避免出现"闸门已生效但预算没生效"的半态。
func TokenBudgetEnabled() bool { return GateEnforceEnabled() }
