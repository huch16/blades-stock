# Baostock 集成方案：blades-stock 量化数据增强

## 诊断时间
2026-07-06 00:53

## Baostock 核心能力

| 能力 | 说明 | blades-stock 现状 |
|------|------|-------------------|
| A股日/周/月K线 | 1990-12-19至今，支持前复权/后复权/不复权 | ✅ 已有6个数据源（MAC→东财→Tushare→新浪→腾讯→TDX） |
| 分钟线 | 5/15/30/60分钟 | ✅ MAC/TDX已支持 |
| 交易日历 | `query_trade_dates`，精确到法定节假日 | ❌ **当前只用周末判断，缺节假日** |
| 季频财务数据 | `query_profit_data`（ROE/净利率/毛利率等） | ❌ **缺失** |
| 季频估值指标 | `query_growth_data`（营收/利润增长率） | ❌ **缺失** |
| 季频营运能力 | `query_operation_data`（应收账款周转/存货周转） | ❌ **缺失** |
| 季频现金流量 | `query_cash_flow_data` | ❌ **缺失** |
| 季频杜邦分析 | `query_dupont_data` | ❌ **缺失** |
| 限售解禁 | `query_restricted_release` | ❌ **缺失** |
| 行业分类 | `query_stock_industry` | ❌ **缺失** |
| 证券基本信息 | `query_stock_basic`（上市日期/退市/类型） | ❌ **缺失** |
| 复权数据 | 前复权"2"/后复权"1"/不复权"3" | ⚠️ 部分数据源支持，不统一 |

## Baostock vs 现有数据源对比

| 维度 | Baostock | Tushare | AKShare | 东方财富 |
|------|----------|---------|---------|----------|
| 费用 | 免费 | 积分制（免费有限） | 免费 | 免费 |
| 注册 | 无需注册（anonymous） | 需注册+Token | 无需注册 | 无需 |
| 稳定性 | 高（官方维护） | 中（限频） | 中（依赖爬虫） | 中（API变更） |
| 财务数据 | ✅ 完整季频 | ✅ 完整 | ✅ 完整 | ❌ 不完整 |
| 交易日历 | ✅ 精确 | ✅ 精确 | ✅ 精确 | ❌ |
| 实时行情 | ❌ 仅历史 | ⚠️ 有限 | ✅ 有 | ✅ 有 |
| 港美股 | ❌ 仅A股 | ✅ | ✅ | ✅ |
| 部署方式 | Python SDK（本地） | HTTP API | Python SDK | HTTP API |

## 建议：Baostock 的"最佳用法"

### 不要用来替换现有K线源
现有6源 fallback 链已经足够鲁棒（MAC→东财→Tushare→新浪→腾讯→TDX），Baostock 在实时性和港美股方面不占优。

### 应该用 Baostock 补齐 3 个关键缺口

#### 1. 交易日历（最高优先级）
**现状**：`trading_day.go` 只判断周末，法定节假日（春节、国庆等）会误判为交易日。
**Baostock 方案**：`query_trade_dates(start, end)` 返回精确的交易日历。
**集成方式**：启动时缓存全年交易日历到 SQLite，`IsTradingDay()` 查本地表。

#### 2. 季频财务数据（高价值）
**现状**：blades-stock 完全没有财务数据（ROE、毛利率、净利率、增长率等）。
**Baostock 方案**：`query_profit_data`/`query_growth_data`/`query_operation_data` 等。
**集成方式**：复用 VPS FastAPI 服务模式（同 AKShare），Go 端通过 HTTP 调用。

#### 3. 行业分类 + 证券基本信息（中等价值）
**现状**：板块数据依赖东方财富爬虫，不够结构化。
**Baostock 方案**：`query_stock_industry` 提供申万行业分类。

### 具体集成架构

```
┌─────────────────────────────────────────┐
│  blades-stock (Go)                      │
│  ├── K线数据：现有6源 fallback（不变）   │
│  ├── 交易日历：本地 SQLite 表            │
│  └── 财务/行业数据：HTTP → VPS FastAPI   │
└────────────────┬────────────────────────┘
                 │ HTTP
┌────────────────▼────────────────────────┐
│  VPS 115.190.72.22                      │
│  ├── akshare_service.py (端口 8089)     │  ← 已运行
│  └── baostock_service.py (端口 8090)    │  ← 新增
│      ├── /health                        │
│      ├── /api/v1/trade_calendar         │
│      ├── /api/v1/financial/profit       │
│      ├── /api/v1/financial/growth       │
│      ├── /api/v1/financial/operation    │
│      ├── /api/v1/financial/cash_flow    │
│      ├── /api/v1/financial/dupont       │
│      ├── /api/v1/stock_industry         │
│      └── /api/v1/stock_basic            │
└─────────────────────────────────────────┘
```

### Baostock 的优势

1. **无需注册/Token** — 比 Tushare 更省心
2. **数据权威** — 来源交易所，不是爬虫
3. **历史完整** — 1990年至今
4. **复权完整** — 三种复权方式
5. **Python SDK 本地运行** — 不依赖外部 API 稳定性（但 blades-stock 需要 HTTP 桥接）

### Baostock 的局限

1. **仅A股** — 不支持港美股
2. **无实时行情** — 只能获取T-1及之前的历史数据
3. **Python only** — 需要通过 FastAPI 桥接给 Go
4. **无分钟级指数** — 分钟线不支持指数

## 实施建议

### Phase 1（立即可做）：交易日历
- 在 VPS 创建 `baostock_service.py`
- `query_trade_dates` 获取全年日历 → blades-stock 启动时同步到 SQLite
- 修复 `IsTradingDay()` 从简单周末判断改为查表

### Phase 2（后续）：财务数据
- 暴露 `query_profit_data` 等端点
- Go 端编写 `baostock_api.go`（类似之前的 `akshare_api.go`）
- 注册为 Agent 工具（`GetStockProfit`、`GetStockGrowth` 等）

### Phase 3（可选）：行业分类
- `query_stock_industry` 补充申万行业分类
- 增强 Agent 的行业分析能力
