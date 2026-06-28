package agent

import (
	"context"
	"fmt"
	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func GetStockAiAgent(ctx context.Context, aiConfig data.AIConfig, question string, agentMode string, extraTools ...bladesTools.Tool) *AgentInstance {
	logger.SugaredLogger.Infof("GetStockAiAgent aiConfig: %v", aiConfig)

	modelProvider, err := createChatModel(ctx, aiConfig)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return nil
	}

	allTools := getToolsByQuestion(question)
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
	}
}

func personalityPrompt() string {
	return `你叫"财智"，是一位拥有20年实战经验、性格鲜明的股票投资大师。

## 人格特质
- 风格：犀利、直率、一针见血，偶尔带点冷幽默，绝不拐弯抹角
- 原则：始终秉持"风险控制第一"，看到用户忽视风险时会直接提醒
- 特长：价值投资、趋势交易、量化分析多策略融合，对A股/港股/美股都有深刻理解
- 语言：善用生活化的比喻解释复杂金融概念，让初学者也能听懂

## 行为准则
1. 必须调用工具获取实时数据，严禁凭记忆编造或使用过时数据
2. 所有投资建议必须附带风险提示
3. 数据获取失败时明确告知用户"当前未能获取到最新数据"，绝不伪造
4. 对模糊的问题主动反问澄清意图
5. 当本地金融工具无法直接回答"上市公司名单/地域分布/城市总部/省份区域/注册地/办公地址"等定位类问题时，必须优先调用 WebSearch 工具进行互联网搜索，禁止直接回答"无法查询"

## 股票推荐方法论（核心！避免仅推荐大公司）
进行市场资讯分析和股票推荐时，必须使用 SearchStockByIndicators 工具按以下维度筛选：
- 先用概念关键词锁定板块（如"机器人概念""半导体""AI算力"等）
- 叠加至少 3 个筛选条件：市值范围(20~200亿中小盘优先)、市盈率>0、营收增长>10%、技术面信号等
- 重点寻找"细分龙头"——市值适中(50~200亿)、有技术壁垒、业绩确定的中小盘标的，而非全行业最大的公司
- 筛选后用 GetStockInfo 核实实时行情和财务数据
- 推荐 3~5 只覆盖不同子方向，使用 BatchCreateAiRecommendStocks 批量保存`
}

func ReasoningMiddleware(next blades.Handler) blades.Handler {
	return blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
		question := inv.Message.Text()

		mode, _ := inv.Message.Metadata["agent_mode"].(string)

		var thinkingPrompt string
		if mode == "plan_execute" {
			thinkingPrompt = `请严格遵循以下思考框架回答问题：

## 思考框架
1. 拆解问题 → 确定需要哪些维度（行情、财务、新闻、宏观等）
2. 规划数据需求 → 明确每一步需要调用什么工具
3. 逐步执行 → 每次调用一个工具，分析返回结果
4. 综合判断 → 将所有信息整合成完整结论
5. 风险提示 → 给出明确的风险警告

不要一次性调用所有工具，应该根据上一步的结果决定下一步的行动。

## 股票筛选方法论（进行市场分析/新闻总结/推荐股票时必须遵守）
当需要从市场资讯中挖掘投资机会时，必须按以下步骤使用筛选工具，避免只看大公司：

第1步 - 锁定方向：根据新闻热点确定目标板块（如：机器人、半导体、AI、消费电子等），用 SearchBk 或 SearchETF 确认板块内个股范围

第2步 - 多维筛选：使用 SearchStockByIndicators 按以下维度组合筛选（至少使用 3 个筛选条件）：
  - 市值范围：排除超大盘股（流通市值>500亿），重点关注中小盘（流通市值 20亿~200亿）
  - 估值区间：市盈率PE >0 且 <60（剔除亏损和泡沫）
  - 业绩增长：营收增长率>10% 或 净利润增长率>20%（择一即可）
  - 技术面：股价站上20日均线 或 MACD金叉
  - 概念匹配：归属目标板块概念（如"机器人概念""芯片概念""AI概念"）
  示例：SearchStockByIndicators("机器人概念,流通市值20亿~200亿,市盈率>0,营收增长率>10%,股价站上20日均线")

第3步 - 交叉验证：筛选结果出来后，用 GetStockInfo 查看具体个股的实时行情，用 GetStockFinancialInfo 核实财务数据真实性

第4步 - 聚焦细分龙头：优先选择目标板块中"市值适中(50~200亿)、业绩确定、有技术壁垒"的细分龙头，而非全行业最大的公司

第5步 - 精炼推荐：最终推荐 3~5 只最具性价比的标的，覆盖不同子方向（如同时覆盖机器人、半导体、消费电子），使用 BatchCreateAiRecommendStocks 批量保存

## 高级分析工具（个股深度诊断时使用）
以下工具用于对特定个股进行深度分析，在用户问及以下场景时使用：

1. **AnalyzeHotMoneyActivity(股票代码)** — 当用户问"这只股票有没有游资参与""谁在炒作"时，分析龙虎榜判断游资风格和参与度
2. **AnalyzeChipConcentration(股票代码)** — 当用户问"主力在建仓还是出货""筹码集中还是分散"时，综合股东户数/大宗交易/资金流向判断主力意图
3. **AnalyzeCapitalGame(股票代码)** — 当用户问"资金博弈如何""主力散户谁在买"时，分析多空资金力量对比和博弈格局`
		} else {
			thinkingPrompt = `请直接调用相关工具获取最新数据后回答。如果问题不明确，主动反问用户。`
		}

		newMsg := blades.UserMessage(question + "\n\n" + thinkingPrompt)
		newMsg.Metadata = inv.Message.Metadata
		inv.Message = newMsg

		return next.Handle(ctx, inv)
	})
}

func buildSkillPrompt(question string) string {
	skills := data.NewSkillApi().GetEnabledSkills()
	if len(skills) == 0 {
		return ""
	}

	var matched []models.Skill
	for _, skill := range skills {
		if skill.TriggerKeywords == "" {
			matched = append(matched, skill)
			continue
		}
		keywords := strings.Split(skill.TriggerKeywords, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(question, kw) {
				matched = append(matched, skill)
				break
			}
		}
	}

	if len(matched) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n===== 【专业技能 - 必须严格遵守】 =====\n")
	for _, skill := range matched {
		sb.WriteString(fmt.Sprintf("\n--- %s ---\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("描述：%s\n", skill.Description))
		}
		if skill.SystemPrompt != "" {
			sb.WriteString(fmt.Sprintf("【规则指令】\n%s\n", skill.SystemPrompt))
		}
		if skill.TriggerKeywords != "" {
			sb.WriteString(fmt.Sprintf("触发关键词：%s\n", skill.TriggerKeywords))
		}
		if skill.Examples != "" {
			sb.WriteString(fmt.Sprintf("【行为示例】\n%s\n", skill.Examples))
		}
	}
	sb.WriteString("\n===== 专业技能指令结束 =====\n")
	return sb.String()
}

func GetAllTools() []bladesTools.Tool {
	var allTools []bladesTools.Tool
	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())
	allTools = append(allTools, tools.GetWebSearchTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetProjectTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	allTools = append(allTools, tools.GetBacktestStatsTool())
	allTools = append(allTools, tools.GetBacktestDetailTool())
	allTools = append(allTools, tools.GetRunBacktestTool())

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	return allTools
}

func getToolsByQuestion(question string) []bladesTools.Tool {
	var allTools []bladesTools.Tool

	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())
	allTools = append(allTools, tools.GetWebSearchTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetProjectTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	allTools = append(allTools, tools.GetBacktestStatsTool())
	allTools = append(allTools, tools.GetBacktestDetailTool())
	allTools = append(allTools, tools.GetRunBacktestTool())

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	groups := tools.ClassifyQuestion(question)
	filtered := tools.FilterToolsByGroups(allTools, groups)

	logger.SugaredLogger.Infof("tool grouping: question=%q, matched_groups=%v, total=%d, filtered=%d",
		question, groupNames(groups), len(allTools), len(filtered))

	return filtered
}

func groupNames(groups map[tools.ToolGroup]bool) []string {
	var names []string
	for g := range groups {
		names = append(names, string(g))
	}
	return names
}

func getMCPTools() []bladesTools.Tool {
	var mcpTools []bladesTools.Tool

	var servers []models.MCPServer
	err := db.Dao.Where("enable = ? AND status = ?", true, "available").Find(&servers).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取MCP服务器列表失败: %v", err)
		return mcpTools
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
			if err := db.Dao.Where("id = ? AND status = ?", id, "available").First(&server).Error; err == nil {
				servers = append(servers, server)
			}
		}
	}

	if len(servers) == 0 {
		return mcpTools
	}

	for _, server := range servers {
		if server.URL == "" {
			continue
		}

		var transport bladesmc.TransportType
		switch server.Type {
		case "sse":
			transport = bladesmc.TransportWebSocket
		default:
			transport = bladesmc.TransportHTTP
		}

		headers := parseMCPHeaders(server.Headers)

		mcpClient, err := bladesmc.NewClient(bladesmc.ClientConfig{
			Name:      server.Name,
			Transport: transport,
			Endpoint:  server.URL,
			Headers:   headers,
			Timeout:   30 * time.Second,
		})
		if err != nil {
			logger.SugaredLogger.Errorf("创建MCP客户端失败 [%s]: %v", server.Name, err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = mcpClient.Connect(ctx)
		cancel()
		if err != nil {
			logger.SugaredLogger.Errorf("MCP连接失败 [%s]: %v", server.Name, err)
			continue
		}

		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		toolList, err := mcpClient.Resolve(ctx)
		cancel()
		if err != nil {
			logger.SugaredLogger.Errorf("获取MCP工具列表失败 [%s]: %v", server.Name, err)
			continue
		}

		if len(toolList) > 0 {
			logger.SugaredLogger.Infof("从MCP服务器 [%s] 加载了 %d 个工具", server.Name, len(toolList))
			mcpTools = append(mcpTools, toolList...)
		}
	}

	return mcpTools
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
	tools.RegisterBacktestHandler(func() string {
		bts := NewBacktestService()
		count, err := bts.RunAllBacktests()
		if err != nil {
			return fmt.Sprintf("回测执行失败: %v", err)
		}
		if count == 0 {
			return "所有分析报告已完成回测，无需重复执行"
		}
		stats := bts.GetOverallStats()
		return fmt.Sprintf("✅ 完成 %d 份分析报告回测！\n当前综合准确率: %.1f%% (已验证 %d 次)", count, stats.AccuracyRate, stats.VerifiedCount)
	})

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

func resolveProjectPath(source, name string) string {
	base := filepath.Join(os.TempDir(), "go-stock-learned-projects")
	os.MkdirAll(base, 0755)
	return filepath.Join(base, name)
}
