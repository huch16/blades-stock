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

### R-15 同名不同义 4 组改名防错 —— 已完成

- `formatFloat`/`formatPct`（语义直接冲突）：data 版（`strategy_screen_api.go:268,272`）输入已是百分比数值（`formatPct` 直接加 `%`，`formatFloat` 保留 2 位小数）→ 改名 `formatPctRaw`/`formatFloat2dp`，调用点 2 处（`:140,141`）同步改名；quant 版（`engine.go:641,652`）输入是比率（`formatPct` 先 `*100`，`formatFloat(v any)` 4 位小数/int/`%v`）→ 改名 `formatPctRatio`/`formatFloatAny`，调用点 11 处（`engine.go` 10 处 + `engine_test.go` 的 `TestFormatFloat`/`TestFormatPct`，断言未改——`formatPctRatio(0.0567)=="5.67%"` 等原断言全部保留）。全仓旧名 `formatFloat(`/`formatPct(` 0 命中。
- `extractJSON`：agent 版（`blades_agent_reflection_impl.go:423`，截取首尾 `{...}`）→ `extractJSONBraces`，调用点 4 处（`blades_agent_evolution_impl.go:91,142,254`、`blades_agent_reflection_impl.go:219`）同步改名；data 版（`stock_changes_api.go:244`，剥 JSONP 正则 `^\w+\((.*)\)$`）→ `stripJSONP`，调用点 1 处（`:125`）同步改名。全仓旧名 `extractJSON` 0 命中。
- `isTradingTime`（cron 版）：`backend/agent/cron_task_api.go:500` 只判断周末 + 时段，节假日会误判为交易时间 → 改名 `isTradingTimeNow`，并复用交易日历 `data.IsTradingDay`（`backend/data/trading_day.go:122`，AData/Baostock 交易日历；缓存未初始化时降级为周末判断，与旧逻辑一致；`InitTradingCalendar` 在 `main.go:144` 与 `app.go:122` 启动时已调用）。冗余的周末判断块随之删除（`IsTradingDay` 两分支均已覆盖周末）。调用点 1 处（`executeStockChangeSave`，`:402`）同步改名。根包 `app.go:691` 的 `isTradingTime(date time.Time)`（带 timor.tech 交易日历）语义不同、不重名冲突，保留不动。**行为变化（方案已明示）：法定节假日的交易时段内，`executeStockChangeSave` 从“误判为交易时间而执行”变为“正确跳过”。**
- `AutoMigrate` 清单收敛（以 `services.AutoMigrateAll()` 为准）：脚本逐项对照三份清单——`main.AutoMigrate()` 39 个模型（去重后）**全部**在 `services.AutoMigrateAll()` 的 50 个直接迁移项内（`main ⊆ services-direct`，差集为空）；`db.AutoMigrate()`（`chat_memory.go:106`，11 项固定 + `AutoMigrateList` 注册的 `ToolRank`、`KnowledgeCrystal`、`AgentMemory`、`AgentSession`、`AgentTrace`）在桌面与 server 两条路径上均经 `db.Init`（`backend/db/db.go:86,97`）先行执行，与 `services.AutoMigrateAll()` 内 `:91` 的 `db.AutoMigrate()` 调用是同一份清单。唯一实际去重：`main.go` 内 `data.FollowedStock` 出现 2 次（`:319` 与 `:328`），删除第 2 次（`AutoMigrate` 本就幂等，纯去重）。收敛后 main 清单 39 项 / 39 唯一；三路合计去重后 58 个模型。删除与调用点迁移留待 R-21（本步只做清单收敛 + 证据留存）。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration -count=1` data、quant、agent、agent/tools、services、db、根包 7 个包全过。

### R-16 拆 data_tools_wrapper.go（4,214 行 → 9 个域文件 + 聚合文件）—— 已完成

- 拆分前实测与方案出入（均已核对并记录）：
  - 文件实际 4,214 行（方案 4,273 行为 `75450ae` 基线行号，后续步骤已移动行号）；以工具名分段为准拆分，不按旧行号。
  - 方案验收口径“`grep -c 'NewDataToolWrapper('` 各域之和 = 81”实为旧文件单文件计数，其中含 **1 处定义 + 2 处注释掉的死注册**（`WebSearch`、`SearchParams`，原 `:2484`、`:2514`，共 68 行注释）；活注册 append 只有 **78 处**（77 个单工具 + 1 处 F10 动态循环注册 10 个工具）。拆分后活 append 78 处 + 定义 1 处 = 79，全包计数 112 → 110（恰好减去 2 处已删注释注册）。真正的工具数以 `TestGetAllDataTools` 为准：拆分前后均为 **84 个**（过滤后），无重名。
  - `GetAllDataTools()` 内除 78 处 `NewDataToolWrapper` append 外，还有 6 处其他 append：`GetETFMomentumRankingTool()`（`:1200`，随 K 线域走）、尾部 5 处跨文件聚合（`GetHotMoneyChipTools`/`GetAkshareTools`/`GetAdataTools`/`GetTickFlowToolWrappers`/`GetStrategyScreenTool`，连同 `IsToolKeyConfigured` 过滤循环留在聚合文件）。
  - `GetAllDataTools()` 体内非空行逐行对账：除上述 68 行注释注册（按方案删除）与函数自身结构行外，全部行均已落入域文件或聚合文件尾部，无遗漏、无多余。
- 拆分结果（原文件 4,214 行 → 聚合文件 111 行；各域文件行数）：
  - `data_tools_screen.go` 406 行（筛选/搜索域，5 个工具：FilterStocks、SearchStockByIndicators、SearchBk、SearchETF、InteractiveAnswer）
  - `data_tools_finance.go` 237 行（资金/财务/研报域，10 个工具）
  - `data_tools_market.go` 416 行（行情/资讯/推荐域，7 个工具）+ `getMarketDataContent` 与其专用的 `APIResponse`/`APIData`/`APIIndexQuote`/`APIUpDownDis`/`APIPurchase` 类型（全仓仅此处使用）
  - `data_tools_ai.go` 249 行（AI 分析/消息域，5 个工具）
  - `data_tools_kline.go` 468 行（K 线/自选写操作域，6 个工具 + `GetETFMomentumRankingTool()`）
  - `data_tools_fund.go` 321 行（基金域，5 个工具）与 `data_tools_hot.go` 621 行（热门/日历域，11 个工具）：原 `fund_hot` 域文件 924 行，按方案“超 800 行再按基金与热门拆两个文件”的条件拆分
  - `data_tools_changes.go` 1,146 行（异动/公告/研报/F10/涨停域，20 处 append：19 个单工具 + F10 循环 10 个工具）+ 涨停专用 helper（`fetchUplimitData`、`floatOrDefault`、`getPlatesStr`、`getStockNameFromPlateStocks`，仅涨停工具调用）
  - `data_tools_wallstreetcn_tdx.go` 359 行（见闻/通达信域，9 个工具：4 个见闻 + 5 个 TDX）
  - `data_tools_wrapper.go` 111 行（聚合文件）：仅留 `NewDataToolWrapper`、`truncateToolResult`、聚合版 `GetAllDataTools()`（依次聚合 9 个域注册函数 + 尾部 5 处跨文件聚合 + 过滤循环）与跨域共享 helper `parseStockCodesFromArgs`（K 线/热门域共用）、`parseInt`（财务/行情/K 线域共用）；跨域 helper 不硬塞进某一域，留在聚合文件。
  - 注册块原文逐字搬家（缩进不变：原在 `GetAllDataTools` 内 1 tab，新注册函数内同样 1 tab）；各文件 import 按实际引用逐项计算生成，无多余 import。
  - 执行中修过一次生成脚本 bug：helper 一度被拼进注册函数体内导致语法错误，已纠正为“helper 放在注册函数之后”，随后 `go build/vet` 通过。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` tools 包与 agent 包全过；`TestGetAllDataTools` 工具总数 84（与基线一致）、`TestEveryAssembledToolIsDeclared`、`TestAssembledToolsHaveNoDuplicates`、`TestGroupCapsAreRespected` 全部通过。

### R-17 拆 backend/data/tools.go（2,698 行 → 9 个域文件 + 分类/查询文件 + 聚合文件）—— 已完成

- 拆分前实测与方案出入（均已核对并记录）：
  - 文件实际 2,698 行（方案 2,704 行为基线行号）；以工具名分段为准拆分，不按旧行号。
  - `Tools()` 内除 94 个具名 schema append 外，还有 1 处 FinGenius 循环注册（`:2210–2241`，`AnalyzeHotMoneyActivity`、`AnalyzeChipConcentration`、`AnalyzeCapitalGame` 3 个工具，游资/筹码/资金博弈分析），随 changes 域走；另有 3 处注释掉的死 schema（`QueryBKDictInfo`、`GetIndustryResearchReport`，`:185–209`；`CailianpressWeb`，`:521–540`）——与 R-16 不同，本步方案未授意删除，故原文保留（前两处随 finance 域、后一处随 market 域，仍为注释状态，零行为变化）；另有 2 处单行注释（`//CreateAiRecommendStocks`、 `//BatchCreateAiRecommendStocks`）随所属工具块走。
  - `Tools()` 体内非空行逐行对账：除函数自身结构行外全部落入域文件、聚合尾部或上述保留的注释/循环单元，无遗漏。
  - `toolSchemaStockCodes`：8 处使用横跨 kline、finance、hot 三个域，按方案“随第一个 schema 文件走或留在原文件”留在了聚合文件 `tools.go`（同包，无需移动）。
- 拆分结果（原文件 2,698 行 → 聚合文件 35 行）：
  - `tools_screen.go` 340 行（12 个：SearchStockByIndicators、SearchBk、SearchETF、InteractiveAnswer、FilterStocks、QueryStockCodeInfo、QueryIwencai、SelectAStock、SelectSector、QueryMacro、QueryZhishu、QueryEvent）
  - `tools_finance.go` 486 行（24 个：研报/资金/财务 10 个 + F10 10 个 + ComparableCompanyAnalysis、SearchInvestor、SearchReport、QueryInsResearch）+ 2 处保留的注释 schema
  - `tools_market.go` 194 行（9 个）+ 1 处保留的注释 schema
  - `tools_ai.go` 104 行（4 个）、`tools_kline.go` 318 行（6 个）、`tools_fund.go` 119 行（5 个）
  - `tools_hot.go` 267 行（14 个：热门/日历域 + GetHolidayInfo、IsTradingDay、GetNextTradingDay）
  - `tools_changes.go` 258 行（11 个 + FinGenius 循环 3 个）
  - `tools_wallstreetcn_tdx.go` 185 行（9 个：5 个 TDX + 4 个见闻）
  - `tool_groups.go` 198 行：`dataToolGroup` 类型 + 常量 + `dataToolGroupMap` + `dataToolGroupKeywordsList`（原文搬运）
  - `tools_query.go` 255 行：`classifyDataToolGroups`、`filterDataToolsByGroups`、`appendAgentParityTools`（含 SendDingDingMessage、GetHolidayYear/Batch、16×`newQueryTool`、5×`newSimpleQueryTool`、GetDailyDimensionStats、GetTypeStatsByDate、GetUplimitPlateStocks）、`newQueryTool`、`newSimpleQueryTool`
  - `tools.go` 35 行：仅留 `toolSchemaStockCodes` 与聚合版 `Tools()`（依次调 9 个 `appendXxxTools` + `appendAgentParityTools` + `FilterToolsByApiKey`）；`strings` import 随之删除（新聚合不再使用）。
  - 各域函数均为 `appendXxxTools(tools []Tool) []Tool`，schema 字面量逐字搬家。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包全过；数量对照（临时测试，跑完即删，未提交）：`len(data.Tools(nil))` 拆分前后均为 **111**，`dataToolGroupMap` 均为 **125 项**，关键词组 7 组/205 个关键词均一致。

### R-18 拆 backend/data/stock_data_api.go（2,880 行、67 个函数 → 核心文件 + 5 个域文件）—— 已完成

- 拆分前实测与方案出入（均已核对并记录）：
  - 文件实际 2,880 行、67 个函数（方案 2,941 行/68 个函数为基线；R-12 已删 `GetHKStockInfo`，故 67 为当前正确总数）。
  - 方案未列出的 11 个交易记录函数（`AddTradingRecord`、`GetTradingRecordList`、`GetTradingRecordStatistics`、`GetTradingRecordById`、`UpdateTradingRecord`、`DeleteTradingRecord`、`CheckFrequentTrading` + 私有 helper `fifoAvgUnitCost`、`normalizeTradingRecordAPI`、`resolveTradingRecordClosePrice`、`fillTradingRecordCloseSnapshot`）自成一域，新增 `stock_trading_record.go`（537 行）；未硬塞进核心文件。
  - `GetStockRZRQInfo`（融资融券）归入 `stock_money.go`（资金/财务域），未留核心文件。
  - 港股域：`GetHKStockInfo` 已在 R-12 删除，只剩 `getDCStockInfo`、`saveHKStockInfo` 两个小函数，按方案并入 `stock_realtime.go`，不单独建 `stock_hk.go`。
  - 只拆方法：全部 struct 定义（含 `StockInfo`、`FollowedStock`、`TradingRecord`、`StockBasic`、`IndexBasic`、`KLineData`、`MinuteData`、`TradingRecordListQuery` 等）与全部 5 个 `TableName()` 留在原文件不动；常量与 `StockDataApi` 接收者 struct 亦留原文件。
- 拆分结果（原文件 2,880 行 → 核心文件 629 行）：
  - `stock_data_api.go` 629 行，18 个函数：13 个核心查询（`NewStockDataApi`、`GetIndexBasic`、`GetStockBaseInfo`、`GetStockList`、`GetAllStockInfoList`、`GetAllStockInfoById`、`AddAllStockInfo`、`DeleteAllStockInfo`、`BatchDeleteAllStockInfo`、`GetAllMarkets`、`GetAllIndustries`、`GetAllConcepts`、`GetStockInfoByCode`）+ 5 个 `TableName()`。
  - `stock_follow.go` 233 行（9 个，自选/提醒域）、`stock_kline.go` 266 行（5 个，K 线域）、`stock_realtime.go` 806 行（14 个，实时行情解析域，含港股两个小函数）、`stock_money.go` 477 行（9 个，资金/财务/概念域）、`stock_trading_record.go` 537 行（11 个，交易记录域）。
  - `JSONToMarkdownTable` 移入 `backend/data/ths_markdown.go`（+48 行，与 `ThsResultToMarkdown` 同处，`fmt` import 随之补上）。
  - 各文件 import 按实际引用逐项计算生成；执行中修过一处误判：`stock_money.go` 的 `context` 只出现在被注释掉的 resty 配置行里，误删后构建通过。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包全过；`grep -c '^func '` 各文件之和 = 67（核心 18 + follow 9 + kline 5 + realtime 14 + money 9 + 交易记录 11 + `ths_markdown.go` 内 `JSONToMarkdownTable` 1），与拆分前一致。

### R-19 拆 app.go（3,058 行 → 65 行 + 8 个域文件）—— 已完成

- 拆分前实测与方案出入（均已核对并记录）：
  - 文件实际 3,058 行（方案 3,072 行为基线行号）；`^func (.*App)` 计数 208 与方案一致（含 3 个以 `*App` 为普通参数的包级函数 `MonitorFundPrices`、`MonitorAiRecommendStockPrices`、`MonitorFollowedStockCostPrices`）。
  - `app.go` 只留 `App` 结构体 + `NewApp()`（AiTools 组装在 `NewApp` 内：`data.Tools(tools)`），65 行。
  - 8 个域文件按方法名前缀与调用的 Api/服务归属划分（共 233 个函数 + `tradingDayCache` 变量 + 3 个类型别名/结构体）：
    - `app_stock.go` 489 行（38 个）：自选/股票基础 + 交易记录 + 分组 + 提醒价监控 helper 链（`syncAllStockInfo`、`CheckStockBaseInfo`、`GetStockInfos`、`addStockFollowData`、`MonitorFollowedStockCostPrices` 等）。
    - `app_market.go` 402 行（36 个）：行情/市场 + K 线/TDX + 电报/指数/资金排行/市场统计 + `MonitorAiRecommendStockPrices` 与告警节流 helper（`canSendAlert` 等 4 个，主调方在该监控函数内，按调用量归属）。
    - `app_agent.go` 811 行（62 个）：Agent 对话/消息/MCP/技能/市场情报报告 + 类型别名 `AiModelInfo`、`PromptTemplateItem`；`GenNotificationMsg`/`getMsgTypeTTL`/`getMsgTypeName` 随调用方 `SendDingDingMessageByType` 走本域；`preFetchMarketSummaryData` 随调用方 `SummaryStockNews` 走本域。
    - `app_fund.go` 79 行（12 个）：基金域（含 `MonitorFundPrices`）。
    - `app_settings.go` 744 行（32 个）：设置/系统/备份/文件操作 + `domReady`（约 300 行启动逻辑，含 `syncAllStockInfo`、`refreshTelegraphList`、`NewsPush` 等跨域调用，同包无影响）+ `Greet`/`BatchGreet` + `GetImageBase`（随调用方 `GetVersionInfo` 走本域）。
    - `app_cron.go` 384 行（16 个）：定时任务域（含 `setCronEntry`/`getCronEntry`）。
    - `app_analysis.go` 230 行（26 个）：诊断/论点/持仓风险/回测/审计/向量回测/纸面交易/复盘/Alpha 挖掘 + 类型 `VectorBTResult`。
    - `app_trading_time.go` 269 行（10 个函数 + `tradingDayCache` 变量）：交易日历/时间域。
  - 两个域文件超方案“200–500 行”预期（agent 811 行、settings 744 行）：系方法分布本身所致（agent 62 个方法、settings 含约 300 行 `domReady`），未再强行细分——再拆会偏离方案的 8 文件划分。
  - 执行中修过两处 import 误判：`a.cron.AddFunc` 是字段访问不是包引用，`app_settings.go` 的 `encoding/json` 与 `cron`、`app_stock.go` 的 `cron` 均属误判导入，已删除（import 判定正则把 `a.cron.` 当成包引用，下同类步骤注意）。
- 验证（硬性验收）：
  - `go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration -count=1 ./...` 全量 **15 个包通过、0 个失败**（与 R-02 后基线一致）。
  - `grep -c '^func (.*App)'`：新文件之和 = 208（agent 55 + analysis 26 + cron 16 + fund 11 + market 34 + settings 32 + stock 34），与拆分前一致。
  - Wails 绑定：沙箱内无 `wails` CLI（`build.ps1` 本身也用 `-skipbindings -skipembedcreate`，即本项目构建默认不重新生成绑定），无法实际执行生成。改用等价验收：AST 提取根包全部导出 `*App` 方法签名（266 个，含 `app_common.go` 等既有文件）拆分前后 diff 为零；`frontend/wailsjs/go/main/App.d.ts` 未动（sha256 `997b7efe…0908e`，`export function` 245 个，与基线一致）。方法签名、接收者、绑定名一字未动，重新生成后 diff 为零是其直接推论；最终仍需在有 wails 的机器上跑一次生成复核。

### R-20a 拆 backend/data/fund_data_api.go（2,324 行、45 个函数 → 核心文件 + 4 个域文件）—— 已完成

- 拆分结果（原文件 2,324 行 → 核心文件 831 行，含全部 struct/var 定义）：
  - `fund_data_api.go` 831 行，14 个函数：`NewFundApi`、自选基金读写（`GetFundList`、`searchFundOnline`、`GetFollowedFund`、`GetFollowedFundPaged`、`FollowFund`、`UnFollowFund`、`AllFund`）、自选净值刷新（`batchCrawlFundData`、`parseSinajsBatchResponse`、`computeFundRates`、`mergeFundBasicDefaults`）+ 2 个 `TableName()`。
  - `fund_data_info.go` 399 行（8 个，基金信息/基础爬取）、`fund_data_netvalue.go` 428 行（11 个，基金净值/历史净值/场内判断）、`fund_data_ranking.go` 230 行（3 个，基金排行/搜索）、`fund_data_holdings.go` 495 行（9 个，基金持仓/行情补全）。
  - 只拆方法：全部 struct 定义（`FollowedFund`、`FundBasic`、`FundNetUnitValue`、`FundHistoryNetValue`、`FundHoldingStock`、`FundRankingItem` 等）与两个 `TableName()`、两个 `var` 正则块留在原文件不动。
  - 文件名说明：设计称第二域为“基金 K 线”，实际内容是净值/估值/历史净值（真正的 K 线逻辑在既有 `fund_kline_api.go` 的 `FundKLineApi` 内），故命名为 `fund_data_netvalue.go` 以免与 `fund_kline_api.go` 混淆；其余三域按设计命名（加 `fund_data_` 前缀）。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包全过；`grep -c '^func '` 各文件之和 = 45（14+8+11+3+9），与拆分前一致。

### R-20b 拆 backend/data/settings_api.go（1,608 行、53 个函数 → 核心文件 + 3 个域文件）—— 已完成

- 拆分结果（原文件 1,608 行 → 核心文件 396 行，含全部 struct/const 定义）：
  - `settings_api.go` 396 行，10 个函数：模型与配置类型方法（2 个 `TableName()`、`SetDefaults`、`UnmarshalJSON`、`fieldSubmitted`、`GetAIConfigThinking`、`GetMcpTimeout`）+ `NewSettingsApi` + 设置读取（`GetSettingConfig`、`GetMaskedSettingConfig`）。
  - `settings_config.go` 346 行（2 个，设置读写域）：`UpdateConfig`（314 行，含工具预算 `ToolRunBudget`/`ToolDailyBudget` 字段处理——预算逻辑嵌在写入路径内，故不单独成文件）与 `Export`（配置导出）。
  - `settings_aiconfig.go` 329 行（7 个，模型配置域）：`GetAiConfigTemplates`、`validateAIConfig`、`MaskSecret`、`MaskWebhook`、`IsMaskedSecret`、`resolveDefaultAiConfigID`、`updateAiConfigsTx`。
  - `settings_backup.go` 605 行（34 个，备份/清理域）：清理、备份/恢复、导入导出 CSV 等全部备份相关函数。
  - 只拆方法：全部 struct 定义（`Settings`、`AIConfig`、`SettingConfig`、`SettingsApi`、`AiConfigTemplate`）与两个 `TableName()`、两组 const（`Backup*`、`Cleanup*`）留在原文件不动。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包全过；`grep -c '^func '` 各文件之和 = 53（10+2+7+34），与拆分前一致。

### R-20c 拆 backend/data/market_news_api.go（1,564 行、40 个函数 → 核心文件 + 5 个域文件）—— 已完成

- 拆分结果（原文件 1,564 行 → 核心文件 13 行，仅 `MarketNewsApi` struct + `NewMarketNewsApi`）：
  - `market_news_list.go` 391 行（6 个，资讯列表：电报列表/新电报/按源列表/新浪新闻/24 小时列表/新闻数据查询）
  - `market_news_flash.go` 160 行（4 个，快讯：TradingView 新闻及详情、路透、财联社）
  - `market_news_sentiment.go` 536 行（16 个，舆情：研报/公告/字典码/券商观点/互动问答/热门股/热点/话题/龙虎榜/涨停热门 + 行业排行/资金排行/资金趋势）
  - `market_news_calendar.go` 252 行（7 个，日历：投资日历、财联社日历、GDP/CPI/PPI/PMI/行业报告信息）
  - `market_news_indexes.go` 294 行（6 个，指数：全球指数及可读化、指数缓存读写）
  - 与设计出入：设计称 4 个域（资讯列表/舆情/日历/快讯），6 个全球指数及缓存函数无处安放（既非快讯也非舆情），故单独立为 `market_news_indexes.go`，避免舆情域膨胀到 800 行以上。
  - 只搬家不改逻辑：R-14 遗留的 `EMDictCode` 缓存读取处裸 `json.Unmarshal` 原样保留在 `market_news_sentiment.go` 内，未顺手改动。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包全过；`grep -c '^func '` 各文件之和 = 40（1+6+4+16+7+6），与拆分前一致。

### R-20d 拆 backend/data/tdx_kline_api.go（1,558 行、44 个函数 → 核心文件 + 3 个域文件）—— 已完成

- 拆分结果（原文件 1,558 行 → 核心文件 354 行）：
  - `tdx_kline_api.go` 354 行，17 个函数：两个 `init()`（其一为 gotdx 日志屏蔽，其二为 `_ = time.DateTime` 空操作，均留原文件）+ `TdxKLineApi` 客户端管理（`NewTdxKLineApi`、`newClient`/`newMACClient`/`newMACExClient`、`ensure*`/`reconnect*`、`tdxRetryDelay`）+ 市场映射 helper（`tdxMarketFromStockCode`、`TdxMarketFromStockCode`、`macExMarketFromStockCode`、`hkCategoryFromCode`）。
  - `tdx_kline.go` 546 行（13 个，K 线查询：集合竞价、普通/MAC/扩展 K 线及全部转换 helper）。
  - `tdx_finance.go` 34 行（3 个，财务：`GetFinanceInfo`、`GetXDXRInfo`、`tdxDateToString`，三者均为小函数）。
  - `tdx_company.go` 677 行（11 个，公司信息：F10 综合/分类列表/分类内容、所属板块、全市场股票列表及入库同步、港美股列表及入库同步）。
  - 只拆方法：全部 struct 定义（`TdxCallAuctionData`、`TdxCompanyInfoSection`、`TdxFinanceInfo`、`TdxXDXRItem`、`TdxCompanyInfoBundle`、`TdxCompanyCategory`、`MACBelongBoardItem`、`TdxStockBasic`、`TdxExStockBasic`）、单例 `var` 块与港美股分类 const 留在原文件不动。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包全过；`grep -c '^func '` 各文件之和 = 44（17+13+3+11），与拆分前一致。

### R-20 小结：4 个必拆文件已完成（R-20a～R-20d），可选 3 文件记入收尾清单

- 已完成：`fund_data_api.go`（2,324→831 行 + 4 域文件）、`settings_api.go`（1,608→396 行 + 3 域文件）、`market_news_api.go`（1,564→13 行 + 5 域文件）、`tdx_kline_api.go`（1,558→354 行 + 3 域文件）。每文件独立提交、独立验证，函数总数对照均与拆前一致。
- 未做（按方案“若时间允许同批拆，否则记入收尾清单”）：`backend/services/market_service.go`（1,283 行、130 个函数，多为小函数）、`backend/agent/market_lifecycle.go`（1,391 行、22 个函数，多为长函数）、`backend/agent/agent.go`（1,224 行、49 个函数）。三者仍超 1,000 行，记入收尾清单，不在本步扩大范围。
- 另：`backend/data/eastmoney_api.go`（1,350 行）仍超 1,000 行，但它在 R-25 搬家范围（整体进 `datasource`），不在 R-20 拆分范围；R-20 的“`backend/data` 再无超 1,000 行文件”预期待 R-25 后达成。

### R-21 main.go 收尾：AutoMigrate 收敛 + SeedDefaultSkills 确认 —— 已完成

- 删除根包 `main.AutoMigrate()`（62 行，39 个模型去重后），`main.go` 由 430 行 → 369 行，仅剩方案要求的 6 个函数：`main()`、`setupCrashOutput()`、`PanicHandler()`、`initGlobalStockIndexCacheTask()`、`updateBasicInfo()`、`checkDir()`。
- 调用点改调：`main()` 内 `AutoMigrate()` → `services.AutoMigrateAll()`；原 `AutoMigrate()` 末尾调用的 `initGlobalStockIndexCacheTask()` 改为在 `main()` 内直接调用（行为不变）；`testing_init_test.go` 的 `TestMain` 同样改调 `services.AutoMigrateAll()`（根包测试无计数断言受种子数据影响，实测通过）。
- 清单等价核对（脚本 `/tmp/r21check.py`，旧清单取自 HEAD 版 main.go）：旧 main 清单 39 个模型**全部**在 `services.AutoMigrateAll()` 的 50 个直接迁移项内（`main ⊆ services-direct`，差集为空，R-15 结论复核一致）；services 另多 11 个模型（`StockThesisLedger`/`StockVerdictHistory`/`KnowledgeCrystal`/`PostMortemAuditRecord`/`PaperPortfolio` 等 5 个纸面交易与自进化模型 + `IndustryNode`/`IndustryEdge` + `AutoAlphaProposal` 等），`db.AutoMigrate()` 的清单经 `AutoMigrateAll()` 第 5 步同样执行。
- `SeedDefaultSkills` 确认：R-11 已合并进 services（`//go:embed`），`main.go` 内已无任何 seed 逻辑；`AutoMigrateAll()` 第 6 步即调用它。
- 空库冒烟（临时程序 `/tmp/r21smoke/main.go`，跑完即删，未提交）：临时空库上 `db.Init` + `services.AutoMigrateAll()`，54 张表逐一核对（旧 39 + services 新增 11 + db 独有 4）全部存在，缺失 0；技能种子数为 2，与预期一致。**SMOKE PASS**。
- 已知的、设计认可的收敛差异（记入，非回退理由）：(1) 原 `main.AutoMigrate()` 带 30 秒超时保护（超时则 Fatal），`services.AutoMigrateAll()` 为同步执行、无超时保护——与 server 路径（`cmd/server/main.go:209` 早已直接调用它）一致；(2) 桌面启动路径现在也会执行 `AutoMigrateAll()` 的第 7–14 步幂等种子/自愈（默认 AI 配置、基准自选股及行情回填、认知账本洗洁、出厂基线备份等）——server 路径在同一共享库上早已执行这些步骤，库的终态不变，只是桌面启动时也会收敛一次。
- `db/chat_memory.go:106` 的清单无需改动：`db.AutoMigrate()` 已在 `AutoMigrateAll()` 第 5 步执行，且桌面与 server 两条路径均经 `db.Init` 先行执行同一份清单（R-15 已核查）。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration -count=1 ./...` 全量 **15 个包通过、0 个失败**（与 R-02 后基线一致）。

### R-22a 拆 StockLightweightKlineChart.vue（5,544 行 → 1,585 行）—— 已完成

- 拆分结果（原文件 5,544 行 → 1,585 行：script 768 行、template 449 行、style 367 行不变）：
  - `src/composables/useKlineIndicators.ts`（2,483 行）：49 个指标开关、指标序列缓存 `ind`、主/副图同步（`syncIndicators` 717 行 + `syncSubPaneIndicators` 896 行）、信号评估与汇总、系列清理 helper。49 个 `showXxx` ref 收敛为 `reactive` 的 `flags` 记录（key 与工具栏注册表一致）；闭包读取的 `chart`/`candleSeries`/`mergedRawRows` 改为 ctx 传入的 getter（组件仍持有图表与数据所有权）；`hoverRawRow`/`mergedRawRowsVersion` 以 ref 原样传入，保持 `indicatorSignalSummary` 的 computed 响应性；`disposeChart` 内的 `ind` 重置块（47 行）改为 `resetInd()`。
  - `src/composables/useKlineLongPosition.ts`（517 行）：多单标注全部状态与函数（价位线、图上点击取价、拖动改价、输入框双向同步 watchers）；`disposeChart` 内的 4 行拖动状态复位改为 `resetLongDragState()`；图表点击处理器读取的 `longSuppressChartClick` 以 `isChartClickSuppressed()` getter 暴露（`let` 非 ref，不可直接解构）。
  - `src/composables/useKlineChip.ts`（126 行）：筹码开关、Canvas 绘制与悬停联动更新。
  - `src/components/kline/indicators/registry.ts`（105 行）：49 个指标的工具栏注册表（label/tip/分组/标题样式均由原模板逐字解析生成，非手写），模板侧边栏 5 个分组（趋势 12 / 波动 10 / 动量 13 / 量价 8 / 强度 6）改为 `v-for` 渲染；筹码按钮保留在强度分组末尾（`v-if="SHOW_CHIP_TOOLBAR_BUTTON"` 不变）。
  - 组件只留：props/emit、图表生命周期（ensureChart/disposeChart/轮询/历史分页/数据加载）、十字光标面板与 6 个纯 helper；文件名、props、emits、CSS 类名全部不变，16 个引用方零改动。
- 数量对照（脚本断言，全部通过）：49 个 flag 在 composable 内的 `flags.<key>` 引用次数与原 `showXxx.value` 次数逐项一致；模板 49 个 label、5 个分组标题、5 组标题样式逐字在 registry 中；NTooltip 49 → 1（v-for），NButton 55 → 7；`ind` 只在被搬代码与 dispose 重置块中使用（另 2 处为 `longLineByKind` 的误匹配）；60 个 calc 导入全部随指标代码搬走，组件侧 0 残留。
- 注意：原文件 script 内有两处 mid-script import（281 行 `indicatorTips`、351 行 `calcChipDistribution`），提取时已剥离归位（前者回组件顶部供模板使用，后者归 chip composable）；提取脚本见 `/tmp/r22a_extract.py`（stage1～stage4），原文件备份见 `/tmp/r22a_backup.vue`。
- 验证：`npx eslint` 5 个文件 0 errors、0 warnings；全量 `npm run lint` 0 errors、158 warnings（与基线一致，未增加）；`npm run build` 退出 0（1m22s），`StockLightweightKlineChart` chunk 120.84 kB（gzip 37.46 kB），`vendor-echarts` 706.14 kB；构建产物 grep 确认 `lw-kline`、`筹码`、`STrend`、`TRIX斜率`、`信号比`、`均幅`、`lw-kline-signal-summary`、`lw-kline-sidebar__section` 等关键标识均在。改前 chunk 对照（HEAD~1 原文件临时回退构建，构建后已恢复提交状态）：`StockLightweightKlineChart` chunk 134.95 kB（gzip 38.10）→ 120.84 kB（gzip 37.46），缩小 14.11 kB（模板 v-for 去重所致）；`vendor-echarts` 706.14 kB、`vendor-naive` 1,268.00 kB、`vendor-markdown` 1,204.17 kB 及两个 index chunk 与改前逐字节一致，无异常膨胀。

### R-22b 拆 stock.vue（2,970 行 → 1,752 行）—— 已完成

- 拆分结果（原文件 2,970 行 → 1,752 行：template/style 基本不变，script 大幅缩减）：
  - `src/composables/useStockTabs.ts`（153 行）：拖拽标签系统（drag refs + 9 个拖拽处理函数 + init/cleanup）；外部依赖仅 `groupList`/`message`。
  - `src/composables/useStockEcharts.ts`（600 行）：分时/日 K 的 ECharts 绘制（`showFsChart` 227 行、`handleKLine` 325 行、`calculateMA`、颜色常量 `upColor`/`downColor`、`handleFeishi`/`clearFeishi`）；`feishiInterval` 的声明仍在组件（`onBeforeUnmount` 里也要清它），作 ctx ref 传入。
  - `src/composables/useStockAi.ts`（353 行）：AI 分析与报告导出（`triggerAiWatchlistScreening`、`aiCheckStock`/`aiReCheckStock`、`fetchActiveThesis`、`saveAsImage`/`saveAsMarkdown`/`saveAsWord`/`getHtml`）；`theme` 不需要（`saveAsImage` 读的是 `document.documentElement` 的 `theme-mode` 属性，旧分析把字符串里的 `theme` 误算为引用）。
  - `src/composables/useStockAlerts.ts`（189 行）：成本/报警（`priceLineAlertCache`、`checkPriceLineAlerts`、`updateCostPriceAndVolumeNew`、`saveTradingPriceToBackend`、`setStock`）；`monitor`/`fromEastMoneyCode` 作 ctx 函数传入（函数声明可提升）。
  - `src/components/FollowedStockGrid.vue`（24 行）：两 pane 的 StockCard 网格抽为子组件（`results`/`showSparkLine`/`groupId`/`openAiEnable`/`groupList`/`thesesMap` 六个 props，13 个 StockCard 事件用 `v-on="$attrs"` + `inheritAttrs: false` 转发；grid 属性与 `:thesis` 表达式按原文件逐字保留）；`frontend/components.d.ts` 由构建自动补注册一行。
  - 组件只留 4 个 composable 调用（252–255 行，紧接 `groupResults` 计算属性之后插入——全部 ctx 输入至此均已声明，`monitor`/`fromEastMoneyCode` 为提升的函数声明）：15 个函数被解构使用；`showFsChart` 只被 composable 内部的 `handleFeishi` 调用，组件不解构。
- 关键纠正（执行中发现，旧分析/方案前提有出入）：
  - 按首行正则算 span 会虚增：`handleTabDragEnd` 实际仅 13 行（非 181 行），`priceLineAlertCache` 仅为 `const … = new Map()` 一行；全部跨度改用花括号匹配重算，存入 `/tmp/r22b_spans.json`。
  - 提取脚本的依赖分析 bug：参数名全局遮蔽——`updateCostPriceAndVolumeNew` 的 `formModel` 参数曾遮蔽 `setStock` 用的顶层 `formModel` ref，导致 alerts composable 漏传 ctx；已改为“全局去重（任意 span 内定义即内部）+ 参数/局部仅遮蔽本函数”，重生成后其余 3 个 composable 字节一致，仅 alerts 补一行 `const formModel = ctx.formModel`。
  - `consultAgent` 仍需留在组件：保留函数 `callAgentForDeepStockAnalysis`（1991 行）也调用它，import 不删；`echarts` 命名空间 import 同理保留（681/684 行的保留清理函数仍用 `echarts.getInstanceByDom`）。
  - import 使用分析必须排除原 import 块本身（R-22a 的教训，初版误把 15 个已搬走的 wailsjs 函数算成“仍在使用”）；模板扫描不能用代码扫描器（会把属性值当字符串剥掉），改用正则取标识符，并剥离 HTML 注释（1458 行注释提到 `showFsChart`，曾误判为组件仍用它）。
  - 导入路径：组件在 `src/components/`，composable 在 `src/composables/`，须用 `../composables/…`（初版 `./composables/…` 导致 Vite 无法解析，构建失败一次）。
  - 唯一非逐字搬运处：`checkPriceLineAlerts` 内 9 行已停用的 `notify.info` 注释代码——构建前检查 `scripts/check_vue_imports.js` 不豁免注释，会把其中的 `h(NAvatar, …)` 误判为未导入渲染标识符而拦截构建（而 eslint 又不允许为注释导入 `NAvatar`）；已改为两行文字说明（原注释代码删除，行为零变化，信息保留）。连同说明文字里又出现 `h(NAvatar` 字样导致二次拦截的教训一并记录。
  - 子组件 props 初版用 TS `any` 类型引入 3 处新增 lint 警告，已改为与 StockCard 同款运行时 props 写法消除。
- 数量对照（脚本断言，全部通过）：两 pane 除 `sortedResults`/`groupResults`、`show-spark-line`/`group-id` 三处外逐字相同；被搬函数名在新文件中的残留为零（已解构的除外）；wailsjs App import 35 → 20（15 个已搬走的函数随 composable 走）；`StockCard`/`consultAgent`/`echarts` 的去留均按实测保留/删除。
- 验证：`npx eslint` 6 个文件 0 errors；全量 `npm run lint` 0 errors、158 warnings（与基线一致，未增加）；`npm run build` 退出 0（1m17s），三项 prebuild 检查均通过；`stock.vue` 打进 `index-*.js`（195,838 B），构建产物 grep 确认 `开始AI分析`、`全部`、`分时`、`价位线`、`认知档案`、`盘前`、`AI分析报告`、`持仓`、`自选`、`StockCard`、`_gi` 等关键标识均在（`轻量K线` 0 命中属正常：该词只出现在源码注释中，注释不进产物）；`StockLightweightKlineChart` chunk 120.84 kB 与 R-22a 后一致。Go 侧无改动（本步纯前端），未跑 Go 验证。
- 提取脚本见 `/tmp/r22b_extract.py`（stageA～stageF：A tabs、B echarts、C AI、D alerts、E 子组件、F stock.vue 重写）。

### R-22c 拆 agent-chat.vue（3,695 行 → 2,943 行）—— 已完成

- 拆分结果（原文件 3,695 行 → 2,943 行：模板 773 行、样式 1,282 行逐字不动，只动 script 1,638 行 → 约 890 行）：
  - `src/composables/useAgentChatSessions.ts`（121 行）：会话持久化域——`ChatSession` 接口（移至模块顶层 export）、`STORAGE_KEY`/`ACTIVE_SESSION_KEY`、sessions/currentSessionId、`messages` 计算属性（含 setter 回写 + 自动保存）、`currentSessionTitle`、load/save/create/switch/delete/formatSessionTime。ctx：`showSessionDrawer`、`scrollToBottom`。
  - `src/composables/useAgentChatPipeline.ts`（442 行）：BUIS 五阶流水线域——`welcomeViewMode`、5 个标的输入 ref（`step4StockInput` 等）、`hotStocks`、`stockDict` + `initStockDict`（含 `onMounted` 注册）、标的填装弹窗（状态 + `setModalPK`/`openStockActionModal`/`submitModalStockAction`）、`pipelineContext`、`setPipelineStock`/`onPipelineStockCodeChange`/`onPipelineStockCodeEnter`、`extractStockFromAssistantText`、executeStep1～5、`viewVerdictStandards`、executeDirectDiag/Risk/PK、`applyPresetPK`、`handleQuickChipClick`、`pipelineNextStep`。ctx：`applyScenario`、`fillInput`、`inputValue`、`messages`、`perceptionSnapshot`、`getTrafficText`（态势感知定时器与生命周期仍留组件）。
  - `src/composables/useAgentChatThesis.ts`（300 行）：买方认知账本域——`lastConsultStock`（归 thesis 所有，组件的 `handleAgentConsult` 用解构出的 ref）、`extractThesisFromText`（147 行）、`handleSaveThesisFromMsg`、`handleExportImage`。ctx：`messages`、`notifyWarning`/`notifySuccess`/`notifyError`；`dialog` 与 `aiConfigStore` 在 composable 内自取（同属 setup 上下文）。
  - 组件只留 3 个 composable 调用（插在 `getTrafficText` 之后、`quickChips` 之前——该位置之后全部 ctx 输入均已声明，`applyScenario`/`fillInput`/`scrollToBottom`/`notify*` 为提升的函数声明，调用时可用）；wailsjs App import 8 → 5（`SaveStockThesis`/`SaveImage`/`GetStockList` 随 composable 走）；`reactive` 与 `h` 的 import 随之删除（仅被搬代码使用）。
- 关键纠正（执行中发现）：
  - `export interface ChatSession` 不能放在函数体内——初版构建失败一次，已移至 sessions composable 模块顶层。
  - `step4StockInput` 无模板使用、只在 pipeline composable 内部用——组件解构多余，eslint 报 unused，已从解构中删除（仍由 pipeline composable 持有并返回清单之外）。
  - `genMsgId` 仍被保留的 `handleSend` 使用（1720/1729 行），组件与 sessions composable 各自 import，初版断言误判已修正。
  - 提取脚本的依赖分析沿用 R-22b 的 `code_idents`（跳过字符串/模板字面量/注释）与“import 使用面排除原 import 块、模板用正则取标识符并剥离 HTML 注释”的做法；composable 体逐字搬运不重缩进（模板字面量内的提示词前导空白零变化）。
- 数量对照（脚本断言 + 人工 diff，全部通过）：模板与样式与原文件逐字节一致；被搬定义在组件内零残留（解构调用除外）；`step4StockInput` 等内部 ref 去留均按实测。
- 验证：`npx eslint` 4 个文件 0 errors、17 warnings（全部是被搬代码自带的 `no-explicit-any`，未新增）；全量 `npm run lint` 0 errors、158 warnings（与基线一致）；`npm run build` 退出 0（1m7s），`agent-chat-*.js` chunk 89,493 B，构建产物 grep 确认 `BUIS QUANTITATIVE INTELLIGENCE LADDER`、`进入 Step 3 · 运行多因子放量突破量化选股`、`存入买方认知账本`、`新对话`、`贵州茅台`、`5-Verdict` 均在；Go 侧无改动，`go build/vet` 通过、`gofmt -l` 0 文件、check-deps.sh 通过。
- 提取脚本见 `/tmp/r22c_extract.py`（stage1 写 3 个 composable、stage2 重写组件），原文件备份见 `/tmp/r22c_backup.vue`。

### R-22d 合并两个资金流图表为 components/market/FundFlowChart.vue —— 已完成

- 合并结果（两文件 690 + 670 行 → 一个文件 705 行，省约 655 行重复代码）：
  - 以 `bkFundFlowChart.vue`（超集，含 `analyzeSector`）为底，新建 `components/market/FundFlowChart.vue`，`mode: 'bk' | 'concept'` prop（validator 限两个值）区分；`kindLabel` computed（板块/概念）驱动全部文案（图表标题、添加占位符、表头、注释），`flowApi` computed 按 mode 分发三组后端 Api（`GetBKFundFlowListByDate`/`GetBKFundFlowTopListByDate`/`GetAllBKCodes` vs `GetConceptFundFlowListByDate`/`GetConceptFundFlowTopListByDate`/`GetAllConceptCodes`）；`BKCodeItem`/`allBKCodes` 改名 `CodeItem`/`allCodes`。
  - 唯一行为差异保留：`analyzeSector`（`show-market-intelligence-serenity` 事件 + 自定义事件）与「🔬 拆解」按钮只在 bk 模式出现（模板 `v-if="mode === 'bk'"` / `v-else` 纯标签），concept 模式渲染结果与旧文件一致。
  - `market.vue`：两个 import 合为一个 `FundFlowChart`，两处使用分别加 `mode="bk"` / `mode="concept"`，其余 props（`darkTheme`、`chartHeight`）不变；删除旧两文件；`frontend/components.d.ts` 由构建自动更新（2 删 1 增）。
- 关键纠正（执行中发现）：新文件深了一层，三个相对 import 须加一级 `../`（wailsjs 两处、`../utils/echarts` 一处）——初版构建失败一次才发现，eslint 不校验 import 路径，下同类搬家步骤注意先核对相对路径。
- 数量对照：两文件 diff 共 62 行，全部差异点（import、类型名、变量名、3 处调用点、5 处注释、标题、占位符、HTML 注释、2 处表头、analyzeSector 9 行、2 处按钮块）逐项映射进合并文件，无遗漏；旧组件名全仓 0 残留。
- 验证：`npx eslint` 新文件 0 errors、29 warnings（两文件各 29 → 合并后 29）；全量 `npm run lint` 0 errors、129 warnings（基线 158 − 29，正好是合并省掉的一份）；`npm run build` 退出 0（1m3s），产物 market chunk 含两组 Api 名、`资金流向 - 多`动态标题与 `show-market-intelligence-serenity`；Go 侧无改动，`go build/vet` 通过、`gofmt -l` 0 文件、check-deps.sh 通过。
- 合并脚本见 `/tmp/r22d_merge.py`，旧文件备份见 `/tmp/r22d_bk_backup.vue`、`/tmp/r22d_concept_backup.vue`。

### R-23 建 backend/datasource 空包 + 依赖护栏升级 —— 已完成

- 新建 `backend/datasource/doc.go`（包注释写明“纯 vendor 客户端，只依赖基础包”）；`scripts/check-deps.sh` 的 datasource 规则（只有 data 能 import 它、datasource 是叶子、交付层不得直引）已在 R-01 预埋，本次核查无新增规则项，空跑通过。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go list ./backend/datasource/` 正常。

### R-24a 搬 httpclient.go 进 datasource —— 已完成

- `git mv backend/data/httpclient.go → backend/datasource/httpclient.go` + 改包名；data 侧留薄转发文件 `backend/data/httpclient.go`：`SharedHTTPClient`（同一指针）、`ConfigureFromSettings`（`SettingConfig` 是 data 类型，datasource 不能引用）、`CreateHTTPClientWithTimeout`、`parseProxyURL`（datasource 侧导出为 `ParseProxyURL`，`openai_tools.go:901` 的调用点不动）。
- `market_news_flash/sentiment/calendar.go` 19 处 `FirefoxUserAgent` 改调 `datasource.FirefoxUserAgent`；`CreateDownloadClient` 零外部调用点，不留转发。
- 外部包（tools/services/根包/cmd）调用点零改动。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data、agent/tools、services、根包全过。

### R-24b 搬 akshare_api.go 进 datasource —— 已完成

- `git mv → backend/datasource/akshare.go` + 改包名；`NewAkshareClient` 改为接收 baseURL 参数（datasource 不读设置表），data 侧留同名转发函数（DB 设置 → 环境变量/默认值解析原样保留），`app_settings.go:58` 的 Ping 调用点零改动。
- 12 个零引用方法（R-05 跳过未删）随文件整体搬家，`Call()` 分发保留。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包与根包全过。

### R-24c 搬 baostock_api.go 进 datasource —— 已完成

- `git mv → backend/datasource/baostock.go` + 改包名；`NewBaostockApi` 改为接收 baseURL 参数（含 115.190.72.22 快速失败的 2 秒超时逻辑），data 侧留同名转发函数，`trading_day.go` 的调用点零改动。
- `normalizeBaostockCode` 导出为 `NormalizeBaostockCode`（data 包的 `TestNormalizeBaostockCode` 改调 `datasource.NormalizeBaostockCode`，断言一字未改）——初版构建失败才发现测试引用了未导出 helper，下同类步骤先 grep 测试引用。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包与 agent 包全过。

### R-24d 搬 tickflow_api.go 进 datasource —— 已完成

- `git mv → backend/datasource/tickflow.go` + 改包名；`NewTickFlowClient` 改为接收 apiKey 参数（datasource 不读设置表；`SharedHTTPClient` 用 datasource 自身的同一指针），data 侧留同名转发函数（从设置读密钥），agent/tools 的 5 处 `data.NewTickFlowClient()` 调用点零改动。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包与 agent/tools 包全过。

### R-24e 搬 trading_day.go 进 datasource —— 已完成

- `git mv → backend/datasource/trading_day.go` + 改包名；`InitTradingCalendar` 改为依赖注入 `InitTradingCalendar(adataDays, baostockDays)`——datasource 不读设置表、不构造 AData/Baostock 客户端；data 侧留同名包装函数（解析服务地址、跳过已知离线主机 115.190.72.22、注入两个取数闭包），`main.go`/`app_settings.go` 的调用点零改动。
- `IsTradingDay`/`IsTodayTradingDay`/`GetLatestTradingDay`/`GetTradingDaysBetween`/`GetNextTradingDay` 在 data 侧留转发（agent/tools/根包不得直接 import datasource，规则 3），全部外部调用点零改动；`backend/data/trading_day_test.go` 的测试调转发函数，断言未改。
- 与方案出入：方案称 trading_day.go 为“交易日历纯函数”，实地核查 `InitTradingCalendar` 并不纯（读设置 + AData/Baostock 两路取数），故用依赖注入而非整体直搬；纯查询函数与缓存仍完整在 datasource 内。
- 验证：`go build/vet` 通过；`gofmt -l` 0 文件；check-deps.sh 通过；`go test -skip integration` data 包、agent/tools 包与根包全过。
