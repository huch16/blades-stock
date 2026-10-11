# 策略注册表 (Strategy Registry)

> 单一事实源：列举例所有内置量化选股 / 评分策略及其适用场景。
> 策略以 YAML 形式存放在 `backend/strategy/strategies/`，由 `StrategyRegistry` 自动加载。

## 策略总览

| 策略 | 分类 | 风险等级 | 持仓周期 | 适用场景 |
|------|------|---------|---------|---------|
| [MACD金叉](#macd金叉) | momentum | 中高风险 | 3-10 个交易日 | 短线反弹 / 动能确认 |
| [均线回踩](#均线回踩) | pullback | 中等风险 | 5-15 个交易日 | 趋势中低吸 |
| [超跌反弹](#超跌反弹) | reversal | 高风险 | 1-5 个交易日 | 暴跌后博弈反弹 |
| [趋势跟随](#趋势跟随) | trend_following | 中等风险 | 10-30 个交易日 | 中线多头趋势 |
| [放量突破](#放量突破) | breakout | 中高风险 | 5-15 个交易日 | 平台突破追涨 |

---

## MACD金叉

- **别名**：`macd_golden_cross` / `macd金叉` / `金叉反弹`
- **分类**：`momentum`
- **判据**：DIF 上穿 DEA 形成 MACD 金叉，且价格位于关键均线上方，捕捉短期反弹动能。
- **描述**：短线动能策略：MACD 金叉预示短期做多动能增强，结合均线与量能确认。
- **基础分**：50

**加分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `macd_golden_cross()` | +30 | MACD 金叉 |
| `macd_above_zero()` | +10 | MACD DIF 位于零轴上方 |
| `above_ma(20)` | +10 | 收盘价站上 20 日均线 |
| `volume_ratio_gt(1.5)` | +10 | 量比 > 1.5，放量确认 |
| `ma_golden_cross(5,10)` | +10 | MA5 同时上穿 MA10 |

**减分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `macd_death_cross()` | -25 | MACD 死叉 |
| `below_ma(60)` | -20 | 收盘价跌破 60 日均线 |
| `change_pct_lt(-5)` | -15 | 单日跌幅 > 5% |

**必需指标**：`macd`, `ma20`, `ma60`

---

## 均线回踩

- **别名**：`pullback_buy` / `回踩低吸` / `均线支撑`
- **分类**：`pullback`
- **判据**：上升趋势中价格回踩关键均线但不破位，缩量回踩确认支撑后低吸。
- **描述**：回踩策略：趋势完好前提下，于均线支撑处低吸，回踩幅度与量能决定进场质量。
- **基础分**：50

**加分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `pullback_to_ma(20)` | +30 | 回踩 20 日均线 |
| `volume_shrink()` | +15 | 成交量萎缩 |
| `macd_above_zero()` | +10 | MACD 零轴上方 |
| `rsi_between(40,60)` | +10 | RSI 处于中位 |
| `higher_low_structure()` | +15 | 高低点抬高 |

**减分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `break_ma(20)` | -30 | 跌破 20 日均线 |
| `break_ma(60)` | -40 | 跌破 60 日均线 |
| `volume_expansion()` | -10 | 放量下跌 |

**必需指标**：`ma20`, `ma60`, `macd`, `rsi`, `volume`

---

## 超跌反弹

- **别名**：`rsi_reversal` / `超跌` / `短线反弹`
- **分类**：`reversal`
- **判据**：股价深度超跌（RSI 进入超卖区）后出现止跌信号，博弈超跌反弹。
- **描述**：反转策略：仅在超卖且出现初步止跌信号时进场，止损严格，快进快出。
- **基础分**：50

**加分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `rsi_lt(30)` | +30 | RSI < 30 超卖 |
| `rsi_cross_above(30)` | +25 | RSI 重新上穿 30 |
| `change_pct_lt(-5)` | +15 | 近 5 日跌幅 > 5% |
| `hammer_candle()` | +20 | 出现锤头/十字星止跌 K 线 |
| `volume_shrink()` | +10 | 缩量止跌 |

**减分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `trend_down()` | -25 | 大趋势仍向下 |
| `break_recent_low()` | -30 | 跌破近期新低 |
| `volume_expansion()` | -15 | 放量下跌（恐慌未止）|

**必需指标**：`rsi`, `macd`, `change_pct`

---

## 趋势跟随

- **别名**：`trend_following` / `均线多头` / `多头趋势`
- **分类**：`trend_following`
- **判据**：均线多头排列、价格站上关键均线且 MACD 位于零轴上方，跟随上升趋势买入。
- **描述**：经典趋势跟踪策略：仅参与已形成多头排列的趋势股，避免抄底与逆势交易。
- **基础分**：50

**加分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `ma_bull_arrangement()` | +30 | 均线多头排列 |
| `above_ma(60)` | +15 | 站上 60 日均线 |
| `macd_above_zero()` | +15 | MACD 零轴上方 |
| `volume_steady()` | +10 | 量能稳定 |
| `higher_highs()` | +10 | 不断抬高的高点 |

**减分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `ma_bear_arrangement()` | -30 | 均线空头排列 |
| `break_ma(20)` | -20 | 跌破 20 日均线 |
| `volume_shrink_on_rise()` | -10 | 上涨缩量 |

**必需指标**：`ma5`, `ma10`, `ma20`, `ma60`, `macd`

---

## 放量突破

- **别名**：`volume_breakout` / `突破新高` / `平台突破`
- **分类**：`breakout`
- **判据**：价格突破近 N 日新高且量能显著放大，确认突破有效性后追涨。
- **描述**：突破策略：放量突破前高是趋势启动的强信号，需量价配合验证突破真实性。
- **基础分**：50

**加分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `break_high_n(20)` | +30 | 突破 20 日新高 |
| `volume_ratio_gt(2)` | +20 | 量比 > 2 |
| `volume_ratio_gt(1.5)` | +10 | 量比 > 1.5 |
| `macd_golden_cross()` | +10 | MACD 金叉 |
| `above_ma(20)` | +10 | 站上 20 日均线 |

**减分项**：
| 条件 | 分值 | 说明 |
|------|------|------|
| `volume_ratio_lt(1)` | -30 | 缩量假突破 |
| `long_upper_shadow()` | -20 | 长上影线 |
| `resistance_zone()` | -15 | 接近强压力位 |

**必需指标**：`high_n`, `volume`, `ma20`, `macd`

---

## 添加新策略

### 步骤

1. 在 `backend/strategy/strategies/` 创建 `xxx.yaml`
2. 按以下 schema 填写：

```yaml
name: 策略显示名
alias:
  - 英文别名1
  - 中文别名
  - 口语化描述
category: momentum | pullback | reversal | trend_following | breakout | ...
judgment_criteria: 一句话判据
description: 详细描述（包含风险与适用）
base_score: 50  # 基础分，候选标的起始分
bonuses:
  - when: "条件表达式"
    score: 30
    desc: "加分说明"
penalties:
  - when: "条件表达式"
    score: 25
    desc: "减分说明"
required_indicators:
  - macd
  - ma20
risk_level: 中高风险 | ...
hold_period: 3-10 个交易日
```

3. 运行单元测试：

```bash
cd backend
go test ./strategy/...
```

4. 在 `catalog/strategies.md` 补充文档

### DSL 函数参考（持续扩展中）

| 函数 | 含义 |
|------|------|
| `macd_golden_cross()` | MACD 金叉 |
| `macd_death_cross()` | MACD 死叉 |
| `macd_above_zero()` | DIF > 0 |
| `ma_bull_arrangement()` | 5>10>20>60 均线多头 |
| `ma_bear_arrangement()` | 空头排列 |
| `ma_golden_cross(fast, slow)` | MA 上穿 |
| `above_ma(n)` | 收盘价 > MA_n |
| `break_ma(n)` | 收盘价 < MA_n |
| `rsi_lt(n)` / `rsi_gt(n)` | RSI 阈值 |
| `rsi_between(a, b)` | RSI 区间 |
| `rsi_cross_above(n)` | RSI 上穿阈值 |
| `volume_ratio_gt(n)` | 量比阈值 |
| `volume_shrink()` / `volume_expansion()` | 量能 |
| `change_pct_lt(n)` | 涨跌幅阈值 |
| `break_high_n(n)` | 突破 N 日新高 |
| `pullback_to_ma(n)` | 回踩 MA_n |
| `hammer_candle()` | 锤头 K 线 |
| `long_upper_shadow()` | 长上影线 |
| `higher_highs()` / `higher_low_structure()` | 结构判断 |

### 相关代码

- `backend/strategy/` — 策略解析与评分
- `backend/quant/` — 向量化回测与因子计算
- `backend/data/stock_data_api.go` — 行情与指标数据源