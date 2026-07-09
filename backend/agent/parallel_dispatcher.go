package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/agent/tools"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"

	"github.com/go-kratos/blades"
	bladesTools "github.com/go-kratos/blades/tools"
	"golang.org/x/sync/semaphore"
)

// PreFetchedData holds the result of one parallel sub-task.
type PreFetchedData struct {
	Group   string
	Summary string
	Fallback bool   // true when this sub-task timed out or errored
}

// shouldParallelize returns true when the question spans multiple independent
// tool groups, making it a good candidate for parallel sub-agent dispatch.
func shouldParallelize(question string) bool {
	groups := tools.ClassifyQuestion(question)
	nonBase := 0
	for g := range groups {
		if g != tools.GroupBase {
			nonBase++
		}
	}
	return nonBase >= 2
}

// decomposeIntoSubPrompts builds a focused sub-prompt per non-base tool group.
// Each sub-prompt instructs the sub-agent what data to gather for that area.
func decomposeIntoSubPrompts(question string, groups map[tools.ToolGroup]bool) map[tools.ToolGroup]string {
	prompts := map[tools.ToolGroup]string{
		tools.GroupBase: question,
	}

	groupHints := map[tools.ToolGroup]string{
		tools.GroupStockAnalysis: "请获取该股票/ETF的详细行情数据、K线数据、财务指标和基本面信息。",
		tools.GroupMarket:        "请获取相关市场指数行情、板块数据和宏观数据。",
		tools.GroupMoneyFlow:     "请获取资金流向数据、主力资金净流入/流出及行业资金排名。",
		tools.GroupNewsResearch:  "请获取最新相关新闻、公告及研究报告。",
		tools.GroupScreening:     "请获取符合筛选条件的股票/ETF/基金列表及其关键指标。",
		tools.GroupAIAnalysis:    "请获取AI历史分析记录和回测数据。",
		tools.GroupOperations:    "请获取基金信息和操作相关数据。",
	}

	for g := range groups {
		if g == tools.GroupBase {
			continue
		}
		hint := groupHints[g]
		if hint == "" {
			hint = "请使用可用工具获取所需数据。"
		}
		prompts[g] = question + "\n\n【专项指令】" + hint + "\n你只需要专注于本领域的数据采集，返回原始数据即可。"
	}

	return prompts
}

// preFetchInParallel identifies independent data requirements from the
// question, dispatches one minimal sub-agent per tool group, executes them
// concurrently (max N concurrent LLM calls, timeout each from config), and returns
// the collected data. Returns nil when the question is too narrow to benefit
// from parallel dispatch.
func preFetchInParallel(ctx context.Context, question string, allTools []bladesTools.Tool, aiConfig data.AIConfig) []PreFetchedData {
	if !shouldParallelize(question) {
		return nil
	}

	// Concurrency limit from config (default 2)
	concurrency := aiConfig.LLMConcurrencyLimit
	if concurrency <= 0 {
		concurrency = 2
	}
	llmSem := semaphore.NewWeighted(int64(concurrency))

	// Timeout per sub-agent from config (default 60s, increased for MCP HTTP calls)
	subAgentTimeoutSec := aiConfig.SubAgentTimeout
	if subAgentTimeoutSec <= 0 {
		subAgentTimeoutSec = 60
	}
	subAgentTimeout := time.Duration(subAgentTimeoutSec) * time.Second

	groups := tools.ClassifyQuestion(question)
	subPrompts := decomposeIntoSubPrompts(question, groups)

	var (
		mu      sync.Mutex
		results []PreFetchedData
		wg      sync.WaitGroup
	)

	for g, prompt := range subPrompts {
		if g == tools.GroupBase {
			continue
		}

		wg.Add(1)
		go func(group tools.ToolGroup, subPrompt string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("panic in sub-agent goroutine [%s]: %v", group, r)
					mu.Lock()
					results = append(results, PreFetchedData{
						Group:    string(group),
						Summary:  "[子Agent异常崩溃] " + fmt.Sprint(r),
						Fallback: true,
					})
					mu.Unlock()
				}
			}()

			// 1) Acquire semaphore (max N concurrent LLM calls from config)
			if err := llmSem.Acquire(ctx, 1); err != nil {
				mu.Lock()
				results = append(results, PreFetchedData{
					Group:    string(group),
					Summary:  "[并发受限] 信号量获取失败: " + err.Error(),
					Fallback: true,
				})
				mu.Unlock()
				return
			}
			defer llmSem.Release(1)

			// 2) Per-sub-agent deadline from config
			subCtx, cancel := context.WithTimeout(ctx, subAgentTimeout)
			defer cancel()

			groupTools := tools.FilterToolsByGroups(allTools, map[tools.ToolGroup]bool{
				tools.GroupBase: true,
				group:           true,
			})

			subModel, subErr := createChatModel(subCtx, aiConfig)
			if subErr != nil {
				logger.SugaredLogger.Errorf("[parallel] failed to create model for %s: %v", group, subErr)
				mu.Lock()
				results = append(results, PreFetchedData{
					Group:    string(group),
					Summary:  "[模型创建失败] " + subErr.Error(),
					Fallback: true,
				})
				mu.Unlock()
				return
			}
			subAgent, err := blades.NewAgent("sub-"+string(group),
				blades.WithModel(subModel),
				blades.WithTools(groupTools...),
				blades.WithMaxIterations(20),
				blades.WithInstruction("你是专业的数据采集助手，专注于"+describeGroup(group)+"领域。\n\n【重要】你拥有 "+fmt.Sprintf("%d", len(groupTools))+" 个可用工具，所有工具名称和功能已自动注入，请直接使用。\n\n【行为准则】\n1. 工具调用失败立即换工具，不要放弃；\n2. 只返回原始数据，不要冗余分析；\n3. 所有工具均失败时，说明已尝试的工具列表。"),
			)
			if err != nil {
				logger.SugaredLogger.Errorf("[parallel] failed to create sub-agent for %s: %v", group, err)
				mu.Lock()
				results = append(results, PreFetchedData{
					Group:    string(group),
					Summary:  "[子Agent创建失败] " + err.Error(),
					Fallback: true,
				})
				mu.Unlock()
				return
			}

			var sb strings.Builder
			// 3) Run sub-agent and collect all assistant text
			// Note: ErrMaxIterationsExceeded means LLM hit iteration limit but may have collected partial data.
			// ErrDeadlineExceeded means sub-context expired (time limit).
			for msg, err := range subAgent.Run(subCtx, &blades.Invocation{
				Message:    blades.UserMessage(subPrompt),
				Streamable: false,
			}) {
				if err != nil {
					break
				}
				if msg != nil && msg.Role == blades.RoleAssistant {
					sb.WriteString(msg.Text())
				}
			}

			// 4) Fallback logic:
			// - sb.Len() > 0: LLM collected data before stopping → use it, don't mark as fallback
			// - subCtx.Err()==DeadlineExceeded: ran out of time → timeout fallback
			// - any error with empty sb: explicit fallback
			fallback := false
			summary := sb.String()
			if summary == "" {
				if subCtx.Err() == context.DeadlineExceeded {
					summary = fmt.Sprintf("[超时] 子Agent执行超过%d秒", subAgentTimeoutSec)
				} else {
					summary = "[无数据] 子Agent未返回有效内容"
				}
				fallback = true
			}

			mu.Lock()
			results = append(results, PreFetchedData{
				Group:    string(group),
				Summary:  summary,
				Fallback: fallback,
			})
			mu.Unlock()
		}(g, prompt)
	}

	wg.Wait()

	if len(results) == 0 {
		return nil
	}
	return results
}

// buildParallelContext builds a system-context string from pre-fetched data
// that the main agent can use without making additional tool calls.
func buildParallelContext(data []PreFetchedData) string {
	if len(data) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("【以下是通过并行数据采集预先获取的原始数据，供你在生成最终回答时直接参考使用】\n\n")
	fallbackCount := 0
	for _, d := range data {
		if d.Fallback {
			fallbackCount++
			sb.WriteString("===== " + describeGroup(tools.ToolGroup(d.Group)) + " [FALLBACK: 数据可能不完整] =====\n")
		} else {
			sb.WriteString("===== " + describeGroup(tools.ToolGroup(d.Group)) + " =====\n")
		}
		sb.WriteString(truncateString(d.Summary, 4000))
		sb.WriteString("\n\n")
	}
	if fallbackCount > 0 {
		sb.WriteString(fmt.Sprintf("【注意】上述 %d 个并行子任务出现超时/降级，数据可能不完整。主Agent可根据需要补充调用工具。\n\n", fallbackCount))
	}
	sb.WriteString("【预采集数据结束】\n\n以上数据已由系统并行采集完毕。你无需再次调用工具查询同样的内容，直接基于以上数据进行分析和回答即可。如有额外需求，仍可使用你的工具。")
	return sb.String()
}

// describeGroup returns a human-readable Chinese description for a tool group.
func describeGroup(g tools.ToolGroup) string {
	switch g {
	case tools.GroupBase:
		return "基础"
	case tools.GroupStockAnalysis:
		return "个股/ETF分析"
	case tools.GroupMarket:
		return "市场与宏观"
	case tools.GroupMoneyFlow:
		return "资金流向"
	case tools.GroupNewsResearch:
		return "新闻与研报"
	case tools.GroupScreening:
		return "条件筛选"
	case tools.GroupAIAnalysis:
		return "AI分析"
	case tools.GroupOperations:
		return "操作"
	case tools.GroupProject:
		return "项目"
	default:
		return string(g)
	}
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "\n\n...(数据过长，已截断)"
}



