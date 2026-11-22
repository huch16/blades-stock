package agent

import (
	"testing"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := testDB.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("failed to migrate audit_logs: %v", err)
	}
	oldDao := db.Dao
	db.Dao = testDB
	t.Cleanup(func() { db.Dao = oldDao })
	return testDB
}

func seedAuditLog(t *testing.T, testDB *gorm.DB, createdAt time.Time, toolName string) {
	t.Helper()
	if err := testDB.Create(&models.AuditLog{
		CreatedAt: createdAt,
		ToolName:  toolName,
		Success:   true,
	}).Error; err != nil {
		t.Fatalf("failed to seed audit log: %v", err)
	}
}

func countAuditLogs(t *testing.T, testDB *gorm.DB, toolName string) int64 {
	t.Helper()
	var n int64
	if err := testDB.Model(&models.AuditLog{}).Where("tool_name = ?", toolName).Count(&n).Error; err != nil {
		t.Fatalf("failed to count audit logs: %v", err)
	}
	return n
}

func TestCleanupOldAuditLogs(t *testing.T) {
	testDB := setupAuditTestDB(t)
	now := time.Now()

	seedAuditLog(t, testDB, now.Add(-100*24*time.Hour), "OldTool")   // 超期，应被清
	seedAuditLog(t, testDB, now.Add(-10*24*time.Hour), "RecentTool") // 期内，应保留
	seedAuditLog(t, testDB, now, "NowTool")                          // 期内，应保留

	deleted := CleanupOldAuditLogs(90 * 24 * time.Hour)
	if deleted != 1 {
		t.Errorf("应恰好删除 1 条超期记录，实际删除 %d 条", deleted)
	}
	if n := countAuditLogs(t, testDB, "OldTool"); n != 0 {
		t.Errorf("超期记录应被清除，残留 %d 条", n)
	}
	if n := countAuditLogs(t, testDB, "RecentTool"); n != 1 {
		t.Errorf("期内记录应保留，实际 %d 条", n)
	}
	if n := countAuditLogs(t, testDB, "NowTool"); n != 1 {
		t.Errorf("当期记录应保留，实际 %d 条", n)
	}
}

func TestCleanupOldAuditLogs_DefaultRetention(t *testing.T) {
	testDB := setupAuditTestDB(t)
	now := time.Now()

	seedAuditLog(t, testDB, now.Add(-100*24*time.Hour), "OldTool")
	seedAuditLog(t, testDB, now.Add(-10*24*time.Hour), "RecentTool")

	// retention <= 0 时用默认保留期 90 天
	deleted := CleanupOldAuditLogs(0)
	if deleted != 1 {
		t.Errorf("默认保留期（90 天）应删除 1 条超期记录，实际删除 %d 条", deleted)
	}
	if n := countAuditLogs(t, testDB, "RecentTool"); n != 1 {
		t.Errorf("期内记录应保留，实际 %d 条", n)
	}
}

func TestCleanupOldAuditLogs_NoDB(t *testing.T) {
	oldDao := db.Dao
	db.Dao = nil
	defer func() { db.Dao = oldDao }()

	if n := CleanupOldAuditLogs(90 * 24 * time.Hour); n != 0 {
		t.Errorf("无 DB 时应返回 0，实际 %d", n)
	}
}

func TestRecordAuditLog(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	if err := testDB.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("failed to migrate audit_logs: %v", err)
	}

	oldDao := db.Dao
	db.Dao = testDB
	defer func() { db.Dao = oldDao }()

	entry := models.AuditLog{
		CreatedAt:   time.Now(),
		ToolName:    "TestTool",
		ServerName:  "blades-stock",
		RequestBody: `{"code":"sz000100"}`,
		Response:    `{"price":5.20}`,
		Success:     true,
		DurationMs:  42,
	}

	if err := RecordAuditLogSync(entry); err != nil {
		t.Fatalf("RecordAuditLogSync failed: %v", err)
	}

	var found models.AuditLog
	if err := testDB.First(&found, "tool_name = ?", "TestTool").Error; err != nil {
		t.Fatalf("failed to find audit log: %v", err)
	}

	if found.DurationMs != 42 || !found.Success || found.ToolName != "TestTool" {
		t.Errorf("unexpected audit log content: %+v", found)
	}
}
