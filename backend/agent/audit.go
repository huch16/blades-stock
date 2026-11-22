package agent

import (
	"sync"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

// auditLogRetention 是 audit_logs 表的保留期：审计行会随每次工具调用、
// LLM 运行不断追加，此前没有任何清理入口，表会无界增长。90 天与
// 常规审计保留窗口一致；行是只追加的操作日志，删旧行不影响任何
// 业务状态（诊断/回测读的是各自的业务表，不是审计日志）。
const auditLogRetention = 90 * 24 * time.Hour

var (
	auditLogCleanupMu sync.Mutex
	auditLogCleanupAt time.Time
)

// CleanupOldAuditLogs 删除早于 retention 的审计日志行，返回删除条数。
// retention <= 0 时用默认保留期 auditLogRetention（90 天）。
// db.Dao == nil（单测环境）时返回 0，不做任何事。
func CleanupOldAuditLogs(retention time.Duration) int {
	if db.Dao == nil {
		return 0
	}
	if retention <= 0 {
		retention = auditLogRetention
	}
	res := db.Dao.Where("created_at < ?", time.Now().Add(-retention)).Delete(&models.AuditLog{})
	if res.Error != nil {
		logger.SugaredLogger.Warnf("清理过期审计日志失败: %v", res.Error)
		return 0
	}
	return int(res.RowsAffected)
}

// maybeCleanupOldAuditLogs 每小时最多触发一次过期清理。
// 触发点复用审计日志自身的写入路径（RecordAuditLog /
// RecordAuditLogSync），与 embedding 缓存清理（SaveCachedEmbedding
// 异步写盘时顺带触发）同一模式，不新开 cron。
func maybeCleanupOldAuditLogs() {
	auditLogCleanupMu.Lock()
	defer auditLogCleanupMu.Unlock()
	if time.Since(auditLogCleanupAt) < time.Hour {
		return
	}
	auditLogCleanupAt = time.Now()
	if n := CleanupOldAuditLogs(auditLogRetention); n > 0 {
		logger.SugaredLogger.Infof("清理过期审计日志 %d 条", n)
	}
}

// RecordAuditLog 异步记录工具、MCP或Agent动作审计日志，防止I/O阻塞实时推理推流
func RecordAuditLog(entry models.AuditLog) {
	maybeCleanupOldAuditLogs()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in RecordAuditLog: %v", r)
			}
		}()
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
	maybeCleanupOldAuditLogs()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if db.Dao == nil {
		return nil
	}
	return db.Dao.Create(&entry).Error
}
