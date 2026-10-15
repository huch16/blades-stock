# Blades Stock — 智能量化投研桌面工作站

<div align="center">

[![Release](https://img.shields.io/badge/Release-v1.4.0-blue.svg?style=flat-square)](https://github.com/huch16/blades-stock/releases)
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

> **提示**：v1.4.0 版本放宽了最大上下文至 `256,000` Tokens，超时时间上限扩展，避免长文研报与复杂工具推理中断。

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
