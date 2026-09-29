package tools

import (
	"os"
	"path/filepath"
	"testing"

	"blades-stock/backend/db"
)

// TestMain initializes a temp SQLite database for tests in this package
// because most tool constructors (and their underlying data.* APIs) read
// settings/AI configs from db.Dao on first call.
//
// P1.1 启动期健康检查会在 db.Init 末尾跑，跳过以兼容临时空 DB。
func TestMain(m *testing.M) {
	os.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	defer os.Unsetenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK")

	tmpDir, err := os.MkdirTemp("", "blades-stock-agent-tools-test-")
	if err != nil {
		panic("test setup: mkdir temp: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "stock.db")
	db.Init(dbPath)

	os.Exit(m.Run())
}
