# Blades-Stock 智能内容系统化总览 (Intelligence Catalog)

> 本文档系统化梳理 blades-stock 桌面智能量化终端中所有 AI、量化、策略、技能、工具与提示词相关的可配置内容，
> 作为 **智能内容单一事实源 (Single Source of Truth)**，所有 AI 智能能力的注册、维护与扩展必须以此目录为准。

## 目录结构

```
docs/intelligence/
├── README.md                              ← 总览（本文档）
├── catalog/
│   ├── tools.md                            ← 工具注册表（Tool Registry）
│   ├── skills.md                           ← 技能注册表（Skill Registry）
│   ├── prompts.md                          ← 提示词注册表（Prompt Registry）
│   ├── strategies.md                       ← 策略注册表（Strategy Registry）
│   └── models.md                           ← AI 模型配置模板
├── prompts/
│   ├── default.yaml                        ← 默认提示词模板（可直接覆盖）
│   ├── personality.yaml                    ← 人格设定
│   ├── discipline-quant.yaml               ← 量化纪律
│   └── discipline-recommend.yaml          ← 推荐纪律
├── strategies/
│   ├── macd_golden_cross.yaml              ← （已迁移到 backend/strategy/strategies/）
│   ├── pullback_buy.yaml
│   ├── rsi_reversal.yaml
│   ├── trend_following.yaml
│   └── volume_breakout.yaml
└── extensions/
    ├── tool-groups.md                      ← 工具分组定义说明
    └── skill-lifecycle.md                  ← 技能生命周期与版本化
```

## 智能内容全景图

### 1. 工具层 (Tools)
- **位置**：`backend/agent/tools/`
- **数量**：40+ 工具实现 + 10 个工具分组
- **注册表**：详见 [`catalog/tools.md`](catalog/tools.md)
- **分组**：`base` / `stock_analysis` / `market` / `screening` / `money_flow` / `news_research` / `ai_analysis` / `operations` / `project`

### 2. 技能层 (Skills)
- **位置**：`backend/agent/` （含 `skill_generator.go`, `skill_validator.go`, `skill_embedding.go`）
- **存储**：`skills` 数据库表
- **机制**：自动生成 + 校验 + 向量化匹配
- **注册表**：详见 [`catalog/skills.md`](catalog/skills.md)

### 3. 提示词层 (Prompts)
- **位置**：`backend/agent/prompt_config.go`（内嵌默认值）+ `docs/intelligence/prompts/`（可覆盖文件）
- **可覆盖**：通过环境变量 `BLADES_STOCK_PROMPT_TEMPLATE_FILE` 指定 JSON/YAML 文件
- **模板字段**：personality / reactThinking / planExecuteThinking / systemRules / subAgentInstruction / stockAnalysisTemplate / genericTaskTemplate
- **注册表**：详见 [`catalog/prompts.md`](catalog/prompts.md)

### 4. 策略层 (Strategies)
- **位置**：`backend/strategy/strategies/` (YAML)
- **数量**：5 个内置策略 + 可扩展
- **机制**：基于条件表达式（DSL）评分
- **注册表**：详见 [`catalog/strategies.md`](catalog/strategies.md)

### 5. 模型层 (Models)
- **位置**：`frontend/src/stores/aiConfig.ts`（前端配置）+ `backend/data/settings_api.go`（后端存储）
- **支持**：DeepSeek / 硅基流动 / 智谱 / 豆包 / Ollama / OpenAI 兼容等
- **模板**：详见 [`catalog/models.md`](catalog/models.md)

## 扩展流程

### 新增一个工具
1. 在 `backend/agent/tools/` 创建 `xxx_tool.go`
2. 在 `backend/agent/tools/tool_register.go` 注册
3. 在 `backend/agent/tools/tool_groups.go` 的 `toolGroupMap` 映射到对应分组
4. 在 `backend/agent/tools/tool_groups.go` 的 `coreToolByGroup` / `groupToolCaps` 中声明白名单与上限
5. 更新 `docs/intelligence/catalog/tools.md`

### 新增一个技能
1. 由 `SkillGenerator` 自动生成（推荐路径）
2. 或手动插入 `skills` 数据库表
3. 在 `skill.ToolGroups` 字段声明所需工具组
4. 更新 `docs/intelligence/catalog/skills.md`

### 新增一个策略
1. 在 `backend/strategy/strategies/` 创建 YAML
2. 在 `StrategyRegistry` 注册（已有自动加载）
3. 更新 `docs/intelligence/catalog/strategies.md`

### 覆盖提示词
1. 复制 `docs/intelligence/prompts/default.yaml` 为 `my-prompts.yaml`
2. 修改需要的字段
3. 设置环境变量 `BLADES_STOCK_PROMPT_TEMPLATE_FILE=/path/to/my-prompts.yaml`
4. 重启应用生效

## 版本化与变更追踪

- **工具**：随代码版本（Git tag）
- **技能**：数据库表 `skill_versions` 自动版本化
- **提示词**：JSON/YAML 文件 + Git 版本控制
- **策略**：YAML 文件 + Git 版本控制

## 质量保障

| 层级 | 测试覆盖 | 备注 |
|------|----------|------|
| 工具 | 单元测试（`tool_groups_test.go` 等） | 工具分组、白名单、过滤 |
| 技能 | `skill_validator_test.go` + `skill_embedding_test.go` | 校验 + 向量化 |
| 提示词 | 集成测试 | 默认值 + 覆盖流程 |
| 策略 | DSL 解析测试 | 表达式 + 评分 |

## 相关文档

- [`docs/intelligent_capabilities_architecture.md`](../intelligent_capabilities_architecture.md) — 智能功能架构全景
- [`docs/agent_architecture_sequence.md`](../agent_architecture_sequence.md) — Agent 架构时序
- [`docs/backtest_improvement_and_skill_optimization_20260705_0202.md`](../backtest_improvement_and_skill_optimization_20260705_0202.md) — 回测与技能优化