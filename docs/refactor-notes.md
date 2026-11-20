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
