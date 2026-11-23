package agent

import (
	"testing"
	"time"

	"blades-stock/backend/db"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTraceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := testDB.AutoMigrate(&AgentTrace{}); err != nil {
		t.Fatalf("failed to migrate agent_traces: %v", err)
	}
	oldDao := db.Dao
	db.Dao = testDB
	t.Cleanup(func() { db.Dao = oldDao })
	return testDB
}

func seedTrace(t *testing.T, testDB *gorm.DB, createdAt time.Time, sessionID string) {
	t.Helper()
	if err := testDB.Create(&AgentTrace{
		SessionID: sessionID,
		Question:  "test question",
		Score:     5,
		CreatedAt: createdAt,
	}).Error; err != nil {
		t.Fatalf("failed to seed trace: %v", err)
	}
}

func countTraces(t *testing.T, testDB *gorm.DB, sessionID string) int64 {
	t.Helper()
	var n int64
	if err := testDB.Model(&AgentTrace{}).Where("session_id = ?", sessionID).Count(&n).Error; err != nil {
		t.Fatalf("failed to count traces: %v", err)
	}
	return n
}

func TestCleanupOldAgentTraces(t *testing.T) {
	testDB := setupTraceTestDB(t)
	now := time.Now()

	seedTrace(t, testDB, now.Add(-100*24*time.Hour), "old-session")   // 超期，应被清
	seedTrace(t, testDB, now.Add(-10*24*time.Hour), "recent-session") // 期内，应保留
	seedTrace(t, testDB, now, "now-session")                          // 期内，应保留

	deleted := CleanupOldAgentTraces(90 * 24 * time.Hour)
	if deleted != 1 {
		t.Errorf("应恰好删除 1 条超期轨迹，实际删除 %d 条", deleted)
	}
	if n := countTraces(t, testDB, "old-session"); n != 0 {
		t.Errorf("超期轨迹应被清除，残留 %d 条", n)
	}
	if n := countTraces(t, testDB, "recent-session"); n != 1 {
		t.Errorf("期内轨迹应保留，实际 %d 条", n)
	}
	if n := countTraces(t, testDB, "now-session"); n != 1 {
		t.Errorf("当期轨迹应保留，实际 %d 条", n)
	}
}

func TestCleanupOldAgentTraces_DefaultRetention(t *testing.T) {
	testDB := setupTraceTestDB(t)
	now := time.Now()

	seedTrace(t, testDB, now.Add(-100*24*time.Hour), "old-session")
	seedTrace(t, testDB, now.Add(-10*24*time.Hour), "recent-session")

	// retention <= 0 时用默认保留期 90 天
	deleted := CleanupOldAgentTraces(0)
	if deleted != 1 {
		t.Errorf("默认保留期（90 天）应删除 1 条超期轨迹，实际删除 %d 条", deleted)
	}
	if n := countTraces(t, testDB, "recent-session"); n != 1 {
		t.Errorf("期内轨迹应保留，实际 %d 条", n)
	}
}

func TestCleanupOldAgentTraces_NoDB(t *testing.T) {
	oldDao := db.Dao
	db.Dao = nil
	defer func() { db.Dao = oldDao }()

	if n := CleanupOldAgentTraces(90 * 24 * time.Hour); n != 0 {
		t.Errorf("无 DB 时应返回 0，实际 %d", n)
	}
}
