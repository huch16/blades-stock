# Blades Stock（go-stock）——基于大模型的股票分析桌面应用

轻量、可扩展的桌面股票分析工具，使用 Wails + Vue 构建前端界面，Go 驱动后端逻辑，并集成大语言模型用于市场情绪与股票分析。适合想用 AI 助力阅读市场、做快速选股与分析的开发者与爱好者。项目同时包含本地和远程模型接入能力，支持 A 股、港股、美股的数据展示与分析。

---

## 主要特点
- AI 驱动的市场与个股分析（基于大语言模型）
- 多市场支持：A 股 / 港股 / 美股（未来计划扩展基金/ETF）
- K 线、分时、资金流向、行业排名等常用分析图表与数据
- 多轮对话式 AI 分析，支持自定义提示词模板
- 支持多种模型与平台接入：OpenAI、Ollama、LMStudio、本地聚合平台等
- 基于 Wails 的跨平台桌面应用（Windows / macOS / Linux）

---

## 技术栈
- 语言：Go（后端）、Vue（前端）
- 框架 / 工具：Wails v2（桌面封装）、Vite（前端构建）、NaiveUI（UI 组件）
- 主要依赖：wailsapp/wails、Vite、NaiveUI、以及若干数据与 AI 平台相关库

---

## 仓库结构（高层）
```text
. 
├── backend/           # Go 后端代码（数据、模型、工具函数）
├── frontend/          # Vue 前端（Wails 前端资源，Vite 项目）
├── build/             # 打包与截图资源
├── data/              # 本地数据与静态资源（如 stock_basic.json）
├── docs/              # 使用手册与说明文档
├── go.mod / go.sum    # Go 模块依赖
├── wails.json         # Wails 项目配置
├── main.go / app.go   # 应用入口与平台适配
└── README.md          # 项目说明（你将在此处替换）
```

---

## 快速开始（开发环境）
先决条件（建议）：
- Go 1.26.x（与 go.mod 中的 go 版本一致）
- Node.js（用于前端构建，建议 Node 16+）
- Wails CLI（用于开发与打包；可通过 Go 安装）
- Git

开发运行（典型流程）：
1. 克隆仓库：
   git clone https://github.com/huch16/blades-stock.git
   cd blades-stock

2. 安装 Wails CLI（若尚未安装）：
   go install github.com/wailsapp/wails/v2/cmd/wails@latest

3. 启动前端依赖安装（可选，仅用于单独前端开发）：
   cd frontend
   npm install
   # 或 pnpm / yarn 根据你的包管理器

4. 本地开发（在仓库根目录，使用 Wails 启动整应用）：
   wails dev

5. 构建发布版：
   # 构建前端静态资源
   cd frontend
   npm run build
   cd ..
   # 使用 Wails 打包桌面应用
   wails build

注意：如果你只想快速在开发环境查看前端界面，也可以先运行前端 dev server（frontend）并在后端调试。

---

## 配置与数据
- 配置文件：项目通过 `wails.json` 与代码内配置读取运行时相关设置（如 AI 平台密钥、爬虫浏览器路径等）。
- 数据文件：`stock_basic.json`、`backend/data/` 下存放本地基础数据与词典。部分中文分词/字典数据源来自开源项目（请参考 backend/data/data/dict/ 下的说明）。
- 外部数据源：项目历史上使用过 Tushare 等数据源，部分功能可能需要注册并提供 API Key。

在首次运行前，请检查并在设置中填写必要的 API Key 与模型接入信息（OpenAI、Anspire 等），并配置爬虫浏览器路径（若使用资讯抓取功能）。

---

## 功能概览（用户视角）
- 市场总览：全球行情、热门话题、重大事件时间轴
- 个股详情：K 线、分时、资金流向、研报与公告检索
- AI 分析：基于预��或自定义提示词模板生成分析报告；支持多轮跟进问题
- 选股与策略：AI 智能选股、指标选股、行业排名
- 通知：钉钉等告警通知集成（可配置）

---

## 常见问题 & 调试建议
- 在不同平台上打包时，注意 Wails 与平台依赖（macOS 需交叉编译或在 macOS 上打包）。
- 前端构建失败：先在 `frontend/` 目录执行 `npm ci` 或 `npm install`，确认 Node 版本兼容。
- AI 请求失败：检查模型接入配置与网络权限（公司环境可能需要代理或允许外网访问）。
- 数据延迟或缺失：确认数据源接口有效，或检查 `backend/data` 中静态文件是否齐全。

---

## 贡献与交流
欢迎通过 Issue / PR 报告问题或提交改进。若需要长期协作或有商业咨询，请在仓库 Issue 中标注或在项目文档中查看联系方式。

贡献流程（简单）：
1. Fork 项目并创建分支：feature/xxx 或 fix/yyy
2. 提交并推送分支，发起 PR，描述变更与复现步骤
3. 通过 CI 与代码审查后合并

---

## 许可
本项目采用 GNU GPLv3 许可（请参见仓库 LICENSE 文件）。

---

感谢使用与关注。如果你愿意，我可以：
- 把上面的 README 替你提交成一个 PR（需你授权目标 repo 和分支信息），
- 或根据你想突出的内容（例如企业品牌、安装包下载链接、截图更新）进一步定制 README。
