# 2026-07-02：MCP 工具注册/超时问题修复

## 问题描述

MCP 服务器（悟道A股 67工具 + tushareMcp 258工具）加载成功，但 AI Agent 在执行计划时声称部分工具"不可用"并拒绝调用。

**表现：**
- AI 说 `GetMarketData`、`GlobalStockIndexesReadable`、`GetStockChanges` 等工具不在可用列表中
- 实际上这些工具已加载（430个工具传给 LLM）
- 子 Agent（news_research/money_flow/market）全部 `context deadline exceeded` 超时

## 根本原因

1. **子 Agent 超时太短**：默认 20 秒，MCP HTTP API 调用经常超时
2. **LLM 拒绝陌生工具名**：即使工具在 tool list 中，LLM 也因训练数据不包含这些名称而拒绝调用
3. **子 Agent 系统提示词太泛**：未明确告知有 MCP 工具、不熟悉名称也要尝试

## 修复内容

### 1. parallel_dispatcher.go：子 Agent 超时 20s → 60s

```go
// 原来：
subAgentTimeoutSec = 20

// 修改后：
subAgentTimeoutSec = 60
```

### 2. parallel_dispatcher.go：子 Agent 系统提示词增强

**原提示词：**
> 你是专业的数据采集助手，专注于{group}领域。请使用你拥有的工具获取数据，返回获取到的原始结果。如果工具调用失败，请明确说明。不要进行冗余分析。

**新提示词：**
> 你是专业的数据采集助手，专注于{group}领域。
>
> 【重要】你拥有 N 个可用工具，包含来自 MCP 服务器的工具（名称可能与通用命名习惯不同，如 index_market、capital_flow 等）。
>
> 【行为准则】
> 1. 不要因为工具名称不熟悉就拒绝调用——请直接尝试你认为相关的工具；
> 2. 如果工具调用超时/报错，立即使用替代工具继续，不要放弃；
> 3. 优先调用能直接返回结构化数据的工具（如 index_market、GetMarketData、capital_flow 等）；
> 4. 只返回原始数据，不要冗余分析；
> 5. 所有工具均失败时，明确说明并列出已尝试的工具。

## 验证

- ✅ `go build` 成功，无编译错误
- 📍 构建产物：`build/bin/blades-stock.exe`
- ⚠️ 需要重启应用验证运行时效果
