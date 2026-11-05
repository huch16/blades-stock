package agent

import (
	"strings"
	"testing"

	"blades-stock/backend/agent/tools"
	"blades-stock/backend/models"
)

// =============================================================================
// IsValidToolGroup / validToolGroupNames
// =============================================================================

func TestIsValidToolGroup(t *testing.T) {
	valid := []string{"base", "stock_analysis", "market", "screening", "money_flow",
		"news_research", "ai_analysis", "operations", "project"}
	for _, g := range valid {
		if !IsValidToolGroup(g) {
			t.Errorf("IsValidToolGroup(%q) = false, want true", g)
		}
	}
	invalid := []string{"", "invalid_group", "money_flw", "stocks", "Money_Flow", "MARKET"}
	for _, g := range invalid {
		if IsValidToolGroup(g) {
			t.Errorf("IsValidToolGroup(%q) = true, want false", g)
		}
	}
}

func TestValidToolGroupNames_NineGroups(t *testing.T) {
	// 与 AGENTS.md / tool_groups.go 保持 9 个 ToolGroup 一致
	if len(validToolGroupNames) != 9 {
		t.Errorf("expected 9 ToolGroups, got %d: %v", len(validToolGroupNames), validToolGroupNames)
	}
}

// =============================================================================
// ValidateOneSkillToolGroups
// =============================================================================

func TestValidateOneSkillToolGroups_Empty(t *testing.T) {
	s := models.Skill{Name: "test", ToolGroups: ""}
	if err := ValidateOneSkillToolGroups(s); err != nil {
		t.Errorf("empty ToolGroups should be valid, got %v", err)
	}
}

func TestValidateOneSkillToolGroups_AllValid(t *testing.T) {
	s := models.Skill{Name: "test", ToolGroups: "money_flow,news_research,stock_analysis"}
	if err := ValidateOneSkillToolGroups(s); err != nil {
		t.Errorf("all-valid ToolGroups should pass, got %v", err)
	}
}

func TestValidateOneSkillToolGroups_TypoDetected(t *testing.T) {
	s := models.Skill{Name: "test", ToolGroups: "money_flw,stock_analysis"}
	err := ValidateOneSkillToolGroups(s)
	if err == nil {
		t.Fatal("typo should fail validation")
	}
	if !strings.Contains(err.Error(), "money_flw") {
		t.Errorf("error should mention the typo: %v", err)
	}
}

func TestValidateOneSkillToolGroups_WhitespaceAndEmpty(t *testing.T) {
	s := models.Skill{Name: "test", ToolGroups: "  money_flow  , , stock_analysis "}
	if err := ValidateOneSkillToolGroups(s); err != nil {
		t.Errorf("should handle whitespace and empty entries, got %v", err)
	}
}

// =============================================================================
// CheckSkillDependencies
// =============================================================================

func TestCheckSkillDependencies_AllAvailable(t *testing.T) {
	s := models.Skill{ToolGroups: "money_flow,news_research"}
	available := map[tools.ToolGroup]bool{
		tools.GroupMoneyFlow:    true,
		tools.GroupNewsResearch: true,
		tools.GroupMarket:       true,
	}
	missing := CheckSkillDependencies(s, available)
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
}

func TestCheckSkillDependencies_SomeMissing(t *testing.T) {
	s := models.Skill{ToolGroups: "money_flow,news_research,stock_analysis"}
	available := map[tools.ToolGroup]bool{
		tools.GroupMoneyFlow: true,
	}
	missing := CheckSkillDependencies(s, available)
	if len(missing) != 2 {
		t.Errorf("expected 2 missing, got %v", missing)
	}
	wantMissing := map[string]bool{"news_research": true, "stock_analysis": true}
	for _, m := range missing {
		if !wantMissing[m] {
			t.Errorf("unexpected missing: %s", m)
		}
	}
}

func TestCheckSkillDependencies_EmptyGroups(t *testing.T) {
	s := models.Skill{ToolGroups: ""}
	available := map[tools.ToolGroup]bool{tools.GroupBase: true}
	missing := CheckSkillDependencies(s, available)
	if len(missing) != 0 {
		t.Errorf("empty ToolGroups should have no missing, got %v", missing)
	}
}

// =============================================================================
// WarnMissingSkillDependencies
// =============================================================================

func TestWarnMissingSkillDependencies_AllSatisfied(t *testing.T) {
	skills := []models.Skill{
		{Name: "skill-1", ToolGroups: "money_flow", Directive: "原始指令"},
	}
	available := map[tools.ToolGroup]bool{tools.GroupMoneyFlow: true}
	out := WarnMissingSkillDependencies(skills, available)
	if len(out) != 1 {
		t.Fatal("expected 1 skill back")
	}
	if strings.Contains(out[0].Directive, "⚠️") {
		t.Error("satisfied skill should not get warning")
	}
	if out[0].Directive != "原始指令" {
		t.Errorf("Directive should be unchanged: %q", out[0].Directive)
	}
}

func TestWarnMissingSkillDependencies_AppendsWarning(t *testing.T) {
	skills := []models.Skill{
		{Name: "skill-1", ToolGroups: "money_flow,news_research", Directive: "原始指令"},
	}
	available := map[tools.ToolGroup]bool{tools.GroupMoneyFlow: true}
	out := WarnMissingSkillDependencies(skills, available)
	if len(out) != 1 {
		t.Fatal("expected 1 skill back")
	}
	if !strings.Contains(out[0].Directive, "⚠️") {
		t.Error("missing skill should get warning")
	}
	if !strings.Contains(out[0].Directive, "news_research") {
		t.Error("warning should mention the missing group")
	}
	if !strings.Contains(out[0].Directive, "原始指令") {
		t.Error("original Directive should still be present")
	}
}

func TestWarnMissingSkillDependencies_NoDoubleWarning(t *testing.T) {
	// 已含 ⚠️ 的 skill 不应被再次注入
	original := "⚠️ 之前已有警告\n原始指令"
	skills := []models.Skill{{Name: "skill-1", ToolGroups: "money_flow", Directive: original}}
	available := map[tools.ToolGroup]bool{tools.GroupBase: true}
	out := WarnMissingSkillDependencies(skills, available)
	occurrences := strings.Count(out[0].Directive, "⚠️")
	if occurrences != 1 {
		t.Errorf("expected 1 ⚠️, got %d (Directive: %q)", occurrences, out[0].Directive)
	}
}

func TestWarnMissingSkillDependencies_DoesNotMutateOriginal(t *testing.T) {
	originalDirective := "原始指令"
	skills := []models.Skill{
		{Name: "skill-1", ToolGroups: "money_flow", Directive: originalDirective},
	}
	available := map[tools.ToolGroup]bool{tools.GroupBase: true}
	_ = WarnMissingSkillDependencies(skills, available)
	if skills[0].Directive != originalDirective {
		t.Errorf("original Directive was changed: %q", skills[0].Directive)
	}
}
