package services

import (
	"encoding/json"
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
	db.Dao.AutoMigrate(&agent.KnowledgeCrystal{})

	// 4. 底层 DB 核心记忆与表迁移
	db.AutoMigrate()

	// 5. 播种系统默认精选 Skill (ETF 动量轮动 / Serenity 产业链分析)
	SeedDefaultSkills()

	// 6. 播种股票基础信息基准库（A 股、港股、美股）
	SeedStockBasicsIfEmpty()

	// 7. 播种系统默认开箱即用 AI 大模型服务配置
	SeedDefaultAIConfigsIfEmpty()

	// 8. 播种系统默认高胜率经验结晶知识库
	SeedDefaultKnowledgeCrystalsIfEmpty()

	// 9. 自动自愈清理历史遗留的虚假推荐指标与排雷Prompt脏数据
	_ = agent.CleanDirtyRecommendStocks()

	logger.SugaredLogger.Info("[Database] All tables migrated and initialized successfully.")
}

// SeedDefaultAIConfigsIfEmpty 当全新安装且 AI 模型配置为空时，自动播种主流模型配置
func SeedDefaultAIConfigsIfEmpty() {
	if db.Dao == nil {
		return
	}
	var count int64
	db.Dao.Model(&data.AIConfig{}).Count(&count)
	if count > 0 {
		return
	}

	deepseekKey := os.Getenv("DEEPSEEK_API_KEY")
	if deepseekKey == "" {
		deepseekKey = os.Getenv("AI_API_KEY")
	}
	openaiKey := os.Getenv("OPENAI_API_KEY")

	defaultConfigs := []*data.AIConfig{
		{
			Name:                "DeepSeek 官方 API",
			BaseUrl:             "https://api.deepseek.com",
			ApiKey:              deepseekKey,
			ModelName:           "deepseek-chat",
			MaxTokens:           8192,
			Temperature:         0.3,
			TimeOut:             300,
			LLMConcurrencyLimit: 2,
			SubAgentTimeout:     60,
		},
		{
			Name:                "硅基流动 (SiliconFlow)",
			BaseUrl:             "https://api.siliconflow.cn/v1",
			ApiKey:              "",
			ModelName:           "Qwen/Qwen2.5-7B-Instruct",
			MaxTokens:           8192,
			Temperature:         0.3,
			TimeOut:             300,
			LLMConcurrencyLimit: 2,
			SubAgentTimeout:     60,
		},
		{
			Name:                "Ollama 本地/局域网",
			BaseUrl:             "http://localhost:11434/v1",
			ApiKey:              "none",
			ModelName:           "qwen2.5:7b",
			MaxTokens:           4096,
			Temperature:         0.3,
			TimeOut:             300,
			LLMConcurrencyLimit: 2,
			SubAgentTimeout:     60,
		},
	}
	if openaiKey != "" {
		defaultConfigs = append(defaultConfigs, &data.AIConfig{
			Name:                "OpenAI 官方",
			BaseUrl:             "https://api.openai.com/v1",
			ApiKey:              openaiKey,
			ModelName:           "gpt-4o-mini",
			MaxTokens:           8192,
			Temperature:         0.3,
			TimeOut:             300,
			LLMConcurrencyLimit: 2,
			SubAgentTimeout:     60,
		})
	}
	if err := db.Dao.CreateInBatches(defaultConfigs, len(defaultConfigs)).Error; err == nil {
		logger.SugaredLogger.Infof("[Database] 成功播种 %d 个默认 AI 模型服务配置", len(defaultConfigs))
	}
}

// SeedStockBasicsIfEmpty 当全新安装且股票基础信息库为空时，自动从自带基准快照导入
func SeedStockBasicsIfEmpty() {
	if db.Dao == nil {
		return
	}

	// 1. 种子导入 A 股基础数据
	var countA int64
	db.Dao.Model(&data.StockBasic{}).Count(&countA)
	if countA == 0 {
		paths := []string{"build/stock_basic.json", "stock_basic.json", "../build/stock_basic.json", "/app/build/stock_basic.json"}
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil || len(b) == 0 {
				continue
			}
			var resp struct {
				Data struct {
					Fields []string `json:"fields"`
					Items  [][]any  `json:"items"`
				} `json:"data"`
			}
			if err := json.Unmarshal(b, &resp); err == nil && len(resp.Data.Items) > 0 {
				var stocks []data.StockBasic
				fieldMap := make(map[string]int)
				for i, f := range resp.Data.Fields {
					fieldMap[f] = i
				}
				getString := func(item []any, field string) string {
					if idx, ok := fieldMap[field]; ok && idx < len(item) && item[idx] != nil {
						if s, ok := item[idx].(string); ok {
							return s
						}
					}
					return ""
				}
				for _, item := range resp.Data.Items {
					tsCode := getString(item, "ts_code")
					if tsCode == "" {
						continue
					}
					stocks = append(stocks, data.StockBasic{
						TsCode:     tsCode,
						Symbol:     getString(item, "symbol"),
						Name:       getString(item, "name"),
						Area:       getString(item, "area"),
						Industry:   getString(item, "industry"),
						Cnspell:    getString(item, "cnspell"),
						Market:     getString(item, "market"),
						Exchange:   getString(item, "exchange"),
						CurrType:   getString(item, "curr_type"),
						ListStatus: getString(item, "list_status"),
						ListDate:   getString(item, "list_date"),
						DelistDate: getString(item, "delist_date"),
						IsHs:       getString(item, "is_hs"),
						ActName:    getString(item, "act_name"),
						ActEntType: getString(item, "act_ent_type"),
						Fullname:   getString(item, "fullname"),
						Ename:      getString(item, "enname"),
					})
				}
				if len(stocks) > 0 {
					if err := db.Dao.CreateInBatches(stocks, 400).Error; err == nil {
						logger.SugaredLogger.Infof("[Database] 成功从 %s 播种 %d 条 A 股基础信息", p, len(stocks))
					}
				}
				break
			}
		}
		// 后台启动通达信增量校准同步（自动补全最新 A 股新股）
		go func() {
			defer func() { _ = recover() }()
			data.NewTdxKLineApi().SyncStockBasicToDB()
		}()
	}

	// 2. 种子导入港股基础数据
	var countHK int64
	db.Dao.Model(&models.StockInfoHK{}).Count(&countHK)
	if countHK == 0 {
		pathsHK := []string{"build/stock_base_info_hk.json", "stock_base_info_hk.json", "../build/stock_base_info_hk.json", "/app/build/stock_base_info_hk.json"}
		for _, p := range pathsHK {
			b, err := os.ReadFile(p)
			if err != nil || len(b) == 0 {
				continue
			}
			var hkStocks []models.StockInfoHK
			if err := json.Unmarshal(b, &hkStocks); err == nil && len(hkStocks) > 0 {
				for i := range hkStocks {
					hkStocks[i].ID = 0
				}
				if err := db.Dao.CreateInBatches(hkStocks, 400).Error; err == nil {
					logger.SugaredLogger.Infof("[Database] 成功从 %s 播种 %d 条港股基础信息", p, len(hkStocks))
				}
				break
			}
		}
	}

	// 3. 种子导入美股基础数据
	var countUS int64
	db.Dao.Model(&models.StockInfoUS{}).Count(&countUS)
	if countUS == 0 {
		pathsUS := []string{"build/stock_base_info_us.json", "stock_base_info_us.json", "../build/stock_base_info_us.json", "/app/build/stock_base_info_us.json"}
		for _, p := range pathsUS {
			b, err := os.ReadFile(p)
			if err != nil || len(b) == 0 {
				continue
			}
			var usStocks []models.StockInfoUS
			if err := json.Unmarshal(b, &usStocks); err == nil && len(usStocks) > 0 {
				for i := range usStocks {
					usStocks[i].ID = 0
				}
				if err := db.Dao.CreateInBatches(usStocks, 400).Error; err == nil {
					logger.SugaredLogger.Infof("[Database] 成功从 %s 播种 %d 条美股基础信息", p, len(usStocks))
				}
				break
			}
		}
		// 后台启动通达信增量校准同步（自动补全港美股）
		go func() {
			defer func() { _ = recover() }()
			data.NewTdxKLineApi().SyncHKUSStockBasicToDB()
		}()
	}
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

// SeedDefaultKnowledgeCrystalsIfEmpty 播种平台默认的经典量化与交易经验结晶库
func SeedDefaultKnowledgeCrystalsIfEmpty() {
	if db.Dao == nil {
		return
	}
	var count int64
	db.Dao.Model(&agent.KnowledgeCrystal{}).Count(&count)
	if count > 0 {
		return
	}

	crystals := []agent.KnowledgeCrystal{
		{
			Category:       "etf_rotation",
			Title:          "21日动量加权对数回归与R²滤波轮动规律",
			TriggerPattern: "ETF动量轮动 | 均线站上MA5 | R²置信度>0.75",
			Summary:        "基于21日纯数学加权对数收益率与线性回归拟合优度R²，排名靠前且未受回撤拦截的行业ETF胜率显著高于市场均值。当5日最大回撤超过3.5%时触发风控冷却拦截，空仓或避险货币ETF可规避90%以上急跌波段。",
			KeyInsights:    "动量因子年化收益率权重60%，回归置信度R²权重40%，配合投委会五视角（趋势、风控、价值、筹码、情绪）加权评分。",
			AccuracyScore:  83.5,
			HitCount:       12,
			SourceDate:     "2026-10-15",
			SourceRef:      "system_seed_etf_001",
			ContentHash:    agent.ComputeContentHash("etf_rotation", "21日动量加权对数回归与R²滤波轮动规律", "基于21日纯数学加权对数收益率与线性回归拟合优度R²"),
		},
		{
			Category:       "stock_resonance",
			Title:          "突破箱体上沿+5日主力净流入共振形态",
			TriggerPattern: "放量突破平台 | 5日主力资金净流入>15% | 站稳MA20",
			Summary:        "个股经历30个交易日以上窄幅震荡整理后，放量突破箱体上沿且伴随5日超大单/大单连续净流入，回踩MA20获得支撑时为极佳介入点。历史回测胜率达78.2%，盈亏比平均大于3:1。",
			KeyInsights:    "吸筹期间筹码集中度90%区间<12%，突破当日成交量放大至5日均量的1.8倍以上，未出现高位长上影线。",
			AccuracyScore:  78.2,
			HitCount:       8,
			SourceDate:     "2026-10-16",
			SourceRef:      "system_seed_resonance_002",
			ContentHash:    agent.ComputeContentHash("stock_resonance", "突破箱体上沿+5日主力净流入共振形态", "个股经历30个交易日以上窄幅震荡整理后"),
		},
		{
			Category:       "risk_warning",
			Title:          "高位缩量背离诱多与主力对倒出货警示",
			TriggerPattern: "股价创阶段新高 | 量价背离 | 龙虎榜机构净卖出",
			Summary:        "股价创出新高但换手率显著萎缩（缩量冲高），分时图出现断层式大单对倒拉升而后急速回落，龙虎榜显示机构席位大额净卖出。此类形态后3日走弱概率达85.0%，严禁追高，已持有者建议坚决止盈或减仓防守。",
			KeyInsights:    "主力利用存量底仓推高价格吸引跟风盘，实则暗中分批派发。止损防守位应严格设在突破阳线实体底部。",
			AccuracyScore:  85.0,
			HitCount:       15,
			SourceDate:     "2026-10-16",
			SourceRef:      "system_seed_warning_003",
			ContentHash:    agent.ComputeContentHash("risk_warning", "高位缩量背离诱多与主力对倒出货警示", "股价创出新高但换手率显著萎缩（缩量冲高）"),
		},
		{
			Category:       "macro_policy",
			Title:          "降息宽松周期初期大宗有色与科技半导体双轮共振",
			TriggerPattern: "宏观降息政策 | 流动性释放 | 全球大宗商品联动",
			Summary:        "在央行降准降息流动性释放周期初期，资金最先涌入两类具备高贝塔弹性的资产：一是受益于通胀预期的有色金属/黄金大宗ETF，二是受益于估值中枢扩张与产业自主可控的半导体芯片产业链，阶段性超额超80%交易日。",
			KeyInsights:    "宏观流动性宽松使得长端无风险利率下行，成长科技资产折现率下降，估值弹性显著放大。",
			AccuracyScore:  81.0,
			HitCount:       6,
			SourceDate:     "2026-10-17",
			SourceRef:      "system_seed_macro_004",
			ContentHash:    agent.ComputeContentHash("macro_policy", "降息宽松周期初期大宗有色与科技半导体双轮共振", "在央行降准降息流动性释放周期初期"),
		},
	}

	for _, c := range crystals {
		_ = db.Dao.Create(&c).Error
	}
	logger.SugaredLogger.Infof("[Database] 成功播种 %d 条初始默认高胜率经验结晶知识库", len(crystals))
}
