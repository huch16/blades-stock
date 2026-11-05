// Package tools 的代码图谱查询工具（P3.3）。
//
// 设计目标：让 Agent 在对话中能直接查询代码图谱，
// 例如"app.go 里 NewApp 被谁调用？"→ 调 GraphQuery(action="callers", target="NewApp")
//
// 实现：
//   - 启动期懒加载 .graph/graph.json 到内存索引
//   - 支持 4 种 action：callers / callees / search / community
//   - 查询结果以 markdown 格式返回，便于 LLM 直接消费
//
// 设计要点：
//   - 不修改 graph.json（仅读），符合 P3.4 的"仅 build.ps1 触发重建"约束
//   - 文件不存在或格式错误时返回可读错误，LLM 会理解"图谱不可用"
//   - 单查询结果限制 50 项，避免 prompt 爆炸
package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/logger"
	"blades-stock/backend/paths"

	bladesTools "github.com/go-kratos/blades/tools"
)

// GraphQueryArgs 是 GraphQuery 工具的入参 schema。
type GraphQueryArgs struct {
	// Action 决定查询类型：callers | callees | search | community
	Action string `json:"action" jsonschema:"description=查询类型: callers|callees|search|community"`

	// Target 查询目标（callers/callees/community 用）；search 时为搜索关键词。
	Target string `json:"target" jsonschema:"description=查询目标，如 NewApp 或 app.go"`

	// MaxDepth 仅 callees 用：递归展开的深度（默认 1）。
	MaxDepth int `json:"max_depth,omitempty" jsonschema:"description=递归展开深度，默认 1"`
}

// graphData 内部数据结构（仅需关注最小字段）
type graphData struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

type graphNode struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Kind           string `json:"kind"`
	SourceFile     string `json:"source_file"`
	SourceLocation string `json:"source_location"`
}

type graphEdge struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Relation   string `json:"relation"`
	Confidence string `json:"confidence"`
}

// graphManifest 对应 build_graph.py 写入的 manifest.json
type graphManifest struct {
	CommitSHA     string `json:"commit_sha"`
	BuiltAt       string `json:"built_at"`
	StaleAfter    string `json:"stale_after"`
	MaxStaleDays  int    `json:"max_stale_days"`
	NodeCount     int    `json:"node_count"`
	EdgeCount     int    `json:"edge_count"`
	SchemaVersion int    `json:"schema_version"`
}

// 内存缓存 + 加载状态
var (
	graphOnce   sync.Once
	graphCache  *graphData
	graphLoaded bool
	graphErr    error
	graphPath   string

	// P4.1: 后台重建 manifest 的单飞闸 + 状态追踪
	rebuildMu     sync.Mutex
	rebuildingNow bool
	lastRebuild   rebuildStatus
)

type rebuildStatus struct {
	StartedAt time.Time
	EndedAt   time.Time
	OK        bool
	Message   string
}

// rebuildTrigger 是 P4.1 默认的重建命令调用入口，可被测试 mock。
var rebuildTrigger = func() error {
	root := paths.BaseDir()
	cmd := exec.Command("python", filepath.Join(root, "scripts", "build_graph.py"),
		"--root", root,
		"--output", filepath.Join(root, ".graph"),
		"--commit", "HEAD")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// loadGraph 懒加载 .graph/graph.json。线程安全。
// 设计：失败一次后下次调用会重试（防止 build.ps1 重建后仍是 stale 错误）。
func loadGraph() (*graphData, error) {
	graphOnce.Do(func() {
		graphPath = filepath.Join(paths.BaseDir(), ".graph", "graph.json")
		data, err := os.ReadFile(graphPath)
		if err != nil {
			graphErr = fmt.Errorf("graph.json 不存在或不可读（path=%s）: %w", graphPath, err)
			return
		}
		var g graphData
		if err := json.Unmarshal(data, &g); err != nil {
			graphErr = fmt.Errorf("graph.json 解析失败: %w", err)
			return
		}
		graphCache = &g
		graphLoaded = true
		manifestPath := filepath.Join(paths.BaseDir(), ".graph", "manifest.json")
		staleWarning := checkManifestStale(manifestPath)
		if staleWarning != "" {
			logger.SugaredLogger.Warnf("GraphQuery: %s", staleWarning)
		}
		mismatchWarning := checkCommitMismatch(manifestPath)
		if mismatchWarning != "" {
			logger.SugaredLogger.Warnf("GraphQuery: %s", mismatchWarning)
			// P4.1 用户选项 B：Warn + 后台异步重建 manifest
			triggerRebuildAsync(mismatchWarning)
		}
		logger.SugaredLogger.Infof("GraphQuery: 已加载 %d nodes / %d edges from %s",
			len(g.Nodes), len(g.Edges), graphPath)
	})
	if !graphLoaded {
		return nil, graphErr
	}
	return graphCache, nil
}

// checkManifestStale 读 manifest.json 判断是否过期。返回警告消息或空字符串。
// 过期策略：now > stale_after → 警告（不阻断，让 build.ps1 触发重建）
func checkManifestStale(manifestPath string) string {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "" // 无 manifest 不警告（不影响功能）
	}
	var m graphManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if m.StaleAfter == "" {
		return ""
	}
	staleAt, err := time.Parse(time.RFC3339, m.StaleAfter)
	if err != nil {
		return ""
	}
	if time.Now().After(staleAt) {
		return fmt.Sprintf("manifest 已过期（commit=%s, stale_after=%s）；请跑 build.ps1 触发 Rebuild-Graph",
			m.CommitSHA, m.StaleAfter)
	}
	return ""
}

// gitHeadSHA 是 git rev-parse --short HEAD 的获取函数。
// 设计为 var 供测试 mock；默认走 git CLI。
var gitHeadSHA = func() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// triggerRebuildAsync 后台异步重建 manifest（单飞：同时只跑一个）。
// 设计：发现 commit 不匹配时调用，不阻塞当前 loadGraph。
// 完成后会自动让 resetGraphCache() 失效，下次 loadGraph 重新加载。
//
// 设计要点：
//   - 单飞闸：同时只跑一个重建进程（避免开发期频繁 commit 重复跑）
//   - 非致命：重建失败仅记日志，不影响 GraphQuery 后续查询
//   - 适用于 CI：环境无 git / python 时安全降级
func triggerRebuildAsync(reason string) {
	rebuildMu.Lock()
	if rebuildingNow {
		rebuildMu.Unlock()
		logger.SugaredLogger.Debugf("GraphQuery: rebuild already in flight, skip (reason=%s)", reason)
		return
	}
	rebuildingNow = true
	status := rebuildStatus{StartedAt: time.Now()}
	rebuildMu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("GraphQuery: 后台重建 manifest panic: %v", r)
				status.OK = false
				status.Message = fmt.Sprintf("panic: %v", r)
				status.EndedAt = time.Now()
			}
			rebuildMu.Lock()
			rebuildingNow = false
			lastRebuild = status
			rebuildMu.Unlock()
			// 重建成功后让下次 loadGraph 重新读（不破坏 sync.Once 已加载的 cache，仅供下次进程重启）
			if status.OK {
				resetGraphCache()
			}
		}()
		logger.SugaredLogger.Infof("GraphQuery: 后台重建 manifest 启动（reason=%s）", reason)
		err := rebuildTrigger()
		status.EndedAt = time.Now()
		if err != nil {
			status.OK = false
			status.Message = err.Error()
			logger.SugaredLogger.Warnf("GraphQuery: 后台重建 manifest 失败: %v", err)
		} else {
			status.OK = true
			status.Message = "ok"
			logger.SugaredLogger.Infof("GraphQuery: 后台重建 manifest 完成")
		}
	}()
}

// GetRebuildStatus 供 UI / 健康检查查询重建状态。
func GetRebuildStatus() (inProgress bool, last rebuildStatus) {
	rebuildMu.Lock()
	defer rebuildMu.Unlock()
	return rebuildingNow, lastRebuild
}

// checkCommitMismatch 对比 manifest.commit_sha 与 git HEAD 短 hash。
// 返回警告消息或空字符串：
//   - manifest 不存在 / 无 commit_sha → 空（不影响功能）
//   - git CLI 不可用 → 空（静默，避免开发环境无 git 报错）
//   - 不一致 → 警告（提示用户跑 build.ps1 重建）
//
// 设计原则：仅 Warn，不阻断，避免开发期提交一次就报警。
func checkCommitMismatch(manifestPath string) string {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	var m graphManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if m.CommitSHA == "" || m.CommitSHA == "unknown" {
		return ""
	}
	head, err := gitHeadSHA()
	if err != nil || head == "" {
		return ""
	}
	if head == m.CommitSHA {
		return ""
	}
	return fmt.Sprintf(
		"manifest 与 git HEAD 不匹配：manifest=%s, HEAD=%s；建议跑 build.ps1 触发 Rebuild-Graph 重建 manifest",
		m.CommitSHA, head)
}

// resetGraphCache 仅供测试 / build.ps1 重建后调用。
func resetGraphCache() {
	graphOnce = sync.Once{}
	graphCache = nil
	graphLoaded = false
	graphErr = nil
	graphPath = ""
	// 同时清理重建状态，避免测试间互串扰
	rebuildMu.Lock()
	rebuildingNow = false
	lastRebuild = rebuildStatus{}
	rebuildMu.Unlock()
}

// GetGraphQueryTool 暴露给 Agent 的代码图谱查询入口。
// 输入：GraphQueryArgs JSON
// 输出：markdown 格式的查询结果（限 50 行）
func GetGraphQueryTool() bladesTools.Tool {
	type localArgs struct {
		Action   string `json:"action"`
		Target   string `json:"target"`
		MaxDepth int    `json:"max_depth"`
	}
	return types.NewTool(
		"GraphQuery",
		"查询本地代码图谱（.graph/graph.json）。支持 4 种 action：callers（谁引用 target）、callees（target 引用谁）、search（按标签搜索）、community（target 所在包的所有节点）。",
		map[string]*types.ParamDef{
			"action": {
				Type:     "string",
				Desc:     "查询动作：callers|callees|search|community",
				Required: true,
			},
			"target": {
				Type:     "string",
				Desc:     "目标函数名、结构体名或包名",
				Required: true,
			},
			"max_depth": {
				Type: "integer",
				Desc: "引用链最大深度（默认1）",
			},
		},
		func(input string) (string, error) {
			var args localArgs
			if err := json.Unmarshal([]byte(input), &args); err != nil {
				return "", fmt.Errorf("参数解析失败: %w", err)
			}
			g, err := loadGraph()
			if err != nil {
				return fmt.Sprintf("⚠️ 代码图谱不可用: %v", err), nil
			}
			switch args.Action {
			case "callers":
				return formatCallers(g, args.Target, 50), nil
			case "callees":
				depth := args.MaxDepth
				if depth <= 0 {
					depth = 1
				}
				return formatCallees(g, args.Target, depth, 50), nil
			case "search":
				return formatSearch(g, args.Target, 50), nil
			case "community":
				return formatCommunity(g, args.Target, 50), nil
			default:
				return "", fmt.Errorf("未知 action %q（应为 callers|callees|search|community）", args.Action)
			}
		},
	)
}

// findNodesByName 按 name 模糊匹配 node（按 id/label 后缀）
func findNodesByName(g *graphData, target string) []graphNode {
	if target == "" {
		return nil
	}
	var found []graphNode
	for _, n := range g.Nodes {
		if n.ID == target || n.Label == target ||
			strings.HasSuffix(n.ID, "#"+target) ||
			strings.HasSuffix(n.ID, "/"+target) ||
			strings.Contains(strings.ToLower(n.Label), strings.ToLower(target)) {
			found = append(found, n)
		}
	}
	return found
}

func formatCallers(g *graphData, target string, limit int) string {
	nodes := findNodesByName(g, target)
	if len(nodes) == 0 {
		return fmt.Sprintf("未找到节点 %q。建议先用 action=search 查确切名称。", target)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s 的调用者 / 引用者\n", target)
	for _, n := range nodes {
		// 找以 n.ID 为 target 的边
		var refs []graphEdge
		for _, e := range g.Edges {
			if e.Target == n.ID {
				refs = append(refs, e)
			}
		}
		fmt.Fprintf(&sb, "\n### %s (%s)\n- 来源位置: %s\n- 被引用次数: %d\n", n.ID, n.Kind, n.SourceLocation, len(refs))
		if len(refs) == 0 {
			sb.WriteString("- 无直接引用者\n")
			continue
		}
		for i, r := range refs {
			if i >= limit {
				fmt.Fprintf(&sb, "- …(省略 %d 项)\n", len(refs)-limit)
				break
			}
			fmt.Fprintf(&sb, "- `%s` (%s)\n", r.Source, r.Relation)
		}
	}
	return sb.String()
}

func formatCallees(g *graphData, target string, depth, limit int) string {
	nodes := findNodesByName(g, target)
	if len(nodes) == 0 {
		return fmt.Sprintf("未找到节点 %q。建议先用 action=search 查确切名称。", target)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s 引用的实体（深度 %d）\n", target, depth)
	visited := make(map[string]bool)
	var walk func(nodeID string, d int)
	walk = func(nodeID string, d int) {
		if visited[nodeID] || d > depth {
			return
		}
		visited[nodeID] = true
		count := 0
		for _, e := range g.Edges {
			if e.Source == nodeID {
				if count >= limit {
					fmt.Fprintf(&sb, "  - …(省略 %d 项)\n", -1)
					break
				}
				fmt.Fprintf(&sb, "  - `%s` (%s)\n", e.Target, e.Relation)
				count++
				if d < depth {
					walk(e.Target, d+1)
				}
			}
		}
	}
	for _, n := range nodes {
		fmt.Fprintf(&sb, "\n### %s (%s)\n", n.ID, n.Kind)
		walk(n.ID, 1)
	}
	return sb.String()
}

func formatSearch(g *graphData, query string, limit int) string {
	if query == "" {
		return "请提供 target 参数（搜索关键词）"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "## 搜索: %q\n", query)
	lower := strings.ToLower(query)
	count := 0
	for _, n := range g.Nodes {
		if strings.Contains(strings.ToLower(n.Label), lower) ||
			strings.Contains(strings.ToLower(n.ID), lower) {
			if count >= limit {
				fmt.Fprintf(&sb, "- …(省略更多匹配)\n")
				break
			}
			fmt.Fprintf(&sb, "- `%s` [%s] %s\n", n.ID, n.Kind, n.SourceLocation)
			count++
		}
	}
	fmt.Fprintf(&sb, "\n共匹配 ≥ %d 节点（上限 %d）\n", count, limit)
	return sb.String()
}

func formatCommunity(g *graphData, target string, limit int) string {
	nodes := findNodesByName(g, target)
	if len(nodes) == 0 {
		return fmt.Sprintf("未找到节点 %q", target)
	}
	// 取首个节点的 source_file，列出同文件所有节点
	file := nodes[0].SourceFile
	if file == "" {
		return fmt.Sprintf("节点 %q 没有 source_file，无法确定 community", target)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s 的 community（同一文件 %s）\n", target, file)
	count := 0
	for _, n := range g.Nodes {
		if n.SourceFile == file {
			if count >= limit {
				fmt.Fprintf(&sb, "- …(省略更多)\n")
				break
			}
			fmt.Fprintf(&sb, "- `%s` [%s] %s\n", n.ID, n.Kind, n.SourceLocation)
			count++
		}
	}
	fmt.Fprintf(&sb, "\n共 %d 节点\n", count)
	return sb.String()
}
