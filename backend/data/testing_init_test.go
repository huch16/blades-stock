package data

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/db"
)

// TestMain initializes the SQLite test database once per package run so that
// the existing test files (which call data.NewStockDataApi, GetSettingConfig,
// etc.) can rely on a populated `db.Dao`.
//
// We point the database at a temp file (rather than ./data/stock.db) so the
// host project database is never mutated by `go test`.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "go-stock-test-")
	if err != nil {
		panic("test setup: mkdir temp: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "stock.db")
	db.Init(dbPath)

	os.Exit(m.Run())
}
