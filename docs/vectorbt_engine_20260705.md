# VectorBT 风格向量化回测引擎文档
> 基于 VectorBT 理念，完全替代 blades-stock 旧版事件驱动回测

## 📐 设计理念

VectorBT 的核心突破：**将 O(n) 的循环计算变为 O(1) 的矩阵运算**

| 范式 | 计算方式 | 复杂度 | 代表框架 |
|------|---------|--------|---------|
| 事件驱动 | 逐事件处理 | O(n) | Backtrader |
| 迭代型 | 逐K线迭代 | O(n) | Zipline |
| **向量化** | **矩阵运算批处理** | **O(1)** | **VectorBT / 本引擎** |

性能对比（理论值）：
- 10股 × 10年日线 × 100组参数：
  - 事件驱动：~47分钟
  - 向量化：~0.08秒（提速 35000 倍）

## 📁 文件结构

```
backend/quant/
├── vector.go       # 向量运算（NumPy 等效）
├── indicators.go    # 技术指标
├── portfolio.go    # 组合模拟器
├── engine.go       # 回测引擎 + 网格扫描
└── integration.go  # blades-stock 集成
```

## 🚀 快速开始

### 1. 单次回测

```go
loader := NewStockDataLoader()
close, high, low, open, volume, dates, _ := loader.LoadKLineData("000001", 365)

cfg := DefaultBacktestConfig()
cfg.InitCash = 100000
cfg.Fees = 1.5
cfg.StopLoss = 0.05  // 5% 止损
cfg.TakeProfit = 0.10 // 10% 止盈

engine := NewBacktestEngine().
    Data(close, high, low, open, volume, dates).
    Config(cfg).
    Build()

// 均线交叉策略
strategy := &MAStrategy{FastWindow: 5, SlowWindow: 20}
result := engine.Run(strategy)

fmt.Println(PrintResult(result))
```

### 2. 参数网格扫描

```go
strategies := GenerateMAStrategies(
    []int{3, 5, 7, 10, 15, 20},  // fast windows
    []int{30, 60, 120, 250},      // slow windows
)

result := engine.RunGridSearch(strategies)
fmt.Printf("最优参数: %v, 夏普比率: %.4f\n", result.BestParams, result.BestStats.SharpeRatio)
```

### 3. Agent API 调用

```go
// Agent 工具中直接调用
VectorBTHandler("000001", "MA", map[string]any{"fast": 5, "slow": 20})
VectorBTGridSearchHandler("000001", "MACD")
```

## 📊 预定义策略

| 策略 | 参数 | 说明 |
|------|------|------|
| `MAStrategy` | fast, slow | 均线交叉（金叉买入，死叉卖出）|
| `MACDStrategy` | fast, slow, signal | MACD 交叉 |
| `RSIStrategy` | window, overbought, oversold | RSI 超买超卖 |
| `BollingerStrategy` | window, numStd | 布林带突破 |
| `DualThrustStrategy` | k, period | Dual Thrust 日内策略 |

## 📈 性能指标

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 策略: MA
⚙️  参数: fast=5, slow=20
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💰 总收益率: 25.43%
📅 年化收益率: 8.72%
📉 最大回撤: -12.35%
⚡ 夏普比率: 1.23
📐 索提诺比率: 1.45
🔥 卡玛比率: 0.71
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🎯 胜率: 58.62%
💎 盈亏比: 1.82
📈 交易次数: 29
  ✅ 盈利: 17  ❌ 亏损: 12
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## 🔧 配置选项

```go
type BacktestConfig struct {
    InitCash         float64  // 初始资金，默认 10万
    Fees             float64  // 手续费‰，默认 1.5‰
    Slippage         float64  // 滑点‰，默认 1.0‰
    StopLoss         float64  // 止损%，默认 5%
    TakeProfit       float64  // 止盈%，默认 10%
    TrailStop        float64  // 移动止损%，默认 0（禁用）
    ShortEnabled     bool     // 允许做空，默认 false
    VerifyDays       int      // 验证窗口，默认 20
    AccuracyThreshold float64 // 准确率阈值，默认 5.0%
}
```

## 🔬 向量化运算 API

```go
// 向量创建
v1 := NewVector([]float64{1, 2, 3, 4, 5})
v2 := NewZeros(5)
v3 := v1.AddScalar(10).MulScalar(2) // (1+10)*2, (2+10)*2, ...

// 滚动窗口
sma := v1.RollingMean(3)       // 3日均线
std := v1.RollingStd(5)        // 5日标准差
max := v1.RollingMax(10)       // 10日最高

// 比较运算
crossUp := v1.CrossAbove(100)  // 上穿 100
crossDown := v1.CrossBelow(v2) // v1 下穿 v2

// 布尔组合
signal := entries.And(entries.Or(exits.Not()))
```

## 🆚 vs 旧版 Backtest

| 维度 | 旧版 (event-driven) | 新版 (VectorBT) |
|------|---------------------|-----------------|
| 计算复杂度 | O(n) 每根K线 | O(1) 矩阵运算 |
| 参数扫描 | 不支持 | 数千组合并行 |
| 止损止盈 | 简单实现 | SL/TP/移动止损 |
| 性能指标 | 基础 | 完整（夏普/索提诺/卡玛）|
| 多空支持 | 仅做多 | 多空双向 |
| 代码量 | ~500行 | ~1200行（5文件）|

## 📝 与 blades-stock 现有系统集成

```go
// 替换旧版 BacktestService
import "blades-stock/backend/quant"

// 旧的
service := NewBacktestService()
service.RunAllBacktests()

// 新的
service := NewAIReportBacktest()
service.BacktestAIReports(ctx, 100)
```
