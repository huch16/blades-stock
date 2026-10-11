package agent

import (
	"testing"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

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
