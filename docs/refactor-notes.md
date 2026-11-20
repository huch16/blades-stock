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
