package data

import (
	"os"
	"path/filepath"
	"testing"

	backenddata "blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// TestMain 与父包 backend/agent/tools/testing_init_test.go 同款：
// 给本包测试配一个临时 SQLite 空库（真实 data/stock.db 绝不被碰）。
// P1.1 启动期健康检查会在 db.Init 末尾跑，跳过以兼容临时空 DB。
func TestMain(m *testing.M) {
	os.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	defer os.Unsetenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK")

	tmpDir, err := os.MkdirTemp("", "blades-stock-agent-tools-data-test-")
	if err != nil {
		panic("test setup: mkdir temp: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "stock.db")
	db.Init(dbPath)

	_ = db.Dao.AutoMigrate(
		&backenddata.Settings{},
		&backenddata.AIConfig{},
		&backenddata.StockBasic{},
		&models.AllStockInfo{},
		&models.StockThesisLedger{},
		&models.StockVerdictHistory{},
		&models.AiRecommendStocks{},
		&models.AIResponseResult{},
	)

	os.Exit(m.Run())
}
