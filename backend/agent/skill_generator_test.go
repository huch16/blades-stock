package agent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// TestImportProject_EmptySourceNoDuplicate 验证 sourceURL 为空时
// 重复导入同一项目不产生多条 pending 记录：旧实现此时跳过
// 查找、每次都 INSERT 新行。
func TestImportProject_EmptySourceNoDuplicate(t *testing.T) {
	if err := db.Dao.AutoMigrate(&models.ProjectAnalysis{}); err != nil {
		t.Fatalf("migrate project_analyses: %v", err)
	}
	const name = "test-project-import-dedup"
	t.Cleanup(func() {
		db.Dao.Where("name = ?", name).Delete(&models.ProjectAnalysis{})
	})
	sg := NewSkillGenerator(nil)
	a1, err := sg.ImportProject("", name)
	if err != nil {
		t.Fatalf("first ImportProject failed: %v", err)
	}
	a2, err := sg.ImportProject("", name)
	if err != nil {
		t.Fatalf("second ImportProject failed: %v", err)
	}
	if a1.ID != a2.ID {
		t.Errorf("duplicate import should return the same record, got IDs %d and %d", a1.ID, a2.ID)
	}
	var count int64
	db.Dao.Model(&models.ProjectAnalysis{}).Where("name = ?", name).Count(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 project record, got %d", count)
	}
}

// TestBuildSystemPrompt_ReadmeTruncationRuneSafe 回归：README 超长截断
// 曾按字节切 readmePreview[:2000]，中文处会切出非法 UTF-8，污染
// 注入 prompt 的 skill 文本。修复后按 rune 截断，输出恒为合法 UTF-8。
func TestBuildSystemPrompt_ReadmeTruncationRuneSafe(t *testing.T) {
	sg := NewSkillGenerator(nil)
	longReadme := strings.Repeat("这是一个很长的中文说明，", 200) // 2400+ runes
	prompt := sg.buildSystemPrompt(&SkillGenerationRequest{
		Name:   "测试项目",
		Readme: longReadme,
	})
	if !utf8.ValidString(prompt) {
		t.Fatal("buildSystemPrompt produced invalid UTF-8 after truncating a long Chinese README")
	}
	if !strings.Contains(prompt, "已截断") {
		t.Error("long README should be marked as truncated")
	}
}
