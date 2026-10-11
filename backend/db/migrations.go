package db

import (
	"fmt"
	"time"

	"blades-stock/backend/logger"
	"gorm.io/gorm"
)

// SchemaMigration 记录已执行的版本化数据库迁移
type SchemaMigration struct {
	Version     int64     `gorm:"primaryKey"`
	Description string    `gorm:"size:255;not null"`
	AppliedAt   time.Time `gorm:"not null"`
}

func (SchemaMigration) TableName() string {
	return "schema_migrations"
}

// MigrationStep 定义单个版本化迁移任务
type MigrationStep struct {
	Version     int64
	Description string
	Migrate     func(tx *gorm.DB) error
}

var migrationRegistry = []MigrationStep{
	{
		Version:     20260401001,
		Description: "ensure mcp_timeout column in settings",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn("settings", "mcp_timeout") {
				return tx.Exec("ALTER TABLE settings ADD COLUMN mcp_timeout INTEGER DEFAULT 8").Error
			}
			return nil
		},
	},
	{
		Version:     20260401002,
		Description: "ensure index on embedding_caches hash",
		Migrate: func(tx *gorm.DB) error {
			if tx.Migrator().HasTable("embedding_caches") {
				_ = tx.Exec("CREATE INDEX IF NOT EXISTS idx_embedding_caches_model ON embedding_caches(model_name)").Error
			}
			return nil
		},
	},
	{
		Version:     20260401003,
		Description: "ensure tool quota and budget columns in settings",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn("settings", "tool_run_budget") {
				if err := tx.Exec("ALTER TABLE settings ADD COLUMN tool_run_budget INTEGER DEFAULT 15").Error; err != nil {
					return err
				}
			}
			if !tx.Migrator().HasColumn("settings", "tool_daily_budget") {
				if err := tx.Exec("ALTER TABLE settings ADD COLUMN tool_daily_budget INTEGER DEFAULT 200").Error; err != nil {
					return err
				}
			}
			if !tx.Migrator().HasColumn("settings", "tool_daily_cost_usd") {
				if err := tx.Exec("ALTER TABLE settings ADD COLUMN tool_daily_cost_usd REAL DEFAULT 2.0").Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:     20260401004,
		Description: "ensure audit_logs table and performance indexes",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("audit_logs") {
				if err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS audit_logs (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						created_at DATETIME,
						user_id TEXT,
						agent_id TEXT,
						skill_id INTEGER,
						tool_name TEXT,
						server_name TEXT,
						request_body TEXT,
						response TEXT,
						success BOOLEAN,
						error_msg TEXT,
						duration_ms INTEGER
					);
				`).Error; err != nil {
					return err
				}
			}
			_ = tx.Exec("CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at)").Error
			_ = tx.Exec("CREATE INDEX IF NOT EXISTS idx_audit_logs_tool_name ON audit_logs(tool_name)").Error
			_ = tx.Exec("CREATE INDEX IF NOT EXISTS idx_audit_logs_success ON audit_logs(success)").Error
			return nil
		},
	},
	{
		Version:     20260401005,
		Description: "create skill_change_requests table",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("skill_change_requests") {
				return tx.Exec(`
					CREATE TABLE IF NOT EXISTS skill_change_requests (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						created_at DATETIME,
						skill_id INTEGER,
						change_type TEXT,
						new_skill TEXT,
						old_skill TEXT,
						status TEXT,
						reviewer_id TEXT,
						reviewed_at DATETIME
					);
				`).Error
			}
			return nil
		},
	},
}

// RunVersionedMigrations 依次执行未应用的版本化迁移
func RunVersionedMigrations(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("database connection is nil")
	}

	// 1. 确保迁移版本表存在
	if err := database.AutoMigrate(&SchemaMigration{}); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	for _, step := range migrationRegistry {
		var count int64
		if err := database.Model(&SchemaMigration{}).Where("version = ?", step.Version).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to check migration status for version %d: %w", step.Version, err)
		}

		if count > 0 {
			continue // 已执行，跳过
		}

		logger.SugaredLogger.Infof("⏳ [DB Migration] 正在执行迁移 v%d: %s", step.Version, step.Description)

		err := database.Transaction(func(tx *gorm.DB) error {
			if err := step.Migrate(tx); err != nil {
				return err
			}
			record := SchemaMigration{
				Version:     step.Version,
				Description: step.Description,
				AppliedAt:   time.Now(),
			}
			return tx.Create(&record).Error
		})

		if err != nil {
			logger.SugaredLogger.Errorf("❌ [DB Migration] 迁移 v%d 执行失败: %v", step.Version, err)
			return fmt.Errorf("migration %d failed: %w", step.Version, err)
		}
		logger.SugaredLogger.Infof("✅ [DB Migration] 迁移 v%d 完成: %s", step.Version, step.Description)
	}

	return nil
}
