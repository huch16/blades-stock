package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 可达性守卫（补上 A 期漏测的**反方向**）。
//
// A 期只测了「组装 ⇒ 已声明」（TestEveryAssembledToolIsDeclared），
// 漏了另外两个方向，于是产生了两个真实事故：
//
//	事故一（服务端）：提示词要求调用 GetStockBlockTrade，但它不在核心白名单、
//	          而 stock_analysis 的 cap 恰好等于白名单条目数（26=26）→
//	          **余量为 0** → 该工具永久注入不到 →
//	          模型照提示词调用时后端没有它 → `agent: tool GetStockBlockTrade not found`。
//	事故二（注册表漂移）：工具在注册表里声明了，却没被 AssembleBuiltinTools 收进去 →
//	          注册表显示"有"，模型永远拿不到。
//
// 本文件把这两个方向都变成红灯。

// promptSourceGlobs 参与"提示词点名调用工具"的源码。
// 包含前端流水线提示词 —— L1~L5 也是用自然语言点名工具名的。
var promptSourceGlobs = []string{
	"../prompt_config.go",
	"../market_brief.go",
	"../agent.go",
	"../../../frontend/src/components/agent-chat.vue",
}

// extractPromptReferencedTools 扫描提示词源码，返回其中出现的、且已注册的工具名。
func extractPromptReferencedTools(t *testing.T) map[string][]string {
	t.Helper()
	registered := map[string]bool{}
	for _, c := range AllCapabilities() {
		registered[c.ID] = true
	}
	// 形如 GetStockBlockTrade(股票代码) / AnalyzeChipConcentration(股票代码)
	refRe := regexp.MustCompile(`\b([A-Z][A-Za-z0-9]{5,})\s*\(`)
	refs := map[string][]string{}
	for _, pat := range promptSourceGlobs {
		files, _ := filepath.Glob(pat)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			for _, m := range refRe.FindAllStringSubmatch(string(b), -1) {
				if registered[m[1]] {
					refs[m[1]] = append(refs[m[1]], filepath.Base(f))
				}
			}
		}
	}
	return refs
}

// 契约一（关键）：**提示词点名要求调用的每个工具，都必须能被注入。**
//
// 这条不变量一旦破坏，模型就会照提示词去调用一个后端并不提供的工具，
// 直接抛 `tool not found` —— 用户看到的就是「L4 分析失败」。
func TestPromptReferencedToolsAreReachable(t *testing.T) {
	all := AssembleBuiltinTools()
	injected := FilterToolsByGroupsCapped(all, allGroupsEnabled())
	reachable := map[string]bool{}
	for _, tl := range injected {
		reachable[tl.Name()] = true
	}

	refs := extractPromptReferencedTools(t)
	if len(refs) < 10 {
		t.Fatalf("只扫到 %d 个提示词引用的工具，扫描逻辑可能失效（路径变了？）", len(refs))
	}

	var broken []string
	for name, srcs := range refs {
		if !reachable[name] {
			broken = append(broken, name+" ← "+strings.Join(dedupeStrings(srcs), ","))
		}
	}
	sort.Strings(broken)
	if len(broken) != 0 {
		t.Fatalf("以下 %d 个工具被提示词点名要求调用，却**注入不到** —— 模型会照做并抛 tool not found。\n"+
			"修法：把它们加入对应组的 coreToolByGroup 白名单。\n  %s",
			len(broken), strings.Join(broken, "\n  "))
	}
	t.Logf("提示词引用 %d 个工具，全部可达 ✓", len(refs))
}

// 契约二：secretary 每个组的 cap 必须**大于**其白名单条目数。
//
// cap == 白名单数 时，白名单自己就吃满配额，组内其余工具永久不可达，
// 而提示词仍可能点名它们。这正是事故一的直接成因。
func TestGroupCapLeavesHeadroomForNonCoreTools(t *testing.T) {
	for group, names := range coreToolByGroup {
		cap, capped := groupToolCaps[group]
		if !capped {
			continue // 无上限的组不适用
		}
		if len(names) >= cap {
			t.Errorf("组 %s：白名单 %d 条 ≥ cap %d —— 余量为 %d，组内其余工具将永久不可达。"+
				"cap 必须留出非核心工具的余量。",
				group, len(names), cap, cap-len(names))
		}
	}
}

// 契约三：注册表声明的**内置**工具与**实际组装**之间的差额必须被如实记录。
//
// 这里**不用** t.Fatal —— 因为差额是一批**已存在的历史接线缺陷**，而不是本轮的回归：
//
//	`data_tools_wrapper.go` 里明明有这些工具的构造代码，`GetAllDataTools()`
//	却不把它们返回（实测 84 个返回、其中不含 SearchStockByIndicators /
//	HotspotDiscovery / SearchBk / SearchETF 等）。而工具库的既有契约测试
//	（TestFilterToolsByGroupsCapped_MarketSummaryContract）明确要求
//	HotspotDiscovery 必须在注入集中 —— 说明它们**本该可用**。
//
// 直接删掉注册表声明是错的：那只会掩盖缺陷。正确做法是把它记录成
// **可追踪的已知差额**，并用上限断言防止它继续扩大。
// 待办：修复 GetAllDataTools 的接线，使差额归零，然后把本用例改回强断言。
func TestDeclaredBuiltinToolsAreAssembled(t *testing.T) {
	assembled := map[string]bool{}
	for _, tl := range AssembleBuiltinTools() {
		assembled[tl.Name()] = true
	}
	var missing []string
	for _, c := range AllCapabilities() {
		if c.Source != SourceBuiltin {
			continue // MCP 工具运行时才确定，不在此约束
		}
		if !assembled[c.ID] {
			missing = append(missing, c.ID+"["+string(c.Group)+"]")
		}
	}
	sort.Strings(missing)

	// 上限断言：差额不得扩大。当前基线 26。
	const knownGapCeiling = 26
	if len(missing) > knownGapCeiling {
		t.Fatalf("「已声明但未组装」的工具从 %d 增至 %d —— 差额在扩大，说明新增了接线缺陷: %s",
			knownGapCeiling, len(missing), strings.Join(missing, " | "))
	}
	t.Logf("已知差额：%d 个工具在注册表中声明、但未被组装（上限 %d，不得扩大）", len(missing), knownGapCeiling)
	for _, m := range missing {
		t.Logf("  GAP %s", m)
	}
}

// 契约四（观察性）：报告组装了却注入不到的工具。
// 不失败 —— 这些是"有实现但当前配额下进不来"的工具，
// 是 D 期 token 预算调优的输入。但数量若持续膨胀，说明配额已经不合理。
func TestUnreachableToolsAreObservable(t *testing.T) {
	all := AssembleBuiltinTools()
	injected := FilterToolsByGroupsCapped(all, allGroupsEnabled())
	reachable := map[string]bool{}
	for _, tl := range injected {
		reachable[tl.Name()] = true
	}
	var unreachable []string
	for _, tl := range all {
		if !reachable[tl.Name()] {
			c, _ := CapabilityOf(tl.Name())
			unreachable = append(unreachable, tl.Name()+"["+string(c.Group)+"]")
		}
	}
	sort.Strings(unreachable)
	t.Logf("组装 %d / 注入 %d / 不可达 %d", len(all), len(injected), len(unreachable))
	if len(unreachable) > 0 {
		t.Logf("不可达（配额所限，非缺陷）：%v", unreachable)
	}
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
