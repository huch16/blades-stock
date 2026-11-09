package data

import (
	"strings"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

// 运行记录的保留上限：只保留最近这么多条，避免长期运行把库撑大。
// 这不是“限流/配额”，纯粹是日志表的滚动修剪，不影响任何业务能力。
const agentRunRecordKeepLimit = 500

// RecordAgentRun 写入一条运行记录。
//
// 语义：**纯观测，绝不阻塞、绝不失败业务**。写不进去只记日志。
func RecordAgentRun(rec *models.AgentRunRecord) {
	if rec == nil || db.Dao == nil {
		return
	}
	if strings.TrimSpace(rec.Kind) == "" {
		rec.Kind = "chat"
	}
	if err := db.Dao.Create(rec).Error; err != nil {
		logger.SugaredLogger.Warnf("[AgentRun] 写入运行记录失败（不影响业务）: %v", err)
		return
	}
	// 滚动修剪：删除超出保留上限的旧记录
	var count int64
	if err := db.Dao.Model(&models.AgentRunRecord{}).Count(&count).Error; err != nil {
		return
	}
	if count > agentRunRecordKeepLimit {
		var cutoffID uint
		offset := agentRunRecordKeepLimit - 1
		if err := db.Dao.Model(&models.AgentRunRecord{}).
			Order("id DESC").Offset(offset).Limit(1).Pluck("id", &cutoffID).Error; err == nil && cutoffID > 0 {
			if err := db.Dao.Where("id < ?", cutoffID).Delete(&models.AgentRunRecord{}).Error; err != nil {
				logger.SugaredLogger.Warnf("[AgentRun] 修剪旧运行记录失败: %v", err)
			}
		}
	}
}

// GetRecentAgentRuns 倒序返回最近的运行记录（limit <= 0 时默认 50，上限 500）。
func GetRecentAgentRuns(limit int) []models.AgentRunRecord {
	out := make([]models.AgentRunRecord, 0)
	if db.Dao == nil {
		return out
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > agentRunRecordKeepLimit {
		limit = agentRunRecordKeepLimit
	}
	if err := db.Dao.Model(&models.AgentRunRecord{}).
		Order("id DESC").Limit(limit).Find(&out).Error; err != nil {
		logger.SugaredLogger.Warnf("[AgentRun] 读取运行记录失败: %v", err)
		return []models.AgentRunRecord{}
	}
	return out
}

// ClearAgentRuns 清空全部运行记录，返回删除条数。
func ClearAgentRuns() int64 {
	if db.Dao == nil {
		return 0
	}
	res := db.Dao.Where("1 = 1").Delete(&models.AgentRunRecord{})
	if res.Error != nil {
		logger.SugaredLogger.Warnf("[AgentRun] 清空运行记录失败: %v", res.Error)
		return 0
	}
	return res.RowsAffected
}
