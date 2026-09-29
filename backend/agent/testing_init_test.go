package agent

import (
	"os"
	"path/filepath"
	"testing"

	"blades-stock/backend/db"
)

// TestMain initializes a temp SQLite database for the duration of the test
// package run. Tests that exercise data.* APIs (which require db.Dao) run
// against this ephemeral store so the host project DB is left untouched.
//
// P1.1 启动期健康检查会在 db.Init 末尾跑 integrity_check + 关键表存在性。
// 测试环境是临时空 DB（无任何业务表），跳过健康检查避免 log.Fatal。
func TestMain(m *testing.M) {
	// 在 db.Init 前设置 env var（Init 内部会读取）
	os.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	defer os.Unsetenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK")

	tmpDir, err := os.MkdirTemp("", "blades-stock-agent-test-")
	if err != nil {
		panic("test setup: mkdir temp: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "stock.db")
	db.Init(dbPath)

	// 确保 skills 表存在（供 SkillEmbeddingService.MatchSkills 查询）
	db.Dao.Exec(`CREATE TABLE IF NOT EXISTS skills (
		id INTEGER PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		category VARCHAR(50),
		system_prompt TEXT,
		directive TEXT,
		examples TEXT,
		trigger_keywords VARCHAR(500),
		synonyms VARCHAR(500),
		tool_groups VARCHAR(500),
		mcp_server_ids VARCHAR(500),
		enable BOOLEAN DEFAULT TRUE,
		sort_order INTEGER DEFAULT 0,
		created_at TIMESTAMP,
		updated_at TIMESTAMP
	)`)

	// 插入测试技能数据（用于 P4.2 同义词扩展测试）
	db.Dao.Exec(`INSERT OR IGNORE INTO skills (id, name, trigger_keywords, synonyms, enable) VALUES (1, '测试技能', '', 'ETF,轮动', 1)`)
	db.Dao.Exec(`INSERT OR IGNORE INTO skills (id, name, trigger_keywords, synonyms, enable) VALUES (2, '技能B', '', '深市', 1)`)
	db.Dao.Exec(`INSERT OR IGNORE INTO skills (id, name, trigger_keywords, synonyms, enable) VALUES (3, '技能A', 'ETF', '轮动', 1)`)

	os.Exit(m.Run())
}
