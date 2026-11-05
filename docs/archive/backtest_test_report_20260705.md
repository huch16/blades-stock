# Backtest 功能测试报告（2026-07-05）

## 测试概况

**测试时间**：2026-07-05 01:52  
**测试工具**：`cmd/backtest/main.go`  
**可执行文件**：`build/bin/backtest.exe`（61,179,904 字节）  
**数据库**：`data/stock.db`  

## 测试结果

### 1. 回测执行

**命令输出**：
```
=== Backtest Test Tool ===

Reports pending backtest: true

Running backtest...
Backtest completed: 0 reports processed
```

**跳过详情**（22 条）：
- **无效股票代码**（15 条）：包含"市场资讯"、"agent"等字符串
- **价格数据不足**（5 条）：无 2026-07-03 后的价格数据
- **其他**（2 条）：已验证，但预测不准确

### 2. 现有回测结果

**最近 2 条结果**：
1. **ID=4**：TCL 科技（sz000100），看涨，不准确，价格变化 -4.56%
2. **ID=1**：TCL 科技（sz000100），看涨，不准确，价格变化 -4.56%

### 3. 统计摘要

| 指标 | 数值 |
|------|------|
| 总报告数 | 22 |
| 已验证报告 | 2 |
| 准确预测 | 0 |
| 准确率 | 0.00% |
| 平均置信度 | 0.40 |

**方向分解**：
- **看涨**：0/2 (0.00%)

## 问题诊断

### 1. 数据质量问题（高优先级）

**现象**：15/22 条报告使用无效股票代码。

**根本原因**：
- Agent 分析输出中可能包含"市场资讯"、"agent"等字符串作为"股票代码"
- 数据库约束可能不足，允许无效数据插入

**修复建议**：
1. 加强 Agent 输出解析逻辑（`executeBacktest()` 函数）
2. 在数据库层添加 CHECK 约束，拒绝无效股票代码
3. 定期清理无效报告

### 2. 价格数据缺失（中优先级）

**现象**：5/22 条报告无后续价格数据。

**示例**：
- `sz002906`：无 2026-07-04 后价格数据
- `sh600671`：无 2026-07-04 后价格数据

**可能原因**：
- 股票已退市
- 数据提供方（东方财富）缺失数据
- 数据库同步失败

**修复建议**：
1. 在 `verifyBacktest()` 中添加更详细的错误日志
2. 检查数据同步任务是否正常运行
3. 添加备用数据源

### 3. 预测不准确（低优先级）

**现象**：2 条有效报告均预测错误。

**示例**：
- TCL 科技（sz000100）：预测看涨，实际下跌 4.56%

**可能原因**：
1. Agent 预测模型不准确
2. 验证时间窗口太短（仅 1-2 天）
3. 市场波动性大，短期预测困难

**修复建议**：
1. 延长验证时间窗口（如 5 天、10 天）
2. 调整准确率阈值（当前 5% 可能太严格）
3. 改进 Agent 提示词，提高预测质量

## 代码审查发现

### 1. `analysis_backtest.go` 审查

**函数**：
- `executeBacktest(report BacktestReport) BacktestResult`
- `verifyBacktest(result BacktestResult) (bool, float64, error)`
- `runBatchBacktest(reports []BacktestReport) []BacktestVerification`

**发现**：
- ✅ 预测提取逻辑支持多种格式
- ✅ 准确率计算正确（`1 - |实际价格 - 预测价格| / 实际价格`）
- ⚠️ 准确率阈值固定为 5%（可能太严格）

### 2. `cmd/backtest/main.go` 修复

**编译错误**（已修复）：
1. 未使用的导入：`blades-stock/backend/data`、`blades-stock/backend/models`、`time`
2. 未定义的函数：`db.InitDB()` → `db.Init("data/stock.db")`

**修复后的代码**：
```go
import (
	"fmt"
	"blades-stock/backend/agent"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"os"
)

func main() {
	// Initialize database
	db.Init("data/stock.db")

	// Initialize logger
	logger.InitLogger()

	// ... 其余代码
}
```

## 下一步行动

### 高优先级
1. **清理无效数据**：删除 15 条无效报告
2. **改进预测提取**：加强 Agent 输出解析，避免无效股票代码
3. **添加有效数据**：手动插入几条有效报告进行测试

### 中优先级
1. **检查数据同步**：确保价格数据正常更新
2. **调整验证窗口**：延长验证时间窗口，提高准确率
3. **添加错误日志**：在 `verifyBacktest()` 中记录详细错误

### 低优先级
1. **改进 Agent 提示词**：提高预测质量
2. **添加备用数据源**：防止数据缺失
3. **优化数据库约束**：拒绝无效数据插入

## 附录：完整命令输出

```
=== Backtest Test Tool ===

Reports pending backtest: true

Running backtest...
2026-07-05 01:53:01 [WARN] backtest report 6 skipped: invalid stock code: 市场资讯
2026-07-05 01:53:01 [WARN] backtest report 7 skipped: invalid stock code: 市场资讯
... (共 22 条跳过警告)
Backtest completed: 0 reports processed

Recent 2 backtest results:
  ID=4, Stock=sz000100(TCL科技), Direction=看涨, Accurate=false, Change=-4.56%
  ID=1, Stock=sz000100(TCL科技), Direction=看涨, Accurate=false, Change=-4.56%

=== Overall Backtest Statistics ===
Total reports: 22
Verified reports: 2
Accurate predictions: 0
Accuracy rate: 0.00%
Average confidence: 0.40

Direction breakdown:
  看涨: 0/2 (0.00%)

=== Backtest Test Completed ===
```

---

**报告生成时间**：2026-07-05 01:55  
**报告生成者**：QClaw Agent
