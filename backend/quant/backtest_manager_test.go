package quant

import (
	"context"
	"os"
	"testing"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// TestRunAIReportBacktest_NoReports 测试空报告场景下回测管理器行为。
// 使用内存 SQLite + 跳过启动期健康检查，避免依赖真实数据库文件。
func TestRunAIReportBacktest_NoReports(t *testing.T) {
	// Use in-memory SQLite for isolation
	sqlitePath := ":memory:"
	dbPath := sqlitePath + "?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288"

	// 测试使用空 DB，跳过启动期文件健康检查
	os.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	db.Init(dbPath)
	defer func() {
		if db.Dao != nil {
			sqlDB, err := db.Dao.DB()
			if err != nil {
				t.Errorf("failed to get sql db: %v", err)
			}
			sqlDB.Close()
		}
	}()

	// Auto migrate needed tables
	if err := db.Dao.AutoMigrate(&models.AIResponseResult{}, &models.BacktestResult{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	mgr := NewBacktestManager()
	// Replace loader with a mock that returns empty data to avoid needing real price data
	mgr.loader = &mockStockDataLoader{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := mgr.RunAIReportBacktest(ctx, 0)
	if err != nil {
		t.Errorf("RunAIReportBacktest returned error: %v", err)
	}
	if result == nil {
		t.Errorf("RunAIReportBacktest returned nil result")
	}
	if result.Total != 0 {
		t.Errorf("expected Total=0, got %d", result.Total)
	}
}

// mockStockDataLoader implements StockDataLoaderInterface for testing.
// It returns empty data so that backtest operations skip price lookup.
type mockStockDataLoader struct{}

func (m *mockStockDataLoader) LoadKLineData(code string, days int) ([]float64, []float64, []float64, []float64, []float64, []string, error) {
	return nil, nil, nil, nil, nil, nil, nil
}
