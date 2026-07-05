# AKShare 集成总结（2026-07-04）

## 集成架构

```
blades-stock (Go)
    │
    ├── akshare_api.go          ← Go HTTP 客户端（resty/v2）
    ├── akshare_tools_wrapper.go ← Agent 工具封装（12 个工具）
    └── data_tools_wrapper.go   ← 工具注册入口
              │
              ▼
         VPS (115.190.72.22:8089)
              │
              └── akshare_service.py  ← FastAPI 代理（AKShare 1.18.64）
                       │
                       ├── pandas 2.2.3（降级解决兼容性问题）
                       └── akshare 1.18.64
```

## 部署信息

| 项目 | 详情 |
|------|------|
| VPS | 115.190.72.22 |
| 服务端口 | 8089 |
| 服务管理 | systemd (akshare.service) |
| AKShare 版本 | 1.18.64 |
| pandas 版本 | 2.2.3（从 3.0.3 降级） |
| 配置文件 | `/etc/systemd/system/akshare.service` |
| 服务文件 | `/root/akshare-app/akshare_service.py` |

## 已注册的 Agent 工具

| 工具名 | 功能 | 状态 |
|--------|------|------|
| `GetAkshareFuturesSpot` | 商品期货实时行情 | ✅ 正常 |
| `GetAkshareETFSpot` | ETF 实时行情 | ✅ 正常 |
| `GetAkshareETFHistorical` | ETF 历史行情 | ✅ 正常 |
| `GetAkshareBoardConcept` | A股概念板块 | ✅ 正常 |
| `GetAkshareBoardIndustry` | A股行业板块 | ✅ 正常 |
| `GetAkshareMacroCPI` | CPI 数据 | ✅ 正常 |
| `GetAkshareMacroGDP` | GDP 数据 | ✅ 正常 |
| `GetAkshareMacroPMI` | PMI 数据 | ✅ 正常 |
| `GetAkshareMacroMoneySupply` | 货币供应量 | ✅ 正常 |
| `GetAkshareLongHuToday` | 今日龙虎榜 | ✅ 正常 |
| `GetAkshareConvertibleBondSpot` | 可转债实时行情 | ✅ 正常 |
| `CallAkshareFunction` | 通用函数调用 | ✅ 正常 |

## 关键修复记录

### 1. pandas 版本兼容性
- **问题**：`futures_zh_spot`、`futures_treasure_spot` 等函数在 pandas 3.0.3 下崩溃（`ValueError: Length mismatch`）
- **解决**：降级至 pandas 2.2.3
  ```bash
  pip install 'pandas<3.0'
  ```

### 2. URL 编码问题
- **问题**：Go 客户端发送 `market=中金所` 未做 URL 编码，VPS 收到乱码返回 400
- **解决**：`akshare_api.go` 添加 `net/url` + `url.QueryEscape()`
  ```go
  marketEscaped := url.QueryEscape(market)
  resp, err := c.get(fmt.Sprintf("/api/v1/futures/spot?market=%s", marketEscaped))
  ```

### 3. 期货端点旧函数不存在
- **问题**：`akshare_service.py` 使用 `futures_spot_price_sina`（不存在）
- **解决**：替换为 `futures_spot_price`，并添加 `get_latest_trading_date()` 自动找最近交易日

### 4. 工具描述误导性内容
- **问题**：`GetAkshareFuturesSpot` 描述列出"中金所"，但 AKShare 不支持 CFFEX 实时行情
- **解决**：修正描述，明确标注"不支持中金所（金融期货）"

## 数据覆盖范围

### ✅ 已覆盖
- **商品期货**（郑商所/大商所/上期所/广期所/INE）：54 条记录，含现货价、基差等
- **ETF**：实时行情、历史行情（daily/weekly/monthly）
- **板块**：概念板块、行业板块（A 股）
- **宏观**：CPI、GDP、PMI、货币供应量
- **龙虎榜**：异动股票、买卖营业部
- **可转债**：实时行情（含溢价率）

### ❌ 不支持
- **中金所金融期货**（IF/IC/IM/IH 股指期货，T/TF/TS/TL 国债期货）：AKShare 无可用接口
- **原因**：AKShare 1.18.64 中 `futures_zh_spot` 为单合约历史数据函数，非实时行情列表；`futures_treasure_spot` 不存在
- **替代方案**：专业金融终端（Wind、同花顺期货、东方财富期货）或 CFFEX 官方数据

## 本地文件变更

| 文件 | 变更 |
|------|------|
| `backend/data/akshare_api.go` | 添加 net/url，修复 URL 编码 |
| `backend/agent/tools/akshare_tools_wrapper.go` | 修正描述（移除"中金所"） |
| `backend/agent/tools/tool_groups.go` | 工具分组映射（GroupMarket） |
| `backend/agent/tools/data_tools_wrapper.go` | 注册 GetAllDataTools |

## VPS 服务管理命令

```bash
# 查看状态
systemctl status akshare

# 重启服务
systemctl restart akshare

# 查看日志
journalctl -u akshare -f

# 健康检查
curl http://localhost:8089/health
```
