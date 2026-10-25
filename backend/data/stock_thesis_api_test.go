package data

import (
	"testing"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func TestExtractThesisFromContent(t *testing.T) {
	api := NewStockThesisApi()

	// 1. 测试标准结构化输出提取
	sampleStructured := `
### 核心总结
长电科技在先进封装领域持续突破，稼动率稳步回升。

### 🎯 买方认知沉淀与假说检验
- **本次判定**：[逻辑强化]
- **核心逻辑**：先进封装2.5D/3D产能饱满，核心客户订单加速导入
- **验证催化**：关注下个月高算力封装芯片放量情况与季度毛利率突破20%
- **证伪条件**：海外大客户转单台积电或单季度毛利率跌破14%
`
	v, vText, thesis, catalyst, risk := api.ExtractThesisFromContent(sampleStructured)
	if v != models.VerdictThesisStrengthen || vText != "逻辑强化" {
		t.Errorf("Expected VerdictThesisStrengthen, got %s (%s)", v, vText)
	}
	if thesis != "先进封装2.5D/3D产能饱满，核心客户订单加速导入" {
		t.Errorf("Unexpected thesis: %s", thesis)
	}
	if catalyst != "关注下个月高算力封装芯片放量情况与季度毛利率突破20%" {
		t.Errorf("Unexpected catalyst: %s", catalyst)
	}
	if risk != "海外大客户转单台积电或单季度毛利率跌破14%" {
		t.Errorf("Unexpected risk: %s", risk)
	}

	// 2. 测试弱结构 / 兜底识别
	sampleFallback := `
长电科技经过盘面观察，由于消费电子复苏放缓，原先的复苏假说面临考验，短期动能衰减，本次判定逻辑削弱。
建议降低仓位观望。
`
	v2, vText2, _, _, _ := api.ExtractThesisFromContent(sampleFallback)
	if v2 != models.VerdictThesisWeaken || vText2 != "逻辑削弱" {
		t.Errorf("Expected VerdictThesisWeaken, got %s (%s)", v2, vText2)
	}

	// 3. 测试首次建档
	sampleNew := `
本次为中际旭创首次覆盖，AI算力光模块需求爆发，给予新论点判定。
- **本次判定**：新论点
- **核心逻辑**：800G/1.6T高速光模块全球市占率领先
`
	v3, vText3, thesis3, _, _ := api.ExtractThesisFromContent(sampleNew)
	if v3 != models.VerdictNewThesis || vText3 != "新论点" {
		t.Errorf("Expected VerdictNewThesis, got %s (%s)", v3, vText3)
	}
	if thesis3 != "800G/1.6T高速光模块全球市占率领先" {
		t.Errorf("Unexpected thesis3: %s", thesis3)
	}

	// 4. 测试带 Emoji、引用符、多标点、复杂前缀输出
	sampleEmoji := `
> ### 🎯 买方认知沉淀与假说检验
> 1. **本次判定**：🟢 逻辑强化
> 2. **核心投资逻辑**：海外算力芯片需求持续超预期，订单能见度延伸至明年
> 3. **重点跟踪催化**：关注三季报毛利率以及算力供应链出货进度
> 4. **关键证伪条件**：若北美云厂商下调资本开支预算或大客户砍单
`
	v4, vText4, thesis4, cat4, risk4 := api.ExtractThesisFromContent(sampleEmoji)
	if v4 != models.VerdictThesisStrengthen || vText4 != "逻辑强化" {
		t.Errorf("Expected VerdictThesisStrengthen, got %s (%s)", v4, vText4)
	}
	if thesis4 != "海外算力芯片需求持续超预期，订单能见度延伸至明年" {
		t.Errorf("Unexpected thesis4: %s", thesis4)
	}
	if cat4 != "关注三季报毛利率以及算力供应链出货进度" {
		t.Errorf("Unexpected cat4: %s", cat4)
	}
	if risk4 != "若北美云厂商下调资本开支预算或大客户砍单" {
		t.Errorf("Unexpected risk4: %s", risk4)
	}

	// 5. 测试逻辑证伪提取
	sampleBroken := `
### 研判总结
该股已经跌破关键趋势线，原有成长逻辑被破坏。
- 逻辑判定: 🔴 逻辑证伪
- 驱动逻辑: 行业竞争加剧导致产品价格大幅腰斩
- 证伪指标: 单季度扣非净利润转负
`
	v5, vText5, thesis5, _, risk5 := api.ExtractThesisFromContent(sampleBroken)
	if v5 != models.VerdictThesisBroken || vText5 != "逻辑证伪" {
		t.Errorf("Expected VerdictThesisBroken, got %s (%s)", v5, vText5)
	}
	if thesis5 != "行业竞争加剧导致产品价格大幅腰斩" {
		t.Errorf("Unexpected thesis5: %s", thesis5)
	}
	if risk5 != "单季度扣非净利润转负" {
		t.Errorf("Unexpected risk5: %s", risk5)
	}
}

func TestStockThesisApi_DatabaseOperations(t *testing.T) {
	if db.Dao == nil {
		t.Skip("Database not initialized")
	}

	_ = db.Dao.AutoMigrate(&models.StockThesisLedger{}, &models.StockVerdictHistory{})

	api := NewStockThesisApi()
	stockCode := "sh600584"
	stockName := "长电科技"

	// 1. 测试手动保存
	thesis := &models.StockThesisLedger{
		StockCode:    stockCode,
		StockName:    stockName,
		Verdict:      models.VerdictNewThesis,
		VerdictText:  "新论点",
		CoreThesis:   "测试核心假说",
		KeyCatalysts: "测试催化剂",
		RiskFactors:  "测试风险点",
		ModelName:    "gpt-4o",
	}

	err := api.SaveStockThesis(thesis)
	if err != nil {
		t.Fatalf("SaveStockThesis failed: %v", err)
	}

	// 2. 查询验证
	got, err := api.GetStockThesis(stockCode)
	if err != nil || got == nil {
		t.Fatalf("GetStockThesis failed: %v", err)
	}
	if got.CoreThesis != "测试核心假说" || got.Verdict != models.VerdictNewThesis {
		t.Errorf("Retrieved thesis mismatch: %+v", got)
	}

	// 3. 测试无前缀查询兼容性 ("600584")
	gotByNum, err := api.GetStockThesis("600584")
	if err != nil || gotByNum == nil {
		t.Fatalf("GetStockThesis without prefix failed: %v", err)
	}
	if gotByNum.StockCode != stockCode {
		t.Errorf("Expected stockCode %s, got %s", stockCode, gotByNum.StockCode)
	}

	// 4. 测试研报解析并更新
	mockReport := `
### 盘后综合点评
今日放量突破年线阻力位。
### 🎯 买方认知沉淀与假说检验
- **本次判定**：[逻辑强化]
- **核心逻辑**：年线突破确认右侧上升通道开启
- **验证催化**：关注后续3日缩量回踩年线支撑有效性
- **证伪条件**：跌破年线且三天内未能收复
`
	updated, err := api.ExtractAndUpdateThesisFromReport(stockCode, stockName, mockReport, "deepseek-r1", 1001)
	if err != nil {
		t.Fatalf("ExtractAndUpdateThesisFromReport failed: %v", err)
	}
	if updated.Verdict != models.VerdictThesisStrengthen || updated.VerdictText != "逻辑强化" {
		t.Errorf("Expected VerdictThesisStrengthen, got %s", updated.Verdict)
	}

	// 5. 查询历史时间线
	hist, err := api.GetStockThesisHistory(stockCode, 10)
	if err != nil {
		t.Fatalf("GetStockThesisHistory failed: %v", err)
	}
	if len(hist) < 2 {
		t.Errorf("Expected at least 2 history records, got %d", len(hist))
	}

	// 6. 验证全量映射表
	allMap, err := api.GetAllStockTheses()
	if err != nil {
		t.Fatalf("GetAllStockTheses failed: %v", err)
	}
	if _, ok := allMap[stockCode]; !ok {
		t.Errorf("Stock code %s not found in allMap", stockCode)
	}
}
