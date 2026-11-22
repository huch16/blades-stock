package agent

import (
	"testing"

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
