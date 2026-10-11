# Blades Stock（blades-stock）

一个以大语言模型（LLM）为核心、面向桌面与后端的股票分析与智能助理平台。它把多源行情/资金/财务数据、可执行工具（Tool）、以及多步骤 Agent 推理流水线结合在一起，提供：3D 个股量化诊断、向量化回测、AI 投研助手与系统级联动（Agent Actions）。

- 语言：Go（后端）、Vue + TypeScript（前端）
- 框架：Blades Agent（Agent 管道与工具）、Wails（桌面包装）
- 许可：GPL-3.0（详见 LICENSE）

---

## 主要功能

- AI 投研助手（Agent）：多模型容灾、Plan-Execute 与 React 两类推理流程、技能（Skill）注入与语义匹配。
- 3D 个股量化诊断：量化回测（VectorBT）、资金筹码分析与技术形态结合得出综合评分与操作建议。
- 向量化检索与技能系统：Embedding 驱动的 Skill 匹配与策略注入。
- MCP（可扩展远程工具）：从远程 MCP 服务动态加载工具能力并按白名单/权限过滤。
- 桌面 UI：基于 Vue 的界面，支持交互式诊断抽屉、K 线可视化与全局 Agent 控制总线（AgentControlBus）。

---

## 仓库结构（简要）

- backend/ : 后端代码（agent、quant、data、models、db等）
- frontend/: 前端（Vue + TypeScript）与桌面绑定组件
- cmd/     : 若干辅助命令/工具
- docs/    : 文档与架构说明
- data/    : 本地静态数据（如 stock_basic）

---

## 快速开始（开发环境）

先决条件：Go 1.26+, Node.js 16+, Git, Wails CLI（可选用于桌面打包）。

1. 克隆仓库

```bash
git clone https://github.com/huch16/blades-stock.git
cd blades-stock
```

2. 后端（开发）

```bash
# 安装依赖并构建
go mod download
go build ./...

# 运行测试
go test ./...
```

3. 前端（开发）

```bash
cd frontend
npm install
npm run dev
```

4. 启动桌面应用（使用 Wails）

```bash
# 桌面开发模式（需安装 wails）
wails dev
```

构建桌面包时，仓库中提供了封装的脚本（如 build.ps1），可以根据目标平台执行对应脚本。

---

## 配置要点

- 数据库：使用 GORM 管理 Skill、MCPServer、ProjectAnalysis 等表；首次启动会自动建表。
- Embedding/语义服务：在需要语义匹配时配置 Embedding 服务的 baseURL、model 与 apiKey。
- MCP：在 DB 中配置 MCPServer，并通过白名单管理允许加载的工具。
- 环境变量：生产构建中请通过 CI 注入敏感值（例如 BuildKey、API Key 等），不要硬编码。

---

## 开发注意事项

- Agent 输出中可能包含 `agent-action` 代码块（例如：```agent-action
{"type":"OPEN_DIAGNOSIS", "payload":{"code":"600671.SH"}}
```
  后端会解析并在安全检查通过后通过事件下发到前端执行。为避免误触发，后端会过滤掉包含占位符（如 `<<STOCK_CODE>>` 或 `{{STOCK_NAME}}`）的示例动作。
- 前端通过 AgentControlBus（frontend/src/utils/agentControlBus.ts）监听来自后端的 `agent:action` 事件，并在本地注册 handler 执行页面导航、打开诊断抽屉、打开 K 线等联动。
- 若修改 Agent 提示模板或技能相关逻辑，请一并更新对应的测试与示例（backend/agent/types/agent_action_test.go、backend/agent/*）。

---

## 贡献指南

1. Fork → 创建 feature/ 或 fix/ 分支
2. 运行并通过本地测试
3. 提交 PR，说明变更背景、设计与测试方法

---

## 参考与数据源

- 东方财富 / EastMoney（行情、K 线）
- TDX、Tushare（可选数据源）
- 项目内部封装的 Web 爬取与新闻抓取工具

---

## 致谢

感谢开源社区与使用的第三方库：Blades、Naive UI、Wails 等。欢迎提出 Issue、PR 与建议。

---

## 许可证

本项目采用 GNU GPL v3 许可，详见 LICENSE 文件。
