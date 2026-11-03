package agent

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"blades-stock/backend/paths"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupTestDistillerDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "test_distiller.db")
	_ = os.Setenv("BLADES_STOCK_DATA_DIR", testDir)
	_ = os.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")

	testDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test sqlite db: %v", err)
	}

	// 迁移测试所需的精炼表结构及系统配置基础表
	err = testDB.AutoMigrate(
		&data.Settings{},
		&data.AIConfig{},
		&KnowledgeCrystal{},
		&models.Telegraph{},
		&models.BacktestResult{},
		&models.AIResponseResult{},
		&AgentMemory{},
		&AgentSession{},
	)
	if err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}

	origDB := db.Dao
	db.Dao = testDB

	t.Cleanup(func() {
		sqlDB, err := testDB.DB()
		if err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
		db.Dao = origDB
	})

	return testDB
}

func TestKnowledgeCrystalVectorPacking(t *testing.T) {
	origVec := []float32{0.123, -0.456, 0.789, 1.0, -1.0, 0.0}
	packed := Float32sToBytesLE(origVec)
	if len(packed) != len(origVec)*4 {
		t.Fatalf("packed byte slice length = %d, want %d", len(packed), len(origVec)*4)
	}

	unpacked := BytesToFloat32sLE(packed)
	if len(unpacked) != len(origVec) {
		t.Fatalf("unpacked length = %d, want %d", len(unpacked), len(origVec))
	}

	for i := range origVec {
		if math.Abs(float64(origVec[i]-unpacked[i])) > 1e-6 {
			t.Errorf("mismatch at index %d: orig=%f, unpacked=%f", i, origVec[i], unpacked[i])
		}
	}

	crystal := &KnowledgeCrystal{}
	crystal.SetVector(origVec)
	if crystal.VectorDim != len(origVec) {
		t.Errorf("crystal.VectorDim = %d, want %d", crystal.VectorDim, len(origVec))
	}

	vecOut := crystal.GetVector()
	if len(vecOut) != len(origVec) {
		t.Fatalf("crystal.GetVector() length = %d, want %d", len(vecOut), len(origVec))
	}
}

func TestComputeContentHash(t *testing.T) {
	h1 := ComputeContentHash("etf_rotation", "标题A", "内容摘要A")
	h2 := ComputeContentHash("etf_rotation", "标题A", "内容摘要A")
	h3 := ComputeContentHash("stock_resonance", "标题A", "内容摘要A")

	if h1 != h2 {
		t.Errorf("identical content should produce identical hash: h1=%s, h2=%s", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("different category should produce different hash: h1=%s, h3=%s", h1, h3)
	}
}

func TestDistillMarketNews(t *testing.T) {
	setupTestDistillerDB(t)
	ctx := context.Background()

	// 插入测试快讯：包括 3 条关于同一主题的重复快讯和 1 条独立快讯
	now := time.Now()
	items := []models.Telegraph{
		{
			Title:   "中国人民银行宣布降准0.5个百分点 释放长期流动性约1万亿元",
			Content: "央行今天下午正式发布公告，决定下调金融机构存款准备金率0.5个百分点，向市场注入宽裕流动性。",
			Time:    now.Format("15:04:05"),
		},
		{
			Title:   "央行宣布降准0.5个百分点 释放流动性1万亿元", // 重复/包含
			Content: "快讯：央行降准0.5%，释放流动性。",
			Time:    now.Format("15:04:05"),
		},
		{
			Title:   "【重磅】中国人民银行宣布降准0.5个百分点", // 重复/包含
			Content: "央行全面降准0.5个百分点，今日实施。",
			Time:    now.Format("15:04:05"),
		},
		{
			Title:   "半导体产业供应链关键设备自主替代率稳步提升", // 独立主题
			Content: "根据最新调研，半导体清洗设备与刻蚀设备国内市占率取得重大突破。",
			Time:    now.Format("15:04:05"),
		},
	}
	for i := range items {
		_ = db.Dao.Create(&items[i])
	}

	distiller := NewSemanticDistiller(nil)
	pruned, clusters, err := distiller.DistillMarketNews(ctx, 24)
	if err != nil {
		t.Fatalf("DistillMarketNews failed: %v", err)
	}

	if clusters < 2 {
		t.Errorf("expected at least 2 distinct clusters, got %d", clusters)
	}
	_ = pruned

	var remaining []models.Telegraph
	db.Dao.Find(&remaining)
	// 确保杜绝删新闻负优化：保留全部原始真实客观快讯，不作物理硬删除
	if len(remaining) < 4 {
		t.Errorf("expected all 4 items preserved without destructive pruning, got %d", len(remaining))
	}
}

func TestDistillAIReportsAndBacktests(t *testing.T) {
	setupTestDistillerDB(t)
	ctx := context.Background()

	// 1. 模拟 AI 研报与已验证的高胜率回测记录
	resp := models.AIResponseResult{
		ChatId:    "chat_test_001",
		ModelName: "deepseek-chat",
		StockCode: "002906",
		StockName: "华阳集团",
		Question:  "002906 是否出现均线多头共振？",
		Content:   "华阳集团在智能座舱HUD具备极高市场份额，当前均线多头排列，量价配合理想，看好后续突破。",
	}
	_ = db.Dao.Create(&resp)

	bt := models.BacktestResult{
		ReportID:     resp.ID,
		StockCode:    "002906",
		StockName:    "华阳集团",
		AnalysisDate: "2026-10-10",
		Direction:    "看涨",
		TargetPrice:  38.5,
		VerifyDate:   "2026-10-15",
		VerifyPrice:  41.2,
		PriceChange:  7.8, // 实际涨幅显著
		MaxUpside:    8.5,
		MaxDrawdown:  1.2,
		IsAccurate:   true,
		Confidence:   0.88,
		Summary:      "均线突破平台回踩确认，主力资金持续净买入，成功达到目标位。",
	}
	_ = db.Dao.Create(&bt)

	distiller := NewSemanticDistiller(nil)
	created, prunedRaw, err := distiller.DistillAIReportsAndBacktests(ctx, 60.0)
	if err != nil {
		t.Fatalf("DistillAIReportsAndBacktests failed: %v", err)
	}

	if created != 1 {
		t.Fatalf("expected 1 knowledge crystal created, got %d", created)
	}

	// 验证结晶数据
	var crystals []KnowledgeCrystal
	db.Dao.Find(&crystals)
	if len(crystals) != 1 {
		t.Fatalf("expected 1 crystal in db, got %d", len(crystals))
	}

	c := crystals[0]
	if c.Category != "stock_resonance" {
		t.Errorf("crystal.Category = %s, want stock_resonance", c.Category)
	}
	if c.AccuracyScore < 80.0 {
		t.Errorf("crystal.AccuracyScore = %f, want >= 80.0", c.AccuracyScore)
	}

	// 重复调用，哈希防重机制应拦截二次创建
	created2, _, _ := distiller.DistillAIReportsAndBacktests(ctx, 60.0)
	if created2 != 0 {
		t.Errorf("second call should create 0 crystals due to deduplication, got %d", created2)
	}
	_ = prunedRaw
}

func TestDistillAgentMemories(t *testing.T) {
	setupTestDistillerDB(t)
	ctx := context.Background()

	// 插入老旧的 working memory
	agedTime := time.Now().AddDate(0, 0, -10)
	sessionID := "sess_test_123"

	for i := 0; i < 5; i++ {
		mem := AgentMemory{
			SessionID: sessionID,
			Role:      "user",
			Content:   fmt.Sprintf("用户关于标的 %d 的咨询问答", i),
			Category:  "working",
			CreatedAt: agedTime,
		}
		_ = db.Dao.Create(&mem)
	}

	distiller := NewSemanticDistiller(nil)
	pruned, factsCreated, err := distiller.DistillAgentMemories(ctx, 7)
	if err != nil {
		t.Fatalf("DistillAgentMemories failed: %v", err)
	}

	if pruned != 5 {
		t.Errorf("expected 5 aged memories pruned, got %d", pruned)
	}
	if factsCreated != 1 {
		t.Errorf("expected 1 fact created from folded session, got %d", factsCreated)
	}

	var remainingWorking []AgentMemory
	db.Dao.Where("category = ?", "working").Find(&remainingWorking)
	if len(remainingWorking) != 0 {
		t.Errorf("expected 0 working memories remaining, got %d", len(remainingWorking))
	}

	var facts []AgentMemory
	db.Dao.Where("category = ?", "fact").Find(&facts)
	if len(facts) != 1 {
		t.Errorf("expected 1 fact memory created, got %d", len(facts))
	}
}

func TestSearchKnowledgeCrystalsAndPromptFormatting(t *testing.T) {
	setupTestDistillerDB(t)
	ctx := context.Background()

	// 插入两组经验结晶
	c1 := KnowledgeCrystal{
		Category:       "etf_rotation",
		Title:          "ETF动量轮动双均线滤波规律",
		TriggerPattern: "ETF轮动 | MA5突破",
		Summary:        "基于21日纯数学动量矩阵与R²置信度打分，胜率83.5%。",
		KeyInsights:    "动量因子60%，R²置信度40%。",
		AccuracyScore:  83.5,
		HitCount:       3,
		ContentHash:    ComputeContentHash("etf_rotation", "ETF动量轮动双均线滤波规律", "基于21日纯数学动量矩阵与R²置信度打分，胜率83.5%。"),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	c2 := KnowledgeCrystal{
		Category:       "risk_warning",
		Title:          "高位缩量背离出货风险警示",
		TriggerPattern: "股价新高 | 量价背离",
		Summary:        "股价创阶段新高但换手率骤降且龙虎榜机构席位卖出，多为诱多陷阱。",
		KeyInsights:    "跌破突破阳线实体底部严格止损。",
		AccuracyScore:  85.0,
		HitCount:       5,
		ContentHash:    ComputeContentHash("risk_warning", "高位缩量背离出货风险警示", "股价创阶段新高但换手率骤降且龙虎榜机构席位卖出，多为诱多陷阱。"),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	_ = db.Dao.Create(&c1)
	_ = db.Dao.Create(&c2)

	distiller := NewSemanticDistiller(nil)

	// 搜索 ETF 轮动
	results, err := distiller.SearchKnowledgeCrystals(ctx, "ETF 轮动 均线", "etf_rotation", 3)
	if err != nil {
		t.Fatalf("SearchKnowledgeCrystals failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 matching crystal for etf_rotation, got %d", len(results))
	}
	if results[0].Title != c1.Title {
		t.Errorf("expected title %s, got %s", c1.Title, results[0].Title)
	}

	// 验证 Prompt 格式化
	prompt := distiller.FormatCrystalsForPrompt(results)
	if prompt == "" {
		t.Fatalf("FormatCrystalsForPrompt returned empty string")
	}
	if !containsString(prompt, "ETF动量轮动双均线滤波规律") {
		t.Errorf("prompt missing crystal title: %s", prompt)
	}
	if !containsString(prompt, "83.5%") {
		t.Errorf("prompt missing accuracy score: %s", prompt)
	}
}

func TestRunMaintenanceCycle(t *testing.T) {
	setupTestDistillerDB(t)
	ctx := context.Background()

	distiller := NewSemanticDistiller(nil)
	stats, err := distiller.RunMaintenanceCycle(ctx)
	if err != nil {
		t.Fatalf("RunMaintenanceCycle failed: %v", err)
	}

	if stats == nil {
		t.Fatalf("expected stats non-nil")
	}
	if stats.CompressionRatio <= 0 {
		t.Errorf("expected positive compression ratio, got %f", stats.CompressionRatio)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && filepath.Base(s) != "" && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var _ = paths.DataDir
