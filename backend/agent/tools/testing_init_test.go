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
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "blades-stock-agent-tools-test-")
	if err != nil {
		panic("test setup: mkdir temp: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "stock.db")
	db.Init(dbPath)

	os.Exit(m.Run())
}
