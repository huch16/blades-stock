package agent

import (
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

// RecordAuditLog 异步记录工具、MCP或Agent动作审计日志，防止I/O阻塞实时推理推流
func RecordAuditLog(entry models.AuditLog) {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	go func() {
		if db.Dao == nil {
			return
		}
		if err := db.Dao.Create(&entry).Error; err != nil {
			logger.SugaredLogger.Warnf("⚠️ [AuditLog] 写入审计日志失败: %v", err)
		}
	}()
}

// RecordAuditLogSync 同步记录审计日志，用于严格审计场景或单次任务
func RecordAuditLogSync(entry models.AuditLog) error {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if db.Dao == nil {
		return nil
	}
	return db.Dao.Create(&entry).Error
}
