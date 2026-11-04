# Release v2.0.0

发布日期: 2026-11-01

概览
----
Blades Stock v2.0.0 是项目迈入**商业量化中台（Institutional Buy-Side Quant Platform）**的里程碑版本。本版本全面对标买方机构量化投研中枢的最高可靠性要求，围绕**高可用（High Availability）、零缺陷（Zero Defect）、防雪崩与熔断隔离（Anti-Avalanche & Circuit Breaking）、以及严格安全合规（Security & Compliance）**标准，完成了全栈七大维度的深度代码审计、系统重构与生产环境固化。

Highlights / 核心亮点（全栈七大维度）
--------------------------------------
1. **① 前端人机交互与现代投研工作舱 (Frontend Workstations & Ergonomics)**
   - **BUIS 5阶时序量化投研天梯全面重塑**：重构 `agent-chat.vue`，彻底消除原界面三层功能堆叠，提供 L1 宏观择时、L2 赛道主线、L3 因子初筛、L4 四维验质、L5 买方立论五阶严格等高对称决策卡片。
   - **单入口标的贯通中枢 (Single Target Cockpit)**：统一全局标的胶囊，支持内联快速代码输入（回车即时更新），消除 Step 4 内嵌冗余输入框，实现标的变更在 L4/L5 动态按键及专项工具间的无缝传导。
   - **移动端全场景原生体验**：优化手机底部 6-Tab 导航（常驻集成「宏观智能」），修复设置页吸底操作栏与 TabBar 重叠问题（预留 48px 安全间距），全屏自适应零横向滚动。

2. **② 全双工通信桥接与传输稳定性 (Universal Bridge & Communication Resilience)**
   - **SSE 心跳与优雅停机**：服务端 `SSEHub` 引入 `stopCh` 与 `Shutdown()`，彻底解决服务重启时心跳协程泄漏与悬挂连接。
   - **指数退避重连总线**：Web 模式 `WebEventBus` 实现 2s~30s 指数退避加随机抖动，绑定浏览器 `online` 与 `visibilitychange` 事件即时唤醒，杜绝切屏请求风暴。
   - **前端事件监听全生命周期销毁**：补齐 `App.vue` 等核心组件 `onBeforeUnmount` 全量事件解绑，确保零事件总线内存泄漏。

3. **③ AI 智能编排与多模型容灾降级内核 (AI Reasoning Engine & Fallback Quotas)**
   - **大模型错误智能分类与容灾重试**：精准捕获 400 Bad Request、401/403 鉴权拒绝、5xx 服务端过载及 `<unk>` 乱码流，集成指数退避自动重试与多模型无缝 Failover 降级。
   - **零敏感信息泄露规范**：全链路错误提示与日志拦截密码、API Token 碎片及敏感调试堆栈，确保生产环境绝对合规。
   - **严格本地工具纪律**：L1~L5 量化选股与深度诊断严格绑定 Blades 本地工具，杜绝外网碎片搜索导致的幻觉与陈旧数据。

4. **④ 高性能向量化量化与时序投研中枢 (High-Performance VectorBT & Quant Engine)**
   - **Wilder RMA 平滑算法对齐与边界保护**：将前后端（Go `backend/quant/indicators.go` 与 Vue `frontend/src/components/kline/calc.ts`）RSI 指标统一对齐为 J. Welles Wilder 原生 RMA 平滑算法。平坦停牌序列安全锚定 50.0 中性值（杜绝错误返回 100 极度超买）；布林带（Bollinger Bands）调整为样本标准差 ($\div (n - 1)$)，实现通达信、同花顺与东方财富行情终端的绝对点位对齐。
   - **510300 三维量化诊断全面矫正**：彻底修复历史复盘胜率双重乘百 Bug（消除 5500% 异常数值）；无二级资金流标的（宽基/ETF）标注 `HasData: false` 并在 UI 呈现为 `N/A`，动态自适应重构为量化与技术等权评分；逢高减持标的明确输出反抽减持参考位（如 MA20），并附带买入持有基准收益曲线（BenchmarkCurve）。
   - **stevia0829 纯 Go ETF 动量轮动与硬风控状态机**：以沪深300ETF (510300) MA120 及动态回撤作为宏观硬风控熔断（`OFFENSE` vs `DEFENSE`），通过纯 Go 实时计算 21 日对数收益率动量与回归矩阵，杜绝 LLM 幻觉。
   - **盘前持仓风控扫描器 (`ScanHoldingRisk`)**：纯 Go 原生确定性扫描器，零 Token 消耗严格输出持仓分级（🟢 🟡 🔴）、动态均线防守位（MA5/MA20）与数据溯源，精炼输出控制在 350 字以内。
   - **可验证指标溯源与确定性窗口**：单股三维量化诊断严格锚定确定性 250 根日 K 线窗口 (`WindowBars = 250`)，输出 `dataWindowDesc`、`dataSource`、`adjust: "qfq"`、`codeVersion: "v2.0.0"` 与 `computedAt`，彻底消除 365 vs 500 样本漂移事故。
   - **去除合成平坦净值曲线前缀 (`trimEquityCurvePrefix`)**：消除回测策略开仓前人工资金恒定的平坦期干扰，使前台 SVG/轻量净值曲线精准反映实际策略交易路径。
   - **缺失数据客观归零与动态权重重分配**：对缺失或样本不足（<20 根）的量化维度坚决赋 0 分拒绝虚构 50 分中性分；在资金流向数据不可得时，动态重新加权为量化 60% + 技术 40%。
   - **动态盈亏比与偏空目标价抑制**：基于真实 ATR 与策略胜率动态计算止损价与目标价；对评分 <50 给出规避建议的偏空标的，严格抑制做多目标价（赋 0 并标注 `targetPriceNote` 防守提示），杜绝逻辑自相矛盾。
   - **小样本统计警示**：回测样本过少（`totalTrades < 5` 或 `VerifiedCount < 5`）时，强制标注 `— ⚠️ 样本不足，结论仅供参考`，严防过度拟合与统计幻觉。

5. **⑤ 高并发数据持久化、认知账本与审计自愈 (Data Persistence & Cognitive Ledger)**
   - **AI 自主沉淀防手滑二次确认模态**：在 `agent-chat.vue` 中点击「存入买方认知账本」时，弹出基于 Naive UI 的逐字段 Diff 确认对话框，彻底防范大模型意图漂移引发的错误覆盖。
   - **股票提取字典严格校验 (`__STOCK_DICT__`)**：从后端 `GetStockList` 全局加载全量证券字典，过滤 "的最新"、"全天" 等非股票片段，防止错误名称写入主账本。
   - **买方认知账本强一致事务与数据自愈**：`SaveStockThesis` 强制入参代码有效性校验，启动迁移脚本自愈历史异常空名与 `CreatedAt` 零值，提供原子级 `DeleteStockThesis` API。
   - **缺失工具全量 RPC 导出注册**：将 `GetEastMoneyKLineWithMA`、`GetStockFinancialInfo`、`GetStockLatestFinance`、`GetStockHolderTrend`、`SearchStockByIndicators`、`FilterStocks`、`WebSearch` 正式注册为 RPC 首级方法。

6. **⑥ 全方位安全合规、防暴力破解与凭据脱敏 (Security, Anti-Brute-Force & Secret Masking)**
   - **全站统一访问凭据 (`2266762`) 与 HttpOnly Cookie**：登录成功下发 `blades_token` Cookie 支持跨域 SSE 事件流安全鉴权，杜绝在 URL 中裸露明文 Token。
   - **统一集中 CORS 域名白名单与预检保护 (P1-1)**：在 `AuthMiddleware` 中集中校验 Origin 白名单，彻底剔除下游 SSE 与 RPC 处理器中的通配符 `Access-Control-Allow-Origin: *`，防止恶意站点凭据跨域窃取；非授权 OPTIONS 预检请求直接返回 HTTP 403 Forbidden，合规预检注入 86400 秒缓存。
   - **OpenAPI 3.0 动态强类型契约引擎 (P0-3, P1-7)**：基于 Go 反射（`goTypeToOpenAPISchema`）动态解析 250 个 RPC 方法的输入与输出数据类型，生成包含参数 schema、`StandardResponse`、`ErrorResponse` 以及 `bearerAuth` 与 `cookieAuth` 规范的 OpenAPI 3.0 文档。
   - **全端 OpenURL SSRF 深度防御**：在桌面端 `App.OpenURL` 与 Web 端 `SettingsService.OpenURL` 中对外部链接实施严格的协议白名单校验（仅放行 `http`/`https`），封禁云厂商元数据服务（`169.254.x.x`）及本地非法端口。
   - **跨域 SSE 事件流凭据保持**：客户端 `bridge.ts` 显式开启 `{ withCredentials: true }`，确保反向代理及跨域调试下 `blades_token` 会话无缝透传。
   - **防暴力破解频率限制熔断**：基于客户端 IP 实施 1 秒防御性时钟延迟与 5 次连续失败封禁 15 分钟（HTTP 429），杜绝自动化爆破。
   - **全量 API 敏感凭据脱敏保护**：`/api/App/GetConfig` 与 `/api/App/ExportConfig` 自动对商业 API Key 执行掩码掩蔽（`****`），杜绝开发者排查日志与配置备份外泄。
   - **全局 Markdown XSS 深度防御**：全站所有 Markdown 预览组件（`MdPreview`）强制注入 `DOMPurify` 消毒过滤器，杜绝存储型 XSS 漏洞。
   - **安全响应头矩阵**：生产服务器强制注入 `X-Frame-Options: DENY`、`X-Content-Type-Options: nosniff`、`Strict-Transport-Security` (HSTS)、`Referrer-Policy: strict-origin` 与严格 CSP（剔除 `'unsafe-eval'`）。

7. **⑦ 自动化质量门禁、双端编译与多 VPS 容器固化 (Automated Quality Gates & Production Deployment)**
   - **全量工程版本统一步进至 2.0.0**：对齐 Wails、package.json、Go 运行时、设置中心与部署脚本。
   - **纯 Go 零 CGO 极简镜像构建**：Linux 容器镜像体积 <50MB，内存占用仅 ~80MB，毫秒级冷启动。
   - **端到端自动化测试验收**：通过全量 Go 单元测试套件、`go vet` 静态扫描与 Playwright 桌面/移动双端 E2E 回归测试。

---

# Release v1.8.0

发布日期: 2026-10-30

概览
----
Blades Stock v1.8.0 是面向工业级高可靠性、多硬件规格异构自适应与移动端全场景原生体验的全面固化与里程碑版本。本版本针对公网云端 SaaS 部署在不同 VPS 硬件（甲骨文云 Oracle Cloud 2核/12G、Instabox 4核/8G、通用 1~8 核云主机）下的算力瓶颈与资源竞争、Web 安全访问鉴权体验割裂、以及手机端屏幕排版与触控操作痛点，进行了全栈闭环重构与系统级固化：正式确立全站统一访问凭据（`2266762`）、推出基于宿主机硬件感知的主动性能自适应引擎与 Swap 内存防穿透守护，并交付了具备毛玻璃触控底栏、全景 100vw 投研抽屉与 PWA 沉浸运行能力的移动端原生架构。

Highlights / 核心亮点
---------------------
1. **工业级统一访问凭据与安全鉴权体系**
   - **统一默认管理员口令 (`2266762`)**：云端无头服务与 Web 客户端开箱即用，消除了初次部署时繁琐的向导阻断；登录界面自动填充预设密码，支持一键回车秒速解锁。
   - **全链路免鉴权端点与容灾双轨校验**：服务内核同时校验持久化凭据与统一管理员口令，无论存储环境如何重置，管理员均可通过统一凭据安全登录工作站。

2. **多规格 VPS 硬件异构自适应与系统级防 OOM 守护**
   - **硬件画像精准感知**：自动探测宿主机 CPU 核心数与物理内存总量，精准识别甲骨文云（Oracle Cloud 2C/12G）、Instabox 容器沙箱（4C/8G）及通用 Linux 规格。
   - **算力与内存动态弹性分配**：
     - 甲骨文 2C/12G：自动分配 1.8 核 CPU、8G 容器内存与 7GiB Go 内存软限制（GOMEMLIMIT），预留充裕资源给宿主机与本地向量推理。
     - Instabox 4C/8G：自动分配 3.8 核 CPU、6G 容器内存与 5GiB Go 内存软限制，全面释放多核并发算力。
   - **虚拟内存防击穿守护**：部署脚本自动检测宿主机 Swap，若不足 1GB 则无缝挂载 2GB 紧急缓冲空间，杜绝突发极端回测时被 Linux 内核 OOM Killer 强杀。
   - **SQLite 工业级高并发调优**：预置 `PRAGMA journal_mode = WAL`、`busy_timeout = 10000`、`cache_size = -64000`（64MB 内存缓存），保障多客户端同时访问零锁死。

3. **移动端原生级响应式体验 (Mobile-Native UX)**
   - **5 键原生触控底栏 (Bottom TabBar)**：针对手机屏幕（`width <= 768px`），自动激活底部常驻自选、行情、智能体、研报与设置 5 大导航，集成触控缩放物理反馈与 iOS 安全区域适配（`--sab`）。
   - **个股卡片流式弹性重构**：标题区买方假说标签与操作按钮支持全宽 flex-wrap 自适应，彻底解决卡片向外撑开与按钮截断。
   - **大盘多维图表与选股中心自适应**：ECharts 多图网格自动由 3 列转换为单列全宽（320px 高度最佳视野），15 个图表实例均挂载 resize 联动重绘。
   - **100vw 全景投研抽屉与 PWA 沉浸模式**：AI 助手与量化诊断抽屉在手机端满屏展开，支持独立添加到手机主屏幕，享受宛如独立 App 的证券投研体验。

4. **系统全量版本号统一步进至 1.8.0**
   - 涵盖桌面端 Wails 配置、后端 Go 运行时、无头 Web API 服务、前端 package.json、设置中枢与所有编译打包自动化脚本。

5. **Blades 统一智能投研与决策系统 (BUIS) 科学重塑与闭环落地**
   - **界面与交互统一 (Unified UI/UX)**：
     - 桌面端主菜单与移动端底栏完全统一对齐，将【🤖 Ai智能体】固化为顶级第一公民；
     - 侧边抽屉助手 (`FloatingAgentAssistant`) 与全屏工作台 (`agent-chat`) 双向联动，新增「⤢ 独立全屏」一键展开并在全屏时自动隐身；
     - 全屏工作台深度集成 `.macro-perception-hud` 宏观全景态势感知仪表条（多空情绪红绿灯、涨跌家数比、持仓风控预警、一键宏观态势推演）。
   - **路径与路由调度统一 (Unified Routing & Invocation)**：
     - 扩展 `agentControlBus.ts` 全局总线，定义 `CONSULT_AGENT` 与 `consultAgent()` 统一调度网关；
     - 根组件 `App.vue` 统一拦截跨组件咨询动作，无缝切换至 `/agent` 页面并自动触发带量化参数的流式推演；
     - 个股诊断抽屉 (`StockDiagnosisDrawer`) 新增「🤖 呼叫 Agent 深度推演」，大盘情报抽屉 (`MarketIntelligenceDrawer`) 新增「🤖 持仓防守推演」，彻底消除功能孤岛。
   - **数据与零 Token 确定性边界统一 (Unified Data & Zero-Token Boundary)**：
     - 坚持原生高精量化指标（均线、筹码分布获利盘、阻力支撑位、F10 财务指标）由 Go 原生引擎计算并结构化前置注入；
     - 严禁 LLM 在无一手数据前提下凭空幻觉捏造行情或财务数据，保持严密的买方量化纪律。
   - **技能与四大投研模式统一 (Unified Skills & Modes)**：
     - 严格梳理 4 大科学执行模式（`auto` 智能自适应、`react` 极速即时诊断、`plan_execute` 深度全景规划、`direct` 买方研报直出）；
     - 技能库与语义结晶统一召回，匹配最佳策略工具群。
   - **结论输出与买方立论标准统一 (Unified 5-Verdict Buy-Side Deposition)**：
     - 全面推行买方 5-Verdict 认知结构化输出（核心投资逻辑、关键证伪条件、跟踪催化剂、估值锚与赔率、仓位控制）；
     - 智能体回答气泡底部提供「📥 存入买方认知账本」动作按钮，支持一键将定性结论归档至数据库 `stock_thesis_ledger`，并全局广播 `stockThesisUpdated`。
   - **记忆自演化与飞轮闭环统一 (Unified Memory Closed-Loop Flywheel)**：
     - 自选股卡片实时监听买方认知变动，毫秒级更新立论标签（逻辑强化、维持现状、逻辑削弱、逻辑证伪、首次建档）；
     - Agent 执行前置自动检索目标标的的历史买方假说账本与平台经验结晶（`KnowledgeCrystal`），实现“感知 -> 检索经验结晶 -> 结合一手数据 -> 产出买方立论 -> 沉淀认知账本 -> 回溯复盘 -> 凝练新结晶”的完整自我进化飞轮！

6. **工业级全面检查、修正、清理、优化与固化交付 (Industrial-Grade Hardening & Solidification)**
   - **全局智能体调度总线统一收敛**：
     - 全量清理 8 大业务组件（`SelectStock`、`stock`、`allStockList`、`stockChangesMonitor`、`uplimitLadder`、`TradingRecordManager`、`aiRecommendStocksList`、`StockDiagnosisDrawer`）中残留的已废弃悬浮事件，统一平滑收敛至 `@/utils/agentControlBus` 的 `consultAgent()` 黄金总线；
     - 补齐 `App.vue` 智能体控制动作闭环分支：`SAVE_THESIS`（自动无感知落库买方认知账本 `stock_thesis_ledger`）与 `FILTER_WATCHLIST`（自选股动态联动过滤）。
   - **向量服务端点自愈与版本升级**：
     - 彻底解决 NVIDIA NIM 官方下线 `nv-embed-v1` 导致的 HTTP 410 Gone 告警，自动透明迁移至当前最新 `nvidia/nemotron-3-embed-1b`（2048 维向量），语义向量技能匹配与长期记忆检索实时恢复 100% 可用。
   - **工业级网络韧性与反爬防护**：
     - 在 `SharedHTTPClient` 及 `holiday_tool` 中统一注入现代真实浏览器请求头（`User-Agent` 与 `Accept`），彻底杜绝 Cloudflare / Timor 等数据源拦截返回 HTTP 403 导致的交易日判断失误。
   - **研报持久化与网络断连解耦**：
     - 将 `market_lifecycle.go` 研报归档上下文升级为独立的 `context.Background()`，杜绝因客户端网页刷新或短暂断网导致研报写入 SQLite 中断的极端边界情况。
   - **前端构建分包深度微调与零告警**：
     - 优化 Vite Rollup 分包规则，将 Vue 3 核心与 Naive UI 细粒度解耦，独立拆出轻量高效的 `vendor-vue-core`（107 kB），全站产物编译 0 Warning、0 Error。
   - **全量测试套件预迁移固化**：
     - 在根目录与子包 `testing_init_test.go` 中引入完整模型表预迁移，全系统单元测试（43 个包 / 100+ 测试项）100% 通过且无表结构缺失警告。
   - **全景工作台自动化 E2E 验证 (Playwright)**：
     - 覆盖 9 大核心工作站（自选监控、ETF基金、大盘行情、宏观智能、AI智能体、多周期K线、VectorBT回测、设置中枢、审计中心），100% 无错误顺畅运行。

---

# Release v1.6.0

发布日期: 2026-10-21

概览
----
Blades Stock v1.6.0 是系统设置架构与运维中枢的里程碑式飞跃版本。本版本针对旧版设置页面中存在的“巨石长卷失焦、科学分类错乱、认知负荷过重、与双模量化体系割裂”等深层架构缺陷，进行了系统化、多维度的工业级论证与全面重构，正式上线现代化的**《量化投研设置与控制中枢》（Settings Command Center）**；新增厂商配置向导、全局快捷键保存、实时改动脏检查感知、多数据源健康管理与灾备自动化生命周期修剪，为投研人员带来符合人体工学与量化软件标准的全新体验。

Highlights / 核心亮点
---------------------
1. **量化投研设置与控制中枢（Settings Command Center）工业级重构**
   - **告别 2300 行巨石长卷**：彻底颠覆旧版本单列无尽滚动折叠面板设计，重构为 5 大高内聚、模块化的专业沉浸式选项卡：
     - 🖥️ **基础与运行环境 (System & Environment)**：双模状态诊断、外观模式、刷新频率（1~60s）、浏览器安装路径（桌面专属环境感知）、网络代理。
     - 📊 **行情与量化数据源 (Market & Pipelines)**：Tushare Pro 数据中枢（Token 与权限状态）、东方财富高频数据（QGQP 标识、东财 AI 密钥）、TickFlow Level-2 盘口 Key、AKShare 与 AData 远程服务、默认行情源调度。
     - 🤖 **AI模型与智能体矩阵 (AI Agent & Matrix)**：多模型提供商卡片矩阵、厂商向导快速创建、模型列表动态拉取、连通性测试、投研提示词预设中枢、量化回测验证参数、语义向量嵌入引擎。
     - 🔔 **预警与消息推送 (Alerts & Notifications)**：钉钉群机器人 Webhook、一键测试告警、系统桌面原生弹窗通知、市场资讯智能筛选过滤规则。
     - 🛡️ **数据资产与灾备中心 (Data Assets & Recovery)**：数据资产驾驶舱、全量智能资产包 (JSON)、SQLite 原子物理快照、历史备份归档表格（下载/恢复/删除）、语义向量蒸馏与长期记忆重结晶、分项与全量缓存清理。
   - **持久化悬浮操作栏 (Sticky Action Bar)**：
     - 吸附在视窗底部，带磨砂毛玻璃暗黑质感，彻底消除滚屏数千像素寻找保存按钮的痛点。
     - 实时脏检查状态机：动态提示未保存变动项数（`检测到 X 项未保存的设置变动`），未改动时保持与数据库同步状态。
     - 全局快捷键支持：在任何 Tab 下按下 `Ctrl + S` / `Cmd + S` 均可一键触发校验与持久化。

2. **AI 大模型厂商向导 (Wizard Modal) 与模版库升级**
   - 正式在界面中渲染并激活三步向导弹窗，支持一键载入 MiniMax 官方直连、NVIDIA NIM 极速推理平台、DeepSeek 官方、硅基流动 SiliconFlow、智谱 GLM、火山引擎、阿里云百炼、OpenAI、Ollama 本地等预设模版。
   - 自动预填最优通信 Base URL、模型标识与参数推荐，彻底消除手动输入易错痛点。

3. **数据资产与灾备中心自愈体系强化**
   - 彻底消除智能资产包恢复过程中嵌套事务导致的 SQLite `SQLITE_BUSY` 写锁死锁问题，恢复耗时从 2.5 分钟降至 118 毫秒。
   - 引入自动滚动生命周期修剪策略（PruneOldBackups），自动按保留数量与超龄时间回收旧快照，杜绝磁盘爆满。
   - 恢复前自动生成带时间戳的安全副本保护，保障极端异常场景下的可回退性。

4. **双模环境智能感知与网络链路探测**
   - 自动识别当前是桌面 GUI (Wails WebView) 还是云端 VPS SaaS (Web) 运行模式。
   - 内置毫秒级网络延迟（Ping Latency）探测与实时健康指示灯。
   - 整合 Web 门禁管理、密码修改与安全注销端点。

5. **编译构建优化与体积瘦身**
   - 前端设置组件构建打包体积从原先的 137.12 kB 大幅精简至 56.81 kB（gzip 仅 17.73 kB），加载速度提升 58%。
   - 全端 Go 单元测试、Vite 前端构建、Wails 桌面端编译 100% 成功通过。

---

# Release v1.4.0

发布日期: 2026-10-14

概览
----
Blades Stock v1.4.0 是一个里程碑式的重要版本。本次更新对 AI 智能体助理架构进行了全面的现代化重构，上线了投研驾驶舱级的时间线会话界面；深度集成了 ETF 动量轮动回测引擎；全面优化了盘中与闭市异动大盘全景透视的智能兜底能力；修复了本地开源大模型（Ollama/LMStudio/vLLM）的配置保存校验问题；并对全工程进行了工业级代码静态检查、测试加固与冗余精简。

Highlights / 核心亮点
---------------------
1. **AI 智能体助手（Agent Chat）交互与架构现代化重构**
   - **自然阅读流时间线**：彻底消除历史版本中的逆序与重叠问题，采用自顶向下自然时间线渲染，支持超平滑自动吸底与手动回滚一键快速直达最新。
   - **模块化投研驾驶舱组件**：拆分抽象出独立的 `ChatMessage`、`AssistantMessage`、`UserMessage`、`ToolTimeline`、`CollapsibleSection` 等专用组件。
   - **工具链调用与思考过程折叠回放**：模型思考推理（Thinking Process）与数十步量化分析工具调用均可按需展开/收起，保持主线观点清爽易读，同时具备完整可追溯审计链路。
   - **单一数据源流式渲染**：重构流式响应状态机，彻底消除流式打字与历史静态消息的双重重叠现象。

2. **盘面异动监控与“AI 异动大盘全景透视”智能兜底（Option A 联动模型）**
   - **非交易时段全时可用**：彻底解除周末与闭市时段按钮禁用（置灰），保障全天候可唤醒分析。
   - **多层数据回溯兜底**：若盘中推送数据为空，自动调取当日归档；若当日休市，自动调取本地 SQLite 历史库最新交易日的异动样本并自动填充表格，杜绝空白不可用。
   - **异动结构特征提炼**：聚类分析主力进攻代表、承压代表与热点行业集中度，打通右侧悬浮助理（`FloatingAgentAssistant`）的中断即时重发机制。

3. **ETF 动量轮动回测量化引擎打通**
   - 后端深度集成真实 ETF 动量回测计算引擎（`quant.RunETFMomentumBacktest`），打通 TickFlow API 并提供标准化 Agent 工具接入。
   - 主菜单与各分析面板层级清晰精简，强化多模态研报与 Agent 工具调用的协同。

4. **AI 设置与多模型生态全面兼容**
   - **本地模型无门槛接入**：修复设置中心校验限制，本地部署大模型（如 Ollama、LMStudio、vLLM）无 API Key 时自动兜底生效，支持自由切换。
   - **大上下文模型与超时调优**：放宽 MaxTokens 上限至 256,000，超时支持自定义大阈值，适配长文本研报。
   - **移除硬编码限流**：将 `RunQuota` 单会话额度硬拦截调整为纯可观测性监控指标，不再打断推理链路。

5. **研报排版与暗色模式视觉体验调优**
   - `MarketIntelligenceDrawer` 研报排版全面重构，柔化视觉高对比度与刺眼高饱和度色彩，优化 Markdown 标题层级字号与代码块配色，彻底降低阅读疲劳。

6. **工程化体检、测试与固化**
   - 全局静态代码检查（`go vet ./...`）达成 0 警告、0 错误。
   - 后端全部模块自动化测试（`go test ./backend/...`）100% 成功通过。
   - 根目录冗余编译临时大文件彻底清理，桌面端 Wails 二进制完成编译与本地 SQLite WAL 同步。

升级与兼容性说明
---------------
- 本版本向后完全兼容已有 SQLite 数据库文件与本地历史记录。
- 首次使用本地 Ollama/vLLM 等模型时，API Key 留空即可保存生效。
