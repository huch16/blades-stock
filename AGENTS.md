# blades-stock — Agent instructions

## Build & Verify
- `go build ./...` — Go-only compile check (fastest)
- `make lint` — `go vet` + `staticcheck` + `golangci-lint`
- `make test` — unit tests only (`-run "^Test" -skip "integration"`)
- `make build` — `wails build -clean -skipbindings` (full desktop build)
- `go test -run TestName ./backend/agent/` — focused single test
- `GOTOOLCHAIN=go1.26.4` must be set before Wails builds

## Architecture
- **Wails v2** desktop app: Go backend + Vue 3 frontend (Naive UI + Pinia + Vite 7)
- **Module**: `blades-stock`, Go 1.26, Wails v2.12.0
- **AI framework**: `github.com/go-kratos/blades` v0.3.0 (NOT Eino)
- **Entrypoint**: `main.go` → `NewApp()` → `wails.Run()` with `App` bound to frontend
- **No vendor dir** — causes Wails "package without types" for blades

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
- Static files: `backend/data/stock_basic.json`, `build/*.json` from remote server `http://8.134.249.145:18080`

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
- Source changes → `wails build -skipbindings` to apply; frontend changes → `npm run build` or `wails dev`
- Chinese chars in Go source must be UTF-8; use `\uXXXX` escapes if encoding corrupted
