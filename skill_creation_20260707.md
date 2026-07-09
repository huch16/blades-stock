# Skill 创建报告 — 2026-07-07

## 目标
为 blades-stock 项目创建 3 个新 Skill，填补现有 Agent 技能覆盖缺口。

## 背景
项目已有 3 个 Skill（ID=9/10/11）：Serenity产业分析、ETF轮动、项目交付工作流。
项目共有 **144 个工具**，分为 9 个工具组（base/stock_analysis/market/screening/money_flow/news_research/ai_analysis/operations/project）。

## 缺口分析
审查 `backend/agent/tools/` 下 29 个工具文件，识别出 3 个高价值缺口：
1. **持仓风险诊断** — 有游资追踪、筹码集中度、资金博弈、龙虎榜等工具，缺少系统化诊断框架
2. **市场全景看盘** — 有全球指数、北向资金、板块资金流、涨停梯队、异动扫描等工具，缺少看盘/复盘流程
3. **个股深度诊断** — 有 K线、财务、估值、股东、研报等丰富工具，缺少系统化个股诊断框架

## 创建的 3 个 Skill

| ID | 名称 | Category | Directive 长度 | 触发关键词数 |
|----|------|----------|---------------|-------------|
| 12 | 持仓风险诊断与仓位管理 | risk-management | 1659 chars | 20 |
| 13 | 市场全景看盘与复盘 | market-analysis | 2021 chars | 24 |
| 14 | 个股深度诊断与交易决策 | stock-analysis | 2088 chars | 16 |

### Skill 12: 持仓风险诊断与仓位管理
- **4阶段流程**：持仓扫描 → 风险信号扫描（游资/筹码/资金/涨停/公告 5维度）→ 5级风险评级 → 仓位调整建议
- **工具链**：AnalyzeHotMoneyActivity, AnalyzeChipConcentration, AnalyzeCapitalGame, GetStockMoneyData, GetUplimitLadder, SearchAnnouncement, SetTradingPrice 等

### Skill 13: 市场全景看盘与复盘
- **3场景流程**：盘前分析（全球+宏观+新闻+板块预判）/ 盘中看盘（实时行情+资金面+涨停监控+板块轮动）/ 盘后复盘（指数总结+涨停复盘+龙虎榜+资金流+回测检查）
- **工具链**：GlobalStockIndexesReadable, GetMarketData, GetADataNorthFlow, GetUplimitLadder, GetLongHuToday, GetIndustryMoneyRank, GetBacktestStats 等

### Skill 14: 个股深度诊断与交易决策
- **5维度诊断**：基本面（财务+公司）→ 技术面（K线+均线+回测）→ 资金面（主力+北向+游资+筹码）→ 估值（PE/PB分位+机构目标价+研报）→ 风险信号（公告+融资融券+大宗+红旗清单）
- **5星评级**：⭐⭐⭐⭐⭐ 强烈推荐 → ⭐ 回避
- **工具链**：GetStockFinancialInfo, GetStockKLine, GetStockMoneyData, AnalyzeHotMoneyActivity, GetStockValuationPercentile, GetStockResearchReport, SearchAnnouncement 等

## Skill Workshop Proposals
同时通过 `skill_workshop` 创建了 3 个 pending proposal：
- portfolio-risk-diagnosis-20260707-7d49d79070
- market-panorama-daily-20260707-862b447c8a
- stock-deep-diagnosis-20260707-88c7a3daf3

## 验证
- `go build ./...` ✅ 通过
- `go vet ./...` ✅ 通过
- 数据库 6 个 Skill 全部启用，Directive 字段已填充（1.6k-3.2k chars）
- 临时文件已清理（check_db.py, cmd/query_skills/, cmd/insert_skills/）

## 结论
3 个新 Skill 成功写入 blades-stock 数据库，填补了持仓风险管理、市场看盘复盘、个股深度诊断三个核心场景的框架缺口。Agent 可根据用户问题通过 bge-m3 语义匹配或关键词触发自动选择对应 Skill，按结构化流程调用 144 个已有工具。
