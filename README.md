# Blades Stock（go-stock）——以 LLM 为核心的股票分析与智能助理

这是一个以 LLM 为核心的股票分析与助手桌面/后端项目（以 Agent 为中心的"股市智能助理"），把检索型工具、企业/财务/行情数据与多步骤推理流水线组装在一起，目标用户是需要交互式、数据驱动的个股深度分析与选股策略的研究者或量化/投顾开发者。

---

## Stack

- **语言**：Go 为后端主实现，前端使用 Vue（仓库语言统计显示有 Vue 与 TypeScript），有少量 Python 用作脚本/工具支持。
- **框架 / 运行时**：Blades agent 框架（`github.com/go-kratos/blades`）用于构建 LLM agent；桌面绑定/交互层使用 Wails（代码中有 wails 相关结构标签/约定）。
- **主要依赖**：
  - `github.com/go-kratos/blades` (+ blades contrib/mcp) — Agent、工具与 MCP（远程工具）支持
  - `sonic` — 高效 JSON 反序列化（在 agent 中用于解析 MCP args/env/headers）
  - 内置 HTTP 客户端与自建 EmbeddingService（用于向量化/语义匹配）

---

## 仓库结构

```
backend/
  agent/          // Agent 管道实现：GetStockAiAgent, ReasoningMiddleware, buildSkillPrompt, EmbeddingService, SkillGenerator
  data/           // DB 层/Api 封装：SkillApi 等（读取/更新 Skill、MCPServer 配置）
  models/         // 数据模型定义（DB 映射结构体，大量 domain 类型）
cmd/              // 小工具/命令行程序（如技能/检查点工具等）
frontend/         // 前端（Vue）代码与桌面打包绑定
build/            // 打包与截图资源
docs/             // 使用手册与说明文档
data/             // 本地数据与静态资源（如 stock_basic.json）
main.go / app.go // 应用入口与平台适配
```

---

## 架构：各组件如何协同

### Agent 管道

后端以 `GetStockAiAgent()` 为入口（`backend/agent/agent.go`），创建 `blades.Agent` 并注入 model provider、工具集与中间件。工具集合既有本地实现也可通过 MCP（remote tool servers）动态加载。

请求进入后会通过 `ReasoningMiddleware` 将"思考/执行框架"或"直接获取"提示注入用户问题（根据 `agent_mode` 决定 `plan_execute` 或 `react` 流程），把增强后的 `UserMessage` 传给 agent 执行。

### 技能（Skill）机制

`buildSkillPrompt` 从数据库读取已启用的 Skill（`data.NewSkillApi().GetEnabledSkills()`），优先使用向量语义匹配（`EmbeddingService` + `SkillEmbedding`），回退到触发关键词匹配，然后把选中 Skill 的 Directive 或 SystemPrompt 注入到最终 prompt 中，形成"专业技能 - 必须严格遵守"的规则块。Skill 的增删改查由 `backend/data/skill_api.go` 管理。

### Skill 自动生成

`SkillGenerator` 可以把已分析的项目（`ProjectAnalysis`）生成 Skill（用于把项目知识作为技能注入 agent），并保存到 DB，支持把项目源码索引并构造成 `SystemPrompt` / `Examples`（见 `backend/agent/skill_generator.go`）。

### Embedding 与相似度

`EmbeddingService` 封装了对外的 embedding HTTP 接口（POST `/v1/embeddings`），并实现了 `CosineSimilarity`，用于语义匹配 Skill。若 EmbeddingService 未配置，则 `buildSkillPrompt` 会回退到关键词匹配（见 `backend/agent/embedding_service.go` 与 `agent.go`）。

---

## 快速开始（开发环境）

### 先决条件
- Go 1.26.x（与 go.mod 中的 go 版本一致）
- Node.js（建议 Node 16+）
- Wails CLI（用于开发与打包；可通过 Go 安装）
- Git

### 开发运行

```bash
# 编译所有 go 包
go build ./...

# 运行单元测试
go test ./...

# 前端（开发模式）
cd frontend
npm install
npm run dev

# 桌面打包（Windows）
./build.ps1 <task>
```

### 环境配置要点
- **数据库**（gorm）用于 Skill、MCPServer、ProjectAnalysis 等表；首次运行会自动建表。
- **语义匹配**：若启用，需要配置 Embedding 服务的 `baseURL`、`modelName`、`apiKey`。
- **MCP（远程工具服务器）**：需在 DB 中配置 MCPServer 并确保可连接（`agent.go` 中有 `connectMCPServer` 的连接与 Resolve 流程与超时处理）。

---

## 实际关键点（基于代码阅读）

- **Skill 注入逻辑只会选出有限的技能**：
  - 若有 EmbeddingService，会调用 `MatchSkills(question, 1)`（topN = 1），只取最相关的一条 skill；
  - 回退关键词匹配也在找到第一条后就 break；若某 skill 的 `TriggerKeywords` 为空，则会被优先选中（作为默认 skill）；
  - 插入提示时优先使用 `skill.Directive`（短指令），否则使用 `skill.SystemPrompt`（完整系统提示）。
  - 这会导致复杂问题下只有一个 skill 被注入，若需多个技能并用，需要在 Match/回退逻辑上调整。

- **ReasoningMiddleware** 根据 `agent_mode` 注入不同的"思考框架"提示（`plan_execute` 时会注入详尽的多步执行模板），这决定 agent 是一步到位回答还是按规划-执行-再规划循环调用工具。

- **MCP（外部工具）** 加载是并发的，并对返回的工具列表根据 DB 白名单进行过滤；对外部工具的调用被包装为 JSON 错误响应以避免单个工具异常导致 agent 崩溃（`wrapMCPToolHandler`）。

- **Skill 生成器**会把项目的 README、结构、入口、API 摘要等汇入 SystemPrompt，并有工具提示（`ReadProjectFile`、`SearchProjectFiles`）以便 agent 能检索源码细节。

---

## 数据源

- **东方财富**（push2his/push2.eastmoney.com）— K 线、实时行情、资金流向
- **TDX**（TdxKLineApi）— 股票基本信息同步、扩展行情数据
- **CLS / Sina / TradingView** — 新闻资讯
- **SearXNG** — 网络搜索
- **Tushare** — 历史数据（可选 API Key）
- 静态文件：`backend/data/stock_basic.json`

---

## 功能概览（用户视角）

- **市场总览**：全球行情、热门话题、重大事件时间轴
- **个股详情**：K 线、分时、资金流向、研报与公告检索
- **AI 分析**：基于预设或自定义提示词模板生成分析报告；支持多轮跟进问题
- **选股与策略**：AI 智能选股、指标选股、行业排名
- **通知**：钉钉等告警通知集成（可配置）

---

## 贡献与许可

欢迎通过 Issue / PR 报告问题或提交改进。

贡献流程：
1. Fork 项目并创建分支：`feature/xxx` 或 `fix/yyy`
2. 提交并推送分支，发起 PR，描述变更与复现步骤
3. 通过 CI 与代码审查后合并

本项目采用 **GNU GPLv3** 许可（请参见仓库 LICENSE 文件）。
