# 设置页重构规格蓝图 (Settings Page Refactor Spec)

> 版本: 1.0 | 状态: 存档备用 | 创建时间: 2024-12 | 关联提交: 后续迭代参考

---

## 🎯 核心目标
按 **功能分组 + 渐进式暴露** 重构设置页：
- 高级/危险选项默认收起
- 每字段含说明、示例、校验
- 敏感项（API Key、DB 路径）安全提示与保护
- 提供“恢复默认”、“导出/导入”、“一键 checkpoint/备份”
- 配置概要、生效提示、备份入口

---

## 🔴 现状痛点
- 设置项杂乱：开发/运行/打包参数与业务配置混杂
- 缺说明/示例：字段无上下文，易填错
- 敏感信息处理不当：API Key 可能明文显示/导出
- 无恢复/迁移能力：无“恢复默认/重置/导出配置”
- 桌面版 DB 策略未显性化：误操作可能破坏数据

---

## 📋 分层改进计划

### 1️⃣ 快速可见改进（低成本、高收益）
- **分组折叠**：通用 / AI 模型&Key / 数据与DB / 接口服务URL / 工具技能开关 / 开发者调试 / 主题本地化
- **字段元数据**：短说明 + 示例 + 校验（URL/端口/布尔/数字范围）
- **保存/取消/重置**：未保存提示、单组/全局“恢复默认”
- **敏感字段**：API Key 掩码显示、显示/复制/删除操作、禁止明文导出

### 2️⃣ 中期改进（中等开发量）
- **普通/高级模式**：高级模式默认关，开启后显示 DB路径/自动迁移/Wails打包参数
- **配置概要面板**：只读卡片显示关键配置（DB位置、启用技能、当前模型Provider）
- **导入/导出 JSON**：自动去敏感字段 + 提醒
- **生效标识**：立即生效 vs 需重启/Checkpoint，清晰提示 + 一键链接
- **变更历史**：最后修改时间/用户

### 3️⃣ 深度/长期优化（高成本）
- **配置档**：多套配置
- **技能/工具可视化**：触发词、注入prompt预览、黑白名单
- **权限审计**：谁改了什么（关联 agent_sessions/agent_traces）
- **远程同步**：静态数据URL/远端config服务 + 本地优先
- **打包快照**：导出含 build/bin/data/stock.db snapshot 规则

---

## 🔧 针对 blades-stock 的具体对接点

### DB Canonical + Checkpoint
- 显示 `paths.BaseDir()` 解析出的仓库根 `data/stock.db` 为唯一权威库
- 显示当前运行时 DB 路径、修改时间、文件大小
- **一键 Checkpoint**：调用 `cmd/checkpoint_db/` 或后端 RPC，完成后 toast+操作日志
- 禁止直接选 `build/bin/data/stock.db`，提示风险并引导“复制到仓库根”

### AI 模型与密钥
- Provider 选择器 + 支持模型列表（来自 `chat_model_factory.go`）
- API Key 掩码 + OS keyring/加密存储（Wails 调用 OS keyring）
- **测试连接**按钮：每个 Provider + BLADES_STOCK_* URL 均提供

### Skills & Tools
- Skill 列表：启用/禁用开关、查看关键词、预览注入 prompt
- Tool Groups 黑/白名单多选

### 日志/诊断/开发者
- 日志级别选择器（即时生效）+ 当前日志路径/片段
- **导出诊断包**：日志 + agent_traces + agent_tool_ranks → 压缩包

---

## 📦 建议设置页结构（字段示例）

| 分组 | 关键字段 |
|------|----------|
| **通用** | 语言、主题、自动更新 |
| **AI & LLM** | Provider、Model、API Key(掩码+测试)、超时/Tokens、流式、系统提示预览 |
| **数据 & DB** | 当前DB路径(只读+打开位置)、更改DB根路径(高级+警告)、一键Checkpoint、自动Checkpoint频率、备份目录 |
| **服务与接口** | STATIC_DATA_URL/ADATA_URL/AKSHARE_URL (URL+测试按钮) |
| **工具/技能** | Skill列表(开关+关键词预览)、Tool Groups 黑白名单 |
| **安全隐私** | API Key存储策略、导出配置(掩码提醒) |
| **开发者(高级)** | 日志级别、自动迁移、Wails打包标志、Go/Wails版本(只读) |
| **账号/导入导出** | 导出/导入JSON、配置档管理 |

---

## 💻 前端实现建议 (Vue 3 + Naive UI + Pinia)

### 组件划分
- `SettingsPage.vue`：主容器，加载/保存到 Pinia `settingsStore`
- `SettingsGroup.vue`：通用折叠面板（说明、重置默认按钮）
- 字段组件：`TextField`、`MaskedField`(API Key)、`UrlField`(测试按钮)、`PathPicker`(权限校验)、`SwitchField`、`SelectField`、`ProfileManager`

### 状态管理
- Pinia `settingsStore`：当前配置、原始配置(用于取消/对比)、修改时间、undo 支持
- 保存流程：Wails binding → 后端 RPC → 更新本地 store；标记需重启项

### 后端绑定 (Wails RPC)
- `GetSettings()` / `SaveSettings(map)` / `TestServiceURL(url)` / `RunCheckpoint()` / `ExportDiagnostics()`
- 敏感信息不返回前端（仅返回是否已保存标志）

### UX 细节
- Debounce autosave（可选）+ 明确 Save/Cancel
- 测试连接/Checkpoint：Loading 状态 + 成功/错误消息 + 日志链接
- 危险操作：确认弹窗 + 后果说明 + dry-run 预览

---

## 🧪 校验/测试/部署建议
- 单测：store 序列化/反序列化、恢复默认、profile 导入导出
- E2E：Playwright 覆盖修改 API Key、测试连接、导出/导入、Checkpoint
- 文档：README/docs 增加“设置页使用说明”，特别是 DB canonical + Checkpoint 命令
- 迁移策略：配置 schema 变更时提供迁移脚本，保证平滑升级

---

## 📌 现有代码复用点（零成本起步）

| 能力 | 现有文件/模块 |
|------|--------------|
| Settings/ AIConfig 结构与 Get/Set | `backend/data/settings_api.go` |
| Skill CRUD + MCP Server 绑定 | `backend/data/skill_api.go` |
| Provider/模型检测 | `backend/agent/chat_model_factory.go` |
| 前端入口 | `frontend/src/stores/aiConfig.ts`、`frontend/src/components/settings.vue` |
| 审计/指标导出 | `backend/agent/audit.go`、`backend/agent/telemetry.go` |
| Checkpoint 实现 | `cmd/checkpoint_db/` |
| HTTP 客户端/服务测试 | `backend/data/httpclient.go`、`backend/data/market_statistic_api.go` |

---

## 📋 执行顺序建议（ROI 排序）
1. **快速胜利**：分组折叠 + 说明/示例 + 掩码 + 保存/取消/重置
2. **核心安全**：API Key 加密存储、DB路径只读 + Checkpoint 按钮
3. **可迁移**：导出/导入 JSON(去敏感)、配置档
4. **诊断增强**：一键导出诊断包、服务URL测试、日志级别热切换

---

## 📌 存档说明
- 本文档作为后续迭代蓝图长期保存
- 相关代码基础已就绪（见上表），可分阶段落地
- 后续迭代可直接参考此蓝图，按阶段拆分 Issue/PR

---

*存档于: docs/settings_page_refactor_spec.md*
*关联提交: 待后续迭代创建*
