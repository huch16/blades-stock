package quant

import (
	"context"
	"testing"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"blades-stock/backend/paths"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRunAIReportBacktest_NoReports(t *testing.T) {
	// Use in-memory SQLite for isolation
	sqlitePath := ":memory:"
	dbPath := sqlitePath + "?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288"
	if err := db.Init(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
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
	mgr.loader = &MockStockDataLoader{}

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

// MockStockDataLoader returns empty price data for any request
type MockStockDataLoader struct{}

func (m *MockStockDataLoader) LoadKLineData(code string, days int) ([]float64, []float64, []float64, []float64, []float64, []string, error) {
	return nil, nil, nil, nil, nil, nil, nil
}

func (m *MockStockDataLoader) BatchLoadStockData(codes []string, days int) map[string]*stockPriceData {
	return make(map[string]*stockPriceData)
}