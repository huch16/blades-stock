# Backtest 改进与 Skill 优化进度报告（2026-07-05）

## 完成的工作（选项 B：高优先级）

### 1. 清理无效数据
- ✅ 创建了数据库清理工具 `cmd/clean_backtest/main.go`
- ✅ 软删除了 15 条无效 `AIResponseResult` 记录（股票代码为"市场资讯"、"agent"等）
- ✅ 验证了清理生效（总报告数从 22 降到 7）
- ✅ 备份了数据库（`data/stock.db.backup_20260705_015930`）

**关键发现**：
- 回测的输入数据是 `models.AIResponseResult`（不是 `BacktestReport`）
- 数据库实例变量是 `db.Dao`（不是 `db.DB`）
- 无效记录主要是 Agent 生成报告时错误填充的字段

### 2. 改进预测提取
- ✅ 添加了更多关键词到 `extractPrediction()` 函数（`analysis_backtest.go`）
- ✅ 看涨关键词：添加"牛市"、"多头"、"强势"、"突破"
- ✅ 看跌关键词：添加"熊市"、"空头"、"弱势"、"破位"
- ✅ 中性关键词：添加"整理"、"徘徊"、"均衡"

**效果评估**：
- 预测提取准确性可能略有提高
- 但主要问题仍是 **Agent 预测不准确**（需要改进 Agent 提示词或增加数据源）

### 3. 修复 Backtest 工具编译错误
- ✅ 修复了 `cmd/backtest/main.go` 中的导入错误
- ✅ 将 `db.InitDB()` 改为 `db.Init("data/stock.db")`
- ✅ 删除了未使用的导入（`data`、`models`、`time`）

## 进行中的工作（选项 C：中优先级）

### 1. 优化 skill 生成
- ⏳ 尚未开始
- 📋 需要查看 `skill_generator.go` 文件（330 行）
- 🎯 目标：改进 skill 提示词生成逻辑，提高 Agent 分析质量

### 2. 添加 backtest 配置
- ⏳ 尚未开始
- 📋 需要添加 backtest 配置选项（如准确率阈值、验证时间窗口等）
- 🎯 目标：让用户可以通过前端设置调整 backtest 参数

### 3. 处理复权
- ⏳ 尚未开始
- 📋 需要理解"复权"在股票数据中的含义（可能是前复权/后复权）
- 🎯 目标：确保价格数据准确性，提高 backtest 准确性

## 待解决的问题

### 高优先级
1. **Agent 预测不准确**：2 条有效报告均预测错误（准确率 0.00%）
2. **价格数据缺失**：5/7 条有效报告因价格数据不足被跳过
3. **Backtest 配置缺失**：准确率阈值固定为 5%，验证时间窗口可能太短

### 中优先级
1. **Skill 生成质量**：当前生成的 skill 提示词可能不够精确
2. **复权数据处理**：需要确保价格数据已复权（前复权或后复权）
3. **数据源可靠性**：TickFlow 免费层功能有限，AKShare 服务可能不稳定

## 下一步建议

### 立即行动
1. **改进 Agent 提示词**：在 `analysis_fundamental.go` 或 `analysis_technical.go` 中，添加更明确的预测指令
2. **增加数据源**：在预测时考虑更多因素（如板块轮动、资金流向等）
3. **调整 backtest 参数**：放宽准确率阈值（如从 5% 调到 10%），延长验证时间窗口

### 短期计划
1. **优化 skill 生成**：改进 `skill_generator.go` 中的提示词模板
2. **添加 backtest 配置**：在前端设置页面添加 backtest 参数调整选项
3. **处理复权**：确保从数据源获取的价格数据已复权

## 附录：关键代码片段

### 1. `extractPrediction()` 函数（改进后）
```go
func extractPrediction(content string) prediction {
	p := prediction{Direction: "中性", Confidence: 0.5}

	// --- 方向判断 ---
	bullish := countKeywords(content, []string{"看涨", "看好", "上涨", "反弹", "乐观", "做多", "买入", "加仓", "牛市", "多头", "强势", "突破"})
	bearish := countKeywords(content, []string{"看跌", "看空", "下跌", "回落", "悲观", "做空", "卖出", "减仓", "回调", "熊市", "空头", "弱势", "破位"})
	neutral := countKeywords(content, []string{"震荡", "盘整", "中性", "观望", "横盘", "整理", "徘徊", "均衡"})

	// ... 其余代码不变
}
```

### 2. 数据库清理工具 (`cmd/clean_backtest/main.go`)
```go
// 软删除无效记录
record.IsDel = 1  // Soft delete flag
saveResult := db.Dao.Save(&record)
```

### 3. Backtest 统计结果
```
Total reports: 7
Verified reports: 2
Accurate predictions: 0
Accuracy rate: 0.00%
Average confidence: 0.40
```

---

**报告生成时间**：2026-07-05 02:02  
**报告生成者**：QClaw Agent  
**下一步**：继续选项 C（优化 skill 生成、添加 backtest 配置、处理复权）
