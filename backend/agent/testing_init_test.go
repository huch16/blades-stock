package agent

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/db"
)

// TestMain initializes a temp SQLite database for the duration of the test
// package run. Tests that exercise data.* APIs (which require db.Dao) run
// against this ephemeral store so the host project DB is left untouched.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "go-stock-agent-test-")
	if err != nil {
		panic("test setup: mkdir temp: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "stock.db")
	db.Init(dbPath)

	os.Exit(m.Run())
}
