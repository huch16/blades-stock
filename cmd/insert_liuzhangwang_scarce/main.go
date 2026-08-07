package main

import (
	"fmt"

	"blades-stock/backend/db"
	"blades-stock/backend/data"
	"blades-stock/backend/models"
)

func main() {
	db.Init("data/stock.db?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288")
	db.Dao.AutoMigrate(&models.AiRecommendStocks{})

	const modelName = "六张网稀缺层-20260804"
	const rating = "买入"

	rows := []*models.AiRecommendStocks{
		{ModelName: modelName, Rating: rating, StockCode: "688509.SH", StockName: "正元地信", BkName: "水数字化（水网）", StockPrice: "4.19", RecommendReason: "控制稀缺层：管网漏检与修复技术壁垒，具备国家管网检测资质与长期服务协议，管网数字化市场空间超6000亿", RiskRemarks: "业务集中于单一细分领域，若政策推进放缓可能影响订单", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "600820.SH", StockName: "隧道股份", BkName: "城市地下管网", StockPrice: "5.59", RecommendReason: "控制稀缺层：盾构施工资质、大直径管片制造能力，参与多项地下管廊PPP项目，基建景气带动", RiskRemarks: "受宏观基建投资波动影响，应收账款周期较长", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "601669.SH", StockName: "中国电建", BkName: "新型电网", StockPrice: "4.89", RecommendReason: "控制稀缺层：特高压/柔性直流施工资质，国家电网及南网核心供应商，兼具储能EPC能力", RiskRemarks: "新能源并网进度不及预期可能导致订单延迟", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "601390.SH", StockName: "中国中铁", BkName: "城市地下管网/物流网", StockPrice: "4.52", RecommendReason: "控制稀缺层：铁路施工特级资质，长期管廊PPP项目，铁路物流与市政地下管廊双重受益", RiskRemarks: "受基建投资波动，项目回款周期较长", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，报告标注数据源暂不可用，结论定性降级"},
		{ModelName: modelName, Rating: rating, StockCode: "300007.SZ", StockName: "汉威科技", BkName: "水网", StockPrice: "35.03", RecommendReason: "供应层：气体与液体传感器技术领先，智慧水务监测渗透率提升，管网泄漏预警受益", RiskRemarks: "传感器市场竞争激烈，毛利率波动", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，报告标注数据源暂不可用，结论定性降级"},
		{ModelName: modelName, Rating: rating, StockCode: "300075.SZ", StockName: "数字政通", BkName: "水网/管网", StockPrice: "11.48", RecommendReason: "供应层：智慧城市平台与管网数据中台深度绑定，客户为政府及国企", RiskRemarks: "政府信息化进度不确定，合同周期长", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，报告标注数据源暂不可用，结论定性降级"},
		{ModelName: modelName, Rating: rating, StockCode: "300274.SZ", StockName: "阳光电源", BkName: "新型电网", StockPrice: "106.34", RecommendReason: "供应层：储能逆变器市场份额领先、技术迭代快，储能系统与分布式光伏EPC布局", RiskRemarks: "储能政策补贴变化影响装机节奏", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "300750.SZ", StockName: "宁德时代", BkName: "新型电网/算力网", StockPrice: "395.10", RecommendReason: "供应层：动力及储能电池能量密度领先，储能系统集成能力，兼具数据中心备电应用", RiskRemarks: "原材料（锂、镍）价格波动、产能扩张压力", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "000063.SZ", StockName: "中兴通讯", BkName: "新一代通信网", StockPrice: "34.37", RecommendReason: "供应层：5G基站供应商集中度较高，光模块自给率提升，受益5G/6G及物联网感知层", RiskRemarks: "中美科技博弈可能导致出口受限", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "600498.SH", StockName: "烽火通信", BkName: "新一代通信网/算力网", StockPrice: "34.46", RecommendReason: "供应层：光纤光缆与数据中心互联光模块布局，液冷方案覆盖算力网散热需求", RiskRemarks: "光通信周期性波动，客户集中度中等", Remarks: "六张网稀缺层报告核验入库，代码-名称已核验，报告标注数据源暂不可用，结论定性降级"},
	}

	if err := data.NewAiRecommendStocksService().BatchCreateAiRecommendStocks(rows); err != nil {
		panic(fmt.Sprintf("插入失败: %v", err))
	}
	fmt.Printf("SUCCESS: 已入库 %d 条记录\n", len(rows))

	var total int64
	db.Dao.Model(&models.AiRecommendStocks{}).Where("model_name = ? AND deleted_at IS NULL", modelName).Count(&total)
	fmt.Printf("校验：model_name=%s 共 %d 条\n", modelName, total)
}
