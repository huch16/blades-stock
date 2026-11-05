# 回测问题诊断报告

## 诊断时间
2026-07-06 00:15

## 问题
回测引擎报告 `no kline data for 天目药业 - 600671.SH` 等错误，导致回测全部跳过。

## 根本原因

### 1. K线数据获取正常
- `FetchKLineWithFallback("sh600671", ...)` 成功返回 30 条日K数据（来源: tdx-mac）
- 所有数据源（MAC/TDX → 东方财富 → Tushare → Sina → Tencent → TDX）的代码转换逻辑都支持 `sh600671` 格式
- 之前 `no kline data` 错误出现在 `backend/quant/integration.go:84`，是量化分析模块（VectorBT等），不是回测引擎

### 2. 回测跳过的真正原因
回测引擎（`analysis_backtest.go`）运行结果：
```
report 13 skipped: insufficient price data after 2026-07-03
report 15 skipped: insufficient price data after 2026-07-03
report 20 skipped: insufficient price data after 2026-07-03
report 23 skipped: no price data after 2026-07-04 for sz002906
report 28 skipped: no price data after 2026-07-04 for sh600671
```

**真正原因**：报告生成日期（7月3日/4日）之后没有足够的交易日数据来验证预测。
- 7月5日是周六，7月6日是周日——非交易日
- 回测需要报告日期之后至少 2 个交易日的价格数据（`verifyWindow` 默认 20 个交易日）
- 当前只有 7月3日之前的数据，不满足验证窗口

### 3. 回测结果
- 总报告数：7
- 已验证：2（都是 sz000100 TCL科技，预测看涨，实际下跌 -4.56%，不准确）
- 准确率：0%
- 5 条报告因数据不足跳过

## 结论
这不是 bug，而是**时间条件不满足**。等交易日开启后（7月7日周一），积累足够交易日数据后回测会自动正常执行。

## 关键路径
- 回测入口：`cmd/backtest/main.go` → `agent.NewBacktestService()` → `RunAllBacktests()`
- K线加载：`backend/quant/integration.go:82` → `data.FetchKLineWithFallback()`
- 回测逻辑：`backend/agent/analysis_backtest.go:210` → `RunSingleBacktest()`
- 数据库：`G:\stock\blades-stock\data\stock.db`，表 `ai_response_result`
- 股票代码格式：`sh600671`（小写前缀+数字），所有数据源均支持
