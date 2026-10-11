# 工具注册表 (Tool Registry)

> **单一事实源**：本文件列举 blades-stock Agent 注入到 LLM 的全部工具，及其分组、用途、调用示例。
> 与代码 `backend/agent/tools/tool_groups.go` 保持一致；变更请同步更新两侧。

## 工具分组总览

| 分组 (Group) | 标识符 | 工具数 | 主要用途 | Token 上限 |
|-------------|--------|--------|----------|-----------|
| 基础 | `base` | 11 | 通用工具：股票代码、交易日、自选股等 | 全量 |
| 个股分析 | `stock_analysis` | ~50 | 单股行情、K线、财务、估值、筹码 | 有上限 |
| 市场行情 | `market` | ~25 | 大盘/板块/异动/宏观/全球指数 | 有上限 |
| 选股筛选 | `screening` | 8 | 条件选股、板块/ETF筛选、热门策略 | 有上限 |
| 资金流向 | `money_flow` | 5 | 主力/北向/板块资金流 | 有上限 |
| 资讯研报 | `news_research` | ~30 | 新闻、公告、研报、龙虎榜、涨停 | 有上限 |
| AI 分析 | `ai_analysis` | 8 | AI 诊股、回测、向量回测、AI 推荐 | 全量 |
| 操作执行 | `operation` | 7 | 钉钉推送、基金、价位置警、宏观 | 全量 |
| 项目工具 | `project` | 7 | 文件/目录/Git/学习项目/执行命令 | 全量 |

> 注：`WebSearch` 与 `FetchWebPage` 不在 `toolGroupMap` 中，默认对所有会话可用。

## 1. 基础 (base)

| 工具名 | 描述 | 触发关键词 |
|--------|------|------------|
| `QueryStockCodeInfo` | 查询股票代码 / 名称 / 拼音映射 | 代码、名称、查询 |
| `QueryBKDictInfo` | 查询板块（行业 / 概念）字典 | 板块字典 |
| `GetCurrentTime` | 获取本地时区当前时间 | 时间 |
| `GetFollowedStocks` | 列出用户自选股 | 自选股 |
| `GetHolidayInfo` | 查询某年节假日 | 节假日 |
| `GetHolidayYear` | 获取某年节假日数据 | 节假日年份 |
| `GetHolidayBatch` | 批量查询节假日 | 节假日批量 |
| `IsTradingDay` | 判断是否交易日 | 交易日 |
| `GetNextTradingDay` | 获取下一个交易日 | 下一交易日 |
| `CreateAiRecommendStocks` | 创建单条 AI 推荐记录 | 保存推荐 |
| `BatchCreateAiRecommendStocks` | 批量保存 AI 推荐 | 批量推荐 |

## 2. 个股分析 (stock_analysis)

| 工具名 | 描述 |
|--------|------|
| `GetStockInfo` | 个股实时行情（价格/涨跌幅/成交量） |
| `GetStockKLine` | 日/周/月 K 线（基础） |
| `GetEastMoneyKLine` | 东方财富 K 线（增强） |
| `GetEastMoneyKLineWithMA` | K 线含均线 |
| `GetStockMinuteData` | 分时数据 |
| `GetStockFinancialInfo` | 财务三表（资产负债/利润/现金流） |
| `GetStockLatestFinance` | 最新一期财务摘要 |
| `GetStockQtrMainFinance` | 单季度财务摘要 |
| `GetStockOrgPredict` | 机构一致预测 |
| `GetStockPredictSummary` | 预测摘要 |
| `GetStockValuationPercentile` | PE/PB 历史百分位 |
| `GetStockHolderNum` | 股东户数 |
| `GetStockHolderTrend` | 股东户数趋势 |
| `GetStockRZRQInfo` | 融资融券 |
| `GetStockMarginTrading` | 两融余额变化 |
| `GetStockBlockTrade` | 大宗交易 |
| `GetStockBillboard` | 龙虎榜详情 |
| `GetStockOperationDeptTrade` | 营业部成交 |
| `AnalyzeHotMoneyActivity` | 游资风格分析 |
| `AnalyzeChipConcentration` | 筹码集中度分析 |
| `AnalyzeCapitalGame` | 资金博弈分析 |
| `ComparableCompanyAnalysis` | 可比公司对标 |
| `IndustryResearch` | 行业研究 |
| `TrackingReport` | 个股/行业跟踪报告 |
| `FinanceDataQuery` | 金融数据查询 |
| `GetETFMomentumRanking` | ETF 21日加权动量排名 |
| `GetStockConceptInfo` | 所属概念 |
| `GetETFHistorical` | ETF 历史净值 |
| `GetOpenFundNetValue` | 开放式基金净值 |
| `GetADataKLine` | AData 代理 K 线 |

## 3. 市场行情 (market)

| 工具名 | 描述 |
|--------|------|
| `GetMarketData` | 大盘实时行情 |
| `GlobalStockIndexesReadable` | 全球指数 |
| `GetStockChanges` | 个股异动 |
| `GetStockChangeHistoryList` | 异动历史 |
| `GetDailyChangeStats` | 每日异动统计 |
| `GetChangeRank` | 异动排行 |
| `GetDailyDimensionStats` | 维度统计 |
| `GetTypeStatsByDate` | 类型统计 |
| `HotspotDiscovery` | 热点发现 |
| `GetWallstreetcnMarketReal` | 华尔街见闻实时市场 |
| `GetWallstreetcnKline` | 华尔街见闻 K 线 |
| `GetFuturesSpot` | 期货行情 |
| `GetETFSpot` | ETF 现货 |
| `GetBoardConcept` | 概念板块 |
| `GetBoardIndustry` | 行业板块 |
| `GetMacroCPI` | 宏观 CPI |
| `GetMacroGDP` | 宏观 GDP |
| `GetMacroPMI` | 宏观 PMI |
| `GetMacroMoneySupply` | 货币供应量 |
| `GetConvertibleBondSpot` | 可转债 |
| `GetADataMarketCurrent` | AData 大盘实时 |

## 4. 选股筛选 (screening)

| 工具名 | 描述 |
|--------|------|
| `FilterStocks` | 条件选股 |
| `SearchStockByIndicators` | 指标选股 |
| `SearchBk` | 板块搜索 |
| `SearchETF` | ETF 搜索 |
| `HotStrategyTable` | 热门策略 |
| `HotStockTable` | 热门股票 |
| `ScreenByStrategy` | 策略选股 |

## 5. 资金流向 (money_flow)

| 工具名 | 描述 |
|--------|------|
| `GetStockMoneyData` | 个股资金流 |
| `GetMutualTop10Deal` | 沪深港通前十大成交 |
| `GetStockHistoryMoneyData` | 历史资金流 |
| `GetIndustryMoneyRank` | 行业资金排行 |
| `GetADataNorthFlow` | 北向资金（AData 代理） |

## 6. 资讯研报 (news_research)

| 工具名 | 描述 |
|--------|------|
| `QueryStockNewsTool` | 个股新闻 |
| `FetchWebPage` | 抓取任意网页 |
| `GetNewsListData` | 新闻列表 |
| `GetStockResearchReport` | 个股研报 |
| `GetIndustryResearchReport` | 行业研报 |
| `GetSecuritiesCompanyOpinion` | 券商观点 |
| `StockNotice` | 个股公告 |
| `GetStockNotice` | 个股公告（v2） |
| `InteractiveAnswer` | 智能问答 |
| `GetInvestCalendar` | 投资日历 |
| `GetLongTigerList` | 龙虎榜 |
| `GetHotStockList` | 热门股 |
| `GetHotEventList` | 热门事件 |
| `GetUplimitLadder` | 涨停梯队 |
| `GetUplimitHotPlates` | 涨停热门板块 |
| `GetUplimitHotStocks` | 涨停热门股 |
| `GetUplimitExplodedStocks` | 炸板股 |
| `GetUplimitPlateStocks` | 涨停板块股 |
| `GetWallstreetcnLives` | 华尔街见闻 7x24 |
| `GetWallstreetcnCalendar` | 华尔街见闻日历 |
| `FinanceSearch` | 金融搜索 |
| `GetLongHuToday` | 今日龙虎榜 |

## 7. AI 分析 (ai_analysis)

| 工具名 | 描述 |
|--------|------|
| `FinancialQA` | 金融问答 |
| `AiRecommendStocks` | AI 推荐 |
| `GetAIAnalysisHistory` | AI 分析历史 |
| `GetAIAnalysisDetail` | AI 分析详情 |
| `GetAIAnalysisContent` | AI 分析内容 |
| `GetBacktestStats` | 回测统计 |
| `GetBacktestDetail` | 回测详情 |
| `RunBacktest` | 执行回测 |
| `VectorBacktest` | 单标的向量化回测 |
| `RunVectorizedBacktest` | 多标的 / 池向量化回测 |
| `DiagnoseStock` | 智能 3D 量化诊断（个股 / ETF） |
| `VectorGridSearch` | 参数网格搜索 |

## 8. 操作执行 (operations)

| 工具名 | 描述 |
|--------|------|
| `SetTradingPrice` | 设置价位置警 |
| `SendDingDingMessage` | 发送钉钉消息 |
| `SendToDingDing` | 同上（兼容名） |
| `SearchFund` | 搜索基金 |
| `GetFundInfo` | 基金详情 |
| `GetEconomicData` | 经济数据 |

## 9. 项目工具 (project)

| 工具名 | 描述 |
|--------|------|
| `ReadProjectFile` | 读取项目文件 |
| `ListProjectDir` | 列出目录 |
| `SearchProjectFiles` | 文件内搜索 |
| `GitOperation` | Git 命令 |
| `LearnProject` | 学习项目（生成技能） |
| `WriteProjectFile` | 写入文件 |
| `RunCommand` | 执行 shell 命令 |

## 跨组工具

| 工具名 | 描述 | 默认注入 |
|--------|------|----------|
| `WebSearch` | SearXNG 联网搜索 | ✅ 总是注入 |

## 工具注入策略

### 自动分类 (`ClassifyQuestion`)
- 关键词匹配 + 股票代码正则
- 命中关键词的工具组自动激活

### 分组白名单 (`coreToolByGroup`)
- 各组下"必注入"的核心工具
- LLM 容易高频调用的工具优先保留

### 分组上限 (`groupToolCaps`)
- 个别工具组（market / screening / news_research / ai_analysis / money_flow / operations）有 token 上限
- 超上限时按白名单 + 出现顺序补充

### 相关代码

- `backend/agent/tools/tool_groups.go` — 分组定义与映射
- `backend/agent/tools/tool_register.go` — 工具注册入口
- `backend/agent/tools/tool_groups_test.go` — 分组逻辑测试