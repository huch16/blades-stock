# Blades Stock — 智能量化投研工作站

<div align="center">

[![Release](https://img.shields.io/badge/Release-v3.1.3-blue.svg?style=flat-square)](https://github.com/huch16/blades-stock/releases)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8.svg?style=flat-square&logo=go)](https://golang.org/)
[![Wails](https://img.shields.io/badge/Wails-v2.12.0-red.svg?style=flat-square)](https://wails.io/)
[![Vue](https://img.shields.io/badge/Vue-3.5%2B-4FC08D.svg?style=flat-square&logo=vuedotjs)](https://vuejs.org/)
[![License](https://img.shields.io/badge/License-GPL--3.0-green.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey.svg?style=flat-square)](https://github.com/huch16/blades-stock)

**Blades Stock** 是一款专为量化研究员、行业分析师与个人投资者打造的**桌面级智能量化投研工作站**。

深度融合 **AI 智能体（Agent）**、多维量化回测引擎（VectorBT & ETF 动量）、实时盘口异动感知与全网金融情报聚合，提供开箱即用的智能化人机协同投研决策体验。支持 **桌面端 GUI** 与 **Web 云端 SaaS** 双模架构，可部署于任意 VPS，多端浏览器访问。

[核心亮点](#-核心亮点) • [系统架构](#-系统架构) • [功能图景](#-核心功能) • [快速开始](#-快速开始) • [部署指南](#-部署指南) • [配置参数](#-配置参数) • [开发与构建](#-开发与构建) • [项目规范](#-项目规范) • [开源协议](#-开源许可与免责声明)

</div>

---

## 🌟 核心亮点

### 🤖 AI 投研驾驶舱（Agent Cockpit）
- **流式思考与工具链回放**：模型思考链（Thinking Process）与数十步量化分析工具（行情拉取、回测、搜索等）支持按需展开/折叠，研报主线清爽易读，底层审计链路完整可溯。
- **单一状态机流式渲染**：彻底消除流式打字与历史快照消息的双重重叠现象。
- **AI 模型无缝接入**：支持 DeepSeek、OpenAI、Ollama、LM Studio、vLLM、SiliconFlow、火山方舟、阿里云百炼等任意 OpenAI 兼容端点，支持统一代理与超时配置。

### 📊 3D 个股量化综合诊断
- **技术面**：均线系统、MACD、RSI、布林带、KDJ 等技术指标。
- **基本面**：营收、利润、现金流、ROE、ROA、毛利率、净利率等财务指标。
- **量化回测**：基于 VectorBT 策略回测，输出胜率、夏普比率、最大回撤、盈亏比等指标。
- **一键诊断**：生成买卖与观望评级，建议止损/目标位及风险预警。

### ⚡ AI 异动大盘全景透视（Market Anomaly Vision）
- **全时可用监控**：打破“非开盘不可看”的限制，交易时段实时捕捉涨跌停、火箭发射、巨量砸盘等盘口异动；闭市与节假日自动载入最近交易日全景数据进行复盘。
- **主力攻防画像**：多维度聚类提炼强势领涨标的与承压板块，支持一键发送到悬浮助手开展专项推演。

### 📈 ETF 动量轮动回测引擎
- 后端深度集成真实动量回测计算引擎（`quant.RunETFMomentumBacktest`），支持自定义回测窗口、换仓周期与基准标的。
- 输出总收益率、年化收益率、最大回撤、胜率、盈亏比等确定性指标。

### 🧠 向量化技能库动态注入
- 内置 Embedding 向量化检索与余弦相似度匹配，精准命中投研技能（如 Serenity 行业分析师提示模板），动态注入 System Prompt，显著提升垂类推理质量。

### 🌐 多源情报与 SearXNG 深度联网
- 预设连接开源隐私聚合搜索引擎 SearXNG（默认端点：`https://search.woha.men`），涵盖多源资讯并提供百度 HTTP / 真实无头浏览器双重降级方案。
- 支持 `FetchWebPage` 提取公告与研报正文。

### 🌐 双模架构与私有云部署（Dual-Mode & Edge-Native Architecture）
- **一套核心资产，双模形态自适应**：同一仓库既支持编译跨平台独立桌面程序（Windows/macOS/Linux），又可通过内置 Universal Bridge 通信桥接无缝作为 **Web 私有投研工作站** 运行。
- **Docker 容器化秒级交付**：纯 Go 静态无 CGO 依赖编译，镜像体积 <50MB，运行时内存仅需约 80MB；内置访问口令安全防护（`BLADES_AUTH_TOKEN`）与 SSE 流式实时推送。
- **单节点权威嵌入式架构（Single-Node SQLite WAL）**：采用 `busy_timeout=30s` 与 `_txlock=immediate` 优化，采用独立运维数据库，零外部数据库依赖、零网络反序列化损耗，单节点垂直弹性伸缩，提供亚毫秒级行情计算吞吐。

### 🔌 MCP（Model Context Protocol）扩展生态
- 支持外部 MCP 服务动态注册、工具白名单管控、自动化心跳探活与故障自愈降级。
- 支持 HTTP/Stdio 传输层，兼容标准 MCP 协议生态。

### 🛡️ 高可用韧性架构
- 全并发 Goroutine 具备 `recover()` 异常隔离；集成熔断器（CircuitBreaker）、路径防穿越（PathGuard）与 SQLite WAL 优雅退出机制。
- 跨平台支持：Windows（原生 Toast 通知）、macOS（AppleScript 通知）、Linux（notify-send 通知）。
- **前后端门禁**：前端 `npm run lint` 0 errors，`npm run build` 三道门禁；后端 `gofmt` / `go build` / `go vet` / 15 个 Go 测试包全绿。

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
│  │ • Embedding 技能检索 │  │ • ETF 动量轮动回测  │  │ • 资金筹码计算 │  │
│  │ • 上下文滑动窗口管理 │  │ • 3D 个股综合打分   │  │ • 异动聚类归档 │  │
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

## 💻 核心功能

| 模块 | 功能 | 说明 |
|------|------|------|
| **AI 投研驾驶舱** | 多轮对话、流式回答、工具调用可视化 | 支持自定义系统提示词与温度参数 |
| **个股量化诊断** | 技术面 / 基本面 / 量化回测三维体检 | 输出买卖观望评级与止损/目标价 |
| **盘面异动监控** | 实时涨跌停、连板梯度、主力资金流向 | 全时段可用，闭市自动回溯 |
| **ETF 动量轮动** | 21 日加权对数动量排名 + 硬风控状态机 | 自定义标的池与超参数 |
| **AI 技能库** | 向量化检索与余弦相似度匹配 | 动态注入 System Prompt |
| **MCP 服务器** | 外部工具动态注册与白名单过滤 | 支持 HTTP/Stdio 传输 |
| **报告生成** | 盘前前瞻 / 盘后复盘 / ETF 轮动 / Serenity 研究 | 异步后台生成，SSE 推送结果 |

---

## 🚀 快速开始

### 系统先决条件

| 组件 | 版本要求 | 说明 |
|------|---------|------|
| **操作系统** | Windows 10/11 (x64), macOS 12+, Linux (Ubuntu 20.04+) | 桌面版 |
| **Go 环境** | Go 1.23+ | 构建与运行后端 |
| **Node.js** | Node.js 18.x+, npm / pnpm | 构建前端 |
| **Wails CLI** | wails/v2 | 桌面端打包（可选） |

### 1. 获取代码

```bash
git clone https://github.com/huch16/blades-stock.git
cd blades-stock
```

### 2. 依赖安装

```bash
# Go 后端依赖
go mod download

# 前端依赖
cd frontend
npm install
cd ..
```

### 3. 开发模式运行

#### 方式 A：Wails 开发模式（推荐）
```bash
wails dev
# 或
go run .
```

#### 方式 B：前后端分离调试
```bash
# 终端 1：启动后端
go run ./cmd/server

# 终端 2：启动前端
cd frontend
npm run dev
```

---

## 🐳 部署指南

### Docker 快速部署（推荐）

```bash
# 构建镜像
docker build -t blades-stock:latest .

# 运行容器（端口 8000，访问口令 2266762）
docker run -d \
  --name blades-stock \
  -p 8000:8000 \
  -e BLADES_AUTH_TOKEN=2266762 \
  -e BLADES_DATA_DIR=/app/data \
  blades-stock:latest

# 查看日志
docker logs -f blades-stock
```

浏览器访问 `http://<服务器IP>:8000`，输入访问口令 `2266762` 即可使用。

### 生产环境部署（VPS）

```bash
# 1. 克隆仓库
git clone https://github.com/huch16/blades-stock.git
cd blades-stock

# 2. 执行部署脚本（自动处理依赖、数据库、口令生成）
chmod +x scripts/deploy-vps.sh
./scripts/deploy-vps.sh

# 3. 访问地址
# http://<你的公网IP>:8000
# 访问口令：2266762（可在 .env 中修改 BLADES_AUTH_TOKEN）
```

### 推荐硬件配置
| 规格 | CPU | 内存 | 磁盘 | 适用场景 |
|------|-----|------|------|----------|
| 推荐 | 2 核 | 12 GB | 200 GB SSD | 生产环境，可额外挂载 Ollama |
| 最低 | 2 核 | 4 GB | 50 GB SSD | 单用户轻量使用 |

> 容器限制建议：最大 1.8 核 CPU / 4 GB 内存，预留 0.2 核给宿主机网络与系统；剩余内存可用于 Ollama 离线大模型或私有 SearXNG 搜索集群。

---

## ⚙️ 配置参数

### 环境变量

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `BLADES_AUTH_TOKEN` | `2266762` | Web 端访问口令 |
| `BLADES_DATA_DIR` | `./data` | 数据目录路径 |
| `BLADES_STOCK_DATA_DIR` | `./data` | 数据目录（备用） |
| `MCP_TIMEOUT_SECONDS` | `8` | MCP 请求超时（秒） |
| `AGENT_TOOL_RUN_BUDGET` | `15` | 单次工具调用预算 |
| `AGENT_TOOL_DAILY_BUDGET` | `200` | 单日工具调用总预算 |
| `AGENT_TOOL_DAILY_COST_USD` | `2.0` | 单日工具调用成本上限（美元） |

### AI 模型配置

支持在「AI 模型注册表」中配置多张模型卡：

| 字段 | 说明 | 示例 |
|------|------|------|
| **名称** | 卡片名称 | `Gemini Pro` |
| **Base URL** | 模型网关地址 | `https://api.deepseek.com/v1` |
| **API Key** | 身份验证密钥 | `sk-xxxxx` |
| **模型名** | 模型标识 | `deepseek-reasoner` |
| **最大输出长度** | maxTokens | `8192` |
| **温度** | temperature | `0.7` |
| **请求超时** | timeOut（秒） | `600`（慢速模型建议 ≥ 300） |
| **深度思考** | thinking | 开/关 |
| **HTTP 代理** | httpProxy | `http://127.0.0.1:7890` |
| **深度思考** | thinking | true/false |

> **注意**：同一 Base URL 下多张卡片，系统优先匹配模型名精确匹配，其次回退到 Base URL 首张卡片。

---

## 🛠️ 开发与构建

### 代码规范

后端：
```bash
gofmt -w .          # 格式化
go vet ./...        # 静态检查
go build ./...      # 编译检查
go test ./...       # 单元测试（15 个包全绿）
```

前端：
```bash
cd frontend
npm run lint        # ESLint（0 errors）
npm run build       # 生产构建（含 CSP / UI 纯度门禁）
```

### 构建桌面端

```bash
# Windows
.\build.ps1 wails

# Linux/macOS
wails build
```

产物位于 `build/bin/` 目录：
- `blades-stock.exe` / `blades-stock`：桌面客户端
- `blades-stock-server.exe` / `blades-stock-server`：Web 服务端

---

## 📄 项目规范

### 提交规范
遵循 约定式提交 格式：
- `feat:` 新功能
- `fix:` 修复 Bug
- `chore:` 构建/工具/依赖
- `docs:` 文档更新

### 测试门禁
- 后端：`go test ./...` 必须全部通过
- 前端：`npm run lint` 必须 0 errors
- 部署脚本：`scripts/smoke-after-deploy.sh` 必须 27/27 通过

---

## 📄 开源许可与免责声明

### 开源许可
本项目基于 **[GNU General Public License v3.0 (GPL-3.0)](LICENSE)** 协议开源。欢迎社区提交 PR、报告 Issue 或参与共建。

### 免责声明
1. **非投资建议**：Blades Stock 及其内嵌 AI Agent、量化回测模型、评分体系输出的全部内容仅供个人学习、量化研究与技术探讨使用，**不构成任何形式的投资建议、买卖要约或收益承诺**。
2. **市场风险提示**：金融市场有风险，投资需谨慎。投资者基于本系统数据、AI 建议或量化模型做出的任何交易决策，须自行承担全部盈亏与合规责任。

---

<div align="center">
Made with ❤️ by the Blades Stock Team & Contributors.<br/>
把 AI 与量化回测带入真正的日常投研工作流。
</div>