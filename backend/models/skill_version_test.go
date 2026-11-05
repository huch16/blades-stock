package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSkillVersionJSON(t *testing.T) {
	// Prepare a sample struct
	orig := SkillVersion{
		ID:            1,
		SkillID:       10,
		VersionNumber: 1,
		Content:       `{"name":"testSkill","description":"test"}`, // Simplified JSON
		CreatedAt:     time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		IsActive:      true,
	}
	// Marshal to JSON
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	// Unmarshal back
	var decoded SkillVersion
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	// Verify fields are equal
	if decoded.ID != orig.ID {
		t.Errorf("ID mismatch: got %d want %d", decoded.ID, orig.ID)
	}
	if decoded.SkillID != orig.SkillID {
		t.Errorf("SkillID mismatch: got %d want %d", decoded.SkillID, orig.SkillID)
	}
	if decoded.VersionNumber != orig.VersionNumber {
		t.Errorf("VersionNumber mismatch: got %d want %d", decoded.VersionNumber, orig.VersionNumber)
	}
	if decoded.Content != orig.Content {
		t.Errorf("Content mismatch: got %s want %s", decoded.Content, orig.Content)
	}
	// Compare time (need to check UTC)
	if !decoded.CreatedAt.Equal(orig.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v want %v", decoded.CreatedAt, orig.CreatedAt)
	}
	if decoded.IsActive != orig.IsActive {
		t.Errorf("IsActive mismatch: got %v want %v", decoded.IsActive, orig.IsActive)
	}
}

func TestSkillVersionTableName(t *testing.T) {
	var sv SkillVersion
	if sv.TableName() != "skill_versions" {
		t.Errorf("TableName mismatch: got %s want skill_versions", sv.TableName())
	}
}
