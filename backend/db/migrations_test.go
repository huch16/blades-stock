package db

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunVersionedMigrations(t *testing.T) {
	// 使用内存数据库验证迁移引擎
	testDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("skipping test in pure Go mode (no cgo): %v", err)
		return
	}

	// 模拟预置表
	err = testDB.Exec("CREATE TABLE settings (id INTEGER PRIMARY KEY, name TEXT)").Error
	if err != nil {
		t.Fatalf("failed to create dummy table: %v", err)
	}

	// 1. 首次执行版本化迁移
	if err := RunVersionedMigrations(testDB); err != nil {
		t.Fatalf("RunVersionedMigrations failed: %v", err)
	}

	// 验证 schema_migrations 是否记录了版本
	var count int64
	testDB.Model(&SchemaMigration{}).Count(&count)
	if count < 2 {
		t.Errorf("expected at least 2 migrations recorded, got %d", count)
	}

	// 验证 mcp_timeout 列是否被添加
	if !testDB.Migrator().HasColumn("settings", "mcp_timeout") {
		t.Errorf("expected column mcp_timeout to exist in settings table")
	}

	// 2. 幂等性测试：再次执行应无报错且无重复记录
	if err := RunVersionedMigrations(testDB); err != nil {
		t.Fatalf("second RunVersionedMigrations failed: %v", err)
	}
	var countAfter int64
	testDB.Model(&SchemaMigration{}).Count(&countAfter)
	if countAfter != count {
		t.Errorf("migrations count changed on replay: %d != %d", countAfter, count)
	}
}
