package services

import (
	"testing"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func init() {
	db.Init("../../data/stock.db")
}

func TestQuantService_Init(t *testing.T) {
	qs := NewQuantService()
	if qs == nil {
		t.Fatal("expected non-nil QuantService")
	}

	// 验证空代码错误防御
	_, err := qs.DiagnoseStock("")
	if err == nil {
		t.Error("expected error for empty stock code, got nil")
	}
}

func TestSettingsService_Basic(t *testing.T) {
	ss := NewSettingsService()
	if ss == nil {
		t.Fatal("expected non-nil SettingsService")
	}

	cfg := ss.GetConfig()
	if cfg == nil {
		t.Fatal("expected non-nil SettingConfig")
	}

	templates := ss.GetAiConfigTemplates()
	if len(templates) == 0 {
		t.Log("warning: no templates returned")
	}

	mid := ss.GetMachineId()
	if mid == "" {
		t.Log("machine id is empty in test environment")
	}
}

func TestAgentService_Basic(t *testing.T) {
	as := NewAgentService()
	if as == nil {
		t.Fatal("expected non-nil AgentService")
	}

	skills := as.GetAllSkills()
	t.Logf("found %d skills in database", len(skills))

	quota := as.GetToolQuotaStatus()
	t.Logf("quota status: dailyCalls=%d, dailyCost=%.4f", quota.DailyCalls, quota.DailyCostUSD)

	snapshot := as.GetSystemStateSnapshot()
	if snapshot == nil {
		t.Fatal("expected non-nil SystemStateSnapshot")
	}
}

func TestMarketService_Basic(t *testing.T) {
	ms := NewMarketService()
	if ms == nil {
		t.Fatal("expected non-nil MarketService")
	}

	groups := ms.GetGroupList()
	t.Logf("found %d stock groups", len(groups))

	// 搜索测试
	results := ms.GetStockList("600519")
	t.Logf("found %d stocks matching 600519", len(results))

	// 基金列表测试
	funds := ms.GetFollowedFund()
	t.Logf("found %d followed funds", len(funds))
}

func TestSettingsService_AuditLog(t *testing.T) {
	ss := NewSettingsService()
	resp, err := ss.GetAuditLogList(models.AuditLogQuery{
		Page:     1,
		PageSize: 5,
	})
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil AuditLogPageResp")
	}
	t.Logf("audit log total: %d, current page size: %d", resp.Data.Total, len(resp.Data.List))
}
