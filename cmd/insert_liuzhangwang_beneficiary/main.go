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

	const modelName = "六张网龙头-20260804"
	const rating = "买入"
	const risk = "政策落地时点风险：特高压、东数西算等项目仍在批复及开工阶段，项目延迟或预算削减将冲击业绩；行业竞争加剧可能压缩利润率；部分公司依赖政府采购，投产慢于预期收入增速回落；宏观流动性收紧将抬升高杠杆企业融资成本。"

	rows := []*models.AiRecommendStocks{
		{ModelName: modelName, Rating: rating, StockCode: "601669.SH", StockName: "中国电建", BkName: "水网", StockPrice: "4.89", RecommendReason: "国家级水利工程（引江补汉等）承建方，已获超120亿元水网项目合同；绿电装机超4200万千瓦，兼得水网+电网双轮红利", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验"},
		{ModelName: modelName, Rating: rating, StockCode: "002457.SZ", StockName: "青龙管业", BkName: "水网/城市地下管网", StockPrice: "10.83", RecommendReason: "水网、城市管网用管材龙头，管材需求随水网、地下管网扩容刚性增长", RiskRemarks: risk, Remarks: "报告原写300199.SZ(实为翰宇药业)，已修正为002457.SZ"},
		{ModelName: modelName, Rating: rating, StockCode: "002372.SZ", StockName: "伟星新材", BkName: "城市地下管网/水网", StockPrice: "8.84", RecommendReason: "PPR/PE/PVC全品类管材，渠道覆盖全国城镇化改造，受益城市老旧管网改造", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验"},
		{ModelName: modelName, Rating: rating, StockCode: "600406.SH", StockName: "国电南瑞", BkName: "新型电网", StockPrice: "24.97", RecommendReason: "智能调度、柔性直流、特高压、智能配网全产业链供应商，电网投资预计超5万亿元，市占率居行业第一", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验"},
		{ModelName: modelName, Rating: rating, StockCode: "600312.SH", StockName: "平高电气", BkName: "新型电网/特高压", StockPrice: "21.12", RecommendReason: "特高压开关、变压器等核心设备供应商，2026年特高压标段中标金额超23亿元，特高压GIS市占率领先", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验"},
		{ModelName: modelName, Rating: rating, StockCode: "603019.SH", StockName: "中科曙光", BkName: "算力网", StockPrice: "85.55", RecommendReason: "算力网国家队，拥有芯片、整机、液冷、智算服务完整链路，深度参与东数西算国家级平台建设", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验"},
		{ModelName: modelName, Rating: rating, StockCode: "600522.SH", StockName: "中天科技", BkName: "算力网/新一代通信网", StockPrice: "29.94", RecommendReason: "光纤光缆、电力线缆双龙头，兼具算力网与新一代通信网两大赛道需求，提供OPGW光缆等底层硬件", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验"},
		{ModelName: modelName, Rating: rating, StockCode: "300308.SZ", StockName: "中际旭创", BkName: "算力网/光模块", StockPrice: "1021.99", RecommendReason: "光模块龙头，算力网、5G/6G、数据中心光模块需求直接受益；2026Q1营收194.96亿(+192%)、归母净利57.35亿(+262%)", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验，Q1业绩已实查"},
		{ModelName: modelName, Rating: rating, StockCode: "000063.SZ", StockName: "中兴通讯", BkName: "新一代通信网", StockPrice: "34.37", RecommendReason: "5G-A、6G、万兆光宽带、低轨卫星等全域通信设备供应商，行业份额居国内第一梯队，直接受益通信网升级", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验"},
		{ModelName: modelName, Rating: rating, StockCode: "300007.SZ", StockName: "汉威科技", BkName: "城市地下管网", StockPrice: "35.03", RecommendReason: "智能管网感知设备供应商，产品覆盖燃气、供水、排水管线传感器，受益城市管网数字化改造", RiskRemarks: risk, Remarks: "报告原写300211.SZ(实为*ST亿通)，已修正为300007.SZ"},
		{ModelName: modelName, Rating: rating, StockCode: "601800.SH", StockName: "中国交建", BkName: "物流网", StockPrice: "6.20", RecommendReason: "物流枢纽、港口航道、临港基建央企，承接宁波舟山港、海南临港等超级工程，物流网基础设施核心承建方", RiskRemarks: risk, Remarks: "六张网受益标的报告核验入库，代码-名称已核验"},
	}

	if err := data.NewAiRecommendStocksService().BatchCreateAiRecommendStocks(rows); err != nil {
		panic(fmt.Sprintf("插入失败: %v", err))
	}
	fmt.Printf("SUCCESS: 已入库 %d 条记录\n", len(rows))

	var total int64
	db.Dao.Model(&models.AiRecommendStocks{}).Where("model_name = ? AND deleted_at IS NULL", modelName).Count(&total)
	fmt.Printf("校验：model_name=%s 共 %d 条\n", modelName, total)
}
