# Blades Stock v3.0.0 使用手册

> 本文件通过 `//go:embed` 打包进二进制，并由 RPC `GetUserManual()` 返回。
> 请始终以 **UTF-8** 保存（历史版本曾以 GBK 保存，导致接口返回乱码）。

---

## 1. 两种运行形态

| 形态 | 启动方式 | 适用场景 |
|---|---|---|
| **桌面 GUI** | `build.ps1 wails`（Windows）/ `wails build`（Linux/macOS） | 本机个人使用，免部署 |
| **云端 Web/SaaS** | `bash scripts/deploy-vps.sh`（Docker） | 多设备访问、与团队共享 |

两者共用同一套前端与后端代码（Single Source of Truth）；
Web 模式下前端通过 `frontend/src/api/bridge.ts` 把 Wails 调用透明代理为 `/api/App/{Method}`。

### 登录

Web 模式的访问口令**没有默认值**，必须自行设定（推荐写入 `.env` 的 `BLADES_AUTH_TOKEN`）：

```bash
# .env
BLADES_AUTH_TOKEN=<your-token>
```

未设定时，服务端首次启动会随机生成一个口令并固化到 `data/auth.token`（0600），
同时在启动日志里打印一次，请及时保存。部署脚本（`scripts/deploy-vps.sh` /
`scripts/deploy-docker.sh`）会在你未设定时自动生成一个随机口令并在部署成功后
打印一次。**旧的公开默认口令已废除**：任何已部署的旧服务请立即更换口令并删除
旧的 `data/auth.token`（见 `SECURITY.md`）。

登录成功后服务端下发 **HttpOnly** Cookie `blades_token`（30 天），
SSE 事件流与 RPC 均可凭该 Cookie 鉴权，无需在 URL 中暴露 Token。

---

## 2. 六大主页面

| 路由 | 页面 | 用途 |
|---|---|---|
| `#/` | **股票看板** | 自选股分组、实时盘口、分时/多周期K线、个股三维量化诊断 |
| `#/market` | **宏观智能** | 指数全景、资金流向、板块热力、龙虎榜、投资日历 |
| `#/research` | **投研账本** | 研究中心 6 个页签（见下）、AI 研报与认知账本 |
| `#/agent` | **Agent 助手** | 与投研 Agent 对话，支持 L1–L5 五阶量化投研天梯 |
| `#/settings` | **设置中枢** | 模型注册表、Prompt 中枢、数据源与推送、缓存维护 |
| `#/fund` | **基金行情** | 基金排行、持仓穿透、净值走势 |

> 应用使用 **hash 路由**，因此可直接把 `https://<域名>/#/research` 这样的链接分享给别人。

### 研究中心 6 个页签（`#/research`）

1. **⚡ 盘面大单异动监控** — 大单异动实时流，支持「排雷 / K线 / 诊断」一键下钻
2. **🪜 涨停连板梯队复盘** — 连板高度与梯队结构
3. **🎯 量化选股策略库** — 内置策略筛选与回测入口
4. **📝 实盘交易台账** — 交易记录与盈亏统计
5. **⏰ 定时自动化任务** — 盘前/盘后自动研报与推送
6. **📑 AI研报与认知账本** — 历史研报检索 + 买方认知（Thesis）演化时间线

---

## 3. 核心功能

### 3.1 个股三维量化诊断

打开任意自选股的「📊 诊断」，得到：

- **综合评分（0–100）**：量化 / 资金筹码 / 技术波动 三维度加权
- **纯 Go 确定性回测**：近 250 个交易日、前复权（QFQ）、MA5/MA20 趋势跟踪、
  费率 佣金 0.025% + 滑点 0.1%，附带**策略净值 vs 买入持有基准**曲线
- **ATR 动态风控**：止损价与第一目标价均基于实时 ATR 波动通道测算
- **数据溯源**：窗口、复权口径、数据源与 `codeVersion` 全部显式披露

> ⚠️ 现价跌破 MA20 时会强制锁定防守并抑制做多目标价；交易样本 < 20 次会做置信度折价。

### 3.2 ETF 动量轮动（stevia0829）

以 **510300（沪深300ETF）MA120 + 动态回撤** 作为宏观硬风控熔断：

- 逻辑上穿 → `OFFENSE`（进攻）；逻辑破位 → `DEFENSE`（防守，硬门禁）
- 21 日加权对数回归动量与 R² 由**纯 Go 矩阵运算**得出，**不经过大模型**
- 处于 `DEFENSE` 时，动量第一名的 ETF 仅作为**观察标的**，权益仓位为 0

### 3.3 盘前持仓风控扫描

零 Token 消耗的确定性扫描器，输出 🟢/🟡/🔴 风险分级、动态防守位（MA5/MA20）
与数据溯源；宏观 DEFENSE、5 日回撤、Thesis 证伪条件均为**硬覆盖**。

### 3.4 定时任务

设置页「数据源与预警推送」可配置钉钉机器人 Webhook 与桌面原生弹窗；
盘前/盘后研报由 `#/research` 的「定时自动化任务」管理（Cron 表达式 + 参数 JSON 预览）。

---

## 4. 模型接入

设置页「🤖 AI模型注册表」中每张模型卡只需 **5 个字段**：
`名称 / Base URL / API Key / Model Name / Thinking`。

其余调度参数（超时、并发、Token 上限）由 Go 运行时自动调优，无需手工填写。

常见接入：

| 厂商 | Base URL | Model Name |
|---|---|---|
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |
| MiniMax | `https://api.minimaxi.com/v1` | `MiniMax-M3` |
| 硅基流动 | `https://api.siliconflow.cn/v1` | `deepseek-ai/DeepSeek-V3` |
| Ollama（本地） | `http://127.0.0.1:11434/v1` | `qwen2.5:7b` |

> API Key 在读取与导出配置时自动掩码（`sk-****`），不会明文外泄。

---

## 5. 运维速查（Web / Docker）

```bash
docker compose logs -f blades-stock      # 实时日志
docker compose restart                   # 重启
bash scripts/deploy-vps.sh               # 部署（自动跑 27 项冒烟自检）
bash scripts/smoke-after-deploy.sh       # 单独重跑冒烟自检
curl -s http://127.0.0.1:8000/health     # 健康 + 发布韧性指标
```

### 健康指标怎么看

`/health` 的 `release_resilience` 段：

| 字段 | 含义 |
|---|---|
| `previous_dist_armed` | 旧版 chunk 回退是否已武装（**必须为 true**） |
| `stale_chunk_fallback` | 旧 chunk 命中回退的次数（>0 说明仍有用户停留在旧版本页面） |
| `stale_chunk_miss` | 当前与上一版都找不到的 chunk 数（**必须为 0，非 0 即用户白屏**） |
| `denied_path_probes` | 被敏感路径清单拦截的探测次数（扫描器/攻击探测） |
| `spa_fallback_hits` | 非根路径回落到 SPA 壳的次数（用于区分真实页面与 404 掩盖） |

---

## 6. 常见问题

**Q：部署后用户点击菜单无反应 / 白屏？**
A：这是旧 chunk 404。检查 `/health` 的 `previous_dist_armed` 是否为 `true`；
若为 `false`，说明部署时没有快照 `frontend/dist-previous/`——请始终用 `scripts/deploy-vps.sh` 部署。

**Q：分时图 / 日K图空白？**
A：生产 CSP 为 `img-src 'self' data: blob: https:`，任何 `http://` 图片都会被静默阻断。
构建期由 `frontend/scripts/check_csp_compat.js` 拦截此类引用。

**Q：实时推送、Agent 流式输出没反应？**
A：验证 `curl -m 5 -H "Authorization: Bearer <口令>" http://127.0.0.1:8000/api/events/stream`
应返回 200 且 `Content-Type: text/event-stream`；无凭证应返回 401。

**Q：`npm run lint` 报错？**
A：当前基线是 **0 errors / 150 warnings**；warnings 均为存量 `no-explicit-any`，
不阻断。若出现 error，请修复后再提交。

---

## 7. 免责声明

本系统为**量化投研算法辅助工具**。所有指标、回测数据、ATR 阻力通道与风控结论
均基于客观公开数据推演，**不构成任何投资建议或买卖要约**。
证券市场有风险，投资决策需审慎独立。
