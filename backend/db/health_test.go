package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTempDB 在 t.TempDir() 下建一个临时 sqlite DB，写入 schema 由 setup 控制。
// 返回的路径可被 StartOfUseHealthCheck / QuickCheckOnly 复用。
//
// 故意不用 WAL journal 模式：WAL 会留下 -wal / -shm 文件，Windows 测试 cleanup 时
// 容易撞"另一个进程正在使用"导致 RemoveAll 失败。非生产路径用 DELETE 足够。
func newTempDB(t *testing.T, setup func(*gorm.DB)) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "stock.db")

	g, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open temp db failed: %v", err)
	}
	if setup != nil {
		setup(g)
	}
	// 关闭连接以让 quick_check 在独立连接上跑
	sqlDB, _ := g.DB()
	if sqlDB != nil {
		_ = sqlDB.Close()
	}
	return dbPath
}

// makeMissingTablesSchema 仅创建部分表，模拟 "auto migrate 没跑完 / 升级失败"。
func makeMissingTablesSchema(g *gorm.DB) {
	type Stub struct {
		ID uint `gorm:"primaryKey"`
	}
	g.AutoMigrate(&Stub{}) // 仅建一张空表，让 RequiredTables 缺失
}

func TestStartOfUseHealthCheck_OK(t *testing.T) {
	dbPath := newTempDB(t, func(g *gorm.DB) {
		// 建齐所有 RequiredTables，模拟"刚跑完 AutoMigrate"的状态
		for _, tbl := range RequiredTables {
			g.Exec("CREATE TABLE IF NOT EXISTS " + tbl + " (id INTEGER PRIMARY KEY)")
		}
	})

	status, err := StartOfUseHealthCheck(dbPath)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if status == nil {
		t.Fatal("status should not be nil")
	}
	if !status.FileExists {
		t.Error("FileExists should be true")
	}
	if status.FileSizeBytes <= 0 {
		t.Errorf("FileSizeBytes should be > 0, got %d", status.FileSizeBytes)
	}
	if !strings.HasPrefix(status.QuickCheck, "ok") {
		t.Errorf("QuickCheck should start with 'ok', got %q", status.QuickCheck)
	}
	if len(status.MissingTables) != 0 {
		t.Errorf("MissingTables should be empty, got %v", status.MissingTables)
	}
	if status.Duration <= 0 {
		t.Error("Duration should be > 0")
	}
}

func TestStartOfUseHealthCheck_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.db")

	status, err := StartOfUseHealthCheck(missing)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if status == nil {
		t.Fatal("status should not be nil even on error")
	}
	if status.FileExists {
		t.Error("FileExists should be false")
	}
	// 文件不存在时不应尝试 SQL 层检查
	if status.QuickCheck != "" {
		t.Errorf("QuickCheck should be empty when file missing, got %q", status.QuickCheck)
	}
}

func TestStartOfUseHealthCheck_FileTooSmall(t *testing.T) {
	dir := t.TempDir()
	tiny := filepath.Join(dir, "tiny.db")
	if err := os.WriteFile(tiny, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	status, err := StartOfUseHealthCheck(tiny)
	if err == nil {
		t.Fatal("expected error for non-sqlite file, got nil")
	}
	if status == nil {
		t.Fatal("status should not be nil")
	}
	if !strings.Contains(err.Error(), "文件过小") {
		t.Errorf("expected '文件过小' in error, got: %v", err)
	}
}

func TestStartOfUseHealthCheck_BadFileHeader(t *testing.T) {
	// 写一个足够大但不是 SQLite 格式的文件
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.db")
	garbage := make([]byte, 1024)
	for i := range garbage {
		garbage[i] = byte(i % 256)
	}
	if err := os.WriteFile(bad, garbage, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := StartOfUseHealthCheck(bad)
	if err == nil {
		t.Fatal("expected error for bad header, got nil")
	}
	if !strings.Contains(err.Error(), "文件头") {
		t.Errorf("expected '文件头' in error, got: %v", err)
	}
}

func TestStartOfUseHealthCheck_MissingTables(t *testing.T) {
	// 建表但漏掉 RequiredTables 中的若干
	dbPath := newTempDB(t, func(g *gorm.DB) {
		// 仅建 1 张表
		g.Exec("CREATE TABLE IF NOT EXISTS skills (id INTEGER PRIMARY KEY)")
	})

	status, err := StartOfUseHealthCheck(dbPath)
	if err == nil {
		t.Fatal("expected error for missing tables, got nil")
	}
	if !strings.Contains(err.Error(), "关键表缺失") {
		t.Errorf("expected '关键表缺失' in error, got: %v", err)
	}
	// 应该至少报 4 个缺失（除 skills 外的全部）
	if len(status.MissingTables) < 4 {
		t.Errorf("expected ≥4 missing tables, got %d: %v", len(status.MissingTables), status.MissingTables)
	}
}

func TestStartOfUseHealthCheck_NoTables(t *testing.T) {
	// 极端情况：DB 文件合法但完全空（无任何表）
	// 建一个空 DB：glebarez/sqlite 创建文件后仅含 header + sqlite_schema 元数据。
	dbPath := newTempDB(t, func(g *gorm.DB) {
		// 创建一个临时表让 DB 真正落地，然后删除
		g.Exec("CREATE TABLE _ephemeral (id INTEGER)")
		g.Exec("DROP TABLE _ephemeral")
	})

	_, err := StartOfUseHealthCheck(dbPath)
	if err == nil {
		t.Fatal("expected error for empty DB, got nil")
	}
	if !strings.Contains(err.Error(), "关键表缺失") {
		t.Errorf("expected '关键表缺失' in error, got: %v", err)
	}
}

func TestQuickCheckOnly_OK(t *testing.T) {
	dbPath := newTempDB(t, func(g *gorm.DB) {
		g.Exec("CREATE TABLE foo (id INTEGER PRIMARY KEY)")
	})
	if err := QuickCheckOnly(dbPath); err != nil {
		t.Errorf("QuickCheckOnly should succeed on valid DB, got: %v", err)
	}
}

func TestQuickCheckOnly_MissingFile(t *testing.T) {
	if err := QuickCheckOnly("/nonexistent/path/foo.db"); err == nil {
		t.Error("QuickCheckOnly should fail on missing file")
	}
}

func TestTouchOpen_NilDB(t *testing.T) {
	if err := TouchOpen(nil); err == nil {
		t.Error("TouchOpen(nil) should return error")
	}
}

func TestTouchOpen_ValidDB(t *testing.T) {
	dbPath := newTempDB(t, nil)
	g, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := TouchOpen(g); err != nil {
		t.Errorf("TouchOpen on valid gorm.DB should succeed, got: %v", err)
	}
	// 显式关闭连接池，释放 Windows 文件锁让 t.TempDir 清理不报错
	sqlDB, _ := g.DB()
	if sqlDB != nil {
		_ = sqlDB.Close()
	}
}

func TestResolveDBPath(t *testing.T) {
	got := ResolveDBPath("/tmp/foo")
	want := filepath.Join("/tmp/foo", "data", "stock.db")
	if got != want {
		t.Errorf("ResolveDBPath = %q, want %q", got, want)
	}

	got = ResolveDBPath("")
	// 空 baseDir 时回退 cwd，结果不应为空
	if got == "" {
		t.Error("ResolveDBPath(\"\") should fallback to cwd, got empty")
	}
	if !filepath.IsAbs(got) {
		t.Errorf("ResolveDBPath result should be absolute, got %q", got)
	}
}

func TestRequiredTables_NotEmpty(t *testing.T) {
	// 防止 RequiredTables 被误清空
	if len(RequiredTables) == 0 {
		t.Fatal("RequiredTables must not be empty")
	}
	// 必须包含 5 张关键表（P1.1 规约要求）
	must := map[string]bool{
		"skills": false, "mcp_servers": false, "project_analysis": false,
		"code_chunks": false, "agent_traces": false,
	}
	for _, tbl := range RequiredTables {
		if _, ok := must[tbl]; ok {
			must[tbl] = true
		}
	}
	for name, ok := range must {
		if !ok {
			t.Errorf("RequiredTables missing critical table %q", name)
		}
	}
}