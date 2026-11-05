# Blades Stock — 智能量化投研桌面工作站

<div align="center">

[![Release](https://img.shields.io/badge/Release-v2.0.0-blue.svg?style=flat-square)](https://github.com/huch16/blades-stock/releases)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8.svg?style=flat-square&logo=go)](https://golang.org/)
[![Wails](https://img.shields.io/badge/Wails-v2.12.0-red.svg?style=flat-square)](https://wails.io/)
[![Vue](https://img.shields.io/badge/Vue-3.5%2B-4FC08D.svg?style=flat-square&logo=vuedotjs)](https://vuejs.org/)
[![License](https://img.shields.io/badge/License-GPL--3.0-green.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey.svg?style=flat-square)](https://github.com/huch16/blades-stock)

**Blades Stock** 是一款专为量化研究员、行业分析师与个人投资者打造的**桌面级智能量化投研工作站**。<br/>
深度融合大语言模型（LLM Agent）、多维量化回测引擎（VectorBT & ETF 动量）、实时盘口异动感知与全网金融情报聚合，提供开箱即用的智能化人机协同投研决策体验。

[核心亮点](#-核心亮点) • [系统架构](#-系统架构) • [功能图景](#-核心功能详解) • [快速开始](#-快速开始) • [模型与数据配置](#-模型与数据配置) • [开发与测试](#-开发与构建) • [开源协议](#-开源许可与免责声明)

</div>

---

## 🌟 核心亮点

- 🤖 **全新 AI 投研驾驶舱（Agent Assistant）**：
  - **自然阅读时间流**：重构自顶向下时间线，杜绝消息逆序与跳动，支持平滑滚动与一键直达最新。
  - **思考推理与工具链回放**：模型思考链（Thinking Process）与数十步量化分析工具（行情拉取、回测、搜索等）支持按需展开/折叠，研报主线清爽易读，底层审计链路完整可溯。
  - **单一状态机流式渲染**：彻底消除流式打字与历史快照消息的双重重叠现象。
- 📊 **3D 个股量化综合诊断**：
  - 结合 **VectorBT 策略回测**（夏普率、最大回撤、盈亏比）、**资金筹码透视**（主力净流入、超大单占比）与**技术形态量化**（均线系统、ATR、乖离率）。
  - 一键输出综合量化评分、买卖与观望评级、建议止损/目标位及主要风控预警。
- ⚡ **AI 异动大盘全景透视（Market Anomaly Vision）**：
  - 盘中实时聚类捕捉主力进攻信号、承压板块与异动特征。
  - **智能回溯兜底机制**：非交易时段（闭市/周末）不置灰，全天候可唤醒；无盘中数据时自动调取当日归档或本地 SQLite 权威库最新交易日异动样本，告别空界面。
  - 与右侧悬浮助理（Floating Agent Assistant）即时中断与多轮推演深度打通。
- 📈 **ETF 动量轮动回测引擎**：
  - 后端深度集成真实动量回测计算引擎（`quant.RunETFMomentumBacktest`），打通 TickFlow API 并支持自定义回测窗口、换仓周期与基准标的。
- 🧠 **向量化技能库动态注入（Vectorized Skill Retrieval）**：
  - 内置 Embedding 向量化检索与余弦相似度匹配，精准命中投研技能（如 Serenity 行业分析师提示模板），动态注入 System Prompt，显著提升垂类推理质量。
- 🌐 **多源情报与 SearXNG 深度联网**：
  - 预设连接开源隐私聚合搜索引擎 SearXNG（默认端点：`https://search.woha.men`），涵盖多源资讯并提供百度 HTTP / 真实无头浏览器双重降级方案；支持 `FetchWebPage` 提取公告与研报正文。
- ☁️ **双模架构与私有云部署 (Dual-Mode & Edge-Native Architecture)**：
  - **一套核心资产，双模形态自适应**：同一仓库既支持编译跨平台独立桌面程序（Windows/macOS/Linux），又可通过内置 Universal Bridge 通信桥接无缝作为 **Web 私有投研工作站** 运行。
  - **Docker 容器化秒级交付**：纯 Go 静态无 CGO 依赖编译，镜像体积 <50MB，运行时内存仅需约 80MB；内置访问口令安全防护（`BLADES_AUTH_TOKEN`）与 SSE 流式实时推送。
  - **单节点权威嵌入式架构（Single-Node SQLite WAL）**：采用 `busy_timeout=30s` 与 `_txlock=immediate` 优化，专为独立交易员与买方投研团队打造，零外部数据库依赖、零网络反序列化损耗，单节点垂直弹性伸缩，提供亚毫秒级行情计算吞吐。
- 🔌 **MCP (Model Context Protocol) 扩展生态**：
  - 支持外部 MCP 服务动态注册、工具白名单管控、自动化心跳探活与故障自愈降级。
- 🛡️ **高可用韧性架构**：
  - 全并发 Goroutine 具备 `recover()` 异常隔离；集成熔断器（CircuitBreaker）、路径防穿越（PathGuard）与 SQLite WAL 优雅退出机制。
  - 跨平台支持：Windows（原生 Toast 通知）、macOS（AppleScript 通知）、Linux（notify-send 通知）。

---

## 🏗️ 系统架构

Blades Stock 采用现代化桌面端混合架构，前端负责交互呈现与图表渲染，后端承载金融数据流清洗、量化计算与 Agent 决策推演：

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        前端呈现层 (Vue 3 + Vite)                        │
│  Naive UI • ECharts • Lightweight Charts • MdEditorV3 • Chat Cockpit   │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ Wails IPC (零延迟高效通信)
┌───────────────────────────────────┴────────────────────────────────────┐
│                        Go 后端服务层 (Go 1.23+)                         │
│                                                                        │
│  ┌──────────────────────┐  ┌─────────────────────┐  ┌───────────────┐  │
│  │   AI Agent 推演核心   │  │    量化回测引擎组    │  │ 数据调度与清洗 │  │
│  │ ───────────────      │  │ ───────────────     │  │ ───────────── │  │
│  │ • 决策推演 & 规划器   │  │ • VectorBT 回测引擎 │  │ • 行情/K线聚合 │  │
│  │ • Embedding 技能检索 │  │ • ETF 动量轮动回测  │  │ • 异动聚类归档 │  │
│  │ • 上下文滑动窗口管理 │  │ • 3D 个股综合打分   │  │ • 资金筹码计算 │  │
│  └──────────┬───────────┘  └──────────┬──────────┘  └───────┬───────┘  │
│             │                         │                     │          │
│  ┌──────────┴─────────────────────────┴─────────────────────┴────────┐  │
│  │                 高可用与安全保障基础设施 (PathGuard / Breaker)      │  │
│  └────────────────────────────────────┬──────────────────────────────┘  │
└───────────────────────────────────────┼────────────────────────────────┘
                                        │
┌───────────────────────────────────────┴────────────────────────────────┐
│                       外部生态与异构数据源接入                          │
│                                                                        │
│  [金融数据]  East Money (东财) • TDX (通达信) • CLS (财联社) • Sina     │
│  [大模型]    DeepSeek • OpenAI • Ollama • LM Studio • vLLM • Qwen      │
│  [知识检索]  SearXNG (search.woha.men) • 百度引擎 • 本地 SQLite 权威库   │
│  [协议扩展]  MCP (Model Context Protocol) 外部服务集群                  │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 💻 核心功能详解

### 1. AI 投研驾驶舱 (Agent Research Cockpit)
- **多轮沉浸式研报问答**：支持自由输入股票代码、行业产业链、宏观周期政策与交易异动问题。
- **透明执行链路**：每轮推演中，Agent 调用的具体工具（如 `ToolMarketData`、`StockKLineData`、`WebSearch`、`ETFMomentum`）均在工具时间线上逐步折叠呈现，耗时与传参一目了然。
- **结构化输出**：自动格式化 Markdown、富文本资金流向图、买卖建议卡片及行动指令（Agent Actions）。

### 2. 3D 个股量化诊断抽屉
- **全方位雷达透视**：
  - **技术面**：均线多空排列、布林通道开口、MACD 背离、ATR 波动率评估。
  - **资金面**：主力大单超大单连续净流入、筹码分布集中度、换手率异常度。
  - **量化回测面**：经典择时与动量策略历史回测（胜率、最大回撤、夏普比率）。
- **可操作性指引**：生成确定性操作建议，提示目标止盈价位、底线止损位与核心风险触发点。

### 3. AI 异动大盘全景透视
- **全时可用监控**：打破“非开盘不可看”的限制，交易时段实时捕捉涨跌停、火箭发射、巨量砸盘等盘口异动；闭市与节假日自动载入最近交易日全景数据进行复盘。
- **主力攻防画像**：多维度聚类提炼强势领涨标的与承压板块，支持一键发送到悬浮助手开展专项推演。

### 4. ETF 动量轮动量化策略
- **周期动量排名**：计算全市场主流行业/宽基 ETF 的区间动量得分（支持 20 日/60 日/120 日等不同参数）。
- **轮动历史回测**：基于 TickFlow API 真实行情测算动量轮动组合净值走势、年化收益率、超额收益与回撤表现。

---

## 🚀 快速开始

### 系统先决条件
- **操作系统**：Windows 10/11 (x64), macOS 12+, Linux (Ubuntu 20.04+, Arch 等)
- **Go 环境**：`Go 1.23` 或更高版本（需加入系统环境变量 `PATH`）
- **Node.js**：`Node.js 18.x` 或更高版本，`npm` 或 `pnpm`
- **Wails CLI**：
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

---

### 1. 获取代码与依赖安装

```bash
# 克隆仓库（默认开发分支为 dev）
git clone https://github.com/huch16/blades-stock.git
cd blades-stock

# 下载 Go 模块依赖
go mod download

# 安装前端依赖
cd frontend
npm install
cd ..
```

---

### 2. 开发模式运行

#### 方式 A：Wails 一键桌面实时开发（推荐）
```bash
wails dev
```
> Wails 会同时启动 Vite 前端热重载开发服务器与 Go 后端 IPC 绑定调试窗口。

#### 方式 B：前后端独立运行调试
```bash
# 终端 1：运行后端（无界面模式/API）
go run ./...

# 终端 2：运行前端
cd frontend
npm run dev
```

---

### 3. 生产环境打包与发布

项目提供了标准构建脚本，桌面端最终二进制文件将统一存放在 `build/bin/` 目录下：

#### Windows 平台（PowerShell）
```powershell
# 编译全套桌面端可执行程序 (build/bin/blades-stock.exe)
.\build.ps1 wails
```

#### Linux / macOS 平台（Bash）
```bash
chmod +x ./scripts/build.sh
./scripts/build.sh
```

---

## ☁️ 云端 Web / SaaS 部署 (VPS Ubuntu 24.04)

项目原生支持以 **Cloud-Native Web / SaaS** 形态容器化部署在云服务器上，多端浏览器（电脑、iPad、手机）直接访问，实现 7×24 小时全天候无人值守盘面盯盘与策略推演。

### 推荐硬件配置（实测极其富余）
- **推荐规格**：**2核 CPU / 12G 内存 / 200GB SSD**（或入门 2核 2G / 2核 4G）
- **资源分配优化**：
  - `blades-stock` 容器限制最大占用 **1.8 核 CPU** 与 **4G 内存**；
  - 预留 0.2 核 CPU 保障宿主机网络与系统平稳运行；
  - 剩余 **8G 内存** 支持在同一台 VPS 上额外挂载 **Ollama 离线大模型** 或 **私有 SearXNG 搜索集群**。

---

### 方式 1：一键自动化部署脚本（推荐）

在全新的 Ubuntu 24.04 VPS 上，执行以下命令即可全自动完成依赖安装、SQLite 权威库注入、随机安全口令生成与服务启动：

```bash
# 1. 克隆仓库
git clone https://github.com/huch16/blades-stock.git
cd blades-stock

# 2. 赋予执行权限并运行一键部署脚本
chmod +x ./scripts/deploy-vps.sh
./scripts/deploy-vps.sh
```

> **执行完毕后**，控制台将输出专属访问地址（如 `http://<你的公网IP>:8000`）与自动生成的安全访问口令（Token）。在任意设备浏览器打开，输入口令即可开启使用。

---

### 方式 2：标准 Docker Compose 手动编排

```bash
# 1. 创建本地持久化目录
mkdir -p data logs
cp build/bin/data/stock.db data/stock.db 2>/dev/null || true

# 2. 启动服务集群
docker compose up -d --build

# 3. 查看实时运行日志
docker compose logs -f blades-stock
```

### 前端静态质量门禁（`npm run lint`）

本仓库的 ESLint 配置（`frontend/eslint.config.js`）遵循三条原则，请勿改回 `vue/flat/recommended`：

1. **只门禁「正确性」，不门禁「排版」。** `flat/recommended` 会打开 400+ 条 `vue/max-attributes-per-line`、
   `vue/singleline-html-element-content-newline` 之类纯格式规则，实测单文件即可产生 600+ warning，
   使 lint 完全失去效力。本仓库未引入 Prettier，故统一使用 `flat/essential`。
2. **错误阻断、警告不阻断。** `npm run lint` 只对 error 返回非零，当前基线：
   **0 errors / 150 warnings**（警告全部为 `@typescript-eslint/no-explicit-any`，属存量类型债，集中在
   ECharts option 构建与 RPC 负载处）。
3. **把历史踩过的坑固化成规则**（这是本门禁的核心价值）：

| 规则 | 拦截的事故 |
|---|---|
| `vue/no-undef-components` | 模板中使用了未 import、也不在 `components.d.ts` 自动注册清单里的组件。白名单自动从 `components.d.ts` 生成（含 kebab-case），并放行 `n-*` Naive 全局组件。**事故原型：`<NSpace>` 未导入导致整视图被 ErrorBoundary 熔断。** |
| `vue/no-undef-properties` | 模板引用了未声明的变量。Vue 会把它编译为 `_ctx.xxx`，**渲染期静默得到 undefined、构建与首屏均不报错**，但事件处理里一赋值就抛 ReferenceError。**实测在 `stockChangesMonitor.vue` 命中 7 处（`klineCode` / `showKlineModal` / `darkTheme` 等），用户点击「📈 K线」「📊 诊断」即熔断。** |
| `unused-imports/no-unused-imports` | 死导入，支持 `--fix` 自动清理。 |
| `tseslint.parser`（`.vue` 的 `parserOptions.parser`） | **没有它就等于没有 lint**：`<script setup lang="ts">` 里的类型语法会全线报 `Parsing error`，导致 50+ 个文件被静默跳过。 |

配套的构建期门禁（`prebuild` / `postbuild`）见 `frontend/scripts/`：
`check_vue_imports.js`（`h()` 调用中的未导入标识符）+ `check_ui_purity.js`（禁止 TDesign 回潮）+
`check_csp_compat.js`（源码资源引用与 CSP 一致性）。

### 🧱 首屏体积基线（请勿回退）

实测（`gzip -9`，与 `dist/index.html` 的 modulepreload 清单严格对齐）：

| 阶段 | 首屏关键路径 |
|---|---|
| 修复前 | 1305 KB gzip |
| **当前基线** | **699 KB gzip（`dist/index.html` 共 8 个资源）** |

当前首屏仅包含：`index`（业务代码）、`vendor-naive`、`vendor-echarts`、`vendor-vue-core`、
`vendor-utils`（naive-ui 的传递依赖 lodash-es/date-fns）、`vendor-icons`、`vendor-preload`（1 KB）、主 CSS。

**以下三块必须保持「不在首屏预加载列表里」**：

| chunk | gzip | 为何不该上首屏 | 保持方式 |
|---|---|---|---|
| `vendor-codemirror` | 547 KB | 仅服务于 md-editor-v3 编辑器路由 | `vite.config.js` 中把 Vite 的 `__vitePreload` 助手隔离到独立的 `vendor-preload` chunk。**否则 Rollup 会把该助手塞进 `vendor-codemirror`，导致入口静态引用它、`index.html` 把整个 CodeMirror 写进预加载列表。** |
| `vendor-markdown` | 354 KB | 仅研报/技能编辑器懒加载路由 | 保持 `md-editor-v3` 只被懒加载组件引用 |
| `StockLightweightKlineChart` | 37 KB | 仅「多周期K线」弹层需要 | `stock.vue` 中显式 `defineAsyncComponent` 覆盖。**否则 unplugin-vue-components 会把模板里的 `<stock-lightweight-kline-chart>` 解析为静态 import 打进入口。** |

验证方法：`grep -o 'assets/[^"]*' frontend/dist/index.html | sort -u`，输出应只有 8 项且不含上述三个 chunk。

```bash
npm run lint       # 只读检查（CI 口径，0 errors 才放行）
npm run lint:fix   # 自动修复未使用导入 / prefer-const / no-var
```

### 常用运维指令
- **查看容器日志**：`docker compose logs -f blades-stock`
- **重启服务**：`docker compose restart`
- **停止服务**：`docker compose down`
- **修改访问口令**：编辑 `.env` 文件中的 `BLADES_AUTH_TOKEN`，执行 `docker compose up -d` 重新加载。

---

### 🛡️ 发布工程规范：部署不得打断在线用户（P0 强制要求）

**问题本质**：Vite 为每个懒加载路由生成**带内容哈希**的 chunk（如 `agent-chat-AmcV2WRK.js`）。
一次 `docker compose up -d --build` 会整体替换容器内的 `frontend/dist`，**所有旧哈希 chunk 立即消失**。
此时任何「已打开旧页面」的用户点击菜单，浏览器就会请求一个已不存在的 chunk → 404 → 动态 `import()` 被拒绝。

这条失败链路发生在**组件挂载之前**，因此 Vue 的 `onErrorCaptured` / `ErrorBoundary` **完全无法捕获**，
表现为：点击菜单无反应、或整页白屏。**服务器端一切正常、日志干净、无任何告警** —— 这是最危险的静默故障。

**本仓库的四层防线（全部默认开启，请勿关闭）**

| 层级 | 机制 | 位置 | 作用 |
|---|---|---|---|
| 1 | `vite:preloadError` + 一次性静默刷新 | `frontend/src/utils/staleChunkGuard.ts` | 旧页面自动跳到新版本，用户无感 |
| 2 | 15s 冷却窗口 + `sessionStorage` 标记 | 同上 | 严格防止「刷新→又失败→再刷新」的无限重载风暴 |
| 3 | 降级提示条（不依赖任何 UI 库） | 同上 | 冷却期内仍失败时，明确提示「请手动刷新」，而不是一片空白 |
| 4 | **上一版本 chunk 服务端回退** | `cmd/server/static.go` + `docker-compose.yml` 挂载 | 旧 chunk 返回 200 而非 404，前端才有机刷完刷新 |

**运维必须遵守的两条规则**

1. **必须使用 `scripts/deploy-vps.sh` 部署**（或在手动部署时先执行「上一步产物快照」）：
   ```bash
   # 脚本在每次构建前自动执行下述快照，无需手工操作：
   mkdir -p frontend/dist-previous && rm -rf frontend/dist-previous/*
   docker cp blades-stock:/app/frontend/dist/. frontend/dist-previous/
   ```
   该目录已通过 `docker-compose.yml` 以**只读**方式挂载回容器 `/app/frontend/dist-previous`。

2. **不要删除 `frontend/dist-previous/`，也不要移除该挂载**。移除后 `/health` 会显示
   `previous_dist_armed=false`，部署将立即重新回到「打断所有在线用户」的状态。

**如何验证回退已生效**

```bash
# 1) 查看发布韧性指标
curl -s http://127.0.0.1:8000/health | grep -o 'release_resilience.*'
# 期望：{"previous_dist_armed":true,"stale_chunk_fallback":N,"stale_chunk_miss":0,...}

# 2) 直接请求一个已不存在的旧 chunk，应返回 200 且带 X-Release-Fallback: previous
curl -sI http://127.0.0.1:8000/assets/index-B0pDqm7D.js | grep -i 'HTTP/\|x-release-fallback\|cache-control'

# 3) 静态资源不可变缓存确认（应为一年 immutable）
curl -sI http://127.0.0.1:8000/assets/index-<当前哈希>.js | grep -i cache-control
```

> `stale_chunk_miss` 是**唯一需要告警**的指标：它表示某个旧 chunk 在当前与上一版本中都不存在，
> 用户此时必然白屏。正常情况下它应恒为 `0`。

---

## ⚙️ 模型与数据配置

启动应用后，点击右上角 **设置 (Settings)** 即可图形化配置或通过环境变量调优：

### 1. 大模型 (LLM) 接入
Blades Stock 采用标准 OpenAI 兼容协议，支持自由接入任意大模型：

| 模型提供方 | Base URL 示例 | API Key | 推荐模型名 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| **DeepSeek** | `https://api.deepseek.com/v1` | 填写实际 Key | `deepseek-chat` / `deepseek-reasoner` | 极高性价比与出色投研推理能力 |
| **OpenAI** | `https://api.openai.com/v1` | 填写实际 Key | `gpt-4o` / `gpt-4o-mini` | 通用标准接口 |
| **Ollama (本地)** | `http://localhost:11434/v1` | **留空即可（自动兜底生效）** | `qwen2.5:14b` / `deepseek-r1:8b` | 完全离线、数据不出本地 |
| **LM Studio** | `http://localhost:1234/v1` | **留空即可** | 本地载入的模型标识 | 本地可视化加载开源大模型 |
| **vLLM / 私有部署** | `http://your-server:8000/v1` | 视集群鉴权而定 | 对应部署服务名 | 企业私有化算力集群 |

> **提示**：v2.0.0 版本完成了工业级全栈买方量化中台（Institutional Buy-Side Quant Platform）高可用、零缺陷、防雪崩与安全合规体系重塑：
> - **统一安全凭据**：Web/SaaS 模式预设并统一登录口令为 `2266762`，开箱即用，支持多端便捷认证。
> - **多规格 VPS 自适应**：深度优化甲骨文云（Oracle Cloud 2C/12G）、Instabox 云沙箱（4C/8G）及通用 Linux 服务器，自动按硬件配置算力限额与内存软限制（GOMEMLIMIT）。
> - **移动端原生体验**：手机端自适应触控底栏（5 大主键 TabBar）、100vw 全景投研抽屉、流式单列卡片与 PWA 全屏沉浸模式。
> - **量化设置中枢**：支持多大模型厂商向导预设、脏检查实时高亮与快捷键全局保存，最大上下文放宽至 `256,000` Tokens。

### 2. 联网搜索引擎配置
系统内置 SearXNG 隐私聚合搜索路由：
- **默认端点**：`https://search.woha.men`
- **备用节点**：`https://search.sapti.me`、`https://searx.be`
- **自定义配置**：可通过系统环境变量覆盖：
  ```bash
  export SEARXNG_URL="https://your-private-searxng.domain"
  ```
- **百度与浏览器降级**：若 SearXNG 节点全部不可用，系统自动降级至百度搜索；可在设置界面开启/关闭无头浏览器搜索抓取。

### 3. 可选微服务与静态数据配置
在 `backend/data/service_config.go` 中支持通过环境变量灵活扩展投研数据中台：
- `BLADES_STOCK_STATIC_DATA_URL`：基础股票列表与行业分类 JSON（默认 `http://8.134.249.145:18080`）
- `BLADES_STOCK_ADATA_URL`：AData 高级财务与指标服务
- `BLADES_STOCK_AKSHARE_URL`：AKShare 数据服务
- `BLADES_STOCK_BAOSTOCK_URL`：Baostock 历史证券行情服务
- `TICKFLOW_API_KEY`：ETF 动量回测引擎的高频金融数据凭据

---

## 🧪 开发与构建

### 代码静态质量检查
跨平台静态语法与类型安全检查（支持 Windows、Linux、macOS 模拟检测）：
```bash
# 全局静态检查 (0 警告、0 错误标准)
go vet ./...

# 模拟跨平台编译检查
GOOS=linux go vet ./...
GOOS=darwin go vet ./...
```

### 运行自动化单元测试
```bash
# 运行后端全量单元测试（包含量化、Agent、数据层、熔断器等）
go test ./backend/...

# 针对 SearXNG 联网检索运行实时测试
go test -v -run "TestSearXNG" ./backend/data/...
```

### 数据库维护与快照同步
项目采用 SQLite 存储本地历史、会话、Agent 技能与配置。如修改了权威数据库 `data/stock.db`，可执行 WAL 归档检查点：
```bash
go run ./cmd/checkpoint_db/ "data/stock.db"
```

---

## 🔒 安全规范与审计机制

1. **凭证隔离与安全传输**：
   - 所有本地存储的 API Key 与鉴权令牌在 SQLite 中以安全配置形式保存，禁止硬编码提交到 Git 仓库。
2. **Agent 自动动作沙箱防护**：
   - 后端对 Agent 生成的页面联动指令（`agent:action`）实施严格校验，过滤空 payload、非法占位符及恶意外部 URI。
   - 前端触发前进行标的代码二次校验，严防误操作。
3. **并发安全与自愈**：
   - 核心数据流协程全量配备 `recover()`，即使外部数据源解析出现畸形数据，亦可将异常局限在单个任务内，桌面主进程保持稳定运行。

---

## 📄 开源许可与免责声明

### 开源协议
本项目基于 **[GNU General Public License v3.0 (GPL-3.0)](LICENSE)** 协议开源。欢迎社区提交 PR、报告 Issue 或参与共建。

### 免责声明
1. **非投资建议**：Blades Stock 及其内嵌 AI Agent、量化回测模型、评分体系输出的全部内容仅供个人学习、量化研究与技术探讨使用，**不构成任何形式的投资建议、买卖要约或收益承诺**。
2. **市场风险提示**：金融市场有风险，投资需谨慎。投资者基于本系统数据、AI 建议或量化模型做出的任何交易决策，须自行承担全部盈亏与合规责任。

---

<div align="center">
Made with ❤️ by the Blades Stock Team & Contributors.<br/>
把 AI 与量化回测带入真正的日常投研工作流。
</div>
