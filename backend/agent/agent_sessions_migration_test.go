package agent

import (
	"testing"

	"blades-stock/backend/db"
)

// agent_sessions 空表清理（第五轮）的回归测试。
//
// 背景：agent_sessions 只有 AutoMigrate 建表，全仓无任何读写，是空表。
// 本轮只从迁移列表移除建表——旧库里已存在的空表不受影响（不删表、
// 不碰数据），新库不再建该表。本测试锁住“不再建表”这一半；
// “旧表保留”是部署侧行为（AutoMigrate 本来就不删表），
// 由汇报与部署后核对覆盖，不在单测里断言。
func TestAutoMigrateListExcludesAgentSessions(t *testing.T) {
	for _, m := range db.AutoMigrateList {
		if _, ok := m.(*AgentSession); ok {
			t.Errorf("AutoMigrateList 不应再含 AgentSession：该表无读写，第五轮已停掉建表")
		}
	}
	// 邻表仍在：别在清理时误删了真正有用的迁移。
	var sawMemory, sawTrace bool
	for _, m := range db.AutoMigrateList {
		switch m.(type) {
		case *AgentMemory:
			sawMemory = true
		case *AgentTrace:
			sawTrace = true
		}
	}
	if !sawMemory {
		t.Error("AutoMigrateList 缺少 AgentMemory：清理 agent_sessions 时不应波及邻表")
	}
	if !sawTrace {
		t.Error("AutoMigrateList 缺少 AgentTrace：清理 agent_sessions 时不应波及邻表")
	}
}
