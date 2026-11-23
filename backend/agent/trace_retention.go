package agent

import (
	"sync"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
)

// agentTraceRetention 是 agent_traces 表的保留期：反思每次回答都写一条
// 轨迹，此前没有任何清理入口，表会无界增长。90 天与审计日志一致。
// 读侧只用 GetHighQualityTraces(10)（按 score 取前 10 条），删 90 天
// 前的旧行不影响进化学习（它只看高分样本，旧样本早已过期）。
const agentTraceRetention = 90 * 24 * time.Hour

var (
	agentTraceCleanupMu sync.Mutex
	agentTraceCleanupAt time.Time
)

// CleanupOldAgentTraces 删除早于 retention 的反思轨迹行，返回删除条数。
// retention <= 0 时用默认保留期 agentTraceRetention（90 天）。
// db.Dao == nil（单测环境）时返回 0，不做任何事。
func CleanupOldAgentTraces(retention time.Duration) int {
	if db.Dao == nil {
		return 0
	}
	if retention <= 0 {
		retention = agentTraceRetention
	}
	res := db.Dao.Where("created_at < ?", time.Now().Add(-retention)).Delete(&AgentTrace{})
	if res.Error != nil {
		logger.SugaredLogger.Warnf("清理过期反思轨迹失败: %v", res.Error)
		return 0
	}
	return int(res.RowsAffected)
}

// maybeCleanupOldAgentTraces 每小时最多触发一次过期清理。
// 触发点复用轨迹自身的写入路径（GenerateReflection 写轨迹时顺带
// 触发），与审计日志清理（RecordAuditLog 写路径触发）同一模式，
// 不新开 cron。
func maybeCleanupOldAgentTraces() {
	agentTraceCleanupMu.Lock()
	defer agentTraceCleanupMu.Unlock()
	if time.Since(agentTraceCleanupAt) < time.Hour {
		return
	}
	agentTraceCleanupAt = time.Now()
	if n := CleanupOldAgentTraces(agentTraceRetention); n > 0 {
		logger.SugaredLogger.Infof("清理过期反思轨迹 %d 条", n)
	}
}
