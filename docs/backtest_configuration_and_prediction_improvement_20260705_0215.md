# Backtest 配置与预测准确性改进报告（2026-07-05）

## 完成的工作

### ✅ 选项 B（高优先级）已完成
1. **清理无效数据**：
   - 创建了数据库清理工具 `cmd/clean_backtest/main.go`
   - 软删除了 15 条无效 `AIResponseResult` 记录（股票代码为"市场资讯"、"agent"等）
   - 验证了清理生效（总报告数从 22 降到 7）
   - 备份了数据库（`data/stock.db.backup_20260705_015930`）

2. **改进预测提取**：
   - 添加了更多关键词到 `extractPrediction()` 函数（`analysis_backtest.go`）
   - 看涨关键词：添加"牛市"、"多头"、"强势"、"突破"
   - 看跌关键词：添加"熊市"、"空头"、"弱势"、"破位"
   - 中性关键词：添加"整理"、"徘徊"、"均衡"

3. **修复工具编译**：
   - 修复了 `cmd/backtest/main.go` 中的导入错误
   - 将 `db.InitDB()` 改为 `db.Init("data/stock.db")`
   - 删除了未使用的导入（`data`、`models`、`time`）

### ✅ 选项 C（中优先级）进行中
1. **优化 skill 生成**：
   - 改进了描述模板（`skill_generator.go` 第 82 行）
   - 从通用描述改为更专业的描述

2. **添加 backtest 配置（后端）**：
   - ✅ 在 `Settings` 结构体添加了 3 个配置字段：
     - `BacktestAccuracyThreshold`（float64，默认 5.0）
     - `BacktestVerifyDays`（int64，默认 2）
     - `BacktestEnabled`（bool，默认 true）
   - ✅ 修改了 `RunSingleBacktest` 函数读取配置
   - ✅ 后端代码编译成功
   - ⚠️ 临时修复：`verifyWindow` 使用默认值 20（待从配置读取）

3. **添加 backtest 配置（前端）**：
   - ✅ 在前端设置页面（`settings.vue`）添加了 backtest 配置字段到 `formValue`
   - ✅ 在模板部分添加了 backtest 配置的 UI 元素：
     - 回测准确率阈值（%）- `n-input-number`
     - 回测验证天数 - `n-input-number`
     - 启用回测 - `n-switch`
   - ⏳ 待测试：前端 UI 是否正常工作

## 进行中的工作

### ⏳ 改进 Agent 预测准确性
- **问题**：backtest 准确率仍为 0.00%（TCL 科技预测"看涨"，实际跌 4.56%）
- **可能原因**：
  1. 提示词不够精确，Agent 没有充分考虑技术指标和资金流向
  2. 数据源不足（仅用 K 线 + 新闻，缺少板块轮动、龙虎榜等）
  3. 验证时间窗口可能不合适
- **待做**：
  1. 优化 Agent 提示词（查看 `prompt_templates` 表，改进提示词）
  2. 增加数据源（板块轮动、资金流向、龙虎榜等）
  3. 调整 backtest 配置（放宽准确率阈值、延长验证时间窗口）

### ⏳ 处理复权（待开始）
- **目标**：确保价格数据准确性（前复权/后复权）
- **待做**：
  1. 检查 K 线数据 API 是否支持复权数据
  2. 如果不支持，寻找替代数据源

### ⏳ 修复 Wails 构建（阻塞）
- **问题**：`wails build` 失败（`package "github.com/go-kratos/blades" without types`）
- **待做**：
  1. 向 Wails GitHub 提交 Issue
  2. 或尝试其他构建方法

## 测试结果

### Backtest 工具测试
```
Total reports: 7
Verified reports: 2
Accurate predictions: 0
Accuracy rate: 0.00%
Average confidence: 0.40

Direction breakdown:
  看涨: 0/2 (0.00%)
```

### 分析
- 仅 2 条报告被验证（其他 5 条因价格数据不足被跳过）
- 2 条报告均预测"看涨"，但实际价格下跌（准确率 0.00%）
- **结论**：Agent 预测本身不准确，需要改进提示词或增加数据源

## 下一步建议

### 立即行动
1. **改进 Agent 提示词**：
   - 查看 `prompt_templates` 表中的提示词
   - 优化提示词，强调预测准确性
   - 添加更多技术指标和资金流向的分析要求

2. **增加数据源**：
   - 确保 Agent 分析时调用所有可用工具（板块轮动、资金流向、龙虎榜等）
   - 检查 `tool_groups.go` 中所有工具是否已注册

3. **调整 backtest 配置**：
   - 放宽准确率阈值（如从 5% 调到 10%）
   - 延长验证时间窗口（如从 2 天调到 5 天）

### 短期计划
1. **前端 backtest 配置测试**：确保前端 UI 正常工作，配置能保存到数据库
2. **处理复权**：检查 K 线数据 API 是否支持复权数据
3. **优化 skill 生成**：进一步改进 `skill_generator.go` 中的提示词模板

## 关键文件修改记录

### 后端修改
1. `backend/data/settings_api.go`：
   - 添加 `BacktestAccuracyThreshold`、`BacktestVerifyDays`、`BacktestEnabled` 字段

2. `backend/agent/analysis_backtest.go`：
   - 修改 `RunSingleBacktest` 函数读取配置
   - 改进准确率判断逻辑（使用可配置阈值）
   - 临时修复编译错误（`verifyWindow` 使用默认值 20）

3. `backend/agent/skill_generator.go`：
   - 改进描述模板（第 82 行）

### 前端修改
1. `frontend/src/components/settings.vue`：
   - 添加 backtest 配置字段到 `formValue`
   - 添加 backtest 配置的 UI 元素（准确率阈值、验证天数、启用开关）

### 工具创建
1. `cmd/clean_backtest/main.go`：数据库清理工具
2. `cmd/view_prompts/main.go`：查看提示词模板工具（编译失败，待修复）

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

### 2. Backtest 配置字段（`Settings` 结构体）
```go
type Settings struct {
	// ... 其他字段
	BacktestAccuracyThreshold float64 `json:"backtestAccuracyThreshold" gorm:"column:backtest_accuracy_threshold;default:5.0"`
	BacktestVerifyDays        int64   `json:"backtestVerifyDays" gorm:"column:backtest_verify_days;default:2"`
	BacktestEnabled           bool    `json:"backtestEnabled" gorm:"column:backtest_enabled;default:true"`
}
```

### 3. 准确率判断逻辑（改进后）
```go
// 判断方向准确（使用可配置阈值）
isAccurate := false
// accuracyThreshold 和 verifyDays 已在函数开头读取
switch pred.Direction {
case "看涨":
	isAccurate = priceChange > accuracyThreshold
case "看跌":
	isAccurate = priceChange < -accuracyThreshold
case "震荡":
	isAccurate = priceChange > -3 && priceChange < 3
case "中性":
	isAccurate = priceChange > -5 && priceChange < 5
}
```

---

**报告生成时间**：2026-07-05 02:15  
**报告生成者**：QClaw Agent  
**下一步**：改进 Agent 预测准确性（优化提示词、增加数据源）、测试前端 backtest 配置 UI
