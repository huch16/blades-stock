# 重构执行记录（refactor/clean-architecture）

依据：`~/workspace/blades-stock-review/REFACTOR_DESIGN.md`（§3 R-01～R-28）。
基线：dev `75450ae`，工作区干净。分支：`refactor/clean-architecture`。
每步一个提交，提交信息格式 `refactor(R-XX): <内容>`。

## 验证命令（每步必跑）

```bash
export GOTMPDIR=$HOME/.cache/go-tmp TMPDIR=$HOME/.cache/go-tmp
export PATH=$HOME/.local/go/bin:$PATH
go build ./... && go vet ./... && gofmt -l backend/ cmd/ tools/ main.go app*.go
go test -skip integration -count=1 ./...
bash scripts/check-deps.sh   # R-01 建立后每步必跑
```

前端步骤另跑（在 `frontend/` 目录）：`npm run lint`（0 errors）、`npm run build`（退出 0）。

## 基线（R-01，2026-11-20）

- `go build ./...` 通过；`go vet ./...` 退出 0；`gofmt -l` 0 文件。
- `go test -skip integration -count=1 ./...`：**13 个包通过、2 个包失败**，共 5 个失败测试，全部是沙箱网络环境原因：
  - `backend/agent/tools`：`TestIsTradingDay_API`、`TestGetHolidayInfo_API`、`TestGetNextTradingDay_API`、`TestGetHolidayYear_API`（依赖 timor.tech，不可达）
  - `backend/services`：`TestPreflightDialFailsFast`（依赖 TEST-NET-1 地址 192.0.2.1 真实不可达，沙箱网络会接受任意 TCP）
- `bash scripts/check-deps.sh` 退出 0（空跑通过）。
- `go test` 全量耗时约 108 秒（agent 包 63 秒、quant 包 92 秒为大头）。

## 执行日志

### R-01 建分支、基线快照、依赖护栏 —— 已完成

- 新建分支 `refactor/clean-architecture`（自 dev `75450ae`）。
- 新建 `scripts/check-deps.sh`：断言方案 §2.3 的全部禁止项（基础包不依赖业务包、db/datasource/strategy/data/quant/tools/agent/evaluator/services/交付层的 import 禁令、只有 data 能 import datasource、backend 永不 import 根包/cmd）。基线空跑通过。
- 新建本文件 `docs/refactor-notes.md`。
- 验证：build/vet/gofmt 通过；全量测试复现基线 13 通过 / 2 失败（5 个环境原因测试）；check-deps.sh 退出 0。

### R-02 测试门加固：网络依赖测试标为 integration —— 已完成

- 方案二选一（Short 跳过 vs integration 标签），按全仓统一约定（AGENTS.md / Makefile：`//go:build integration` + `make test-integration`）选了 **integration 标签**：只改测试准入，不改任何断言。
  - `backend/agent/tools/holiday_tool_test.go` 的 4 个 `_API` 测试（依赖 timor.tech）→ 新建 `holiday_tool_integration_test.go`（原样搬入，保留原有的 Short() 跳过逻辑）。
  - `backend/services/market_report_async_test.go` 的 `TestPreflightDialFailsFast`（依赖 TEST-NET-1 地址真实不可达）→ 新建 `market_report_async_integration_test.go`。
  - `backend/quant/holding_risk_scanner_test.go` 的 `TestScanHoldingRiskStructuralInvariants`（拉实时 K 线 + `db.Init` 读仓库根 stock.db）→ 新建 `holding_risk_scanner_integration_test.go`。
  - 原文件删除已搬走的测试，并清理随之多余的 import（holiday：context/strings/bladesTools；services：strings；quant：db/strings）。
- 验证：`go build/vet/gofmt` 通过；check-deps.sh 通过；`go test -skip integration -count=1 ./...` **15 个包全过**（基线 13 通过 / 2 失败 → 15 通过 / 0 失败）；`go vet -tags=integration` 三包通过；`-tags=integration -run` 复核：4 个节假日 API 测试与 `TestPreflightDialFailsFast` 在沙箱内仍以原失败模式失败（timor.tech 不可达 / 沙箱 TCP 接受黑洞地址），属环境原因，非回归——跳过的是环境依赖，不是缺陷，三者在有网 CI（`make test-integration`）仍须跑。

### R-03 prometheus.yml 去留 —— 跳过（待用户确认，附录 C #1）

- 无用户确认，不得自行选删除或保留+加固。`prometheus.yml` 原样保留，记入未做事项。

### R-04 删最硬的一批零引用小函数 —— 已完成

- 删除 16 个函数，共 84 行净删（`git diff --stat`：7 文件 +1/-85，含 gofmt 整理）：
  - `backend/data/openai_api.go`：8 个 OpenAI 配置 getter（`GetAPIKey`/`GetBaseURL`/`GetModel`/`GetMaxTokens`/`GetTemperature`/`GetTimeout`/`GetHttpProxy`/`IsHttpProxyEnabled`）
  - `backend/data/settings_api.go`：3 个预算 getter（`GetToolDailyBudget`/`GetToolDailyCostUSD`/`GetToolRunBudget`）
  - 根包：`app.go` 的 `GetSource`、`app_common.go` 的 `GetShanghaiTime`/`FormatShanghaiTime`
  - `backend/agent/retriever.go`：`RetrieverError.Unwrap`
  - `backend/quant/vector.go`：`NewBoolOnes`
  - `backend/data/tools.go`：`ToolsForQuestion`
- 零引用证据：生产代码 grep 命中均为仅定义处 1 次；测试文件 0 命中；前端 `frontend/wailsjs/` 与 `frontend/src/` 对 `GetSource`/`GetShanghaiTime`/`FormatShanghaiTime` 0 命中（且三者是 main 包普通函数，不在 Wails 绑定面内）。附带清理：`GetSource` 是 app.go 里 `golang.org/x/exp/slices` 的唯一使用者，import 随之删除。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件（初版删除后留双空行，`gofmt -w` 修后复核通过）；check-deps.sh 通过；`go test -skip integration` 根包 + data + quant + agent 四包全过。

### R-05 删 12 个 Akshare 客户端方法 —— 跳过（待用户确认，附录 C #2）

- 零引用证据已核查：12 个方法（`BoardConcept`、`BoardIndustry`、`ConvertibleBondSpot`、`ETFHistorical`、`ETFSpot`、`FuturesSpot`、`LongHuToday`、`MacroCPI`、`MacroGDP`、`MacroMoneySupply`、`MacroPMI`、`OpenFundNetValue`）在 Go 代码中无任何直接调用点；`Call()` 是通用远端分发，不调用它们；`capability_registry.go` / `tool_groups.go` 里的同名项只是工具分组字符串标签；`cmd/insert_etf_skill/main.go` 的提及只是技能文本（注明 akshare 系 ETF 接口当前 HTTP 500 不可用）。
- 但方案附录 C #2 要求先确认“Akshare 数据源是否已整体改走其他客户端”，无用户确认则整步跳过，不自行决定。记入未做事项。

### R-06 反思/进化 5 个方法 + NewEvaluator 双构造 —— 跳过（待用户确认，附录 C #3；且方案 (b) 无法按字面执行）

- 实地核查结果（`75450ae` 基础上复核，与方案 §1.5 的“双构造”表述有出入，记录如下）：
  - 两个 `Evaluator` 是**不同类型、方法集不相交**，并非同一构造的 stub/真身之分：`agent.Evaluator`（`blades_agent_reflection_impl.go`，空结构体，关键词评分 + `GenerateReflection`/`ScoreResult`/`RecordFeedback`/`GetHighQualityTraces`/`GetLowQualityTraces` 等 DB 方法）；`evaluator.Evaluator`（`backend/agent/evaluator/evaluator.go`，golden-case 批量评估器，方法为 `Run`/`loadTraces`/`evaluateCase`/`saveCSVReport`，无上述任何方法）。`services` 与 `main.go` 对 evaluator 包的唯一使用是 `AutoMigrate(&evaluator.EvaluatorReport{})`；`evaluator.AutoRun` 全仓无调用点。
  - `agent.NewEvaluator()` 返回空 `&Evaluator{}` 是关键词评分 Evaluator 的**正常构造**，不是 golden-case 评估器的 stub；`GenerateSkillFromTraces`（经 `RunSkillExtractionPass`，每日 02:30 cron 在 `app.go:368` 与 `cmd/server/main.go:377,381` 调度，链路在跑）内部调用 `NewEvaluator().GetHighQualityTraces(10)`。因此方案 (b)“只删 stub，NewEvaluator 改为调用真构造”无法按字面执行——改调 `evaluator.NewEvaluator(goldenPath, threshold)` 会改变返回类型，调用点全部无法编译，属于架构合并而非重构删除。
  - 5 个方法零调用点核查：`DefaultEvaluator`、`GetLowQualityTraces`、`RecordFeedback`、`CleanupToolRanks`、`ProposeEvolutionPlan` 在生产代码与测试中均无调用点（`GetHighQualityTraces` 除外，有 1 处 live 调用）；`App` 的 `evaluator`/`evolver` 字段除 `NewApp()` 赋值外全仓无读取点（字段去留按方案留待 R-27）；`GenerateReflection` 本身无生产调用点（仅测试 `evolution_wiring_test.go:185` 调用），其注释自述为 DORMANT。
  - 结论：(a) 删 5 个方法需用户确认“评估/进化链路是否还在跑”（附录 C #3），拿不到确认不做；(b) 按字面不可执行，需用户或方案先定夺两个 Evaluator 的合并方式。本步零改动，记入未做事项。

### R-07 删其余零引用函数 —— 已完成

- 删除 15 个函数，共 387 行（`git diff --stat`：11 文件 -387）：
  - data 侧 6 个：`AggregateSinaMonthKLine`（67 行）、`GetLivesAsTelegraph`（52 行）、`GetTelegraphListWithPaging`（23 行）、`GetStockChangeStats`（33 行）、`GetCallAuctionLatest`（8 行）、`SearchServers`（14 行）
  - agent 侧 8 个：`GetFormattedHistory`（14 行）、`GetProjectSkills`（22 行）、`GetProjectSummary`（7 行）、`ListProjects`（7 行）、`DeleteProjectAnalysis`（6 行）、`DeleteSkillForProject`（16 行）、`SearchCodeChunks`（56 行）、`SummarizeAndArchive`（55 行）
  - `chat_model_factory.go`：`InvalidateModelProviderCache`（7 行）
- 零引用证据：生产代码、测试文件、技能 prompt 与工具名字符串注册中均 0 命中（逐个 grep，命中均为仅定义处 1 次）；前端 `frontend/wailsjs/` 与 `frontend/src/` 0 命中；定义均为 Api/服务层方法，非 Wails 绑定方法。附带清理：`wallstreetcn_api.go` 删 `GetLivesAsTelegraph` 后 `db`/`models` import 变多余，随之删除。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 与 agent 两包全过。
- 说明：R-07 验证命令中途被运行时 SIGTERM 中断一次（build/vet/gofmt/check-deps 均已输出通过，测试未出结果），重跑 data + agent 测试通过后才提交；至此 51 个零引用函数中已删 31 个（R-04 16 个 + R-07 15 个），R-05 的 12 个 Akshare 方法与 R-06 的 5 个反思/进化方法因待确认未删，另有 3 个口径重叠（审计基线 59 个，审计已删 8 个）。

### R-08 D1 去重：hot-money / chip 16 个工具函数合并 —— 已完成

- 逐函数 diff 结果：15 个共享 helper 中 12 个逐字相同，3 个仅注释/死行漂移（`detectBlockTradePositive` 多 1 行注释；`detectHolderConcentration` 多 `lines := strings.Split` + `_ = lines` 死行；`extractBillboardTotal` 多 1 行注释、data 版多尾部 `_ = lines` 死行），逻辑零差异。
- 落点：共享实现保留在 `backend/data/tool_hot_money_chip.go`（方案附录 A 的备选落点；另一选项“tools 版保留、data 改调”不可行——data import tools 会成环且违反依赖规则），15 个 helper 改为导出（`ExtractBillboardTotal` 等），tools 侧 15 个副本删除、调用点改调 `data.Xxx`；3 个漂移函数以 tools 版正文对齐（仅注释差异）；两处 `_ = lines` 死行连同未使用的 `lines` 变量一并删除。
- data 侧测试 `tool_hot_money_chip_test.go` 仅调用点改名（小写→导出名），断言一字未改。
- 数量对照：data 文件 19 个函数（init + 3 个 handler + 15 个共享 helper）；tools 文件 4 个函数（`GetHotMoneyChipTools` + 3 个工具构造器），`grep -c '^func '` 之和与拆前一致；净删 219 行。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 与 agent/tools 两包全过。

### R-09 D5 去重：thsResultToMarkdown 收敛到 backend/data —— 已完成

- 逐行 diff 结果：两版逻辑与输出格式完全一致（标题、空值、表格列顺序、返回格式均相同），唯一差异是 tools 版多处 `ok` 安全判断、openai_tools 版是直接断言（脏数据会 panic）。以 tools 版为准。
- 落点：新建 `backend/data/ths_markdown.go`，导出 `ThsResultToMarkdown`（与同包 `JSONToMarkdownTable` 同处）；删除 `backend/data/openai_tools.go:119` 的旧版与 `backend/agent/tools/data_tools_wrapper.go:54` 的 tools 版；6 处调用点不动逻辑，只改调用名（tools 内 3 处改调 `data.ThsResultToMarkdown`，data 内 3 处改调同包 `ThsResultToMarkdown`：`tool_search_bk.go:29`、`tool_search_etf_stock.go:37,66`）。
- 全仓只剩 1 处定义；附带清理 `openai_tools.go` 多余的 convertor import。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 与 agent/tools 两包全过；`grep -rn 'func thsResultToMarkdown'` 全仓只剩 1 处定义（导出名）。

### R-10 D3 去重：Float32 转换收敛 —— 已完成（走方案授权的备选方向）

- 方向说明：方案原文要求“删 models 版、统一用 agent 版”，但实地核查证明该方向**结构上不可行**：`models` 不能 import `agent`（`agent` 已 import `models`，反向 import 即成环，且违反依赖规则 2；check-deps.sh 会拦截）。因此按方案同一条的备选条款执行——“若调用方不接受 nil，本步改为‘删 agent 版、统一 models 版’，以调用方为准”。
- 调用方核查（nil 语义无人依赖）：
  - agent 版唯一生产调用点 `semantic_distiller.go:447` 有 `len(vec) > 0` 守卫，永不传入空输入；
  - `GetVector` 的调用方 `semantic_distiller.go:616` 只用 `len(vec) == 0` 判断，nil 与空切片无差别；
  - `SetVector`/`GetVector` 本身（`knowledge_crystal.go:68,73`）只是透传，接受空输入 nil。
  - 其余调用点只有往返测试（非空向量）。
- 执行：先写空输入单测（新建 `backend/models/knowledge_crystal_test.go`，断言空输入返回长度 0 的非 nil 切片 + 非法长度返回 nil + 往返一致），**先跑通过、后删**；删 `backend/agent/blades_agent_memory_models.go` 的 agent 版两函数（`binary`/`math` import 随之清理）；agent 调用点改调 `models.Float32sToBytesLE` / `models.BytesToFloat32sLE`（`semantic_distiller.go:447`、测试文件两处，均已 import models）。
- 全仓只剩 1 处定义（models 版）；`tools/sqlite_search_demo.go:19` 的小写副本按方案留待 R-28。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` models 与 agent 两包全过（含新增 3 个单测）。

### R-11 D8 去重：SeedDefaultSkills 合并 —— 已完成（选 b 方案：services 改用 //go:embed）

- 核查结论（附录 C #6）：发布包**不含** `cmd/*.txt` 运行时文件——`build.ps1` 只向 `build/bin/` 复制 `data/stock.db`，不复制 `cmd/`；两个 txt 文件此前只通过 `main.go` 的 `//go:embed` 进入桌面二进制（编译期），services 版则在运行时用相对路径读文件，发布包/非仓库目录下会静默回退到两行占位常量（`defaultETFDirective`/`defaultSerenityDirective`）。因此方案 (a)（删 main 版、改调 services 版）不安全：桌面 seed 会降级为占位提示词，属于行为变化。
- 执行 b 方案：services 版改用 `//go:embed`。Go 的 embed 模式禁止 `..`，`backend/services` 不能直接 embed `../../cmd/...`，故把两个 txt 文件 `git mv` 到 `backend/services/seeddata/`（内容 0 改动），services 以 `//go:embed seeddata/*.txt` 嵌入；`cmd/insert_etf_skill/main.go` 与 `cmd/insert_serenity_skill/main.go` 的读取路径同步改为 `backend/services/seeddata/*.txt`（二者均在仓库根运行，相对路径仍成立）；删除 main 版 `SeedDefaultSkills()`（90 行）与两个 embed 变量，`main.AutoMigrate()` 改调 `services.SeedDefaultSkills()`；删除两个占位常量。
- 两份技能字面量逐行比对：除 SystemPrompt 来源、日志别名（`log` vs `logger`）与失败日志级别（Errorf vs Warnf）外完全一致；统一后以 services 版为准（Warnf）。
- 验证：临时测试（跑完即删，不提交）在临时库中验证合并后 seed 出 2 个技能、SystemPrompt 与 txt 文件逐字一致、长度 >1000、二次播种幂等——通过；`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` 根包与 services 包全过。分支说明：走的是 embed 分支（编译期嵌入），不再有“无 DB 时走 const 默认值”分支。

### R-12 删 9 个仅测试引用函数 —— 已完成 8 个，跳过 1 个

- 已删 8 个函数（生产代码约 330 行 + 对应测试）：
  - `GetHKStockInfo`（60 行）：仅存一处注释掉的调用（`stock_data_api_test.go:151`）；其测试 `TestGetHKStockInfo` 实际调用的是 `getDCStockInfo` 循环，不动该测试。
  - `GetNewsList`（20 行）：全仓词法精确匹配 0 引用，直接删。
  - `GetNewsList2`（21 行）：两个测试引用。`TestGetNewsList2`（integration 文件，无断言的纯探针）随函数一起删；`TestAnalyzeSentiment`（默认套件，测生产函数 `AnalyzeSentimentWithFreqWeight`）改调生产函数 `GetTelegraphList("")`（`market_service.go:469,485` 与 `app.go:2029` 在用），并清理随之多余的 `random` import。注：`GetNewsList2` 与 `GetNewsList` 近乎相同，前者多一次 `TelegraphList(30)` 刷新调用，改写后该刷新不再发生（测试只需新闻文本，无影响）。
  - `NewBoolVector`（3 行）：16 处测试夹具调用全部改结构体字面量（`&BoolVector{Values: ..., Len: N}`），断言未改；覆盖的测试（BoolVector And/Or/Not/Sum/Any/All、`TrendFilter`、`VolatilityFilter`）测的都是生产方法，保留。
  - `NewMatrixFromVecs`（14 行）：`TestNewMatrixFromVecs` 纯测该构造，随函数删；`TestMatrixRow`/`TestMatrixCol`（测生产方法 Row/Col）改结构体字面量（`&Matrix{Rows: 2, Cols: 2, Values: []float64{1, 3, 2, 4}}`）。
  - `SaveStockChange`（17 行）：词法精确匹配 0 引用（方案提到的 `cmd/server/server_test.go` 引用实际是 `SaveStockChangesWithDedup`，不同函数），直接删。
  - `SearchToJson`（11 行）：测试改调生产函数 `Search`（改名 `TestWebSearchApi_SearchResults`，保留“返回非空”断言）；`SearchToMarkdown` 仍在生产中使用（`web_search_tool.go:44`），不动。
  - `compressExecutedStepResult`（32 行）：唯一调用点是 `TestCompressExecutedStepResult`，纯测死代码，函数与测试一起删。注：删后 `smartContentCompress` 在生产中也无调用点（仅其自身测试调用），不在本次删除清单内，留待后续确认。
- 跳过 1 个：`NewSummaryStockNewsStreamWithTools`（151 行）。它是资讯总结 + 股票推荐的流式生产管线，唯一的调用点是两个 integration 测试（`openai_api_test.go:37`、`app_integration_test.go:107` 的 `TestSummaryStockNews`，全 App→AI 端到端）；其同胞 `NewSummaryStockNewsStream` 同样全仓 0 生产调用点。按方案“相关测试未确认过时则该函数跳过，不准先删函数、后补测试”，本步不删，记入未做事项。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data、quant、agent 三包全过（含改写后的 `TestAnalyzeSentiment` 与 BoolVector/Matrix 测试）。
- 发现的既有问题（非本步造成，不修）：`go vet -tags=integration ./backend/data/` 在基线即失败，integration 测试文件至少有 4 处未定义引用——`markdownToImage`（`gen_test_images_test.go:391`、`markdown_to_image_e2e_test.go:36,78`）、`GetTopNewsList`（`openai_api_test.go:50`）、`SearchStockInfoByCode`（`stock_data_api_test.go:103`）、`parseBingResultsWithGoquery`（`web_search_api_test.go:204`）。已用排除法确认：除这 4 处既有错误外，本步改过的 integration 测试文件无 vet 错误。

### R-13 删 3 个前端死组件 —— 跳过（待用户确认，附录 C #5）

- 无用户确认“不是未完成的功能入口”，整步跳过。`aiRecommendStocksList.vue`、`mcp-server-manager.vue`、`skill-manager.vue` 及 `vue-json-pretty` 依赖原样保留，记入未做事项。

### R-14 L3 样板去重第一期 —— 已完成（安全子集；统一请求 helper 未做，见原因）

- 实测纠正方案前提：`market_news_api.go` 的 31 处 UA 并非同一字面量，而是 **5 种不同 UA**（19× Firefox/140、7× Chrome/117 Edg/117、2× Chrome/120 Edg/120、1× Firefox/146、1× Firefox/142、1× Chrome/117 无 Edg），且超时有 7 种取值（5s/10s/15s/30s/crawlTimeOut 等）、请求分 GET/POST、带 SetBody 结构体/SetFormData/SetQueryParams/各端点 token 与 Cookie、响应键各异（`data`/`re` 等）。因此“1 个 UA 常量替代 31 处”与“1 个 helper 收敛全部请求段”均不可行——统一 UA 会改变 12 处实际发出的 UA（行为变化），helper 会藏起各端点差异头（设计自身提示的回归风险，测试发现不了）。
- 已做的安全子集：
  - `backend/data/httpclient.go` 新增 `FirefoxUserAgent` 常量；`market_news_api.go` 内 19 处完全相同的 Firefox/140 字面量改调常量（其余 12 处各端点专用 UA 保持字面量原样）；`grep -c FirefoxUserAgent`：market_news_api.go 19 处、httpclient.go 定义 1 处。
  - L1 裸 `json.Unmarshal`：修 14 处——7 处 `return []any{}` 模式（IndustryResearchReport、StockResearchReport、StockNotice、EMDictCode、HotTopic、InvestCalendar、ClsCalendar）改为接 error、记日志、返回 `[]any{}`（失败时 respMap 本就为空，后续 ok 判断同样返回 `[]any{}`，行为不变）；TradingViewNews 与 HotEvent 的 `json.Unmarshal(items, ...)` 改为接 error、记日志、返回空结果（items 是刚 Marshal 的值，失败时目标为空，行为不变）；GetGDP/GetCPI/GetPPI/GetPMI/GetIndustryReportInfo 的 5 处 marshal 往返改为接 error、只记日志（res 保持原值返回，行为不变）。
  - 未修 1 处：`EMDictCode` 的缓存读取 `json.Unmarshal(d, &respMap)`（现 906 行）——失败时必须落入网络拉取分支，提前返回会改变行为，故保留。
- 未做统一请求 helper：请求点超时/方法/头/体/后续处理全部异构，helper 需 6 个以上参数且省不了几行，反而藏起端点差异；记入未做事项（后续如需，先按端点分组再抽 helper）。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包全过。
