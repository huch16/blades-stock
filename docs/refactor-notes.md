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
