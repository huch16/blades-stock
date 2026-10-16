package services

import (
	"os"

	"blades-stock/backend/agent"
	"blades-stock/backend/agent/evaluator"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

// AutoMigrateAll 执行全系统完整数据库表结构迁移与默认种子数据初始化
func AutoMigrateAll() {
	if db.Dao == nil {
		return
	}

	logger.SugaredLogger.Info("[Database] Starting full schema auto-migration...")

	// 1. 基础行情与系统设置
	db.Dao.AutoMigrate(&data.StockInfo{})
	db.Dao.AutoMigrate(&data.StockBasic{})
	db.Dao.AutoMigrate(&data.FollowedStock{})
	db.Dao.AutoMigrate(&data.IndexBasic{})
	db.Dao.AutoMigrate(&data.Settings{})
	db.Dao.AutoMigrate(&data.AIConfig{})
	db.Dao.AutoMigrate(&data.FollowedFund{})
	db.Dao.AutoMigrate(&data.FundBasic{})
	db.Dao.AutoMigrate(&data.TradingRecord{})
	db.Dao.AutoMigrate(&data.Group{})
	db.Dao.AutoMigrate(&data.GroupStock{})

	// 2. 资讯舆情与全景研判
	db.Dao.AutoMigrate(&models.AIResponseResult{})
	db.Dao.AutoMigrate(&models.StockInfoHK{})
	db.Dao.AutoMigrate(&models.StockInfoUS{})
	db.Dao.AutoMigrate(&models.Tags{})
	db.Dao.AutoMigrate(&models.Telegraph{})
	db.Dao.AutoMigrate(&models.TelegraphTags{})
	db.Dao.AutoMigrate(&models.LongTigerRankData{})
	db.Dao.AutoMigrate(&models.BKDict{})
	db.Dao.AutoMigrate(&models.WordAnalyze{})
	db.Dao.AutoMigrate(&models.SentimentResultAnalyze{})
	db.Dao.AutoMigrate(&models.AiRecommendStocks{})
	db.Dao.AutoMigrate(&models.AllStockInfo{})
	db.Dao.AutoMigrate(&models.CronTask{})
	db.Dao.AutoMigrate(&models.AiAssistantSession{})
	db.Dao.AutoMigrate(&models.GlobalStockIndex{})
	db.Dao.AutoMigrate(&models.CustomStrategy{})
	db.Dao.AutoMigrate(&models.BKFundFlow{})
	db.Dao.AutoMigrate(&models.ConceptFundFlow{})

	// 3. AI Agent 技能库、上下文记忆与 MCP 工具
	db.Dao.AutoMigrate(&models.MCPServer{})
	db.Dao.AutoMigrate(&models.MCPServerTool{})
	db.Dao.AutoMigrate(&models.Skill{})
	db.Dao.AutoMigrate(&models.ProjectAnalysis{})
	db.Dao.AutoMigrate(&models.CodeChunk{})
	db.Dao.AutoMigrate(&models.BacktestResult{})
	db.Dao.AutoMigrate(&models.AuditLog{})
	db.Dao.AutoMigrate(&agent.PlanTrace{})
	db.Dao.AutoMigrate(&evaluator.EvaluatorReport{})

	// 4. 底层 DB 核心记忆与表迁移
	db.AutoMigrate()

	// 5. 播种系统默认精选 Skill (ETF 动量轮动 / Serenity 产业链分析)
	SeedDefaultSkills()

	logger.SugaredLogger.Info("[Database] All tables migrated and initialized successfully.")
}

// SeedDefaultSkills 初始化默认的专家技能
func SeedDefaultSkills() {
	if db.Dao == nil {
		return
	}
	var count int64
	db.Dao.Model(&models.Skill{}).Count(&count)
	if count > 0 {
		return
	}

	etfPrompt := defaultETFDirective
	if b, err := os.ReadFile("cmd/insert_etf_skill/etf_prompt.txt"); err == nil && len(b) > 0 {
		etfPrompt = string(b)
	}

	serenityPrompt := defaultSerenityDirective
	if b, err := os.ReadFile("cmd/insert_serenity_skill/serenity_prompt.txt"); err == nil && len(b) > 0 {
		serenityPrompt = string(b)
	}

	skills := []models.Skill{
		{
			Name:        "ETF轮动多Agent分析",
			Description: "执行完整的多Agent ETF轮动分析流程：21日加权动量筛选→6路因子并发分析(fan-out)→投委会5视角加权融合→输出开盘前交易决策",
			Category:    "量化策略",
			SystemPrompt: etfPrompt,
			Directive: `# ETF轮动多Agent分析 — 强制执行规则

你必须严格按以下流程执行ETF开盘前分析：

## 第1步：动量筛选
用 SearchStockByIndicators 获取ETF池，按21日加权动量排序，剔除回撤>冷却阈值或盈亏比<1的标的。

## 第2步：6路因子并发分析
对前5只候选ETF，用工具fan-out并行分析以下维度：
- 技术面（均线/KDJ/MACD/布林带）
- 资金面（主力/北向资金流向）
- 消息面（新闻/公告）
- 海外联动（隔夜外围表现）
- 宏观/行业周期
- 申赎/溢价率

## 第3步：投委会5视角加权融合
从趋势、风控、价值、筹码、情绪5个视角评分，加权输出综合得分和决策建议。

## 输出格式
必须包含：ETF池概览→动量排名→各维度分析摘要→投委会评分表→最终交易决策（买入/持有/卖出/观望+仓位建议）`,
			TriggerKeywords: "ETF,轮动,动量,板块轮动,资金流向,北向资金,技术面,消息面,开盘前决策,集合竞价,ETF池,动量轮动,因子分析,投委会,持仓评审,海外联动,宏观,资金流,申赎,主力资金,溢价率,双动量,回撤冷却,盈亏比,仓位管理,集合竞价复核,投委会精选",
			Examples:        "用户：今天ETF开盘前有什么建议？我持有159915和512660\n助手：必须输出完整的ETF轮动分析报告结构，从动量筛选开始逐步分析到最终决策，包含持仓评审。",
			Enable:          true,
			SortOrder:       1,
			ToolGroups:      "money_flow,news_research,stock_analysis,market",
			Version:         "1.0.0",
			Status:          "active",
			InputSchema:     "{}",
			OutputSchema:    "{}",
		},
		{
			Name:        "Serenity式产业分析",
			Description: "强制使用Serenity产业分析框架：产业链拆解→稀缺层识别→优先级排序→证据验证→证伪条件",
			Category:    "资讯研究",
			SystemPrompt: serenityPrompt,
			Directive: `# Serenity式产业分析 — 强制执行规则

你必须严格按Serenity研究框架执行产业分析：

## 第1步：产业链全景拆解
识别目标公司在产业链中的位置（上游/中游/下游），列出关键环节和参与方。

## 第2步：稀缺层识别
找出产业链中技术壁垒最高、替代难度最大的环节。关注：国产替代率<30%、市占率集中度CR5>60%、毛利率>40%的环节。

## 第3步：优先级排序
按稀缺性、可投资性、确定性三维度评分，锁定高优先级方向。

## 第4步：证据链验证
用数据验证判断：营收/利润增速、研发投入占比、客户集中度、产能利用率。必须调用工具获取最新数据，严禁编造。

## 第5步：证伪条件
明确列出什么情况下分析结论会失效（如技术路线变化、政策转向、竞争格局突变）。

## 输出格式
必须包含：产业链全景图→稀缺层分析→优先级评分→证据链→证伪条件→最终判断`,
			TriggerKeywords: "产业分析,产业链,稀缺性,瓶颈,行业研究,赛道分析,上下游,供需格局,渗透率,国产替代,市场份额,竞争格局,华阳集团,投资机会,细分龙头",
			Examples:        "用户：分析华阳集团在汽车电子产业链中的位置\n助手：必须输出Serenity报告结构：产业链全景图→稀缺层识别→优先级排序→证据链→证伪条件",
			Enable:          true,
			SortOrder:       2,
			ToolGroups:      "stock_analysis,market,news_research,screening",
			Version:         "1.0.0",
			Status:          "active",
			InputSchema:     "{}",
			OutputSchema:    "{}",
		},
	}

	for _, s := range skills {
		if err := db.Dao.Create(&s).Error; err != nil {
			logger.SugaredLogger.Warnf("seed skill <%s> failed: %v", s.Name, err)
		} else {
			logger.SugaredLogger.Infof("seed skill <%s> ID=%d", s.Name, s.ID)
		}
	}
}

const defaultETFDirective = `===== Multi-Agent ETF 轮动分析流水线 =====
你是专业的量化投资专家。在收到 ETF 分析请求时，请按动量排序和多视角打分执行分析。`

const defaultSerenityDirective = `===== Serenity 产业深度研究流水线 =====
你是专业的行业与产业链研究专家。请从产业链上下游、稀缺层识别、竞争壁垒和证伪条件展开分析。`
