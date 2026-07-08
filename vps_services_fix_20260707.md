# VPS 数据服务修复与集成 — 2026-07-07

## 目标

修复 blades-stock 项目依赖的三个 VPS 数据服务的健康状态和本地代码集成。

## 诊断发现

| 服务 | 端口 | 修复前状态 | 修复后状态 |
|------|------|-----------|-----------|
| AData | 8000 | ✅ 运行中，但本地代码 API 端点/响应格式不匹配 | ✅ 修复代码，服务正常 |
| AKShare | 8089 | ✅ 运行中（有外部扫描噪音） | ✅ 服务正常 |
| Baostock | 8090 | ❌ systemd 服务不存在，服务文件缺失 | ✅ 已部署并启动 |

## 修复内容

### 1. Baostock 服务部署（VPS）
- 从 `C:\Users\huch\.qclaw\workspace\baostock_service.py` 上传到 VPS `/root/akshare-app/baostock_service.py`
- 创建 systemd 服务文件 `/etc/systemd/system/baostock.service`
- 已 `enable` + `start`，开机自启
- 健康检查：`{"status":"ok","baostock_version":"00.9.20"}`

### 2. adata_api.go 重写
- **问题**：原代码假设 AData 返回 `{"code":0,"data":[...]}` 格式，实际返回 `{"value":[...],"Count":N}`
- **问题**：`Ping()` 调用 `/api/health`（404），实际端点是 `/health`
- **修复**：重写为 `adataValueResponse` 结构体解析 `{"value":[...]}` 格式
- **修复**：`Ping()` 改用 `/health` 端点
- 新增 `GetTradeCalendarDays()` 方法，提取 `trade_status=1` 的日期

### 3. trading_day.go 重写
- **优先级**：AData（8000）→ Baostock（8090）→ 周末判断
- `InitTradingCalendar()` 先尝试 AData，失败降级 Baostock，再失败降级周末判断
- AData 交易日历已验证：2026 年 365 天中 242 个交易日，节假日（元旦、春节、清明、劳动节、国庆等）正确标记为非交易日

### 4. app.go domReady 集成
- 启动时异步初始化 `InitTradingCalendar()`
- 异步执行 AData 和 AKShare 的 `Ping()` 健康检查
- 失败时仅警告日志，不阻塞启动

### 5. adata_tools_wrapper.go 修复
- `GetADataTradeCalendar` 工具调用 `GetTradeCalendar()` 不再传 `year` 参数（AData 返回全年日历）

## 验证

- `go build ./...` ✅
- `go vet ./...` ✅
- `staticcheck ./...` ✅
- 三个 VPS 服务健康检查全部通过
