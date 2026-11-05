// Package db 的启动期健康检查模块。
//
// 设计目标：在 db.Init() 末尾、Dao 赋值之后调用 StartOfUseHealthCheck，
// 把"DB 文件能否打开 / 是否损坏 / 关键表是否齐全"做成硬门槛，
// 避免"启动成功但第一次业务查询才暴露"导致线上 silent failure。
//
// 包含的检查项：
//  1. 物理文件可读 + 文件大小 > 0
//  2. SQLITE_HEADER 合法（文件头是 SQLite 格式）
//  3. PRAGMA quick_check（毫秒级全库分页校验）
//  4. 关键业务表存在性（按 P1.1 规约）
//
// 注意事项：
//   - 必须在 Dao 赋值之后调用（确保 PRAGMA 在同一连接上下文生效）
//   - 仅使用只读 PRAGMA，不修改任何表数据
//   - 失败一律 error 返回；调用方决定是 log.Fatal 还是降级
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// 触发 glebarez/go-sqlite 的 init()，注册名为 "sqlite" 的 database/sql 驱动。
	// db 包其他文件已通过 gorm.Open 使用，但 health.go 是自包含模块，必须显式 import。
	_ "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// HealthStatus 描述一次健康检查的结果，用于日志与监控。
type HealthStatus struct {
	Path          string
	FileExists    bool
	FileSizeBytes int64
	QuickCheck    string
	Duration      time.Duration
	MissingTables []string
}

// RequiredTables 是 Phase 1.1 规约要求校验的关键业务表。
// 表名必须是项目已有的表（参见 main.go AutoMigrate 与各 model 文件）。
// 新增关键表时同步追加；非关键表（缓存表、日志表）不参与校验。
//
// 注意：GORM 默认按 struct 名小写复数生成表名，但部分 model 有自定义 TableName()：
//   - models.MCPServer → "mcp_servers"（注意下划线）
//   - models.CodeChunk → "code_chunks"
//   - models.ProjectAnalysis → "project_analysis"
//   - models.Skill → "skills"
var RequiredTables = []string{
	"skills",           // Skill 模型 — ETF 轮动 / Serenity 产业分析
	"mcp_servers",      // MCPServer 模型 — 外部工具配置（带下划线）
	"project_analysis", // ProjectAnalysis — 学习项目索引
	"code_chunks",      // CodeChunk — 项目源码切片
	"agent_traces",     // Agent 链路追踪
}

// StartOfUseHealthCheck 在 db.Init 末尾调用：对指定 DB 文件做启动期健康检查。
//
// 设计要点：
//   - 单独打开一个只读连接，避免与 Dao 的连接池竞争
//   - 先做文件层检查（fs、文件头），再做 SQL 层检查
//   - 任何一项失败立即返回 error，由调用方决定是否 Fatal
//
// 返回值：
//   - status 非 nil 时包含详细检查结果（即使 error 也有，便于日志）
//   - error 非 nil 时至少有一项检查失败
func StartOfUseHealthCheck(dbPath string) (*HealthStatus, error) {
	start := time.Now()
	status := &HealthStatus{Path: dbPath}

	// 1. 文件存在性 + 大小
	info, err := os.Stat(dbPath)
	if err != nil {
		status.FileExists = false
		return status, fmt.Errorf("db 文件不存在: %w", err)
	}
	status.FileExists = true
	status.FileSizeBytes = info.Size()
	if info.Size() < 100 {
		// SQLite 文件头本身约 100 字节，小于此值一定不是合法 DB
		return status, fmt.Errorf("db 文件过小（%d 字节），不是合法 SQLite 数据库", info.Size())
	}

	// 2. 文件头 magic 校验：避免对错误文件做 SQL 查询浪费时间
	if err := checkSQLiteHeader(dbPath); err != nil {
		return status, err
	}

	// 3. 用只读连接做 SQL 层校验（独立连接，避免影响 Dao 的连接池配置）
	if err := checkSQLiteTables(dbPath, status); err != nil {
		return status, err
	}

	status.Duration = time.Since(start)
	return status, nil
}

// checkSQLiteHeader 校验文件前 16 字节是否为 SQLite 标准头"SQLite format 3\x00"。
// 这是最便宜的反损坏检测，无需打开 SQL 连接。
func checkSQLiteHeader(dbPath string) error {
	f, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("打开 db 文件失败: %w", err)
	}
	defer f.Close()

	header := make([]byte, 16)
	n, err := f.Read(header)
	if err != nil {
		return fmt.Errorf("读取 db 文件头失败: %w", err)
	}
	if n < 16 {
		return fmt.Errorf("db 文件头截断（仅 %d 字节）", n)
	}
	expected := "SQLite format 3\x00"
	if string(header) != expected {
		return fmt.Errorf("db 文件头不匹配 SQLite 格式（实际前 16 字节: %q）", string(header))
	}
	return nil
}

// checkSQLiteTables 用独立只读连接做表存在性 + quick_check。
// 同时填 status.QuickCheck 与 status.MissingTables 便于日志。
func checkSQLiteTables(dbPath string, status *HealthStatus) error {
	// 用 sqlite 驱动而非 glebarez/sqlite — 更轻量、不需要 ORM
	// DSN 加 mode=ro 避免任何意外写入
	roDSN := dbPath + "?mode=ro&_pragma=busy_timeout(2000)"
	conn, err := sql.Open("sqlite", roDSN)
	if err != nil {
		return fmt.Errorf("打开只读连接失败: %w", err)
	}
	defer conn.Close()

	// quick_check：毫秒级全库校验；失败时返回非 ok 字符串
	var quickMsg string
	if err := conn.QueryRow("PRAGMA quick_check").Scan(&quickMsg); err != nil {
		return fmt.Errorf("PRAGMA quick_check 执行失败: %w", err)
	}
	status.QuickCheck = quickMsg
	if !strings.HasPrefix(quickMsg, "ok") {
		return fmt.Errorf("PRAGMA quick_check 返回: %q（DB 损坏）", quickMsg)
	}

	// 关键表存在性
	missing := make([]string, 0, len(RequiredTables))
	for _, tbl := range RequiredTables {
		var n int
		err := conn.QueryRow(
			"SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?",
			tbl,
		).Scan(&n)
		if err != nil {
			missing = append(missing, tbl)
			continue
		}
		if n == 0 {
			missing = append(missing, tbl)
		}
	}
	status.MissingTables = missing
	if len(missing) > 0 {
		return fmt.Errorf("关键表缺失: %v（DB schema 未初始化或升级失败）", missing)
	}
	return nil
}

// QuickCheckOnly 是 StartOfUseHealthCheck 的轻量版本，只做 quick_check 不校验表。
// 用于 health endpoint / 周期性后台巡检。
func QuickCheckOnly(dbPath string) error {
	conn, err := sql.Open("sqlite", dbPath+"?mode=ro&_pragma=busy_timeout(2000)")
	if err != nil {
		return err
	}
	defer conn.Close()
	var msg string
	if err := conn.QueryRow("PRAGMA quick_check").Scan(&msg); err != nil {
		return err
	}
	if !strings.HasPrefix(msg, "ok") {
		return fmt.Errorf("quick_check failed: %s", msg)
	}
	return nil
}

// TouchOpen 仅做"能打开且能 ping"的最轻检测，用于 Dao 赋值后的烟雾测试。
// 不替代 StartOfUseHealthCheck。
func TouchOpen(d *gorm.DB) error {
	if d == nil {
		return errors.New("dao is nil")
	}
	sqlDB, err := d.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Ping(); err != nil {
		return err
	}
	// 跑一个无害的查询确认连接可读
	var one int
	return sqlDB.QueryRow("SELECT 1").Scan(&one)
}

// ResolveDBPath 把 paths 层的 baseDir 与可选 buildBinPath 解析成绝对路径。
// 把路径解析逻辑集中在这里便于测试和复用，避免在 health / pathguard 之间重复。
func ResolveDBPath(baseDir string) string {
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}
	return filepath.Join(baseDir, "data", "stock.db")
}
