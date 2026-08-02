# blades-stock — Agent instructions

## Build & Verify
- `go build ./...` — Go-only compile check (fastest)
- `make lint` — `go vet` + `staticcheck` + `golangci-lint`
- `make test` — unit tests only (`-run "^Test" -skip "integration"`)
- `.\build.ps1 frontend` — build Vue 3 frontend only
- `.\build.ps1 go` — build Go backend binary
- `.\build.ps1 wails` — full Wails desktop build (single entry, auto-checkpoints DB + copies to build/bin/)
- `.\build.ps1 dev` — Wails dev mode with hot-reload (Ctrl+C to exit)
- `go run ./cmd/checkpoint_db/ "data/stock.db"` — checkpoint WAL into main DB before copying to build/bin/
- `go test -run TestName ./backend/agent/` — focused single test
- Wails flags `-skipbindings -skipembedcreate` required for Go 1.26.x compatibility (handled automatically in build.ps1)

> Legacy build scripts (`*.bat`, `clean_build*.ps1`, `do_build.ps1`) remain for reference but `.\build.ps1` is the unified entry point.

## Architecture
- **Wails v2** desktop app: Go backend + Vue 3 frontend (Naive UI + Pinia + Vite 7)
- **Module**: `blades-stock`, Go 1.26, Wails v2.12.0
- **AI framework**: `github.com/go-kratos/blades` v0.3.0 (NOT Eino)
- **Entrypoint**: `main.go` → `NewApp()` → `wails.Run()` with `App` bound to frontend
- **Vendored blades** in `vendor-blades/` — Wails `packages.Load` fails on it (Go 1.26.x regression); use `-skipbindings -skipembedcreate`

## Agent Pipeline (backend/agent/)
1. `ChatWithContext()` (`agent_api.go:80`) receives question
2. `buildSkillPrompt()` (`agent.go:223`) loads matched Skills from DB by keyword
3. `GetStockAiAgent()` creates `blades.NewAgent()` with tools filtered by `ClassifyQuestion()` → `FilterToolsByGroups()`
4. `ReasoningMiddleware` appends thinking framework (react or plan_execute)
5. Agent streams response back to frontend via channel

## AI Config (chat_model_factory.go)
Supports: OpenAI-compatible, Anthropic, Gemini, Ollama, DeepSeek, VolcArk, DashScope, OpenRouter.
Detected by baseURL pattern matching in `detectChatModelProvider()`.

## Tools (backend/agent/tools/)
- 9 `ToolGroup` categories: base, stock_analysis, market, screening, money_flow, news_research, ai_analysis, operations, project
- `ClassifyQuestion()` maps question keywords → tool groups; base always included
- `FilterToolsByGroups()` curates tools per group; groups not in map are excluded
- All tools in `tool_groups.go:23-156` (name → Group mapping)
- `GetAllDataTools()` in `data_tools_wrapper.go` returns all ~30 data tools

## Skills (DB-driven)
- Stored in `models.Skill` table, inserted via `cmd/insert_*_skill/main.go`
- Loaded at runtime by `buildSkillPrompt()` — DB-only changes need app restart only
- Current skills: ETF轮动多Agent分析 (ID=9), Serenity式产业分析 (ID=10)
- Skill keywords trigger injection; non-matching questions skip skill prompt

## Memory / Reflection / Evolution
- `MemoryManager` (working + long-term facts), `Evaluator` (score + LLM reflection), `Evolver` (prompt optimization + skill extraction)
- DB tables: `agent_memories`, `agent_sessions`, `agent_traces`, `agent_tool_ranks`
- Auto-migrated via `init()` in model files + `main.go:AutoMigrate()`

## Data Sources
- **East Money** (push2his/push2.eastmoney.com) — K-line, real-time quotes, money flow
- **TDX** (TdxKLineApi) — stock basic info sync, extended market data
- **CLS / Sina / TradingView** — news feeds
- **SearXNG** (search.whohai.cfd) — web search
- **Tushare** — historical (optional API key)
- Static files: `backend/data/stock_basic.json`, `build/*.json` from remote server `http://8.134.249.145:18080` (configurable via `BLADES_STOCK_STATIC_DATA_URL`)

### Configurable service URLs (backend/data/service_config.go)
- `BLADES_STOCK_STATIC_DATA_URL` — static stock basic info JSON (default `http://8.134.249.145:18080`)
- `BLADES_STOCK_ADATA_URL` — AData service (default `http://115.190.72.22:8000`)
- `BLADES_STOCK_AKSHARE_URL` — AKShare service (default `http://115.190.72.22:8089`)
- `BLADES_STOCK_BAOSTOCK_URL` — Baostock service (default `http://115.190.72.22:8090`)
- Precedence: env var → DB settings field → default constant. Empty DB fields (`akshare_service_url`/`adata_service_url`) defer to env/default.

## Frontend (Vue 3)
- `frontend/src/` — main app code
- `frontend/package.json` scripts: `npm run dev` (Vite), `npm run build`
- Wails bindings generated to `frontend/wailsjs/`

## Testing Conventions
- Unit tests: no DB, network, or external AI (pure local code)
- Integration tests: tagged `integration`, require Chrome (`make test-integration`)
- Coverage: `go test -coverprofile=coverage.out ./...`

## Gotchas
- `KLineData.Close` is `string` — must `strconv.ParseFloat()` before arithmetic
- `GetStockMoneyData()` returns `models.StockMoneyDataResp` (value, not pointer) — check `.Data` length, not nil
- `StockMoneyDataDiff` fields: `F12` (code), `F14` (name), `F62` (net inflow ¥)
- Wails build fails if `go.sum` is stale — run `go mod tidy` first
- Source changes → `.\build.ps1 wails` to apply; frontend changes → `.\build.ps1 dev` or `npm run build`
- `wails dev -skipbindings -skipembedcreate` and `wails build -skipbindings -skipembedcreate` required for Go 1.26.x (blades "without types" bug)
- Chinese chars in Go source must be UTF-8; use `\uXXXX` escapes if encoding corrupted

## MCP Server Setup (Tushare / Wudao)
- `go run ./cmd/setup_mcp/` — inserts/updates MCP server entries in DB for Tushare (smart-financial-mcp) and Wudao (悟道数据)
- Requires env vars: `TUSHARE_TOKEN` and `WUDAO_BEARER_TOKEN` (set before running)
- After setup, restart the app and test connections from 研究中心 → MCP服务

### Tushare MCP allowlist
- `backend/data/tushare_allowlist.go` — curated allowlist of 124 Tushare API fields required for stock market data (daily, weekly, monthly, adj_factor, money_flow, income, balance_sheet, cash_flow)
- Combines unified field names + Tushare field names for smooth API compatibility

### Probe commands (dev tools, build-ignored)
- `cmd/probe_tushare_perms/` — issues `ak_times=1` probe to check which Tushare API fields the current token can access (must set `TUSHARE_TOKEN`)
- `cmd/diag_skill_mcp/`, `cmd/diag_mcp_schema/` — MCP diagnostic helpers, all tagged `//go:build ignore`

## Repo Hygiene / Safety
- **Never** commit `.venv/`, `*.exe`, `.graph/`, or build artifacts (`stock_basic.json`, `.md5` checkpoints)
- `.env` files are gitignored — store API tokens/secrets there, not in Go source
- `frontend/package.json.md5` is untracked (regenerated by `build.ps1` for build cache check)
- `.graph/` is a local code-graph knowledge base, not committed
