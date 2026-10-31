package main

import (
	"os"
	"path/filepath"
	"testing"

	"blades-stock/backend/db"
)

// TestMain initializes a temp SQLite database for the root package tests so
// that NewApp() and other data.* APIs that read settings on first call have
// a valid db.Dao to talk to.
func TestMain(m *testing.M) {
	os.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	defer os.Unsetenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK")

	tmpDir, err := os.MkdirTemp("", "blades-stock-root-test-")
	if err != nil {
		panic("test setup: mkdir temp: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "stock.db")
	db.Init(dbPath)
	AutoMigrate()

	os.Exit(m.Run())
}
