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

	const modelName = "六张网深度-20260805"
	const rating = "买入"

	rows := []*models.AiRecommendStocks{
		{ModelName: modelName, Rating: rating, StockCode: "600312.SH", StockName: "平高电气", BkName: "新型电网/特高压", StockPrice: "20.87", RecommendReason: "控制稀缺层：特高压核心设备供应商，国家电网核心供应商，2026年特高压标段中标超23亿元，直接受益新型电网超5万亿投资，估值安全边际好（市盈率约12倍）", RiskRemarks: "若政策削减电网投资或特高压中标金额低于5亿元，将削弱稀缺层优势", Remarks: "六张网Serenity深度报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "688509.SH", StockName: "正元地信", BkName: "城市地下管网/水网数字化", StockPrice: "4.19", RecommendReason: "控制稀缺层：国内唯一具备三维GIS+管线探测技术的龙头，被列为十五五地下管网数字化核心受益标的，政府长协概率高，管网数字化市场空间超6000亿", RiskRemarks: "业务集中于单一细分领域，政策推进放缓可能影响订单", Remarks: "六张网Serenity深度报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "002929.SZ", StockName: "润建股份", BkName: "算力网/AIDC", StockPrice: "57.74", RecommendReason: "供应层：聚焦token工厂算力网络，拥有五象云谷等核心AIDC资源，计划建设4个万卡算力集群，直接受益算力网4万亿投资", RiskRemarks: "证据强度中等（缺具体中标公告），算力网建设进度不及预期将减弱增长预期", Remarks: "六张网Serenity深度报告核验入库，代码-名称已核验，实时价2026-08-05"},
		{ModelName: modelName, Rating: rating, StockCode: "300857.SZ", StockName: "协创数据", BkName: "算力网/算力租赁", StockPrice: "252.90", RecommendReason: "供应层：独立第三方算力租赁龙头，头部玩家≤3，2025-2027年预计累计投入数千亿资本开支，规模效应凸显，客户预付款常见", RiskRemarks: "证据强度中等（缺具体合同披露），盈利弹性依赖算力需求持续性与大厂资本开支", Remarks: "六张网Serenity深度报告核验入库，代码-名称已核验，实时价2026-08-05"},
	}

	if err := data.NewAiRecommendStocksService().BatchCreateAiRecommendStocks(rows); err != nil {
		panic(fmt.Sprintf("插入失败: %v", err))
	}
	fmt.Printf("SUCCESS: 已入库 %d 条记录\n", len(rows))

	var total int64
	db.Dao.Model(&models.AiRecommendStocks{}).Where("model_name = ? AND deleted_at IS NULL", modelName).Count(&total)
	fmt.Printf("校验：model_name=%s 共 %d 条\n", modelName, total)
}
