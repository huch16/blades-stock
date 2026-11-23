package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// TestAnalyzeProject_ReanalyzeNoDuplicateChunks 验证同一项目
// 重复分析后 chunk 数量稳定：旧实现只 INSERT 新 chunk 不删旧块，
// 每次重分析 chunk 数翻倍增长。
func TestAnalyzeProject_ReanalyzeNoDuplicateChunks(t *testing.T) {
	if err := db.Dao.AutoMigrate(&models.ProjectAnalysis{}, &models.CodeChunk{}); err != nil {
		t.Fatalf("migrate project tables: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "util.go"), []byte("package main\n\nfunc helper() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	analysis := models.ProjectAnalysis{SourceURL: "", Name: "test-analyze-dedup", Status: "pending"}
	if err := db.Dao.Create(&analysis).Error; err != nil {
		t.Fatalf("create project analysis: %v", err)
	}
	t.Cleanup(func() {
		db.Dao.Where("project_analysis_id = ?", analysis.ID).Delete(&models.CodeChunk{})
		db.Dao.Delete(&models.ProjectAnalysis{}, analysis.ID)
	})

	pa := NewProjectAnalyzer(nil) // 无 embedding 服务：chunk 仍生成，embedding 留空
	if err := pa.AnalyzeProject(analysis.ID, dir); err != nil {
		t.Fatalf("first AnalyzeProject failed: %v", err)
	}
	var first int64
	db.Dao.Model(&models.CodeChunk{}).Where("project_analysis_id = ?", analysis.ID).Count(&first)
	if first == 0 {
		t.Fatal("expected at least one chunk after first analysis")
	}
	if err := pa.AnalyzeProject(analysis.ID, dir); err != nil {
		t.Fatalf("second AnalyzeProject failed: %v", err)
	}
	var second int64
	db.Dao.Model(&models.CodeChunk{}).Where("project_analysis_id = ?", analysis.ID).Count(&second)
	if second != first {
		t.Errorf("re-analysis should replace chunks, not duplicate them: first=%d second=%d", first, second)
	}
	var updated models.ProjectAnalysis
	if err := db.Dao.First(&updated, analysis.ID).Error; err != nil {
		t.Fatalf("reload analysis: %v", err)
	}
	if updated.Status != "indexed" {
		t.Errorf("expected status indexed, got %q", updated.Status)
	}
}

// TestFindAPIDefinitions_LongChineseLineRuneSafe 回归：API 行截断曾按
// 字节切 short[:100]，含中文注释的长行会在汉字中间断开，产生非法
// UTF-8。修复后按 rune 截断，输出恒为合法 UTF-8。
func TestFindAPIDefinitions_LongChineseLineRuneSafe(t *testing.T) {
	dir := t.TempDir()
	longLine := "func foo() { r.HandleFunc(\"/x\", h) } // " + strings.Repeat("这是一个很长的中文注释说明", 20)
	content := "package main\n\n" + longLine + "\n"
	if err := os.WriteFile(filepath.Join(dir, "server.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pa := NewProjectAnalyzer(nil)
	found := pa.findAPIDefinitions(dir)
	if len(found) == 0 {
		t.Fatal("expected at least one API definition")
	}
	for _, line := range found {
		if !utf8.ValidString(line) {
			t.Fatalf("findAPIDefinitions produced invalid UTF-8: %q", line)
		}
		if !strings.Contains(line, "...") {
			t.Errorf("long API line should be truncated with ellipsis: %q", line)
		}
	}
}
