# Release v3.2.0

发布日期: 2026-11-16

概览
----
v3.2.0 修复「备用模型链配了却从不启用」的容灾缺陷：推理模型把输出预算
整段耗在思考(reasoning)上时，流会正常结束且不带任何 error，只是正文为空。
旧降级逻辑只在 err != nil 时切换，因此这类空答永远走不到降级那一支，
用户白等数分钟后只拿到 0 字结果，而运行记录里明明写着「备用链: xxx」。

Highlights
----------
1. **空答自动降级（核心）**：`backend/agent/fallback_provider.go` 的
   NewStreaming 在流正常结束后新增空答判定 —— 无正文（剔除 think /
   agent-action 后仍为空）且无工具调用，即切换备用模型重跑本次调用，
   并透传降级通知。`toolPartSeen` 排除合法的工具调用轮次，
   `contentBuf` 设 256KB 上限避免长报告重复驻留内存；无备用模型时不
   panic，交由上层空答检测处理。
2. **新增回归测试**：`TestEmptyOutputFallback` 覆盖三个场景
   （空答应降级 / 有工具调用不得降级 / 无备用模型不 panic）。
   该测试首轮就抓出了测试自身写错的一处（fallbackNames 多传主模型名）。

兼容性
------
- 无破坏性变更；仅在内置的多模型容灾链已配置（≥2 张 AI 卡片）时，
  空答场景会多一次备用模型调用（这正是期望行为）。

# Release v3.1.0

发布日期: 2026-11-12

概览
----
v3.0.0 是一次**精简 · 清理 · 固化**版本。前置的 v2.x 系列已修复大量“知道错了而改”的缺陷，
但缺乏“确认自己改对了”的机制；v3.0.0 的主线工作是把修复成果沉淀为**可被机器验证的约束**，
并清掉为验证而临时引入的冗余与死数据。

Highlights / 核心亮点
--------------------

### 一、代码仓库精减 75.5%（追踪字节 57,304,765 → 14,075,868）

| 项目 | 体积 | 判定依据 | 处置 |
|---|---|---|---|
| `backend/data/data/dict/{jp,zh,en}/**` | **38.5 MB** | 仅 `data/dict/base.txt` 经 `//go:embed` 使用；`seg.LoadDict("zh_s")` 早已注释；jp/zh 词典全仓 0 引用 | 删除 |
| `vendor-blades/`（74 文件） | 979 KB | 疑似误执行 `go mod vendor` 的嵌套产物，无任何代码/脚本引用 | 删除（并加入 .gitignore） |
| `backend/data/words.txt` | 664 KB | 由 `backend/data/utils_test.go` 测试写入的产物被误提交。**注：第一轮仅删除了磁盘文件但未 `git rm --cached`，该文件当时仍在索引中；第二轮才真正移除**（详见第四轮复查） | 删除 |
| `backend/data/stock_basic.json` | 1.18 MB | 0 引用（真正在用的是 `build/stock_basic.json`） | 删除 |
| 未使用的 `build/screenshot/*.png` | 1.4 MB | 仅 `alipay.jpg` / `wxpay.jpg` 被 `main.go` embed | 删除（**18** 个） |
| `build/appicon.png.bk`、`build/hk.json` | 432 KB | 前者为备份文件，后者 0 引用 | 删除 |
| `frontend/src/assets/images/logo-universal.png`、`naive-logo.svg` | 152 KB | 全仓（含 CSS）0 引用 | 删除 |
| `utils.go`（根目录） | 567 B | 曾判定为死代码，但复查发现其中的 `IsMacOS()` 被 `main.go:174` 引用，删除会直接导致编译失败 | **保留**（已回滚删除） |
| 根目录陈旧二进制 `*.exe` | 356 MB | 本地构建产物，已可重新生成 | 删除 |

附加 .gitignore 规则：`vendor-blades/`、`backend/data/words.txt`、`.venv/`、`.graph/`、`.gotmp/`、`.tmp/`、`/*.exe`、`/*.bin`。

> **保留但已确认的“合理大文件”**：`build/stock_base_info_us.json`（4.6 MB）与 `stock_base_info_hk.json`
> （1.2 MB）确实被 `backend/services/migrate.go` 用作**首次启动的本地种子数据路徕**，属于功能而非冗余；
> 删除会改变离线首启行为，故保留。
> `build/stock_basic.json`（1.23 MB）同为迁移种子数据，保留。

### 二、`cmd/` 命令行工具精减（35 → 22 项）

删除 13 个一次性开发期工具（共约 1300 行）：
`check_tools`、`test_kline`、`test_trading_day`、`test_backup`、`test_desktop_save`、
`probe_kline_etf`、`probe_tushare_whitelist`、`smoke_strategy_screen`、`diag_tushare_tools`、
`insert_liuzhangwang_{beneficiary,deep,scarce}`、`run_evaluator`。

保留原则：被 `docs/` / `scripts/` / `AGENTS.md` / `main.go` 的 `go:embed` 引用的、
或具备可复用运维价值的一律保留（`server`、`blades_mcp_server`、`backtest`、`etf_backtest`、
`index_embeddings`、`setup_mcp`、`insert_etf_skill`、`insert_serenity_skill` 等）。

### 三、依赖精减

- **前端移除 8 个未引用依赖**：`@lezer/highlight`、`@vavt/cm-extension`、`@vicons/carbon`、
  `@vicons/ionicons4`、`unplugin-auto-import`（TDesign 拆除后已无 resolver）、`vfonts`、
  `@typescript-eslint/eslint-plugin`、`@typescript-eslint/parser`（由 `typescript-eslint` 元包统一提供）。
  经实测，移除后 `npm run lint`（0 errors）与 `npm run build`（三道门禁）均正常。
- **Go 模块**：`go mod tidy` 无任何变更，模块图已处最精简状态。

### 四、文档固化

- **新建 `docs/README.md` 目录索引**，明确区分「现行文档」与「归档文档」，并约定
  「新增设计文档用不带日期的语义化文件名」。
- **8 份带日期的一次性开发报告归档至 `docs/archive/`**
  （AKShare / Baostock 集成、若干回测改进与测试报告、VectorBT 引擎说明等）。
- **修复内嵌使用手册的编码缺陷**：`docs/blades-stock使用手册.md` 原为 **20 字节的 GBK 乱码桩**
  （经 `//go:embed` 打包后由 `GetUserManual()` 返回乱码）。已重写为完整的 UTF-8 使用手册。
- **修复过时构建文档**：`BUILD_GUIDE.md` 曾引用已被移除的 `.github/workflows/ci.yml`；
  `Makefile` 的 `lint` 目标曾调用未安装的 `golangci-lint`（导致 `make lint` 必然失败）。
  两者均已按真实可用工具链重写，并新增 `help` / `build-web` / `frontend-lint` 目标。
- 删除开源样板文件 `CODE_OF_CONDUCT.md`、`CONTRIBUTING.md`、`.github/ISSUE_TEMPLATE/*`、
  `.github/pull_request_template.md` 与残留的 `INDUSTRIAL_CHECK_SUMMARY.txt`。

### 五、第二轮复查：修复第一轮遗留的 4 类真实缺陷

第一轮清理完成后对仓库做了二次审计，发现并修复以下问题：

#### 5.1 `backend/data/utils.go` — 单行 747,684 字符的畸形源文件

敏感词表被写成 `strings.Split("戊边、戍边、…", "、")` 的**单行字符串字面量**，
使该 `.go` 文件达 751,023 字节。任何微小改动都会重写整行，严重破坏 git diff、
编辑器性能与代码检索。已提取为 `backend/data/sensitive_words.txt`（41,769 条 / 747,639 字节）
并改用 `//go:embed`：

- 源文件 **751,023 → 3,770 字节**（`utils.go` 从 1 行怪物变为 132 行可读代码）
- 经逐字符 round-trip 校验，重建的字符串与原字面量**完全一致**，运行时语义零变更

#### 5.2 `backend/data/words.txt` — .gitignore 规则形同虚设

该 664 KB 产物在第一轮被删除磁盘文件，但**仍是 git 已追踪文件**。
`.gitignore` 中的 `backend/data/words.txt` 规则对已追踪文件完全无效，因此并未真正移除。

- `git rm --cached` 真正移出索引（41,769 行），并验证 ignore 规则自此生效
- 根因修复：`utils_test.go` 原以 `os.WriteFile("words.txt", …)` 写入**包目录**，
  现改写进 `t.TempDir()`，测试结束自动清理，**永不再污染工作区**
- 顺带为词表加载补上断言（`len(SensitiveWords) == 0` 即失败）

#### 5.3 全仓 gofmt 合规性（181 个文件）

首次引入 `gofmt` 门禁时发现 181 个 Go 文件不合规（`gofmt -l` 非空）。
已机械化格式化（含 gofmt 对 import 块内的排序），并以**独立提交**保留可审查性：

- 严格验证：`go build ./...` / `go vet ./...` / 五大核心测试套件全部通过
- `Makefile` 的 `lint` 目标新增 gofmt 门禁（已双向验证：干净时通过、弄脏时拦截）

#### 5.4 前端 `@types/*` 依赖归位

- `@types/dompurify`：`dompurify` v3+ **自带类型**（`dist/purify.cjs.d.ts`），
  该 DefinitelyTyped 包已废弃 → **移除**
- `@types/file-saver`：`file-saver` 不自带类型，仍必需，但属**编译期**依赖，
  不应进入生产 `dependencies` → **移入 `devDependencies`**
- 回归验证：`tsc --noEmit` 错误数 **277 → 277**，错误集合逐条比对完全一致（零回归）

#### 5.5 测试的非确定性脆弱点

`backend/quant/holding_risk_scanner_test.go` 断言了 TCL科技 等标的的**绝对风险分级**，
而分级依赖当日实时 MA 排列 —— 实测 2026-11-05 该标的已由 YELLOW 转为 GREEN，
导致测试必然变红。已重构为只断言**与行情无关的结构性不变量**：

- 分级合法性（∈ GREEN/YELLOW/RED）与分级前缀一致性
- 防守位为正；均线排列与 MA 取值自洽（多头排列需满足 MA5≥MA10≥MA20）
- 硬风控锁定必须落在 RED 档
- **RED 档建议中严禁出现“进攻”话术**（逻辑自洽性）
- 分级计数之和等于明细条数；报告 CJK 长度 ≤ 350

行情相关的绝对分级改为 `t.Logf` 观测输出，仅供人工核对。

### 六、生产部署阻断修复：CSP 门禁在 Docker 布局下失效

**现象**（首次真机执行 `bash scripts/deploy-vps.sh` 时报出）：

```text
[frontend-builder 6/6] RUN npm run build
0.876 ❌ [CSP-Compat] 无法从 cmd/server 读取 Content-Security-Policy，门禁无法工作。
ERROR: failed to solve: process "/bin/sh -c npm run build" did not complete successfully: exit code: 1
```

**根因**：`frontend/scripts/check_csp_compat.js` 以服务端为 CSP 的**唯一权威来源**
（刻意避免两处定义漂移），需读取仓库根的 `cmd/server/main.go`。但 Dockerfile 的
`frontend-builder` 阶段只 `COPY frontend/ ./`，`cmd/` 根本不在构建上下文里 → 门禁拿不到 CSP。

这是一类典型的「门禁在开发环境有效、在交付环境失效」协同缺口：
本地 `npm run build` 正常（能找到 `cmd/server/main.go`），而 Docker 构建必失败。

**修复**（两侧同时加固）：

1. **Dockerfile**：`frontend-builder` 阶段新增 `COPY cmd/ /build/cmd/`。
   目标路径故意写为绝对的 `/build/cmd`，以匹配脚本从 `/build/frontend` 向上查找的逻辑；
   `npm run build` 只作用于 `frontend/`，`/build/cmd` 在 vite root 之外，不会被打包进产物。
2. **门禁脚本健壮化**：从脚本目录**逐级向上查找** `cmd/server/{main.go,static.go}`（最多 5 级），
   同时兼容本地 monorepo 与 Docker 两种布局；新增 `BLADES_CSP_SOURCE` 显式指定入口；
   失败时列出**所有已尝试路径**与修复指引，保持 **fail-closed**（绝不静默跳过）。

**四种布局场景均已实测**（无 Docker 时的最强验证手段）：

| 场景 | 预期 | 实测 |
|---|---|---|
| Docker 布局 + 已 COPY cmd/ | 通过 | ✅ exit 0 |
| Docker 布局 + 源码含 `http://` 违规 | 拦截 | ✅ exit 1（门禁仍有效） |
| Docker 布局 + 忘记 COPY cmd/ | fail-closed 且提示可操作 | ✅ exit 1 + 列出 10 条已尝试路径 |
| `BLADES_CSP_SOURCE` 显式指定 | 生效 | ✅ exit 0 |

另已核对：`smoke-after-deploy.sh` 引用的是服务端 CSP 之外的产物特征，与本次改动无耦合；
首屏预加载仍为 8 个资源且不含 codemirror / markdown / tdesign chunk，部署后 27 项冒烟自检预期全绿。

### 七、P0 修复：控制中枢 · AI 模型注册表无法加载也无法保存

**用户反馈**：「AI输入、配置、保存都成问题，还不如原来的配置」。经实测复现并定位为 `6a33fd1`
（设置页重构）引入的**接口契约不匹配 + 字段静默清零**双重缺陷。

#### 7.1 根因一：载荷形态错位，导致“加载不到、保存必败”

后端 `SettingConfig` 是**匿名嵌入 `*Settings`** 的结构体，JSON 形态是**扁平**的：

```json
{ "ID":1, "prompt":"…", "aiConfigs":[ … ], "embeddingEnabled":true }
```

而重构后的前端改成了嵌套形态 `{ settings: { … }, aiConfigs: [ … ] }`，后果：

| 环节 | 后果 |
|---|---|
| 加载 | `res.settings` 为 `undefined` → 基础设置表单**完全不回显**（提示词/开关/密钥全部停留在默认值） |
| 保存 | Go 侧 `s.Settings == nil` → 直接返回 **「保存失败: 配置数据为空」**，任何修改都存不进去 |

> 旁证：仓库内 **其他 6 个组件**（App.vue / agent-chat / market / allRecommend / allStockList / FundFollow）
> 全部按 **扁平** 形态读取（`res.darkTheme`、`result.openAiEnable`），只有 `settings.vue` 是异类。

#### 7.2 根因二：只提交 6 个字段，后端“写满每一列” ⇢ 静默清零

重构把模型配置的提交字段从“完整回传”改成只提交 6 个核心身份字段，但后端
`UpdateConfig` / `updateAiConfigsTx` 使用 GORM `Updates(map{…})` **显式写入每一列**，
未提交的列全部以零值落库：

| 被静默破坏的配置 | 实际后果 |
|---|---|
| `maxTokens`（前端曾硬编码 4096） | 推理模型 max_tokens 被降到 4096，` thinking` 阶段就耗尽预算 → **回答截断/空白** |
| `temperature`（硬编码 0.7） | 用户调过的采样温度被重置 |
| `httpProxy` / `httpProxyEnabled` | 代理配置被清空 |
| `sessionId` | Agent 会话绑定丢失 |
| `embedding*`（Settings 级） | **向量检索被静默关闭**，语义能力失效 |
| `rateLimitPerSecond` / `burst` / `daily*Quota` | 配额与限流设置清零 |
| `backtest*` / `mcpTimeout` / `toolRunBudget` / `toolDailyBudget` / `toolDailyCostUSD` | 回测与工具预算配置被抹掉 |

#### 7.3 修复

**前端（`settings.vue`）**
1. 新增 `buildSavePayload()`：改为**扁平**载荷，并用 `...formValue.value` 展开**全部**字段;
   每个模型配置也 `...c` **完整回传**（含界面不展示的调度字段）。
2. 新增 `applySettingsSnapshot()`：统一加载入口，回填**所有** Settings 字段，
   并兼容扁平（当前契约）与嵌套（旧导出）两种形态。
3. `CleanAIConfig` 扩展为完整字段集 + `normalizeAiConfig()` 规范化函数，
   **保留后端返回的所有原值**；仅对缺失项补默认值。
4. 删除硬编码的 `temperature/maxTokens/timeOut/llmConcurrencyLimit/subAgentTimeout`。
5. **新增加载完整性门禁**：`settingsLoaded` / `aiConfigsLoaded` 均为真才允许保存，
   避免加载失败时把空/残缺列表写回，从而触发后端 `DELETE FROM ai_config` 误删全部模型。
6. 修复 `aiConfigMeta` 下标错位（删除卡片后 loading/选项串卡）与
   未保存卡片的“设为默认”（新增待定意图 `pendingDefaultModelIndex`，保存后自动生效）。
7. 导入配置同步支持扁平形态。

**后端（`settings_api.go`）—— 纵深防御**
8. 在 `validateAIConfig` **之前**按数据库原值补齐未提交的调度/扩展字段
   （`validateAIConfig` 会用 4096/300/2/20 填零值，从而掩盖“未提交”事实）。
9. Settings 级 `Updates(map)` 对**无界面入口**的字段（embedding / backtest / mcp /
   tool 预算 / updateChannel / promptPlazaApiBase / window）采用“零值即保留原值”语义。
10. 新增配置落库前调用 `SetDefaults()`；空列表且库中本无配置时跳过 DELETE。

#### 7.4 验证（三层，全部通过）

**① Go 回归测试** `backend/data/settings_roundtrip_test.go`（5 个用例）

```
TestGetConfigPayloadIsFlatNotNested          PASS  固化扁平契约，防止回退成嵌套
TestNestedPayloadIsRejected                  PASS  复现旧缺陷：嵌套必返回“保存失败: 配置数据为空”
TestFlatRoundTripPreservesEverything         PASS  扁平完整回传：26 个字段零丢失 + 掩码还原
TestPartialAiConfigPayloadDoesNotWipeFields  PASS  即使只传 6 字段，后 13 个字段也被保留
TestNewAiConfigGetsDefaults                  PASS  新增配置自动补齐调度默认值
```

**② 真机 HTTP 端到端**（临时 DB + `cmd/server` 实跑，30+ 断言全绿）

含：扁平契约、富配置写入后 26 个字段逐个复读比对、嵌套载荷必败复现、
掩码回传不覆盖真实密钥、连续保存后 `maxTokens` 仍为 64000（**无静默降级**）。

**③ 真实浏览器 UI**（ego-browser，实测 `http://127.0.0.1:18099/#/settings`）

| 探针 | 修复前预期 | 实测结果 |
|---|---|---|
| 系统提示词回显 | 默认值（加载失效） | ✅ 回显已存值 `富配置提示词-E2E` |
| AI 卡片回显 | — | ✅ 名称/模型/地址/掩码密钥均正确 |
| 点击「保存设置」 | 「保存失败: 配置数据为空」 | ✅ toast **「系统设置已成功保存并立即生效！」** |
| 刷新后持久化 | 不生效 | ✅ 改名后刷新仍为 `UI改名验证-第二次` |

回归：`npm run lint` 0 errors；`npm run build` 三道门禁全过；
`go build ./...` / `go vet` / `gofmt -l` 均干净；
`backend/data`、`backend/quant`、`cmd/server` 测试全绿。

### 八、P0 修复：「拉取列表」成功后模型标识无处可选

**用户反馈**：「拉取列表 成功后，没有显示 模型标识 (Model Identifier)，如何体现模型？
如 minimaxai/minimax-m3.1-flash；怎么样保存？最基本的功能都没有」。

#### 根因：拉回来的列表从未渲染

`fetchAiModels()` 成功后将模型写入 `aiConfigMeta[idx].modelOptions`，
但**模板里用的是纯 `n-input`**，`modelOptions` 从未绑定到任何控件：

```html
<!-- 修复前：拉取结果存在 state 里，却无人渲染 -->
<n-input v-model:value="cfg.modelName" placeholder="deepseek-chat / MiniMax-M3" />
<!-- `modelOptions` 在整个模板中零引用 -->
```

后果：点击「拉取列表」只弹一个 toast，列表**用户看不到、也选不了**，
只能盲打模型标识——最基本的能力缺失。

#### 修复

| 项 | 修复前 | 修复后 |
|---|---|---|
| 模型标识控件 | 纯文本输入框 | **可搜索下拉框** `n-select`（`filterable` + `tag` + `clearable`） |
| 拉取结果 | 存进 state 但无处可选 | 写入下拉框 `options`，**点击即选** |
| 自定义标识 | 只能手打 | 既可从列表选，也可自由输入（如 `minimaxai/minimax-m3.1-flash`） |
| 反馈 | 仅 toast | 下方常驻提示：**✅ 已获取 N 个模型，点击下拉框直接选择** |
| 未指定模型 | 仍为空 | 拉取成功后**自动回填第一个模型** |
| 值不在列表中 | 无提示 | 明确提示「当前为自定义模型标识，将原样保存」 |
| 拉取失败 | 笼统报错 | 提示检查 Base URL / API Key，或直接手输 |

仍保留手动输入能力（`tag` 允许任意文本），不削能力、只补可见性。
「保存设置」位于顶部操作栏常驻可见，并绑定 `Ctrl+S` / `Cmd+S`。

#### 验证（真实链路，非模拟）

用本机 mock 一个 OpenAI 兼容服务（`GET /v1/models` 返回 7 个模型，含
`minimaxai/minimax-m3.1-flash`），启动真实 `cmd/server`，用 ego-browser 驱动真实 UI：

| 步骤 | 实测结果 |
|---|---|
| 填写 Base URL 指向 mock → 点「拉取列表」 | ✅ 提示「已获取 7 个模型」 |
| 展开下拉框 | ✅ 真实渲染 **7 个可选项**（含 `minimaxai/minimax-m3.1-flash`） |
| 选中该模型 | ✅ 下拉框正确显示所选值 |
| 点「保存设置」 | ✅ toast「系统设置已成功保存并立即生效！」 |
| **直接读数据库校验** | ✅ `modelName="minimaxai/minimax-m3.1-flash"` 已入库 |
| 刷新后回显 | ✅ 4 张卡片的模型标识均正确显示 |

回归：`npm run lint` 0 errors；`npm run build` 三道门禁全通过。

### 九、P0 修复：模型连通性测试与「用的不是我选的模型」

用户反馈：「模型联通测试，要么模型不存在，要么 token 不够，我观察测试列表第一个，
不是我选择的模型，怎么样保存？」——共 4 个独立缺陷，本次一次性修复。

#### 9.1 「token 不够」：连通性测试硬编码 max_tokens=16

`TestAiConnection` 曾固定发 `max_tokens: 16`。对**推理模型**
（MiniMax-M3 / deepseek-reasoner / GLM-Z 等），16 个 token 会在 ` thinking`
阶段就被耗尽 → 服务端报 token 相关错误 → 用户误以为是自己配置错了。

修复：改为「先保守、后兜底」两段式——
1. 第 1 次：`max_tokens: 512`（足够容纳 think 阶段）
2. 识别为预算类错误时，第 2 次**完全不传 `max_tokens`**，交给服务端默认值

实测（mock 推理模型要求预算 ≥ 1024）：

```
POST /v1/chat/completions model=qwen/qwen3-max max_tokens=512   ← 被拒
POST /v1/chat/completions model=qwen/qwen3-max max_tokens=(omitted) ← 自动重试成功
```

#### 9.2 「模型不存在」：错误提示不可操作

新增 `classifyAIError()`，把上游英文错误翻译成**可操作**的中文：

| 场景 | 修复前 | 修复后 |
|---|---|---|
| 模型不在该服务 | `invalid_request_error: The model ... does not exist` | 「当前接口地址下不存在模型「X」。请点「拉取列表」从服务端返回的模型中选择，或核对模型标识拼写」 |
| 预算不足 | 一串英文 | 「输出预算不足（推理模型会在思考阶段消耗 token，已自动重试放宽预算）」 |
| 401/403 | 原样透出 | 「API Key 无效或权限不足（HTTP 401）」 |
| 404 | 原样透出 | 「接口路径不存在：请确认 Base URL 指向 .../v1（不是 .../v1/chat/completions）」 |
| 429 | 原样透出 | 「请求过于频繁或账户配额不足（429）」 |

#### 9.3 「用的不是我选的模型」：输入没提交 + 生效语义不明（两个根因）

**根因 A（控件层面）**：上一版用 `n-select` + `filterable` + `tag`。该组合下
**手动输入的文字不按回车就不会写回 `v-model`** —— 用户看着框里是自己的模型，
实际提交的仍是旧值（往往就是拉取后自动填入的**第一个模型**）。

修复：改用 `n-auto-complete`，做到「**所见即所存**」：输入即时生效，
同时仍可从「拉取列表」的候选中单击选择。并新增常驻提示：

```
将测试 / 保存的模型标识：nvidia/llama-3.3-nemotron-super-49b-v1
```

实测（只输入、不按回车，直接点测试）：mock 收到的请求模型为
`nvidia/llama-3.3-nemotron-super-49b-v1`，**不再是第一个模型**。

**根因 B（运行时层面）**：`agent_api.go` 在 `aiConfigId <= 0`（未指定全局默认）时
会**静默回退到列表第一个配置**。而设置页没有任何地方说明这一点。

修复：设置页新增「生效规则」说明栏，明确告知：Agent 对话 / 研报 / 推送一律使用
标有 `🎯 全局默认` 的卡片；未指定时退回**第 1 张卡片**；并实时显示当前默认是哪一张。

#### 9.4 「怎么样保存」：保存后明确告知生效模型

保存成功后除原有提示外，新增一条明确结论，例如：

```
当前生效模型：【所见即所存验证】 → nvidia/llama-3.3-nemotron-super-49b-v1
```

（未设默认时额外标注「未指定全局默认，系统按第 1 张卡片生效」）

#### 9.5 验证

**Go 单测**（新增 `backend/services/settings_test_connection_test.go`）

```
TestIsTokenBudgetError   PASS  （6 正例 / 4 反例）
TestClassifyAIError      PASS  （6 个场景子用例全过）
```

**真实链路**（mock OpenAI 兼容服务 + 真实 cmd/server + ego-browser 驱动真实 UI）

| 验证点 | 实测结果 |
|---|---|
| 拉取列表 | ✅ 提示「已获取 7 个模型」 |
| 点击空输入框浏览 | ✅ 真实列出全部 7 个候选 |
| 输入 `kimi` 过滤 | ✅ 只剩 `moonshotai/kimi-k2` |
| 输入 `minimax` 过滤 | ✅ 只剩 2 个 |
| 只输入不回车直接测试 | ✅ mock 收到的是**输入的模型**，非第一个 |
| 推理模型 token 预算 | ✅ 512 被拒 → 自动重试不带 max_tokens → 成功 |
| 不存在的模型 | ✅ 返回可操作的中文提示 |
| 保存 | ✅ 「已成功保存」+「当前生效模型：【…】→ …」 |
| 刷新后回显 | ✅ 各卡片模型标识正确 |

回归：`npm run lint` 0 errors；`npm run build` 三道门禁全通过；
`go build` / `go vet` / `gofmt -l` 均干净。

> **附带发现**：Naive UI 的 `n-auto-complete` **没有 `filter` 这个 prop**
> （传入会被当作普通属性静默忽略）；它要求调用方自己把 `options` 过滤好。
> 已改为在模板中调用 `modelOptionsFor(idx, cfg.modelName)` 完成过滤。

### 十、版本号全面升级至 3.0.0

共 17 个文件 / 27 处（`main.go`、`cmd/server`、`wails.json`、`frontend/package.json`、
`frontend/index.html`、设置页与认证弹窗、`scripts/deploy-vps.sh`、`docker-compose.yml`、
`backend/{data,quant,services}`、`build.ps1`、`README.md`）。

替换采用**显式文件清单**而非全局替换，并断言以下不应被改动的字符串保持原样：
- `backend/data/web_search_api.go` 的 `Chrome/122.0.0.0` User-Agent
- `frontend/wailsjs/runtime/package.json` 中 Wails 运行时自身的版本号

验证
----
- `go build ./...` 与 `go vet ./...` 通过（删除词典后 `//go:embed` 仍正常）。
- `go test ./backend/data/` 通过：`TestAnalyzeSentiment`（依赖 embed 的 `base.txt`）与
  `TestReplaceSensitiveWords`（敏感词表加载正常，共 41769 条）均 PASS，
  证明词典清理与 `go:embed` 重构均未损伤链路。
- 核心 Go 测试套件全绿：`backend/quant`、`backend/data`、`cmd/server`、`backend/agent`、
  `backend/agent/tools`。
- `gofmt -l .` 输出为空（181 个文件已全部合规）。
- `npm run lint`：0 errors / 150 warnings（基线不变）。
- `npm run build`：prebuild 三道门禁（未导入标识符 / UI 纯净度 / CSP 一致性）+ postbuild 全通过。
- `tsc --noEmit`：错误集合与变更前逐条一致（277 条，均为存量），零回归。

### 十一、AI 设置全面测试、检查与优化（第四轮）

对「控制中枢 · AI 模型注册表」做了一轮**端到端审计**：前端表单 → bridge RPC → 后端
`SettingConfig` 事务 → SQLite 落库 → 回显，并配套 12 条真实 RPC 用例矩阵（直打
`/api/App/*`）、12 条 Go 回归测试与真实浏览器验证。

#### P0-1：`backtestEnabled=false` 永远无法落库（能力开关关不掉）

根因：`UpdateConfig` 用 GORM `Updates(map)` **显式写入每一列**，而旧代码用
「零值即保留」近似区分「未提交」与「显式 false」。但 `embedding_enabled` /
`backtest_enabled` 的**关闭值恰好就是 false**，于是用户点「关闭」→ 落库又被旧值
覆盖。`embeddingEnabled` 此前看似正常，只是因为库里的默认值恰好也是 false（假阳性）。

修复：`SettingConfig` 新增自定义 `UnmarshalJSON`，记录载荷中**实际出现**的顶层字段名
（存于未导出的 `submittedFields`，不参与序列化，也不出现在 Wails 绑定与 TypeScript 模型中）。
后端改为**按字段存在性**决定「保留旧值」还是「尊重提交值」：
- 未提交 → 保留数据库原值（兼容部分载荷与旧版本缓存分块）；
- 显式提交 `false` / `0` / `""` → 尊重客户端意图。

对 `SettingConfig` 新增 `UnmarshalJSON` 已核对三条链路均安全：
Wails 桌面端（`reflect.New(*SettingConfig)` → `**T` 形态）、Web RPC 派发器（同为 `**T`）、
以及 `Export`（只序列化，不受影响）。并新增契约测试
`TestCustomUnmarshalWorksThroughDoublePointer` 钉死 `**T` 形态下自定义反序列化确实生效。

#### P0-2：删除「全局默认卡片」后 `ai_config_id` 悬空

根因：客户端提交的 `aiConfigId` 可能指向一张**已被本次删除**的卡片，
而旧代码只在 `aiConfigId <= 0` 时才回落到第一张卡片，于是留下悬空指针——
Agent 每次请求都回落到「第一张卡片」并持续刷 `ai config %d not found`。

修复：新增 `resolveDefaultAiConfigID()`，在**同一事务内**于模型 upsert 之后校验：
提交的 ID 仍有效则原样保留；否则回落到库里 ID 最小的存活卡片；一张不剩则置 0。
同时 `GetSettingConfig` 的模型列表增加 `ORDER BY id ASC`，保证「第一张卡片」是确定的。

#### P0-3：Web 模式「服务器延迟」永远显示不可达

根因：`frontend/src/components/settings.vue` 调用 `fetch('/api/health')`，
而服务端**只注册了 `GET /health`**；`/api/*` 还需鉴权预检 → 必然 404。
修复：前端改用 `/health`；同时服务端新增 `/api/health` 同源别名（免鉴权），
以兼容发布韧性机制下仍在使用旧版 chunk 的在线会话（否则灰度期旧页面仍会误判不可达）。

#### P0-4：上游 502 的整页 HTML 错误页被原样弹到用户界面

根因：`frontend/src/api/bridge.ts` 的 `callWebApi` 在 `!response.ok` 时直接
`throw new Error('HTTP ' + status + ': ' + errText)`，而反向代理 / Cloudflare
在网关故障时返回的是**一整页 HTML**（实测含内网主机名），被原样渲染进通知弹窗。

修复：`formatHttpError()` 统一净化：
- 502 / 503 / 504 → 可行动的中文说明（保留 `HTTP <status>` 前缀，
  以便「市场研报」抽屉的 502 轮询兜底逻辑继续生效）；
- 其余 HTML 响应 → 「服务返回了非预期的 HTML 页面」+ 提示查服务端日志；
- HTML 全文仅进 `console.error`，不再进入用户可见文案。

#### P1：模型网关调用链的三处真实缺陷

| 缺陷 | 影响 | 修复 |
|---|---|---|
| `FetchAiModels` 用 `resty.New()` 全新客户端（10s 超时、**无代理**） | 需要代理的网络里「拉取列表」必然失败，而同一张卡片的对话却正常（对话走共享客户端）——典型「能聊不能列」 | 改用 `data.CreateHTTPClientWithTimeout()`，继承全局代理 / UA / 连接池 |
| `FetchAiModelInfo` 对全局单例 `data.SharedHTTPClient` 调用 `SetBaseURL` | 并发请求会串到别的供应商地址上（全局可变状态） | 改用独立 client（共享 transport），不再改动全局单例 |
| `TestAiConnection` 里的 `client.SetProxy(...)` | resty v2 的 `SetProxy` 直接写 `transport.Proxy`；一旦传入共享 transport 就会污染全局所有并发请求 | 移除该调用，代理由 `ConfigureFromSettings` 统一同步到共享 transport |

另：`ConfigureFromSettings` 改为使用**数据库中的最终生效值**而非提交值，
避免「只提交 prompt 的部分载荷」把全局代理误清空。

#### P1：清理与诚实化

- 删除死写 `iwencai_api_key`（该列在库中根本不存在，`Updates(map)` 会静默忽略它；
  已用探针证明其失败**不会污染事务**，删除无隐藏风险）。
- 移除生产代码中的 `TESTAI DEBUG` `fmt.Printf`（会把上游响应体直接打进 stdout）。
- 上游响应体写日志前统一截断到 500 字符。
- 设置页「深度思考」开关补充**如实**说明：该开关决定卡片作为推理模型使用时的推理参数，
  但 **【Agent 对话】页顶部的「深度思考」开关会在发起对话时覆盖本卡片设置**，
  市场资讯 / 研报流水线不读取本开关（此前 UI 未作任何说明，易产生误解）。
- 设置页「拉取列表」为空时的提示补充常见原因（Base URL 需指向 `.../v1`、
  API Key 无效、需开启全局代理并先保存）与下一步动作（点「⚡ 测试连通性」看具体原因）。
- 设置页延迟探测改用 `/health`（与 `WebAuthModal.vue`、部署脚本、冒烟脚本一致）。

#### 验证

- **真实 RPC 用例矩阵 12 组 / 37 项：37 项全绿**（直打 `http://127.0.0.1:18099/api/App/*`）。
  其中「掩码 API Key 不被覆盖」一项按「读到的掩码必然等于提交的掩码」判定会误报，
  已用**直读 SQLite** 复核：库中仍是明文 `sk-test-key-1`，未变成 `sk-t****ey-0`。
- **Go 回归测试 12 条全绿**（`backend/data/settings_roundtrip_test.go`）：
  扁平契约、嵌套拒绝、完整往返零丢失、部分载荷不清零、
  显式关闭两个能力开关、未提交时保留原值、删除默认卡片后指针修复、
  未指定默认时回落到第一张存活卡片、删除全部后清零、`**T` 形态反序列化、内部字段不泄漏。
- **真实浏览器（ego-browser）验证**：
  - 延迟探测显示真实值（143ms / 23ms），不再是 −1；
  - 「拉取列表」后点击模型输入框 → 候选菜单真实弹出 7 个模型，点击即写回（所见即所存）；
  - **删除全局默认卡片 → 确认 → 保存 → 真实 reload → 删除持久化且默认卡片指向存活卡片**
    （直读 SQLite 复核 `ai_config_id=7` 指向存活卡片，无悬空）；
  - 「⚡ 测试连通性」成功路径与失败路径均给出可行动中文提示；
  - 注入 502 HTML 错误页后，界面只出现「HTTP 502: 网关错误 (Bad Gateway)…」，无 HTML、无内网主机名。

### 十二、架构收敛第一步：模型解析唯一入口 + 运行记录（解决「到底哪个模型在跑」）

用户长期反馈「AI 模型配置及运行有很多问题」，并具体表现为
「界面显示模型 A、实际却像在用模型 B」。核实后发现根因是**同一件事有 4 套解析行为**：

| 位置 | 旧行为 |
|---|---|
| `agent/agent_api.go` | ID → 全局默认 → **第一张非 nil**，并直接 `aiConfig.Thinking = thinkingMode` **改写配置** |
| `data/NewDeepSeekOpenAi` | ID → **空配置**（`&AIConfig{}`，没有任何回落） |
| `agent/llm_summarizer.go` | 自维护内联解析，额外要求 ApiKey 非空 |
| `agent/price_alert_watcher.go` | 又一套内联解析 |

于是同一次请求在不同链路上可能落到不同模型，而界面永远只显示**请求的**那个。

**① `data.ResolveModel` —— 唯一解析入口**（`backend/data/model_resolver.go`）

优先级明确且可解释：显式指定卡片 → 全局默认 → 第一张可用卡片。
返回 `ModelResolution`：配置**副本** + `Source` + `Reason` + `FallbackUsed` + 当时存在的卡片 ID。
- `WithThinking()` 只改副本 —— 从根上消灭「卡片写 A、实际按 B 跑」；
- 四处解析全部改为调用它（含 `NewDeepSeekOpenAi` 不再返回空配置）；
- 新增 `ResolveModelForced(requireApiKey)` 供摘要/预警这类必须有密钥的链路复用。

**② 运行记录 —— 把事实固化成可查询字段**

新表 `agent_run_record`（`models.AgentRunRecord`），每次运行记录：
`kind`（chat/盘前/盘后/ETF轮动/产业拆解/…）、`requestedId`、`resolvedId`、`source`、
`fallbackUsed`、`modelName`、`baseUrl`、`thinking`、`resolution`（人话说明）、`fallbackChain`、
`finishReason`、`durationMs`、`contentLen`、`success`、`errorMsg`。
运行类型通过 context 传递（`WithRunKind` / `WithRunKindIfUnset`），**不改动 `ChatWithContext` 签名**。
记录在**关闭流之前**落库，保证消费者拿到 close 时记录已可查询（避免竞态）。

**③ 界面**：设置页「AI 模型注册表」新增「🩺 最近运行记录」面板，每行直接给出
**实际使用的模型**、解析依据（回落时高亮「已回落」并显示 `请求 #x → 实际 #y`）、
备用链、结束原因、耗时与篇幅。并订阅 Storage/SSE？——不，改为进入 AI 标签页时自动刷新。

**验证**
- 新增 9 条解析契约测试（`backend/data/model_resolver_test.go`）：精确命中 /
  全局默认 / 请求卡片已删时回落并如实标记 / 回落第一张 / **WithThinking 不污染存储** /
  无卡片必须报错 / Usable 判定 / requireApiKey / 多次解析确定性。
- 新增 5 条运行记录契约测试（`backend/agent/agent_run_record_test.go`）：
  失败也留记录且模型/依据如实 / RunKind 随上下文标注 / IfUnset 让外层标注优先 /
  无可用模型也留记录 / 倒序与 limit 与清空。
- 端到端（真实服务 + mock）：请求不存在的卡片 #99999 →
  记录显示 `请求ID=99999 → 实际ID=4`、`source=global_default`、`fallbackUsed=true`、
  `resolution=请求的卡片 #99999 已不存在（现存卡片 [4]），改用全局默认「…」`。
- 真实浏览器：设置页面板渲染出该行 ——
  `盘后复盘 | report-model | 全局默认（已回落）· 请求 #99999 → 实际 #5 | 成功 | 2510 ms | 1069 字`。

> 说明：本轮为「第一步」，只收敛**模型解析**与**可观测性**，尚未改动
> 提示词驱动的工具编排（第二步：把工具收敛为声明式能力注册表）。

最终体积
--------

| 阶段 | 追踪字节 | 追踪文件数 |
|---|---|---|
| v2.0.0 基线 | 57,304,765 | 718 |
| v3.0.0 第一轮 | 14,693,884 | 593 |
| v3.0.0 第二轮 | 14,075,868 | 593 |
| **v3.0.0 第四轮（终态）** | **14,129,371** | **595** |

累计减重 **75.3%**（−43.18 MB）。

---

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

## 补丁 2 (Patch 2) — 发布韧性、双 UI 库拆除与构建门禁硬化

发布日期: 2026-11-05  
触发: 生产巡查报告（Agent 页面巡检 + 两项更正）

### P0 · 发布韧性：部署不再打断在线用户

**根因**：Vite 按内容哈希生成懒加载 chunk。`docker compose up -d --build` 整体替换 `frontend/dist` 后，
所有旧哈希 chunk 立即 404；而「已打开旧页面的在线用户」点击菜单时发生的失败是**动态 `import()` 拒绝**，
发生在组件挂载**之前**，因此 `onErrorCaptured` / `ErrorBoundary` 无法捕获 —— 表现为点击无反应或整页白屏，
且服务端日志完全干净、无任何告警（最危险的静默故障）。

- **新增 `frontend/src/utils/staleChunkGuard.ts`**：监听 Vite 官方 `vite:preloadError` 事件 + `unhandledrejection` 兜底；
  命中陈旧 chunk 时执行**一次性静默刷新**，并设置 15s 冷却窗口 + `sessionStorage` 标记，严格杜绝无限重载风暴；
  冷却期内仍失败则降级为不依赖任何 UI 库的顶部提示条，明确告知用户手动刷新。
- **新增 `cmd/server/static.go`（`NewStaticHandler` 抽取重构）**：`/assets/` 下缺失的旧哈希 chunk 现在会回退到
  上一版本产物目录并返回 **200 + `X-Release-Fallback: previous`**，而不是 404。
  同时抽取为可测试函数，新增 `cmd/server/static_test.go` 4 组回归测试。
- **`docker-compose.yml`**：以只读方式挂载 `./frontend/dist-previous:/app/frontend/dist-previous:ro`。
- **`scripts/deploy-vps.sh`**：新增 4.5 步，在每次构建前自动 `docker cp` 快照当前线上产物为「上一版本回退源」。
- **`/health` 新增 `release_resilience` 可观测指标**：`previous_dist_armed` / `stale_chunk_fallback` /
  `stale_chunk_miss` / `immutable_asset_requests` —— 让「部署打断用户」这一静默故障变成可监控、可告警的显式信号。
- **`frontend/src/router/router.js`**：新增 `router.onError` 兜底，覆盖非 Vite 预加载路径的路由级 chunk 失效。
- **静态资源不可变缓存回归测试**：`TestStaticHandler_AssetImmutableCache` 锁定 `/assets/*.hash.js` 必须返回
  `Cache-Control: public, max-age=31536000, immutable`。

### P1 · 拆除 TDesign 双 UI 库税（Agent 路由 -70% 载荷）

**问题**：Agent 路由为 `t-chat-sender` / `t-chat-content` / `t-avatar` / `t-button` 共 4 个组件，
加载了完整 `@tdesign-vue-next/chat` + `tdesign-vue-next`（node_modules 222MB，产物 **raw 1.17MB / br 355KB**）——
全站投入产出比最差的一处；而 `n-avatar` / `n-button` 已在首屏付费。

- `t-avatar` → `n-avatar`；`t-button` → `n-button`（`AssistantMessage.vue`、`UserMessage.vue`、`ToolTimeline.vue`）。
- `t-chat-content` → **新增 `chat/ChatMarkdown.vue`**，基于 `markdown-it` 默认预置（`html:false` 从源头禁注入 +
  `sanitizeMarkdownHtml` 二次消毒 + 外链 `noopener noreferrer`），并修复 `sanitize.ts` 缺失 `br`/`s` 白名单
  导致软换行被消毒器吞掉的缺陷。
- `t-chat-sender` → **新增 `chat/ChatSender.vue`**：零外部 UI 库依赖，原生 textarea + 自适应高度，
  并补齐 TDesign 缺失的 **IME 组合输入保护**（中文候选词回车不再误发送）。
- `tdesign-icons-vue-next` → `@vicons/ionicons5`（已在首屏 `vendor-icons` 中）。
- `vite.config.js`：移除 `TDesignResolver`；新增 `vendor-md-core` 分包规则，避免轻量 markdown-it 被并入
  `vendor-markdown`（404KB br）导致 Agent 路由被迫加载完整编辑器依赖树。
- `package.json`：移除 `@tdesign-vue-next/chat`、`tdesign-icons-vue-next`、`@tdesign-vue-next/auto-import-resolver`；
  显式声明 `markdown-it`。

**实测收益（brotli）**

| 指标 | 修复前 | 修复后 | 变化 |
|---|---|---|---|
| `vendor-tdesign` | 355 KB | **0（已彻底移除）** | −355 KB |
| Agent 路由增量载荷 | ≈ 427 KB | **≈ 86 KB**（`vendor-md-core` 46.5 + `agent-chat` 29.5 + CSS 9.7） | **−80%** |
| 首屏预加载列表 | 含 TDesign | 不含 TDesign | — |

### P1/P2 · 构建门禁：把「静默带病产物」升级为构建失败

- **`frontend/scripts/check_ui_purity.js`（新增，prebuild + postbuild 双重把关）**：四条规则硬阻断双 UI 库回潮 ——
  源码禁止 tdesign import、模板禁止 `<t-*>` 自定义元素、package.json 禁止 tdesign 依赖、产物禁止 tdesign chunk。
- **`frontend/vite.config.js` `onwarn`**：`is not defined` / `is not exported` / `unresolved import` 直接抛出
  `[BUILD-BLOCKER]` 中断打包（此前仅测试环境阻断）。
- **恢复 `unplugin-vue-components` 本地组件自动注册**：在拆除 TDesign resolver 的同时保证 `src/components` 下
  本地组件的自动发现能力不被误删（该插件此前与 TDesign resolver 写在同一次注册中）。
- **新增 `.dockerignore`**：裁剪构建上下文（`frontend/node_modules`、`frontend/dist*`、`data/`、`logs/`），
  既加速镜像构建，又避免宿主平台原生二进制被带入 Linux 构建阶段。
- **README 新增「发布工程规范」章节**：明确「必须使用 `scripts/deploy-vps.sh` 部署」「不得删除
  `frontend/dist-previous/`」，并给出 `stale_chunk_miss` 告警口径与三条自检命令。

---

## 补丁 3 (Patch 3) — 静态质量门禁可用化、首屏减重 136 KB 与存量缺陷清算

发布日期: 2026-11-05

### P1 · `npm run lint` 从「形同虚设」到「真门禁」

**问题**：`frontend/eslint.config.js` 未为 `.vue` 配置 `parserOptions.parser = tseslint.parser`，
导致所有 `<script setup lang="ts">` 的组件报 `Parsing error: Unexpected token :/{/?`，
**50+ 个文件被静默跳过、lint 无任何门禁效力**（`stock.vue` / `settings.vue` 等未改动文件同样报错）。

- 修复 `.vue` 的 TS 解析链路。
- 配置从 `vue/flat/recommended` 换为 `vue/flat/essential`：前者开 400+ 条纯格式规则，
  实测单文件 600+ warning，彻底淹没真实问题；本仓库无 Prettier，格式规则只制造噪声。
- 新增 `eslint-plugin-unused-imports`（支持 `--fix`）与 `lint:fix` 脚本。
- **固化两条历史事故规则**：
  - `vue/no-undef-components`：模板中未 import 且不在 `components.d.ts` 自动注册清单中的组件（白名单由
    `components.d.ts` 自动生成，含 kebab-case，并放行 `n-*`）。事故原型：`<NSpace>` 未导入。
  - `vue/no-undef-properties`：模板引用未声明变量。Vue 会将其编译为 `_ctx.xxx`，渲染期静默为
    undefined，构建与首屏均不报错。

### P0 · lint 挖出的 3 个真实生产缺陷

| # | 位置 | 缺陷 | 影响 |
|---|---|---|---|
| 1 | `stockChangesMonitor.vue` | `klineCode` / `klineName` / `showKlineModal` / `diagnosisStockCode` / `diagnosisStockName` / `showDiagnosisDrawer` / `darkTheme` **共 7 个状态从未声明**，却在模板与点击处理函数中被引用 | 模板静默得 undefined；**用户点击「📈 K线」或「📊 诊断」立即抛 ReferenceError 被 ErrorBoundary 熔断**（与 NSpace 同类）。已补全声明 |
| 2 | `stock.vue` | `openCenteredWindow()` 在非 windows 分支调用 `OpenURL()`，但**从未导入** | 走 default 分支即抛 `ReferenceError: OpenURL is not defined`。已补充导入 |
| 3 | `StockLightweightKlineChart.vue` | `formatEastMoneyEndFromOldest()` 调用的 `formatYmdCompactShanghai` / `formatYmdHmsCompactShanghai` **从未定义或导入** | 东财 K 线 `end` 参数分支必抛 ReferenceError，分时/分钟线拉取中断。已在 `kline/time.ts` 实现（Asia/Shanghai 口径）并接入 |
| 4 | `chat/AssistantMessage.vue` | `const message = useMessage()` 与 props 的 `message: ChatMessage` **同名遮蔽**（`vue/no-dupe-keys` 捕获） | 模板 `{{ message.statusText }}` 恒取 undefined；`$emit('saveThesis', message)` / `$emit('exportImage', message)` 传给父组件的是消息 API 对象而非消息实体 → **「存入买方认知账本」「导出图片」实际失效**。已重命名为 `msgApi` |

其余清算：`cron-task-manager.vue` 计算属性缺省分支返回 undefined（已补）；`HotTopics.vue` / `stock.vue`
`v-for` 缺 `key`；`SelectStock.vue` / `InvestCalendarTimeLine.vue` 松散相等；`bridge.ts` 7 处 `Function`
类型与缺失 `cause` 的包装异常；`stock.vue` 5 处 `case` 内词法声明；`agent-chat.vue` 正则字符类外错误转义风险点；
`researchReport.vue` / `aiRecommendStocksList.vue` 声明却从未使用的 `order` 参数（此前请求体硬编码 `"desc"`，
调用方传参被静默忽略，现按契约透传）。

**一次性清理的量**：53 个文件，494 插入 / 524 删除，其中 189 处死导入、48 处未使用形参/捕获变量、
若干死函数与死数据块（`stock.vue` 78 行、`agent-chat.vue` 54 行）。

### P1 · 首屏入口 chunk 减重 136 KB

**根因**：`unplugin-vue-components` 会把 `stock.vue` 模板里的 `<stock-lightweight-kline-chart>` 解析为
**静态 import**，使 `StockLightweightKlineChart.vue`（lightweight-charts + 全量技术指标计算 + 筹码分布，
134 KB raw / 38 KB br）被直接打进首屏入口 chunk —— 而它只在用户点开「多周期K线」弹层时才需要。

- `stock.vue` 改为 `defineAsyncComponent(() => import('./StockLightweightKlineChart.vue'))`，
  与 `market.vue` / `allStockList.vue` / `FundRanking.vue` 的全站既有写法保持一致。

| 指标 | 修复前 | 修复后 |
|---|---|---|
| 首屏入口 chunk (`index-*.js`) | 328.69 KB raw / 101.80 KB br | **192.30 KB raw / 63.43 KB br** |
| 轻量K线 chunk | 无（已并入入口） | 134.94 KB raw / 38.09 KB br（懒加载） |

实测验证：首屏 canvas 数 = 0（chunk 未加载）→ 点击「多周期K线」→ 弹层正常渲染 7 个画布、零 ErrorBoundary。

### 验证

- `npm run lint`：**0 errors / 150 warnings**（警告均为存量 `no-explicit-any` 类型债，集中在 ECharts option
  与 RPC 负载，共 17 个文件）。
- `npm run build`：prebuild（`check_vue_imports.js` 83 文件 0 未导入标识符 + `check_ui_purity.js`）与
  postbuild（产物无 TDesign chunk）全部通过。
- Ego-Browser 自动化：11 条路由 + 研究中心 6 个 Tab 零 ErrorBoundary；助手消息 Markdown 保真渲染
  （h2/strong/table/pre/blockquote/br/公式）；原报障深链 `#/research?name=AI研报与认知账本` 正常。
- 回归基线对比：与 HEAD 构建产物逐 chunk 比对，确认无 chunk 归属异常。

---

## 补丁 4 (Patch 4) — 首屏减重 606 KB、CSP 一致性门禁、敏感路径 404

发布日期: 2026-11-05
触发: 全面检验报告（11 组测试 / 34 资源 / 250 接口 / 21 懒加载 chunk）

### P0 · 首屏关键路径 1305 KB → 699 KB gzip（−46%）

**根因一（最大单项，−547 KB gzip）**：Vite 为动态 `import()` 注入的共享运行时助手 `__vitePreload`
是一个**虚拟模块**（不属于 `node_modules`），因此不会命中 `manualChunks` 里任何 vendor 规则，
由 Rollup 自行决定归属 —— 实测它被塞进了 `vendor-codemirror`。
由于入口 chunk 必须静态引用 `__vitePreload`，`index.html` 的 modulepreload 就把整块
**1611 KB raw / 547 KB gzip 的 CodeMirror 写进了首屏预加载列表**，
而首屏没有任何一处用到 CodeMirror（它只服务于 `md-editor-v3` 编辑器路由）。
- `vite.config.js`：把 `__vitePreload` / modulepreload polyfill 显式隔离为独立的
  `vendor-preload` chunk（1.82 KB raw / 0.96 KB gzip）。首屏预加载从 547 KB 降到 1 KB。

**根因二（−27 KB gzip）**：应用直接依赖完整 `lodash`（CJS 全量构建）。
- 全量移除 `lodash` 与 `@types/lodash`：
  `_.filter` / `_.map` → 原生 `Array.prototype.filter/map`（`KLineChart.vue` 改为 `byIndex(i)` 辅助函数）、
  `_.uniqBy` → 原生 `Map`+`Set` 去重（`LongTigerRankList.vue`）、
  `keys` → `Object.keys`、`padStart` → 原生 `String.prototype.padStart`（ES2017）。
  `vendor-utils` 由 162.05 KB raw / 53.35 KB gzip → 88.58 KB raw / 25.89 KB gzip。

**实测结果**

| | 修复前 | 修复后 |
|---|---|---|
| 首屏关键路径（gzip -9） | 1305 KB | **699 KB** |
| `dist/index.html` 预加载资源数 | 9 | **8** |
| `vendor-codemirror` | 547 KB gzip（首屏） | 移出首屏（懒加载） |

浏览器实测（Performance Resource Timing）：首屏拉取脚本数 = 8，**CodeMirror 拉取次数 = 0**。

### P1 · CSP 与源码资源一致性（新门禁）

**修正一条误报，但保留其工程价值**：报告称「6 处 `http://image.sinajs.cn` 被自建 CSP 阻断，
导致分时图/日K图全部加载失败」。实测确认：
- 6 处 `http://` 字符串确实存在，与 `img-src 'self' data: blob: https:` 确实冲突；
- **但这 6 个 URL 从未被渲染** —— `<n-image :src="data.fenshiURL" />` 与 `<n-image :src="data.kURL" />`
  两处模板绑定**早已被注释停用**，分时图/日K图实际由 ECharts 绘制
  （`showFsChart` / `handleKLine`）。浏览器实测：分时弹层开启后内部 **canvas=1、img=0**。

因此这不是「用户可见的功能失效」，而是一枚**霿置地雷**：一旦取消注释启用该绑定，图片会被 CSP 静默阻断。
- 6 处全部改为 `https://`（上游 `image.sinajs.cn` http/https 实测同码同体积，零兼容风险）；
- 在停用的模板绑定旁标注停用原因与 CSP 约束，避免后人误判。
- **新增 `frontend/scripts/check_csp_compat.js`（prebuild 阻断）**：
  从 `cmd/server` 读取唯一权威 CSP，扫描 `src/` 中所有绝对 URL 字面量，
  若 scheme 不被对应指令放行则构建失败（放行 `localhost` 与 `www.w3.org` 命名空间）。
  已完成反向验证：临时注入一处 `http://` 图片引用 → 构建如期失败。

### P1 · 敏感路径不再被 SPA 兜底伪装成 200

**问题**：`/.git/config`、`/Dockerfile`、`/Makefile`、`/package.json`、`/.env` 均返回 **200 + index.html**。
不是泄露（返回的是 SPA 壳），但：① 任何安全扫描器都会得出「.git 目录可访问 / Dockerfile 暴露」
等高危误报；② 这类探测永远产生不了真实 404，404 监控失效。

- `cmd/server/static.go` 新增 `shouldDenyStaticPath()`：拒绝所有**以点开头的路径段**
  （`.git`/`.env`/`.ssh`/`.DS_Store`…，`.well-known` 按 RFC 8615 放行）
  与基础设施文件名/前缀（`Dockerfile`/`Makefile`/`docker-compose.*`/`go.mod`/`package.json`/
  `node_modules`/`backend`/`data`/`logs`/`scripts` …）。
- 非根路径的 SPA 兜底响应新增 `X-SPA-Fallback: 1` 标记，使运维能区分
  「真实页面」与「被兜底的正向 200」。
- `/health` 新增 `denied_path_probes` 与 `spa_fallback_hits` 指标，使扫描探测可观测。
- 新增 `TestShouldDenyStaticPath` / `TestStaticHandler_DeniesSensitivePaths` /
  `TestStaticHandler_FlagsSPAFallback` 三组回归测试（均含正反例）。

### 已核实：SSE 实时推送是活的（报告中的待验证项）

报告在未携带凭证的情况下探测，得到 `Set-Cookie: 0`，因此无法判定 SSE 是否可用。
实测（本地直接连 Go 服务端）：

```
POST /api/auth/login            → 200 + Set-Cookie: blades_token=…; Path=/; Max-Age=2592000; HttpOnly; SameSite=Lax
GET  /api/events/stream (无凭证) → 401
GET  /api/events/stream (Bearer)  → 200  text/event-stream  X-Accel-Buffering: no
GET  /api/events/stream (Cookie)  → 200
```

结论：HttpOnly Cookie 只在**登录成功后**下发（符合预期），SSE 三种鉴权路径全部正常。

### 已核实：`Cache-Control: no-store` 不是服务端缺陷

报告在共享隧道域名上测得 `/assets/*` 为 `no-store`。本地**直接连 Go 服务端**（绕过一切代理）实测：

```
/assets/index-<hash>.js  →  Cache-Control: public, max-age=31536000, immutable
/                        →  Cache-Control: no-cache
```

即 `no-store` 由中间代理（instabox 隧道）重写注入，服务端行为正确。**请在真实 VPS 上复测确认。**

### 验证

- `npm run lint`：0 errors / 150 warnings。
- `npm run build`：prebuild 三道门禁（未导入标识符 / UI 纯净度 / CSP 一致性）+ postbuild 全部通过。
- `go test ./cmd/server/`：全部通过（含新增 3 组敏感路径与 SPA 标记测试）。
- Ego-Browser 四组端到端：
  ① 发布韧性 + Agent 路由（旧 chunk 回退 200 / staleChunkGuard 冷却与自动刷新）；
  ② 助手消息 Markdown 保真渲染 + 研究中心 6 Tab + 原报障深链；
  ③ 首屏未拉取 CodeMirror + 10 条懒加载路由零 ErrorBoundary + 敏感路径全 404；
  ④ 分时/日K 弹层 ECharts 正常渲染（canvas=1 / img=0）+ 产物零 `http://` 外部资源。

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
   - **P0：记忆 session 隔离**（`blades_agent_memory_impl.go`）：`workingEntry` 新增 `SessionID` 字段，`WorkingPrompt()`/`GetWorking()`/`SummarizeAndArchive()` 按会话隔离，消除多用户/多标签页并发时的记忆串场风险。
   - **P0：工具成功率记录加锁**（`blades_agent_evolution_impl.go`）：`Evolver` 新增 `sync.Mutex`，保护 `RecordToolUse()`/`RankTools()` 的读-改-写原子性，高并发下工具成功率统计不再丢失。
   - **P1：回测并发控制+行情限流**（`post_mortem_loop.go` / `backtest_manager.go` / 新增 `rate_limiter.go`）：全局令牌桶限流器（默认 2 次/秒）+ K 线获取并发 semaphore（4 个）；每日复盘每只股票只获取一次 K 线（之前每个 horizon 都重复获取），行情请求减少约 2/3，避免行情接口被打爆。
   - **前端：研报生成并发防护**（`MarketIntelligenceDrawer.vue`）：按标签页独立 loading 状态 + 全局互斥锁，同一时间只允许一个研报类型进行大模型推演，避免并发启动多个 Agent 任务。
   - **后端：ETF 动量轮动回测 RPC 接入**（`QuantService.RunETFMomentumBacktest` / `App.RunETFMomentumBacktest`）：`etf_backtest.go` 原有但零生产调用者的回测框架现已通过 RPC 暴露，前端可一键触发 ETF 动量轮动回测。
   - **构建脚本修复**（`build.ps1`）：修复 UTF-8 无 BOM 导致 PowerShell 5.1 解析失败（`MissingCatchOrFinally`）；修复 `$env:PATH` 整体覆盖导致 git 不可用、构建中止的缺陷；新增 `build_scripts_test.go` 防回归门禁（3 个测试，搭在 `go test` 上自动执行）。

升级与兼容性说明
---------------
- 本版本向后完全兼容已有 SQLite 数据库文件与本地历史记录。
- 首次使用本地 Ollama/vLLM 等模型时，API Key 留空即可保存生效。
