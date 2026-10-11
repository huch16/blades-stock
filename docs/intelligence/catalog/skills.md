# 技能注册表 (Skill Registry)

> 单一事实源：记录 blades-stock Agent 可动态生成、存储、检索与匹配的**技能**体系。
> 与代码 `backend/agent/skill_generator.go`、`skill_validator.go`、`skill_embedding.go`、`mcp_skill_tools.go` 保持一致。

## 技能全生命周期

```
用户提供源码/文档/URL
       │
       ▼
┌──────────────────┐
│  SkillGenerator  │  ← 生成技能定义（解析源码/文档，提取 API/结构/入口）
└────────┬─────────┘
         │ 产出
         ▼
┌──────────────────┐
│  SkillValidator  │  ← 校验 ToolGroups 合法性、必填字段、无 typo
└────────┬─────────┘
         │ 合法
         ▼
┌──────────────────┐
│  SkillEmbedding  │  ← 向量化内容（关键词/Readme/结构摘要）存入 skill_embeddings
└────────┬─────────┘
         │ 入库
         ▼
┌──────────────────┐
│  skills 表       │  ← 持久化存储（含版本、状态、依赖工具组）
└────────┬─────────┘
         │ 匹配
         ▼
┌──────────────────┐
│  Agent 运行时    │  ← 问题关键词→工具组→技能向量检索→注入 Directive
└──────────────────┘
```

## 数据模型 (`backend/models/skill_version.go`)

```go
type SkillVersion struct {
    ID            uint      `gorm:"primaryKey"`
    SkillID       uint      `gorm:"index;not null"`
    Version       int       `gorm:"not null"`          // 递增版本号
    Name          string    `gorm:"size:128"`
    Description   string    `gorm:"type:text"`
    ToolGroups    string    `gorm:"size:512"`          // 逗号分隔，如 "base,stock_analysis,market"
    EntryPoints   string    `gorm:"type:text"`         // JSON 数组
    Directive     string    `gorm:"type:text"`         // 注入给 Agent 的指令文本
    Status        string    `gorm:"size:32"`           // draft / published / archived
    SourceURL     string    `gorm:"size:512"`
    SourceHash    string    `gorm:"size:64"`           // 内容哈希，用于增量更新
    CreatedAt     time.Time
    ApprovedAt    *time.Time
    ApprovedBy    string
}
```

## 内置核心技能（示例）

| 技能名 | ToolGroups | 用途 | 来源 |
|--------|------------|------|------|
| ETF 动量轮动 | `stock_analysis,market,ai_analysis` | ETF 21日加权动量排名 + 推荐纪律 | 代码内置 |
| 个股 3D 诊断 | `stock_analysis,money_flow,ai_analysis` | VectorBT 向量回测 + 筹码/资金/技术三维体检 | 代码内置 |
| 多空博弈分析 | `stock_analysis,money_flow` | 游资/筹码/资金博弈 | 代码内置 |
| 可比公司对标 | `stock_analysis` | 同行业估值/财务对标 | 代码内置 |
| 行业跟踪报告 | `stock_analysis,news_research` | 个股/行业动态跟踪 | 代码内置 |
| 代码仓库学习 | `project` | 源码/文档 → 自动生成技能 | 用户触发 |
| MCP 外部技能 | 任意 | 通过 MCP 协议动态注册的外部技能 | 用户配置 |

## 技能生成输入 (`SkillGenerationRequest`)

| 字段 | 必填 | 说明 |
|------|------|------|
| `SourceURL` | ✅ | GitHub URL / 本地路径 / 文档链接 |
| `Name` | ✅ | 技能显示名 |
| `Description` | ✅ | 一句话描述能力边界 |
| `TechStack` | | Go / Python / TypeScript / Rust 等 |
| `Readme` | | README 内容或路径 |
| `Structure` | | 目录结构树 |
| `Summary` | | 核心模块/逻辑摘要 |
| `EntryPoints` | | 入口函数/CLI/HTTP Handler |
| `APISummary` | | 对外暴露的 API 列表 |
| `ModelSummary` | | 使用的 ML 模型/嵌入/推理 |

## 校验规则 (`SkillValidator`)

1. **工具组白名单**：`ToolGroups` 中每个值必须在 `validToolGroupNames` 中
2. **必填字段**：Name / Description / ToolGroups / Directive 均非空
3. **Directive 长度**：建议 ≤ 2000 字符（防止 token 爆炸）
4. **版本唯一性**：同一 `SkillID` 下 Version 唯一递增
5. **状态流转**：`draft` → `published` → `archived`

## 向量化匹配 (`SkillEmbeddingService`)

- **触发**：问题关键词 → 激活工具组 → 向量检索技能 → Top-K 注入
- **Embedding 来源**：`embeddingService`（配置 `embeddingEnabled` + `embeddingBaseUrl` + `embeddingModelName`）
- **内容哈希**：`ContentHash` 用于去重/增量更新（避免重复向量化）
- **缓存**：LRU 过期缓存 `skillID → []float32`

## MCP 技能扩展 (`mcp_skill_tools.go`)

| MCP 端点 | 对应工具 | 说明 |
|----------|----------|------|
| `mcp/list_skills` | `MCPListSkills` | 列出 MCP Server 暴露的技能 |
| `mcp/call_skill` | `MCPCallSkill` | 调用 MCP 技能 |
| `mcp/get_skill_schema` | `MCPGetSkillSchema` | 获取技能参数 Schema |

## 开发新技能（最佳实践）

### 方式 A：手工定义（推荐用于内置核心技能）

1. 确定所需 `ToolGroups`（在 `tool_groups.go` 查阅）
2. 编写 `Directive`（注入给 Agent 的指令，包含触发条件、步骤、输出格式）
3. 插入 `skills` 表（或通过 Admin UI）
4. 标记 `published` 状态

### 方式 B：`LearnProject` 自动生成（推荐用于第三方库/内部项目）

```go
// Agent 侧调用
LearnProject(sourceURL: "https://github.com/xxx/yyy", name: "MySkill")
```

1. Agent 克隆/读取仓库
2. `SkillGenerator` 提取结构/API/入口
3. 自动生成 Directive（含典型调用示例）
4. `SkillValidator` 校验 ToolGroups
5. 入库 + 向量化

### 方式 C：MCP 外部技能

1. 用户在设置页配置 MCP Server（`backend/data/mcp_server_api.go`）
2. Agent 启动时自动发现并注册 `mcp_skill_tools.go` 工具
3. 用户通过自然语言调用，Agent 路由到 MCP

## 技能与工具组的映射建议

| 场景 | 推荐 ToolGroups | 核心工具示例 |
|------|-----------------|--------------|
| 个股深度分析 | `stock_analysis` + `money_flow` + `ai_analysis` | `GetStockInfo`/`AnalyzeCapitalGame`/`DiagnoseStock` |
| 板块/概念追踪 | `market` + `news_research` + `screening` | `HotspotDiscovery`/`GetNewsListData`/`SearchBk` |
| ETF 轮动 | `stock_analysis` + `market` + `ai_analysis` | `GetETFMomentumRanking`/`RunVectorizedBacktest` |
| 宏观/政策研判 | `market` + `operations` + `news_research` | `GetMacroGDP`/`GetNewsListData`/`GetEconomicData` |
| 代码/项目分析 | `project` | `ReadProjectFile`/`ListProjectDir`/`GitOperation` |

## 相关代码

- `backend/agent/skill_generator.go` — 生成器
- `backend/agent/skill_validator.go` — 校验器
- `backend/agent/skill_embedding.go` — 向量化
- `backend/agent/tools/mcp_skill_tools.go` — MCP 技能桥接
- `backend/data/skill_api.go` — REST API (CRUD/版本/审批)
- `backend/models/skill_version.go` — 数据模型