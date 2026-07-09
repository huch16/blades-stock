# 统一回测系统重写验证

**日期**: 2026-07-09  
**时间**: 13:30 - 14:00

## 目标

将三套并存的回测系统统一为一个 BacktestManager，消除代码冗余和路由混乱。

## 变更前状态

| 系统 | 文件 | 状态 |
|------|------|------|
| V1 | `agent/analysis_backtest.go` | 生产在用，路由入口 |
| V2 | `agent/analysis_backtest_v2.go` | 代码完整但无路由 |
| quant包 | `quant/integration.go` + `quant/vectorbt.go` 等 | 测试通过但未接入Agent主流程 |

问题：
- `handleRunBacktest` 仅返回提示消息，未真正调用回测
- `VectorBacktest`/`VectorGridSearch` 工具已注册但未在 `toolGroupMap` 映射
- 三套代码有重复的预测提取、统计查询逻辑

## 变更内容

### 1. 新建 `quant/backtest_manager.go`（统一入口）

- **AI报告回测**: `RunAIReportBacktest`（批量并行8worker）、`RunSingleReportBacktest`、`RunBacktestOnDateRange`
- **策略回测**: 委托给 `VectorBacktestService`（`RunStrategyBacktest`、`RunStrategyGridSearch`）
- **统计查询**: `GetOverallStats`、`GetStockStats`、`NeedBacktest`、`GetLatestResults`
- **预测提取**: `extractPredictionFromReport`（关键词计数+概率提取+目标价解析）
- **方向准确率**: `checkDirectionAccuracy`（按方向设不同阈值）

### 2. 删除旧文件

- `agent/analysis_backtest.go`（V1）
- `agent/analysis_backtest_v2.go`（V2）
- `agent/tools/vectorbt_tools.go`（旧工具注册）

### 3. 重写 `agent/tools/backtest_tools.go`

5个工具全部使用 BacktestManager：
- `GetBacktestStats` — 统计查询
- `GetBacktestDetail` — 单报告详情
- `RunBacktest` — 触发回测
- `VectorBacktest` — 向量化策略回测
- `VectorGridSearch` — 参数网格扫描

### 4. 更新引用

| 文件 | 变更 |
|------|------|
| `agent/agent.go` | 删除 `RegisterBacktestHandler` 回调，移除 quant import |
| `agent/agent_api.go` | `NewBacktestService()` → `quant.NewBacktestManager()` |
| `main.go` | 后台回测改用 `quant.NewBacktestManager()` |
| `cmd/backtest/main.go` | 改用 `quant.NewBacktestManager()` |
| `agent/analysis_backtest_test.go` | 改用 `quant.NewBacktestManager()` |
| `agent/tools/tool_groups.go` | 添加 `VectorBacktest`/`VectorGridSearch` → `GroupAIAnalysis` 映射 |

### 5. 修复 `aiReportResult` 结构体

添加 `maxUpside`、`maxDrawdown` 字段，`SaveBacktestResultToDB` 同步写入这两个字段。

## 验证结果

| 检查项 | 结果 |
|--------|------|
| `go build ./...` | ✅ 通过 |
| `go vet ./...` | ✅ 零警告 |
| `staticcheck ./...` | ✅ 零警告 |
| `go test ./backend/quant/` | ✅ 48/48 通过 |

## 架构总结

```
BacktestManager (quant/backtest_manager.go)
├── AI报告回测
│   ├── RunAIReportBacktest (批量并行)
│   ├── RunSingleReportBacktest (单报告)
│   └── RunBacktestOnDateRange (日期范围)
├── 策略回测 (委托 VectorBacktestService)
│   ├── RunStrategyBacktest
│   └── RunStrategyGridSearch
├── 统计查询
│   ├── GetOverallStats
│   ├── GetStockStats
│   ├── NeedBacktest
│   └── GetLatestResults
└── 预测提取
    ├── extractPredictionFromReport
    ├── extractTargetPrice
    └── extractProbability

工具层 (agent/tools/backtest_tools.go)
├── GetBacktestStats → BacktestManager.GetOverallStats/GetStockStats
├── GetBacktestDetail → BacktestManager.GetLatestResults
├── RunBacktest → BacktestManager.RunAIReportBacktest/RunBacktestOnDateRange
├── VectorBacktest → quant.VectorBTHandler
└── VectorGridSearch → quant.VectorBTGridSearchHandler
```

## 待办

- 数据库中 12 份待回测报告需应用启动后自动执行
- 可选：将 `VectorBTHandler`/`VectorBTGridSearchHandler` 也改为通过 BacktestManager 调用
