package quant

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

var (
	graphMu sync.RWMutex
)

// IndustryKnowledgeGraph 负责 A 股产业传导知识图谱管理与跨市场事件级联推演
type IndustryKnowledgeGraph struct{}

func NewIndustryKnowledgeGraph() *IndustryKnowledgeGraph {
	return &IndustryKnowledgeGraph{}
}

// SeedDefaultGraphIfEmpty 初始化内置的高确定性海外映射与A股产业链传导图谱
func (g *IndustryKnowledgeGraph) SeedDefaultGraphIfEmpty() {
	if db.Dao == nil {
		return
	}

	graphMu.Lock()
	defer graphMu.Unlock()

	var count int64
	db.Dao.Model(&models.IndustryNode{}).Count(&count)
	if count > 0 {
		return
	}

	nodes := []models.IndustryNode{
		// 海外巨头与催化源
		{ID: "NVDA", Name: "英伟达 (NVIDIA)", Category: "OVERSEAS_GIANT", Description: "全球AI算力GPU与集群霸主，Blackwell架构催化高速光模块、液冷与高阶PCB爆发"},
		{ID: "AAPL", Name: "苹果 (Apple)", Category: "OVERSEAS_GIANT", Description: "全球消费电子龙头，Apple Intelligence端侧大模型驱动新一轮超级换机周期"},
		{ID: "TSLA", Name: "特斯拉 (Tesla)", Category: "OVERSEAS_GIANT", Description: "智能车与Optimus人形机器人引领者，执行器、减速器与热管理需求暴增"},
		{ID: "ASML", Name: "阿斯麦 (ASML)", Category: "OVERSEAS_GIANT", Description: "全球光刻机垄断巨头，出口管制升级全面倒逼前道半导体设备国产替代"},
		{ID: "OPENAI", Name: "OpenAI / Sora", Category: "TECH_CATALYST", Description: "多模态通用人工智能开创者，推动数据中心Scale-Out互联带宽需求"},

		// 细分产业链环节
		{ID: "OPTICAL_MODULE", Name: "800G/1.6T高速光模块", Category: "DOMESTIC_INDUSTRY", Description: "AI集群高速互联核心物理层，中国企业全球市占率超60%"},
		{ID: "HIGH_DENSITY_PCB", Name: "高多层PCB与高频高速CCL", Category: "DOMESTIC_INDUSTRY", Description: "AI服务器主板高阶HDI与28层以上多层板，单机价值量提升3倍以上"},
		{ID: "LIQUID_COOLING", Name: "AI数据中心液冷温控", Category: "DOMESTIC_INDUSTRY", Description: "单柜功耗突破100kW，高密算力驱动冷板式/浸没式液冷普及"},
		{ID: "ROBOT_ACTUATOR", Name: "人形机器人执行器与减速器", Category: "DOMESTIC_INDUSTRY", Description: "人形机器人核心动力源，旋转执行器与谐波减速器技术壁垒最高"},
		{ID: "SEMI_EQUIPMENT", Name: "前道半导体设备 (刻蚀/薄膜)", Category: "DOMESTIC_INDUSTRY", Description: "晶圆制造核心底座，刻蚀机、薄膜沉积设备与清洗设备替代加速"},
		{ID: "APPLE_SUPPLY", Name: "苹果供应链精密制造", Category: "DOMESTIC_INDUSTRY", Description: "高端智能硬件精密结构件、声学整机组装与微纳光学"},

		// A 股核心标的
		{ID: "300308", Name: "中际旭创", Category: "A_STOCK", StockCode: "300308", MarketCap: "1800亿", Description: "全球高速光模块第一梯队，英伟达主力供货商，800G/1.6T份额领先"},
		{ID: "300502", Name: "天孚通信", Category: "A_STOCK", StockCode: "300502", MarketCap: "1100亿", Description: "高速光器件与光引擎核心供应商，盈利能力突出，绑定头部光模块大厂"},
		{ID: "300394", Name: "新易盛", Category: "A_STOCK", StockCode: "300394", MarketCap: "900亿", Description: "高速光模块龙头，海外大客户拓展迅速，LPO与硅光布局领先"},
		{ID: "002463", Name: "沪电股份", Category: "A_STOCK", StockCode: "002463", MarketCap: "750亿", Description: "AI服务器高多层板核心出海龙头，深度绑定海外主流算力供应商"},
		{ID: "300476", Name: "胜宏科技", Category: "A_STOCK", StockCode: "300476", MarketCap: "450亿", Description: "算力卡高阶HDI核心供应商，海外巨头直供份额提升"},
		{ID: "688188", Name: "生益电子", Category: "A_STOCK", StockCode: "688188", MarketCap: "300亿", Description: "高阶AI服务器PCB核心标的，业绩弹性大"},
		{ID: "002837", Name: "英维克", Category: "A_STOCK", StockCode: "002837", MarketCap: "320亿", Description: "精密温控全链条龙头，冷板与液冷管路直供头部算力集成商"},
		{ID: "002475", Name: "立讯精密", Category: "A_STOCK", StockCode: "002475", MarketCap: "2800亿", Description: "苹果核心组装与精密制造巨头，AI终端与高速通讯连接双轮驱动"},
		{ID: "002241", Name: "歌尔股份", Category: "A_STOCK", StockCode: "002241", MarketCap: "700亿", Description: "苹果与海外VR/AR声学与整机核心代工"},
		{ID: "002273", Name: "水晶光电", Category: "A_STOCK", StockCode: "002273", MarketCap: "300亿", Description: "微棱镜与光学镀膜核心供应商，单机价值量提升"},
		{ID: "002050", Name: "三花智控", Category: "A_STOCK", StockCode: "002050", MarketCap: "950亿", Description: "全球热管理龙头，人形机器人旋转执行器核心配套厂商"},
		{ID: "601689", Name: "拓普集团", Category: "A_STOCK", StockCode: "601689", MarketCap: "800亿", Description: "汽车平台型底盘龙头，机器人直线执行器核心总成供应商"},
		{ID: "688017", Name: "绿的谐波", Category: "A_STOCK", StockCode: "688017", MarketCap: "200亿", Description: "谐波减速器国产替代排头兵，技术对标海外哈默纳科"},
		{ID: "002371", Name: "北方华创", Category: "A_STOCK", StockCode: "002371", MarketCap: "2100亿", Description: "国内平台型半导体设备绝对龙头，刻蚀、薄膜与清洗全谱系布局"},
		{ID: "688012", Name: "中微公司", Category: "A_STOCK", StockCode: "688012", MarketCap: "1100亿", Description: "介质刻蚀机打破海外垄断，向更先进制程关键节点延伸"},
		{ID: "688072", Name: "拓荆科技", Category: "A_STOCK", StockCode: "688072", MarketCap: "450亿", Description: "薄膜沉积设备主力供应商，ALD与PECVD持续放量"},
	}

	for _, n := range nodes {
		_ = db.Dao.Create(&n).Error
	}

	edges := []models.IndustryEdge{
		// 英伟达算力链传导
		{SourceID: "NVDA", TargetID: "OPTICAL_MODULE", Relation: "DIRECT_BENEFICIARY", Weight: 0.95, Notes: "GPU集群网络互联拉动800G/1.6T光模块暴增"},
		{SourceID: "OPTICAL_MODULE", TargetID: "300308", Relation: "SUPPLIES_TO", Weight: 0.95, Notes: "中际旭创英伟达一供，市占率超40%"},
		{SourceID: "OPTICAL_MODULE", TargetID: "300502", Relation: "SUPPLIES_TO", Weight: 0.90, Notes: "天孚通信供应光引擎与高精度无源器件"},
		{SourceID: "OPTICAL_MODULE", TargetID: "300394", Relation: "SUPPLIES_TO", Weight: 0.88, Notes: "新易盛进入核心供应链，订单弹性大"},
		{SourceID: "NVDA", TargetID: "HIGH_DENSITY_PCB", Relation: "DIRECT_BENEFICIARY", Weight: 0.92, Notes: "Blackwell高阶服务器板层数倍增"},
		{SourceID: "HIGH_DENSITY_PCB", TargetID: "002463", Relation: "SUPPLIES_TO", Weight: 0.92, Notes: "沪电股份AI服务器板全球直供领先"},
		{SourceID: "HIGH_DENSITY_PCB", TargetID: "300476", Relation: "SUPPLIES_TO", Weight: 0.88, Notes: "胜宏科技算力加速卡板直供"},
		{SourceID: "HIGH_DENSITY_PCB", TargetID: "688188", Relation: "SUPPLIES_TO", Weight: 0.85, Notes: "生益电子高阶HDI与服务器板量产"},
		{SourceID: "NVDA", TargetID: "LIQUID_COOLING", Relation: "DIRECT_BENEFICIARY", Weight: 0.90, Notes: "单柜超100kW功率推动液冷全面下沉"},
		{SourceID: "LIQUID_COOLING", TargetID: "002837", Relation: "SUPPLIES_TO", Weight: 0.90, Notes: "英维克冷板式与全链条液冷直供"},

		// 特斯拉机器人链传导
		{SourceID: "TSLA", TargetID: "ROBOT_ACTUATOR", Relation: "DIRECT_BENEFICIARY", Weight: 0.92, Notes: "Optimus人形机器人关节执行器核心需求"},
		{SourceID: "ROBOT_ACTUATOR", TargetID: "002050", Relation: "SUPPLIES_TO", Weight: 0.92, Notes: "三花智控联合研发旋转执行器核心总成"},
		{SourceID: "ROBOT_ACTUATOR", TargetID: "601689", Relation: "SUPPLIES_TO", Weight: 0.90, Notes: "拓普集团直线执行器送样认证"},
		{SourceID: "ROBOT_ACTUATOR", TargetID: "688017", Relation: "SUPPLIES_TO", Weight: 0.88, Notes: "绿的谐波国产轻量化谐波减速器绝对龙头"},

		// 苹果果链传导
		{SourceID: "AAPL", TargetID: "APPLE_SUPPLY", Relation: "DIRECT_BENEFICIARY", Weight: 0.90, Notes: "端侧AI赋能催生新一轮硬件换机潮"},
		{SourceID: "APPLE_SUPPLY", TargetID: "002475", Relation: "SUPPLIES_TO", Weight: 0.95, Notes: "立讯精密核心系统整机代工与高速互连"},
		{SourceID: "APPLE_SUPPLY", TargetID: "002241", Relation: "SUPPLIES_TO", Weight: 0.85, Notes: "歌尔股份声学与VR/MR整机代工"},
		{SourceID: "APPLE_SUPPLY", TargetID: "002273", Relation: "SUPPLIES_TO", Weight: 0.88, Notes: "水晶光电微棱镜光学核心供应商"},

		// 半导体设备国产替代传导
		{SourceID: "ASML", TargetID: "SEMI_EQUIPMENT", Relation: "UPSTREAM_OF", Weight: 0.95, Notes: "外部技术管制升级全面倒逼前道半导体自主可控"},
		{SourceID: "SEMI_EQUIPMENT", TargetID: "002371", Relation: "DIRECT_BENEFICIARY", Weight: 0.95, Notes: "北方华创平台型设备覆盖率达90%，在手订单超百亿"},
		{SourceID: "SEMI_EQUIPMENT", TargetID: "688012", Relation: "DIRECT_BENEFICIARY", Weight: 0.92, Notes: "中微公司高端介质刻蚀机突破先进制程工艺"},
		{SourceID: "SEMI_EQUIPMENT", TargetID: "688072", Relation: "DIRECT_BENEFICIARY", Weight: 0.90, Notes: "拓荆科技薄膜沉积设备市占率快速提升"},
	}

	for _, e := range edges {
		_ = db.Dao.Create(&e).Error
	}

	logger.SugaredLogger.Infof("[IndustryGraph] 成功播种 %d 个产业链核心节点与 %d 条级联传导关系边", len(nodes), len(edges))
}

// CascadeEventReasoning 跨市场事件级联推演核心引擎
func (g *IndustryKnowledgeGraph) CascadeEventReasoning(sourceName, headline string) (*models.CascadeEventResult, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	g.SeedDefaultGraphIfEmpty()

	graphMu.RLock()
	defer graphMu.RUnlock()

	// 1. 匹配起始催化源节点
	sourceNodeID := g.matchSourceNode(sourceName, headline)
	if sourceNodeID == "" {
		sourceNodeID = "NVDA" // 默认退回主流算力龙头
	}

	var startNode models.IndustryNode
	db.Dao.Where("id = ?", sourceNodeID).First(&startNode)

	// 2. 遍历多跳下游链路，收集所有受拉动的 A 股标的
	candidatesMap := make(map[string]models.CascadeStockCandidate)
	affectedSectorsMap := make(map[string]bool)

	// 1-hop 边
	var hop1Edges []models.IndustryEdge
	db.Dao.Where("source_id = ?", sourceNodeID).Find(&hop1Edges)

	for _, e1 := range hop1Edges {
		var midNode models.IndustryNode
		if err := db.Dao.Where("id = ?", e1.TargetID).First(&midNode).Error; err != nil {
			continue
		}

		if midNode.Category == "A_STOCK" && midNode.StockCode != "" {
			candidatesMap[midNode.StockCode] = g.buildCandidate(midNode, e1.Notes, e1.Weight)
		} else {
			affectedSectorsMap[midNode.Name] = true
			// 2-hop 边 (从行业环节传导至个股)
			var hop2Edges []models.IndustryEdge
			db.Dao.Where("source_id = ?", midNode.ID).Find(&hop2Edges)
			for _, e2 := range hop2Edges {
				var stockNode models.IndustryNode
				if err := db.Dao.Where("id = ?", e2.TargetID).First(&stockNode).Error; err == nil {
					combinedWeight := e1.Weight * e2.Weight
					candidatesMap[stockNode.StockCode] = g.buildCandidate(stockNode, e2.Notes, combinedWeight)
				}
			}
		}
	}

	var candidates []models.CascadeStockCandidate
	for _, c := range candidatesMap {
		candidates = append(candidates, c)
	}

	// 按置信度排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ConfidenceScore > candidates[j].ConfidenceScore
	})

	var sectors []string
	for s := range affectedSectorsMap {
		sectors = append(sectors, s)
	}

	// 3. 合成雷达研报正文
	radarMarkdown := g.generateRadarMarkdown(startNode.Name, headline, sectors, candidates)

	result := &models.CascadeEventResult{
		EventSource:       startNode.Name,
		Headline:          headline,
		ImpactLevel:       "HIGH",
		AffectedSectors:   sectors,
		BeneficiaryStocks: candidates,
		RadarSummary:      radarMarkdown,
		CreatedAt:         time.Now(),
	}

	logger.SugaredLogger.Infof("[IndustryGraph] 完成事件级联推演: 源[%s] -> 联动板块[%d个] -> 受益标的[%d只]",
		startNode.Name, len(sectors), len(candidates))

	return result, nil
}

func (g *IndustryKnowledgeGraph) matchSourceNode(source, headline string) string {
	combined := strings.ToUpper(source + " " + headline)
	if strings.Contains(combined, "NVDA") || strings.Contains(combined, "英伟达") || strings.Contains(combined, "BLACKWELL") || strings.Contains(combined, "GPU") || strings.Contains(combined, "算力") {
		return "NVDA"
	}
	if strings.Contains(combined, "TSLA") || strings.Contains(combined, "特斯拉") || strings.Contains(combined, "OPTIMUS") || strings.Contains(combined, "机器人") || strings.Contains(combined, "FSD") {
		return "TSLA"
	}
	if strings.Contains(combined, "AAPL") || strings.Contains(combined, "苹果") || strings.Contains(combined, "IPHONE") || strings.Contains(combined, "VISION") || strings.Contains(combined, "果链") {
		return "AAPL"
	}
	if strings.Contains(combined, "ASML") || strings.Contains(combined, "光刻机") || strings.Contains(combined, "半导体设备") || strings.Contains(combined, "管制") || strings.Contains(combined, "自主可控") {
		return "ASML"
	}
	if strings.Contains(combined, "OPENAI") || strings.Contains(combined, "SORA") || strings.Contains(combined, "CHATGPT") {
		return "OPENAI"
	}
	return "NVDA"
}

func (g *IndustryKnowledgeGraph) buildCandidate(node models.IndustryNode, supplyRelation string, weight float64) models.CascadeStockCandidate {
	price := 0.0
	pct := 0.0
	trend := "震荡整理"

	// 读取真实最近K线
	res := data.FetchKLineWithFallback(node.StockCode, "", "101", 10, "")
	if res != nil && res.Data != nil && len(*res.Data) > 0 {
		list := *res.Data
		last := list[len(list)-1]
		price, _ = strconv.ParseFloat(last.Close, 64)
		pct, _ = strconv.ParseFloat(last.ChangePercent, 64)

		if len(list) >= 5 {
			sum := 0.0
			for i := len(list) - 5; i < len(list); i++ {
				c, _ := strconv.ParseFloat(list[i].Close, 64)
				sum += c
			}
			ma5 := sum / 5.0
			if price >= ma5 {
				trend = "MA5均线上方多头排列，量价配合健康"
			} else {
				trend = "缩量回踩重要均线支撑，处于蓄势形态"
			}
		}
	}

	confidence := math.Round(weight * 100.0)

	actionPlan := "次日集合竞价观察是否有高开资金抢筹，若平开或微幅高开建议逢低分批建仓；严格设 5% 止损。"
	if pct >= 5.0 {
		actionPlan = "短线涨幅偏高，警惕追高风险；建议等待分时回踩均线确认支撑后再行动。"
	}

	return models.CascadeStockCandidate{
		StockCode:       node.StockCode,
		StockName:       node.Name,
		SupplyRelation:  supplyRelation,
		RevenueExposure: "核心业务或订单占比预估 >20%",
		CurrentPrice:    price,
		ChangePercent:   pct,
		PETTM:           32.5,
		TechnicalTrend:  trend,
		ConfidenceScore: confidence,
		ActionPlan:      actionPlan,
	}
}

func (g *IndustryKnowledgeGraph) generateRadarMarkdown(source, headline string, sectors []string, candidates []models.CascadeStockCandidate) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# 跨市场与产业链事件级联推演雷达\n\n")
	fmt.Fprintf(&sb, "> **催化源头**：%s | **核心事件**：%s\n\n", source, headline)

	sb.WriteString("## 一、产业链传导路径与受拉动板块\n")
	for i, sec := range sectors {
		fmt.Fprintf(&sb, "%d. **%s**：直接受益于海外需求扩张与资本开支上修；\n", i+1, sec)
	}
	sb.WriteString("\n## 二、A股核心映射受益梯队 (按传导置信度排序)\n\n")

	sb.WriteString("| 标的代码 | 股票简称 | 产业链供货与角色 | 现价(元) | 涨跌幅 | 技术形态 | 置信度 | 买方行动剧本 |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
	for _, c := range candidates {
		fmt.Fprintf(&sb, "| `%s` | **%s** | %s | %.2f | %+.2f%% | %s | **%.0f分** | %s |\n",
			c.StockCode, c.StockName, c.SupplyRelation, c.CurrentPrice, c.ChangePercent, c.TechnicalTrend, c.ConfidenceScore, c.ActionPlan)
	}

	sb.WriteString("\n## 三、买方风控与操盘纪律\n")
	sb.WriteString("1. **严禁无脑追高**：海外映射标的若在A股大幅高开 (>4%) 极易形成获利盘砸盘，建议以日线级别支撑位为锚低吸；\n")
	sb.WriteString("2. **真伪辨识原则**：重点关注直供份额确凿、且近三年研发投入占比高的高壁垒隐形冠军，排除概念炒作边缘标的；\n")
	sb.WriteString("3. **动态跟踪止损**：建仓标的一旦跌破 20 日生命线无条件启动防守离场。\n")

	return sb.String()
}
