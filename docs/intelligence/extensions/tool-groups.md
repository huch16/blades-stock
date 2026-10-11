# 工具分组定义说明 (Tool Groups Extension)

> 记录 `backend/agent/tools/tool_groups.go` 中工具分组的设计意图、扩展规则、Token 控制策略。

## 设计原则

### 1. 正交性
- 每个工具**仅属于一个**主分组（`toolGroupMap`）
- `WebSearch` / `FetchWebPage` 不属于任何组，始终注入（通用性极高）

### 2. 语义内聚
- 同组工具解决同一类子问题，LLM 易形成"调用套路"
- 组名即能力标签：`stock_analysis` = 单股深度，`market` = 大盘全景

### 3. Token 预算可控
- 组内工具数不等，直接全量注入会超 token
- 引入 **白名单** + **上限** 两级控制

## 分组定义表

| Group 常量 | 字符串值 | 核心工具数 | 上限 | 白名单示例 |
|------------|---------|------------|------|------------|
| `GroupBase` | `base` | 11 | 全量 | 全部 |
| `GroupStockAnalysis` | `stock_analysis` | ~50 | 15 | `GetStockInfo`, `GetEastMoneyKLineWithMA`, `GetStockFinancialInfo`, `DiagnoseStock`, `AnalyzeCapitalGame` |
| `GroupMarket` | `market` | ~25 | 12 | `GetMarketData`, `GlobalStockIndexesReadable`, `GetStockChanges`, `HotspotDiscovery` |
| `GroupScreening` | `screening` | 8 | 6 | `FilterStocks`, `SearchStockByIndicators`, `ScreenByStrategy` |
| `GroupMoneyFlow` | `money_flow` | 5 | 4 | `GetStockMoneyData`, `GetStockHistoryMoneyData`, `GetIndustryMoneyRank` |
| `GroupNewsResearch` | `news_research` | ~30 | 10 | `GetNewsListData`, `GetStockResearchReport`, `GetStockNotice`, `GetLongTigerList` |
| `GroupAIAnalysis` | `ai_analysis` | 8 | 全量 | 全部（高价值、低频） |
| `GroupOperations` | `operations` | 7 | 5 | `SetTradingPrice`, `SendToDingDing`, `SearchFund` |
| `GroupProject` | `project` | 7 | 全量 | 全部（开发场景专用） |

> 完整映射见 `tool_groups.go:toolGroupMap`

## 自动分类关键词 (`groupKeywordsList`)

每个分组维护一组中文/英文关键词，`ClassifyQuestion()` 用于：
1. 用户提问 → 关键词匹配
2. 命中关键词 → 激活对应工具组
3. 未命中任何关键词 → 仅保留 `base`

**关键词设计建议**：
- 覆盖用户自然语言（如"筹码"、"主力"、"游资"对应 `stock_analysis`）
- 包含同义词/口语（"K线"、"k线"、"日K"、"分时"）
- 包含工具名片段（"MACD"、"RSI"、"布林"）

## Token 控制策略 (`FilterToolsByGroupsCapped`)

### 两遍过滤算法

```go
// 第一遍：白名单优先 + 无上限组全量
for _, t := range allTools {
    if !groups[group] { continue }
    if !capped(group) { filtered = append(filtered, t); continue }
    if coreSet[t.Name()] { filtered = append(filtered, t); continue }
}

// 第二遍：白名单未达上限的组，按原顺序补充剩余工具
for _, t := range allTools {
    if added[group] >= cap { continue }
    if coreSet[t.Name()] { continue }
    filtered = append(filtered, t); added[group]++
}
```

### 配置参数 (`groupToolCaps` / `coreToolByGroup`)

| Group | Cap | 核心工具数 | 说明 |
|-------|-----|------------|------|
| `market` | 12 | 6 | 异动/指数/宏观/全球/热点/板块 |
| `screening` | 6 | 4 | 条件选股/策略/板块/ETF |
| `news_research` | 10 | 6 | 新闻/研报/公告/龙虎榜/涨停/日历 |
| `ai_analysis` | 全量 | 8 | 诊断/回测/向量化/AI推荐 |
| `money_flow` | 4 | 3 | 个股/历史/行业/北向/大单 |
| `operations` | 5 | 4 | 价位/钉钉/基金/宏观 |
| `base` | 全量 | 11 | 全部核心 |

## 新增工具组 Checklist

1. 在 `ToolGroup` 常量块添加
2. 在 `validToolGroupNames` 添加字符串
3. 在 `isValidToolGroup` switch 添加 case
4. 在 `toolGroupMap` 映射所有相关工具
4. 在 `coreToolByGroup` 声明白名单
5. 在 `groupToolCaps` 设置上限（或不设置=全量）
6. 在 `groupKeywordsList` 添加关键词（用于自动分类）
7. 更新 `docs/intelligence/catalog/tools.md`
8. 运行 `go test ./backend/agent/tools/... -run ToolGroup`

## 常见问题

| 现象 | 原因 | 解决 |
|------|------|------|
| 新工具从不被调用 | 未加入 `toolGroupMap` | 补全映射 |
| 关键词匹配不生效 | `groupKeywordsList` 缺失 | 添加中英文关键词 |
| Token 爆炸 | 组上限未设置/过大 | 调小 `groupToolCaps` 或扩充白名单 |
| 核心工具被裁剪 | 白名单遗漏 | 加入 `coreToolByGroup` |

## 相关代码

- `backend/agent/tools/tool_groups.go` — 单一事实源
- `backend/agent/tools/tool_groups_test.go` — 分类/过滤测试
- `backend/agent/agent.go` — `ClassifyQuestion` + 注入流程