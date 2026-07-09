# Agent Pipeline 验证报告 — 2026-07-09

## 目标
全面验证 blades-stock Agent Pipeline 全链路完整性，确认 3 个新 Skill 正确集成。

## Pipeline 架构（7 阶段）

### 1. 请求入口 (`agent_api.go`)
- 用户消息 → `StreamChat` → 创建 `EmbeddingService`（可选）→ `buildSkillPrompt` 匹配 Skill
- 系统提示词组装：`settingConfig.Prompt` + `skillPrompt` + 强制规则（数据真实性红线）
- 历史消息注入：`ChatMemoryService.GetHistoryMessages` → 语义压缩 → 对话摘要 → 分析记忆 → 回测记录

### 2. Skill 匹配 (`agent.go:303`)
- **优先**：bge-m3 语义匹配（`EmbeddingService` 可用时）
- **降级**：关键词匹配（`TriggerKeywords` 字段，逗号分隔）
- **输出**：Directive 字段（~800-2000 chars）优先于 SystemPrompt（~15k chars）
- **注入位置**：`sysPrompt` 变量中，在 `settingConfig.Prompt` 之后、强制规则之前

### 3. Agent 创建 (`agent.go:98`)
- `classifyComplexity` → 自动选择 `react` 或 `plan_execute` 模式
- `getToolsByQuestion` → 基础工具 + 数据工具 + 假日工具 + 项目工具 + MCP 工具 + 回测工具
- `ClassifyQuestion` → 关键词匹配 9 个工具组 → `FilterToolsByGroups` 过滤
- `blades.NewAgent` → WithModel + WithTools + WithMaxIterations + WithInstruction + WithMiddleware

### 4. ReasoningMiddleware (`agent.go:194`)
- `plan_execute` 模式：注入详细思考框架（个股分析 6 步模板 + 选股方法论 + 高级分析工具）
- `react` 模式：简短提示直接调用工具
- `buildSkillPrompt` 结果已在 `sysPrompt` 中，middleware 不重复注入

### 5. 工具系统 (144 工具, 9 组)
- **GroupBase** (11): 股票代码/字典/时间/持仓/假日/交易日/推荐
- **GroupStockAnalysis** (40+): K线/财务/股东/融资融券/游资/筹码/资金博弈/估值/研报
- **GroupMarket** (20+): 大盘/全球指数/异动/涨停/板块/宏观/期货/AData
- **GroupScreening** (14): 选股/选ETF/选基金/选可转债/港股/美股
- **GroupMoneyFlow** (4): 主力资金/历史资金/行业资金/北向资金
- **GroupNewsResearch** (20+): 新闻/公告/研报/龙虎榜/涨停/热点/日历
- **GroupAIAnalysis** (6): AI推荐/分析历史/回测
- **GroupOperations** (6): 交易/钉钉/基金/经济数据
- **GroupProject** (8): 文件/目录/搜索/Git/命令
- **WebSearch**: 不在 toolGroupMap 中，始终可用

### 6. MCP 工具集成 (`agent.go:454`)
- 并行连接所有启用的 MCP 服务器（5s 超时）
- 5 分钟缓存（`mcpToolsCache`）
- 工具白名单过滤（`getEnabledToolNames`）
- `wrapMCPToolHandler`：错误捕获为 JSON 响应，防止单个工具崩溃 Agent

### 7. 并行预取 + 后处理
- `preFetchInParallel`：多组问题并行子 Agent 采集数据
- `postProcessResponse`：fire-and-forget → 工作记忆 + 反思评分 + 工具使用记录

## 6 个 Skill 数据库验证

| ID | 名称 | Category | Enable | Directive (chars) | 关键词数 |
|----|------|----------|--------|-------------------|---------|
| 9 | Serenity式产业分析 | research | ✅ | 3167 | - |
| 10 | ETF轮动多Agent分析 | quantitative | ✅ | 2643 | - |
| 11 | 项目交付工作流 | system-engineering | ✅ | 2614 | - |
| 12 | 持仓风险诊断与仓位管理 | risk-management | ✅ | 1659 | 20 |
| 13 | 市场全景看盘与复盘 | market-analysis | ✅ | 2021 | 24 |
| 14 | 个股深度诊断与交易决策 | stock-analysis | ✅ | 2088 | 16 |

## 编译验证
- `go build ./...` ✅ EXIT:0
- `go vet ./...` ✅ EXIT:0

## 结论
Agent Pipeline 全链路完整：
1. Skill 匹配（语义+关键词双路）→ Directive 注入（~1.6k-3.2k chars，远低于 15k SystemPrompt）
2. 工具系统 144 个工具 + 9 组分类 + MCP 动态加载
3. ReasoningMiddleware 提供结构化思考框架
4. 并行预取 + 后处理（记忆/反思/进化）
5. 错误处理：MCP 工具错误隔离、max iterations 提示、thinking 模式检测
