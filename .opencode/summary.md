## Goal
- Complete Serenity skill enforcement, fix chat/settings bugs, optimize agent stock recommendation precision, integrate FinGenius-inspired analysis tools, and rename project from go-stock to blades-stock.

## Constraints & Preferences
- Go 1.26.4 + Wails CLI v2.12.0; `GOTOOLCHAIN=go1.26.4` required for binding generation and build.
- Vue 3 + Naive UI + Pinia frontend.
- No vendor directory (causes Wails "package without types" for `blades`).
- Rename `go-stock` → `blades-stock`: external URLs (`http://8.134.249.145:18080/go-stock/...`, `http://go-stock.sparkmemory.top:...`, `github.com/ArvinLovegood/go-stock`) are preserved; only local identifiers/display names change.

## Progress
### Done
- **Serenity skill inserted into DB** (ID=3, keywords: `产业分析,产业链,稀缺性,瓶颈,行业研究,赛道分析,上下游,供需格局,渗透率,国产替代,市场份额,竞争格局,华阳集团,投资机会,细分龙头`).
- **Skill system prompt rewritten** — shorter (1517 B), strong imperative language, forced Serenity 5‑step output template.
- **Skill prompt moved to front of sysPrompt** — `agent_api.go:142-149`.
- **buildSkillPrompt() format strengthened** — delimiter `===== 【专业技能 - 必须严格遵守】 =====` with explicit rule/example labels.
- **Chat output duplication fixed** — `agent_api.go:262`: skip final message when `text == fullResponse.String()`.
- **`agent-DONE` double‑emit fixed** — `app_common.go:205-208`: removed redundant emit.
- **`agent-DONE` visible in chat fixed** — `agent-chat.vue:402`: content guard skips `agent-DONE` from display.
- **Settings page prompt‑management dead code removed**.
- **Settings page URL junk fixed** — `App.vue:815`: removed `query: { name:"设置" }`.
- **`ResizeObserver loop completed` warning suppressed** — `main.js` + `App.vue:967` + `stock.vue:1970`.
- **ETF轮动 skill created and inserted** — DB ID=4, 7128 B system prompt, 36 trigger keywords.
- **Stock recommendation screening methodology added** — `agent.go:167-199` plan_execute framework + `openai_stream.go` system prompt + `agent.go` personalityPrompt.
- **FinGenius reference analysis completed** — studied `HuaYaoAI/FinGenius` (6‑Agent architecture).
- **Three FinGenius‑inspired analysis tools created** (blades toolchain) — `backend/agent/tools/hot_money_chip_tools.go`: `AnalyzeHotMoneyActivity`, `AnalyzeChipConcentration`, `AnalyzeCapitalGame`.
- **New tools registered in blade agent toolchain** — `data_tools_wrapper.go:4893` + `tool_groups.go` keywords.
- **Three tools registered in legacy `data.Tools()` path** — `backend/data/tool_hot_money_chip.go`: handlers + helper functions. `tools.go`: schemas + `dataToolGroupMap` entries + trigger keywords.
- **Unit tests for legacy tool handlers created** — `backend/data/tool_hot_money_chip_test.go`: 15 tests (helpers + empty‑code handlers + registration), all passing.
- **Pushed to `huch16/blades-stock` remote** — commit `38d6ca1` (31 files, +2235/−169 lines).
- **Project rename `go-stock` → `blades-stock` (COMPLETE)**:
  - `go.mod` module path changed
  - All 129+ `.go` file import paths replaced
  - All Go string literals (display names, temp dirs, test data, descriptions) updated
  - All Vue/JS frontend display strings updated (`agent-chat.vue`, `FloatingAgentAssistant.vue`, `FloatingAiAssistant.vue`, `settings.vue`, `stock.vue`, `App.vue`)
  - `frontend/package.json` name updated
  - `wails.json` name, outputfilename, productName updated
  - External URLs (`github.com/ArvinLovegood/go-stock`, `http://go-stock.sparkmemory.top:...`, `http://8.134.249.145:18080/go-stock/...`) preserved
  - `go build ./...` verified ✅

### In Progress
*(none)*

### Blocked
*(none)*

## Key Decisions
- Serenity skill instructions moved to **front** of system prompt to increase model compliance.
- Duplication fix uses `fullResponse.String()` comparison rather than message `Status` field.
- Settings prompt‑management modal was **dead UI** — removed entirely.
- ETF轮动 skill stored as **text‑only system prompt** for LLM to role‑play multi‑agent pipeline.
- Legacy tool handlers in `data/` package **duplicate helper functions** from `agent/tools/hot_money_chip_tools.go` — cannot import from `agent/tools` due to `agent/tools ← data` import direction.
- Project rename preserves external URLs — only local module path, display names, and internal identifiers change.
- `data.source==="blades-stock"` used as color key in notification tags — must match the source identifier set by `dingding_api.go`.

## Next Steps
1. **Full `go test ./...`** to verify all tests pass after rename.
2. **`wails build -skipbindings`** to produce `build/bin/blades-stock.exe`.
3. **Restart app** and test ETF轮动 skill, recommendation precision, and new analysis tools.
4. **Delete `agent-chat_bk.vue` and `frontend/dist/`** (outdated generated artifacts) after verifying new build works.

## Critical Context
- `buildSkillPrompt()` queries `skills` table at runtime via `GetEnabledSkills()` — DB‑only changes require only app restart, not rebuild.
- Go source changes require `wails build -skipbindings` to take effect.
- `blades/contrib/openai@v0.3.0` `NewStreaming()` double‑yields delta chunks + final accumulated message by design.
- `GOTOOLCHAIN=go1.26.4` must be set before Wails build commands.
- ETF轮动 skill keyword "ETF" is very broad — may activate for any question containing "ETF".
- New FinGenius‑inspired tools rely on **东方财富F10 data APIs** — if APIs fail, tools report "无数据" not crash.
- After rename, binary output is `build/bin/blades-stock.exe`.

## Relevant Files
- `backend/data/tool_hot_money_chip.go`: **NEW** — legacy handlers for 3 analysis tools + helper functions
- `backend/data/tool_hot_money_chip_test.go`: **NEW** — 15 tests for helpers + handlers
- `backend/data/tools.go`: updated — tool schemas + `dataToolGroupMap` + trigger keywords
- `backend/agent/tools/hot_money_chip_tools.go`: blades‑toolchain versions of 3 analysis tools
- `backend/agent/tools/data_tools_wrapper.go:4893`: `GetHotMoneyChipTools()` appended
- `backend/agent/tools/tool_groups.go`: 3 tool names + trigger keywords
- `backend/agent/agent.go`: screening methodology + advanced tool usage guide + personality prompt
- `backend/agent/agent_api.go`: skill‑prompt moved to front; duplication skip
- `backend/data/openai_stream.go`: screening methodology added
- `cmd/insert_etf_skill/main.go` + `etf_prompt.txt`: ETF轮动 skill injector
- `cmd/insert_serenity_skill/main.go` + `serenity_prompt.txt`: Serenity skill injector
- `go.mod`: module path `blades-stock`
- `wails.json`: name/outputfilename `blades-stock`, productName `Blades Stock`
