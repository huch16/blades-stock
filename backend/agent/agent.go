// Package agent provides the AI agent pipeline for stock analysis.
package agent

import (
	"blades-stock/backend/agent/tools"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/go-kratos/blades"
	bladesmc "github.com/go-kratos/blades/contrib/mcp"
	bladesTools "github.com/go-kratos/blades/tools"
)

type AgentMode string

const (
	AgentModeReact       AgentMode = "react"
	AgentModePlanExecute AgentMode = "plan_execute"
)

type AgentInstance struct {
	Agent    blades.Agent
	Mode     AgentMode
	AIConfig data.AIConfig
	AllTools []bladesTools.Tool
}

func classifyComplexity(question string) AgentMode {
	lowerQ := strings.ToLower(question)

	simplePatterns := []string{
		"今天", "当前", "最新", "现在", "实时",
		"查询", "查一下", "帮我查", "告诉我",
		"什么是", "是什么", "多少", "几号",
		"是否", "有没有", "能不能",
		"开盘价", "收盘价", "最高价", "最低价",
		"停牌", "复牌", "上市",
		"代码", "名称", "简称",
	}

	for _, p := range simplePatterns {
		if strings.Contains(lowerQ, p) {
			wordCount := len([]rune(question))
			if wordCount < 30 {
				return AgentModeReact
			}
		}
	}

	complexPatterns := []string{
		"全面分析", "综合分析", "深度分析", "详细分析",
		"多维度", "全方位", "系统分析",
		"投资建议", "操作建议", "买卖建议",
		"对比分析", "比较分析", "横向对比",
		"行业分析", "赛道分析", "产业链",
		"投资组合", "资产配置", "仓位",
		"风险评估", "风险分析",
		"研究报告", "投资报告",
		"宏观", "政策", "周期",
	}

	for _, p := range complexPatterns {
		if strings.Contains(lowerQ, p) {
			return AgentModePlanExecute
		}
	}

	toolGroupCount := 0
	groups := tools.ClassifyQuestion(question)
	for range groups {
		toolGroupCount++
	}
	if toolGroupCount >= 4 {
		return AgentModePlanExecute
	}

	wordCount := len([]rune(question))
	if wordCount > 80 {
		return AgentModePlanExecute
	}

	return AgentModeReact
}

func GetStockAiAgent(ctx context.Context, aiConfig data.AIConfig, question string, agentMode string, matchedSkills []models.Skill, extraTools ...bladesTools.Tool) *AgentInstance {
	logger.SugaredLogger.Infof("GetStockAiAgent aiConfig: %+v", redactAIConfig(aiConfig))

	modelProvider, err := createChatModel(ctx, aiConfig)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return nil
	}

	allTools := getToolsByQuestion(question, matchedSkills)
	allTools = append(allTools, extraTools...)

	var mode AgentMode
	switch AgentMode(agentMode) {
	case AgentModeReact:
		mode = AgentModeReact
	case AgentModePlanExecute:
		mode = AgentModePlanExecute
	default:
		mode = classifyComplexity(question)
	}

	logger.SugaredLogger.Infof("Agent mode selected: %s (user=%q), question=%q, tools=%d", mode, agentMode, question, len(allTools))

	maxStep := len(allTools)*2 + 10
	if maxStep < 30 {
		maxStep = 30
	}

	agent, err := blades.NewAgent("stock-agent",
		blades.WithModel(modelProvider),
		blades.WithTools(allTools...),
		blades.WithMaxIterations(maxStep),
		blades.WithInstruction(personalityPrompt()),
		blades.WithMiddleware(ReasoningMiddleware),
	)
	if err != nil {
		logger.SugaredLogger.Errorf("创建Blades Agent失败: %v", err)
		return nil
	}

	return &AgentInstance{
		Agent:    agent,
		Mode:     mode,
		AIConfig: aiConfig,
		AllTools: allTools,
	}
}

func personalityPrompt() string {
	return GetPromptTemplates().Personality
}

func ReasoningMiddleware(next blades.Handler) blades.Handler {
	return blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
		question := inv.Message.Text()

		mode, _ := inv.Message.Metadata["agent_mode"].(string)
		availableTools, _ := inv.Message.Metadata["available_tools"].([]string)

		var thinkingPrompt string
		tpls := GetPromptTemplates()
		if mode == "plan_execute" {
			thinkingPrompt = tpls.PlanExecuteThinking
			thinkingPrompt = renderAvailableTools(thinkingPrompt, availableTools)
		} else {
			thinkingPrompt = tpls.ReactThinking
		}

		newMsg := blades.UserMessage(question + "\n\n" + thinkingPrompt)
		newMsg.Metadata = inv.Message.Metadata
		inv.Message = newMsg

		return next.Handle(ctx, inv)
	})
}

// renderAvailableTools replaces the {{AVAILABLE_TOOLS}} placeholder in a
// thinking prompt with the actual tools injected for this invocation, so
// plan_execute guidance references the real inventory instead of hardcoded
// tool names the model may not actually have.
func renderAvailableTools(tpl string, toolNames []string) string {
	if !strings.Contains(tpl, "{{AVAILABLE_TOOLS}}") {
		return tpl
	}
	inventory := "（本会话已注入对应工具，按需调用）"
	if len(toolNames) > 0 {
		inventory = strings.Join(toolNames, "、")
	}
	return strings.Replace(tpl, "{{AVAILABLE_TOOLS}}", inventory, 1)
}

// buildSkillPrompt uses bge-m3 semantic matching (via EmbeddingService) to select relevant skills.
// When EmbeddingService is nil or embedding fails, falls back to keyword matching.
// Target prompt size: ~800 chars per skill (Directive mode), ~2000+ chars (full SystemPrompt mode).
func buildSkillPrompt(question string, embedSvc *EmbeddingService) string {
	matched := matchSkills(question, embedSvc)
	if len(matched) == 0 {
		return ""
	}
	return BuildShortSkillPrompt(matched)
}

// matchSkills returns up to 2 enabled skills relevant to the question. It is
// the single source of truth used both for building the skill prompt and for
// merging each matched skill's required tool groups into the agent's tool set.
func matchSkills(question string, embedSvc *EmbeddingService) []models.Skill {
	if len(data.NewSkillApi().GetEnabledSkills()) == 0 {
		return nil
	}
	skillEmbedSvc := GetSkillEmbeddingService(embedSvc)
	return skillEmbedSvc.MatchSkills(question, 2)
}

// BuildSkillPromptForTest exports buildSkillPrompt for testing purposes.
func BuildSkillPromptForTest(question string) string {
	return buildSkillPrompt(question, nil)
}

func GetAllTools() []bladesTools.Tool {
	var allTools []bladesTools.Tool
	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())
	allTools = append(allTools, tools.GetWebSearchTool())
	allTools = append(allTools, tools.GetFetchWebPageTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetProjectTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	allTools = append(allTools, tools.GetBacktestStatsTool())
	allTools = append(allTools, tools.GetBacktestDetailTool())
	allTools = append(allTools, tools.GetRunBacktestTool())
	allTools = append(allTools, tools.GetVectorBacktestTool())
	allTools = append(allTools, tools.GetVectorGridSearchTool())

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	return allTools
}

func getToolsByQuestion(question string, matchedSkills []models.Skill) []bladesTools.Tool {
	var allTools []bladesTools.Tool

	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())
	allTools = append(allTools, tools.GetWebSearchTool())
	allTools = append(allTools, tools.GetFetchWebPageTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetProjectTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	allTools = append(allTools, tools.GetBacktestStatsTool())
	allTools = append(allTools, tools.GetBacktestDetailTool())
	allTools = append(allTools, tools.GetRunBacktestTool())
	allTools = append(allTools, tools.GetVectorBacktestTool())
	allTools = append(allTools, tools.GetVectorGridSearchTool())

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	groups := tools.ClassifyQuestion(question)
	merged := mergeSkillToolGroups(groups, matchedSkills)
	filtered := tools.FilterToolsByGroupsCapped(allTools, groups)

	logger.SugaredLogger.Infof("tool grouping: question=%q, matched_groups=%v, skill_merged_groups=%v, total=%d, filtered=%d",
		question, groupNames(groups), merged, len(allTools), len(filtered))

	// 进化回灌：按 agent_tool_ranks 实测成功率/延迟重排注入顺序。
	return reorderToolsByRank(filtered)
}

// mergeSkillToolGroups force-enables each matched skill's required tool groups
// (Skill.ToolGroups) in addition to keyword classification. Without this, a
// matched skill's directive can never execute because the tools it depends on
// (e.g. money_flow/news_research for the ETF skill) were filtered out.
// Returns the list of group names contributed by skills (for logging).
func mergeSkillToolGroups(groups map[tools.ToolGroup]bool, matchedSkills []models.Skill) []string {
	var skillGroups []string
	for _, skill := range matchedSkills {
		if skill.ToolGroups == "" {
			continue
		}
		for g := range tools.ParseToolGroups(skill.ToolGroups) {
			if !groups[g] {
				skillGroups = append(skillGroups, string(g))
			}
			groups[g] = true
		}
	}
	return skillGroups
}

func groupNames(groups map[tools.ToolGroup]bool) []string {
	var names []string
	for g := range groups {
		names = append(names, string(g))
	}
	return names
}

type mcpServerResult struct {
	tools []bladesTools.Tool
	name  string
}

var (
	mcpToolsCache     []bladesTools.Tool
	mcpToolsCacheTime time.Time
	mcpToolsCacheMu   sync.Mutex
)

func getMCPTools() []bladesTools.Tool {
	mcpToolsCacheMu.Lock()
	if mcpToolsCache != nil && time.Since(mcpToolsCacheTime) < 5*time.Minute {
		cached := mcpToolsCache
		mcpToolsCacheMu.Unlock()
		return cached
	}
	mcpToolsCacheMu.Unlock()
	var servers []models.MCPServer
	err := db.Dao.Where("enable = ?", true).Find(&servers).Error
	if err != nil {
		logger.SugaredLogger.Debugf("获取MCP服务器列表失败（表不存在是正常现象，首次运行会自动建表）: %v", err)
		return nil
	}

	skillServerIDs := getSkillMCPServerIDs()
	for _, id := range skillServerIDs {
		found := false
		for _, s := range servers {
			if s.ID == id {
				found = true
				break
			}
		}
		if !found {
			var server models.MCPServer
			if err := db.Dao.Where("id = ? AND enable = ?", id, true).First(&server).Error; err == nil {
				servers = append(servers, server)
			}
		}
	}

	if len(servers) == 0 {
		return nil
	}

	results := make(chan mcpServerResult, len(servers))
	var wg sync.WaitGroup

	for _, server := range servers {
		wg.Add(1)
		go func(srv models.MCPServer) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("panic connecting MCP server [%s]: %v", srv.Name, r)
					results <- mcpServerResult{tools: nil, name: srv.Name}
				}
			}()
			tools := connectMCPServer(srv)
			results <- mcpServerResult{tools: tools, name: srv.Name}
		}(server)
	}

	wg.Wait()
	close(results)

	var mcpTools []bladesTools.Tool
	for r := range results {
		if len(r.tools) > 0 {
			mcpTools = append(mcpTools, r.tools...)
		}
	}
	mcpToolsCacheMu.Lock()
	mcpToolsCache = mcpTools
	mcpToolsCacheTime = time.Now()
	mcpToolsCacheMu.Unlock()
	return mcpTools
}

func connectMCPServer(server models.MCPServer) []bladesTools.Tool {
	var transport bladesmc.TransportType
	var endpoint string
	var command string
	var args []string
	var env map[string]string
	var headers map[string]string

	switch {
	case server.Command != "":
		transport = bladesmc.TransportStdio
		command = server.Command
		args = parseMCPArgs(server.Args)
		env = parseMCPEnv(server.Env)
	case server.URL != "":
		transport = bladesmc.TransportHTTP
		endpoint = server.URL
		headers = parseMCPHeaders(server.Headers)
	default:
		logger.SugaredLogger.Warnf("MCP服务器 [%s] 未配置URL或Command，跳过", server.Name)
		return nil
	}

	mcpClient, err := bladesmc.NewClient(bladesmc.ClientConfig{
		Name:      server.Name,
		Transport: transport,
		Endpoint:  endpoint,
		Command:   command,
		Args:      args,
		Env:       env,
		Headers:   headers,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建MCP客户端失败 [%s]: %v", server.Name, err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = mcpClient.Connect(ctx)
	cancel()
	if err != nil {
		logger.SugaredLogger.Errorf("MCP连接失败 [%s]: %v", server.Name, err)
		return nil
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	toolList, err := mcpClient.Resolve(ctx)
	cancel()
	if err != nil {
		logger.SugaredLogger.Errorf("获取MCP工具列表失败 [%s]: %v", server.Name, err)
		return nil
	}

	if len(toolList) == 0 {
		return nil
	}

	// 根据 DB 中工具启用状态过滤
	enabledTools := getEnabledToolNames(server.ID)
	if len(enabledTools) > 0 {
		filtered := make([]bladesTools.Tool, 0, len(toolList))
		for _, t := range toolList {
			if enabledTools[t.Name()] {
				filtered = append(filtered, t)
			}
		}
		logger.SugaredLogger.Infof("从MCP服务器 [%s] 使用工具白名单过滤后加载了 %d/%d 个工具", server.Name, len(filtered), len(toolList))
		toolList = filtered
	} else {
		logger.SugaredLogger.Infof("从MCP服务器 [%s] 加载了 %d 个工具（未配置白名单）", server.Name, len(toolList))
	}

	for i := range toolList {
		toolList[i] = wrapMCPToolHandler(toolList[i], server.Name)
	}
	return toolList
}

func getEnabledToolNames(serverID uint) map[string]bool {
	tools := data.NewMCPServerApi().GetEnabledToolsByServerID(serverID)
	if len(tools) == 0 {
		return nil
	}
	m := make(map[string]bool, len(tools))
	for _, t := range tools {
		m[t.ToolName] = true
	}
	return m
}

// wrapMCPToolHandler wraps an MCP tool's handler to return call errors as structured JSON
// responses instead of propagating them as Go errors. This prevents a single bad MCP tool call
// from killing the entire sub-agent (the LLM sees the error text and can retry or switch tools).
func wrapMCPToolHandler(original bladesTools.Tool, serverName string) bladesTools.Tool {
	return bladesTools.NewTool(
		original.Name(),
		original.Description(),
		bladesTools.HandleFunc(func(ctx context.Context, input string) (string, error) {
			result, err := original.Handle(ctx, input)
			if err != nil {
				errMsg := fmt.Sprintf("⚠️ MCP [%s] 工具调用失败: %v", serverName, err)
				logger.SugaredLogger.Warnf("MCP工具错误 [%s/%s]: %v", serverName, original.Name(), err)
				errResp, _ := json.Marshal(map[string]interface{}{
					"error":   true,
					"message": errMsg,
					"tool":    original.Name(),
				})
				return string(errResp), nil
			}
			return result, nil
		}),
		bladesTools.WithInputSchema(original.InputSchema()),
		bladesTools.WithOutputSchema(original.OutputSchema()),
	)
}

func parseMCPArgs(argsStr string) []string {
	if argsStr == "" {
		return nil
	}
	var args []string
	if err := sonic.Unmarshal([]byte(argsStr), &args); err != nil {
		logger.SugaredLogger.Warnf("解析MCP Args失败: %v", err)
		return nil
	}
	return args
}

func parseMCPEnv(envStr string) map[string]string {
	if envStr == "" {
		return nil
	}
	var env map[string]string
	if err := sonic.Unmarshal([]byte(envStr), &env); err != nil {
		logger.SugaredLogger.Warnf("解析MCP Env失败: %v", err)
		return nil
	}
	return env
}

func parseMCPHeaders(headersStr string) map[string]string {
	if headersStr == "" {
		return nil
	}
	var headers map[string]string
	if err := sonic.Unmarshal([]byte(headersStr), &headers); err != nil {
		logger.SugaredLogger.Warnf("解析MCP Headers失败: %v", err)
		return nil
	}
	return headers
}

func getSkillMCPServerIDs() []uint {
	skills := data.NewSkillApi().GetEnabledSkills()
	var ids []uint
	seen := make(map[uint]bool)
	for _, skill := range skills {
		if skill.MCPServerIDs == "" {
			continue
		}
		for _, id := range data.NewSkillApi().GetMCPServerIDs(&skill) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func safeTruncateString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > maxBytes-4 && i >= 0; i-- {
		if utf8.ValidString(s[:i]) {
			if i < len(s) {
				return s[:i] + "...(已截断)"
			}
			return s[:i]
		}
	}
	return s[:maxBytes-10] + "...(已截断)"
}

func normalizeCompressLines(content string) []string {
	raw := strings.Split(content, "\n")
	out := make([]string, 0, len(raw))
	var last string
	for _, line := range raw {
		t := strings.TrimSpace(line)
		if t == "" || isSkippableNoiseLine(t) {
			continue
		}
		if t == last {
			continue
		}
		last = t
		out = append(out, t)
	}
	return collapseLongMarkdownTableRuns(out)
}

func isMarkdownTableRow(s string) bool {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "|") {
		return false
	}
	return strings.Count(t, "|") >= 2
}

func collapseLongMarkdownTableRuns(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	const head, tail = 5, 5
	const minRun = 16
	res := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if !isMarkdownTableRow(lines[i]) {
			res = append(res, lines[i])
			i++
			continue
		}
		start := i
		for i < len(lines) && isMarkdownTableRow(lines[i]) {
			i++
		}
		run := lines[start:i]
		if len(run) < minRun || len(run) <= head+tail {
			res = append(res, run...)
			continue
		}
		omit := len(run) - head - tail
		res = append(res, run[:head]...)
		res = append(res, fmt.Sprintf("…（省略 %d 行 markdown 表格）…", omit))
		res = append(res, run[len(run)-tail:]...)
	}
	return res
}

func isSupplementaryFactLine(s string) bool {
	if len(s) > 260 {
		return false
	}
	if containsApproxISODate(s) {
		return true
	}
	if maxConsecutiveDigitRun(s) >= 6 {
		return true
	}
	if strings.Contains(s, "元") && containsNumbers(s) {
		return true
	}
	return false
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func containsApproxISODate(s string) bool {
	for i := 0; i+10 <= len(s); i++ {
		if !isASCIIDigit(s[i]) {
			continue
		}
		if isASCIIDigit(s[i+1]) && isASCIIDigit(s[i+2]) && isASCIIDigit(s[i+3]) &&
			s[i+4] == '-' &&
			isASCIIDigit(s[i+5]) && isASCIIDigit(s[i+6]) &&
			s[i+7] == '-' &&
			isASCIIDigit(s[i+8]) && isASCIIDigit(s[i+9]) {
			return true
		}
	}
	return false
}

func maxConsecutiveDigitRun(s string) int {
	best, cur := 0, 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 0
		}
	}
	return best
}

func isSkippableNoiseLine(s string) bool {
	if len(s) > 120 {
		return false
	}
	switch {
	case strings.HasPrefix(s, "```"):
		return true
	case strings.HasPrefix(s, "---") && strings.Trim(s, "-") == "":
		return true
	case strings.HasPrefix(s, "***") && strings.Trim(s, "*") == "":
		return true
	case strings.HasPrefix(s, "___") && strings.Trim(s, "_") == "":
		return true
	}
	allSep := true
	for _, r := range s {
		if r != '|' && r != '-' && r != ':' && r != ' ' && r != '\t' {
			allSep = false
			break
		}
	}
	return allSep && strings.Contains(s, "-") && len(s) >= 8
}

func smartContentCompress(content string, maxBytes int) string {
	if maxBytes <= 0 {
		return content
	}

	lines := normalizeCompressLines(content)
	if len(lines) == 0 {
		return ""
	}

	var normJoined strings.Builder
	for i, ln := range lines {
		if i > 0 {
			normJoined.WriteByte('\n')
		}
		normJoined.WriteString(ln)
	}
	nj := normJoined.String()
	if len(nj) <= maxBytes {
		return nj
	}

	var headers, dataLines, summaryLines, otherLines []string
	for _, line := range lines {
		switch {
		case isHeaderLine(line):
			headers = append(headers, line)
		case isDataLine(line) || isSupplementaryFactLine(line):
			dataLines = append(dataLines, line)
		case isSummaryLine(line):
			summaryLines = append(summaryLines, line)
		default:
			otherLines = append(otherLines, line)
		}
	}

	result := make([]string, 0, len(lines))
	used := 0
	tryAdd := func(parts []string) bool {
		for _, p := range parts {
			need := len(p)
			if used > 0 {
				need++
			}
			if used+need > maxBytes {
				return false
			}
			result = append(result, p)
			used += need
		}
		return true
	}

	headerCap := maxBytes / 5
	if headerCap > 1000 {
		headerCap = 1000
	}
	if headerCap < 120 && len(headers) > 0 {
		headerCap = min(maxBytes/3, 400)
	}
	tryAdd(smartTruncateLines(headers, headerCap))

	remaining := maxBytes - used
	if remaining < 1 {
		return strings.Join(result, "\n")
	}

	summaryBudget := max(1, remaining*35/100)
	if len(summaryLines) > 0 && summaryBudget < 80 {
		summaryBudget = min(remaining, 240)
	}
	tryAdd(smartTruncateLinesFromEnd(summaryLines, summaryBudget))

	remaining = maxBytes - used
	if remaining < 1 {
		out := strings.Join(result, "\n")
		if len(out) > maxBytes {
			return safeTruncateString(out, maxBytes)
		}
		return out
	}

	dataBudget := max(1, remaining*55/100)
	if len(dataLines) > 0 && dataBudget < 80 {
		dataBudget = min(remaining, 320)
	}
	tryAdd(smartTruncateLinesFromEnd(dataLines, dataBudget))

	remaining = maxBytes - used
	if remaining > 48 {
		tryAdd(smartTruncateLinesFromEnd(otherLines, remaining))
	}

	finalContent := strings.Join(result, "\n")
	if len(finalContent) > maxBytes {
		finalContent = safeTruncateString(finalContent, maxBytes)
	}
	return finalContent
}

func compressExecutedStepResult(result string, stepIndex, totalSteps int, forReplanner bool) string {
	const (
		execRecent    = 1400
		execOlder     = 750
		replanRecent  = 800
		replanOlder   = 480
		minByteBudget = 400
		recentTail    = 2
	)
	recent := totalSteps <= recentTail || stepIndex >= totalSteps-recentTail

	var budget int
	if forReplanner {
		if recent {
			budget = replanRecent
		} else {
			budget = replanOlder
		}
	} else {
		if recent {
			budget = execRecent
		} else {
			budget = execOlder
		}
	}
	if budget < minByteBudget {
		budget = minByteBudget
	}
	return smartContentCompress(result, budget)
}

func isHeaderLine(line string) bool {
	headerKeywords := []string{
		"分析", "结论", "建议", "总结", "评估", "预测", "风险", "机会",
		"价格", "涨跌", "涨幅", "成交量", "市值", "市盈率", "市净率",
		"营收", "利润", "增长率", "ROE", "ROA", "毛利率", "净利率",
	}

	for _, keyword := range headerKeywords {
		if strings.Contains(line, keyword) && len(line) < 100 {
			return true
		}
	}

	return containsKeyMetrics(line)
}

func isDataLine(line string) bool {
	return strings.Contains(line, "%") ||
		strings.Contains(line, "亿元") ||
		strings.Contains(line, "万元") ||
		(strings.Count(line, " ") > 2 && containsNumbers(line))
}

func isSummaryLine(line string) bool {
	summaryKeywords := []string{
		"总体", "整体", "综合", "综上所述", "总体来看", "整体而言",
		"建议", "推荐", "关注", "警惕", "规避",
	}

	for _, keyword := range summaryKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

func containsKeyMetrics(line string) bool {
	metrics := []string{
		"市盈率", "市净率", "ROE", "ROA", "毛利率", "净利率",
		"营收", "利润", "增长率", "股价", "市值", "成交量",
	}

	for _, metric := range metrics {
		if strings.Contains(line, metric) {
			return true
		}
	}
	return false
}

func containsNumbers(line string) bool {
	for _, r := range line {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func smartTruncateLines(lines []string, maxBytes int) []string {
	var result []string
	var currentSize int

	for _, line := range lines {
		if currentSize+len(line) > maxBytes {
			break
		}
		result = append(result, line)
		currentSize += len(line) + 1
	}

	return result
}

func smartTruncateLinesFromEnd(lines []string, maxBytes int) []string {
	if maxBytes <= 0 || len(lines) == 0 {
		return nil
	}
	var result []string
	currentSize := 0
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		lineLen := len(line)
		if len(result) > 0 {
			lineLen++
		}
		if currentSize+lineLen > maxBytes {
			break
		}
		result = append([]string{line}, result...)
		currentSize += lineLen
	}
	return result
}

func init() {
	tools.RegisterLearnProjectHandler(func(source, name string) string {
		if name == "" {
			parts := strings.Split(strings.TrimSuffix(source, ".git"), "/")
			name = parts[len(parts)-1]
		}

		localPath := resolveProjectPath(source, name)

		if strings.HasPrefix(source, "http") || strings.HasPrefix(source, "git@") {
			cmd := exec.Command("git", "clone", source, localPath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("git clone 失败: %s\n输出: %s", err.Error(), string(output))
			}
		}

		es := &EmbeddingService{}
		sg := NewSkillGenerator(es)
		skill, err := sg.FullPipeline(source, localPath, name)
		if err != nil {
			return fmt.Sprintf("学习项目失败: %v", err)
		}

		return fmt.Sprintf("✅ 成功学习项目「%s」并生成了 Skill「%s」！\n以后你可以直接问我关于 %s 项目的问题，我会用已学的知识回答。\n必要时我会使用 ReadProjectFile 等工具查阅最新的代码细节。", name, skill.Name, name)
	})
}

func redactAIConfig(cfg data.AIConfig) data.AIConfig {
	if cfg.ApiKey != "" {
		cfg.ApiKey = "****" + cfg.ApiKey[max(0, len(cfg.ApiKey)-4):]
	}
	if cfg.HttpProxy != "" && len(cfg.HttpProxy) > 8 {
		cfg.HttpProxy = cfg.HttpProxy[:8] + "****"
	}
	return cfg
}

func resolveProjectPath(source, name string) string {
	base := filepath.Join(os.TempDir(), "blades-stock-learned-projects")
	os.MkdirAll(base, 0755)
	return filepath.Join(base, name)
}
