// Package db provides database connection and operations.
package db

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blades-stock/backend/paths"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Dao *gorm.DB

// BuildSqliteDSN 规范化 SQLite 连接串，注入现代 WAL 与 immediate 锁控制参数。
// 适配 glebarez/sqlite (modernc.org/sqlite) 驱动语法：
// 1. PRAGMA 必须通过 _pragma 参数传递（驱动内 applyQueryParams 只扫描 q["_pragma"]）
// 2. _txlock=immediate 确保写事务以 BEGIN IMMEDIATE 启动，杜绝多连接并发写升级死锁
func BuildSqliteDSN(rawPath string) string {
	if rawPath == "" {
		rawPath = filepath.Join(paths.DataDir(), "stock.db")
	}
	// 剥离可能存在的旧格式 query
	pathOnly := rawPath
	if idx := strings.Index(pathOnly, "?"); idx >= 0 {
		pathOnly = pathOnly[:idx]
	}
	return pathOnly + "?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-64000)&_txlock=immediate"
}

func Init(sqlitePath string) {
	dbLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second * 3,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      false,
			LogLevel:                  logger.Silent,
		},
	)
	var openDb *gorm.DB
	var err error
	dsn := BuildSqliteDSN(sqlitePath)
	openDb, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                                   dbLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              false, // 避免 prepared statement 缓存在连接池中持有表级读锁
	})

	if err != nil {
		log.Fatalf("db connection error is %s", err.Error())
	}

	// 兜底：确保连接池初始连接的 busy_timeout / WAL / synchronous / cache_size 生效
	if err := openDb.Exec("PRAGMA busy_timeout=30000").Error; err != nil {
		log.Printf("warn: set busy_timeout failed: %v", err)
	}
	if err := openDb.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		log.Printf("warn: set WAL mode failed: %v", err)
	}
	if err := openDb.Exec("PRAGMA synchronous=NORMAL").Error; err != nil {
		log.Printf("warn: set synchronous failed: %v", err)
	}
	if err := openDb.Exec("PRAGMA cache_size=-64000").Error; err != nil {
		log.Printf("warn: set cache_size failed: %v", err)
	}

	dbCon, err := openDb.DB()
	if err != nil {
		log.Fatalf("openDb.DB error is  %s", err.Error())
	}
	// 在 WAL 模式下配合 _txlock=immediate 与 busy_timeout(30000)，读写互不阻塞，写写有序等待
	dbCon.SetMaxIdleConns(5)
	dbCon.SetMaxOpenConns(10)
	dbCon.SetConnMaxLifetime(time.Hour)
	Dao = openDb

	// 1. 先行执行关键基础表自动迁移，确保新建立的数据库具备必要业务表结构
	AutoMigrate()

	// 2. 启动期健康检查：DB 文件合法 + 关键表齐全才视为可启动。
	dbPathOnly := dsn
	if idx := strings.Index(dbPathOnly, "?"); idx >= 0 {
		dbPathOnly = dbPathOnly[:idx]
	}
	if os.Getenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK") == "1" {
		log.Print("db: health check skipped (BLADES_STOCK_SKIP_DB_HEALTHCHECK=1)")
	} else if status, err := StartOfUseHealthCheck(dbPathOnly); err != nil {
		// 如果首次创建出现表缺失，尝试再次自愈迁移
		AutoMigrate()
		if status2, err2 := StartOfUseHealthCheck(dbPathOnly); err2 != nil {
			log.Printf("warn: db 启动期检查提示 (正在由上层全量迁移补齐): %v (path=%s, missing=%v)",
				err2, status2.Path, status2.MissingTables)
		} else {
			log.Printf("db: integrity_check ok after auto-migrate (path=%s, size=%d, tables=%d)",
				status2.Path, status2.FileSizeBytes, len(RequiredTables))
		}
	} else {
		log.Printf("db: integrity_check ok (path=%s, size=%d, tables=%d, took=%s)",
			status.Path, status.FileSizeBytes, len(RequiredTables), status.Duration)
	}
}
