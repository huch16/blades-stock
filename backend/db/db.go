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
	if sqlitePath == "" {
		sqlitePath = filepath.Join(paths.DataDir(), "stock.db?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288")
	}
	openDb, err = gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		Logger:                                   dbLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
	})

	if err != nil {
		log.Fatalf("db connection error is %s", err.Error())
	}

	// 兜底：确保 busy_timeout / WAL / synchronous 生效（不同驱动/DSN 参数支持可能存在差异）
	if err := openDb.Exec("PRAGMA busy_timeout=10000").Error; err != nil {
		log.Printf("warn: set busy_timeout failed: %v", err)
	}
	if err := openDb.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		log.Printf("warn: set WAL mode failed: %v", err)
	}
	if err := openDb.Exec("PRAGMA synchronous=NORMAL").Error; err != nil {
		log.Printf("warn: set synchronous failed: %v", err)
	}

	dbCon, err := openDb.DB()
	if err != nil {
		log.Fatalf("openDb.DB error is  %s", err.Error())
	}
	// SQLite 写入是单写入者并发模型：设置 MaxOpenConns(1) 能彻底消除应用层不同连接争用锁造成的 SQLITE_BUSY
	dbCon.SetMaxIdleConns(1)
	dbCon.SetMaxOpenConns(1)
	dbCon.SetConnMaxLifetime(time.Hour)
	Dao = openDb

	// 1. 先行执行关键基础表自动迁移，确保新建立的数据库具备必要业务表结构
	AutoMigrate()

	// 2. 启动期健康检查：DB 文件合法 + 关键表齐全才视为可启动。
	// sqlitePath 可能包含 DSN 后缀（?_busy_timeout=...），健康检查只关心物理文件路径，
	// 所以先剥离 ? 及其后内容。
	dbPathOnly := sqlitePath
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
