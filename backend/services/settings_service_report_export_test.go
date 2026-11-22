package services

import (
	"testing"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// 研报导出按行（B 组任务一）服务层回归测试。
//
// 覆盖 SettingsService 两条新链路（桌面与 Web 共用）：
//   - SaveReportAsMarkdownById(id)：按 ID 导出指定行，旧行必须拿到旧内容；
//   - GetReportById(id)：桌面端取行（含文件名用字段）。
//
// newServicesTestEnv 把 db.Dao 指向临时 SQLite 文件，不碰真实 DB。
func TestSaveReportAsMarkdownByIdReturnsSpecifiedRow(t *testing.T) {
	newServicesTestEnv(t)
	if err := db.Dao.AutoMigrate(&models.AIResponseResult{}); err != nil {
		t.Fatalf("AutoMigrate ai_response_result 失败: %v", err)
	}
	const code = "TESTBYID000002"
	oldRow := models.AIResponseResult{
		StockCode: code,
		StockName: "按行导出测试股",
		ModelName: "test-model",
		Content:   "# 旧报告\n这是旧报告的正文内容。",
		ChatId:    "chat-old",
		Question:  "旧问题",
	}
	oldRow.CreatedAt = time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local)
	if err := db.Dao.Create(&oldRow).Error; err != nil {
		t.Fatalf("写入旧报告失败: %v", err)
	}
	newRow := models.AIResponseResult{
		StockCode: code,
		StockName: "按行导出测试股",
		ModelName: "test-model",
		Content:   "# 新报告\n这是新报告的正文内容，与旧报告完全不同。",
		ChatId:    "chat-new",
		Question:  "新问题",
	}
	newRow.CreatedAt = time.Date(2026, 1, 2, 10, 0, 0, 0, time.Local)
	if err := db.Dao.Create(&newRow).Error; err != nil {
		t.Fatalf("写入新报告失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Dao.Unscoped().Where("stock_code = ?", code).Delete(&models.AIResponseResult{}).Error
	})

	svc := NewSettingsService()

	// 1. 按 ID 导出旧行：必须拿到旧内容。
	if got := svc.SaveReportAsMarkdownById(int(oldRow.ID)); got != oldRow.Content {
		t.Fatalf("按 ID 导出旧行内容错误:\n got: %q\nwant: %q", got, oldRow.Content)
	}
	// 2. 按 ID 导出新行：精确命中新内容。
	if got := svc.SaveReportAsMarkdownById(int(newRow.ID)); got != newRow.Content {
		t.Fatalf("按 ID 导出新行内容错误:\n got: %q\nwant: %q", got, newRow.Content)
	}
	// 3. 对照：按 code 导出仍是最新一条（旧入口语义不变）。
	if got := svc.SaveAsMarkdown(code, "按行导出测试股"); got != newRow.Content {
		t.Fatalf("按 code 导出最新语义被破坏:\n got: %q\nwant: %q", got, newRow.Content)
	}
	// 4. 非法 ID：与旧入口相同的错误文案，前端按原有分支处理。
	for _, badID := range []int{0, -1, 999999999} {
		if got := svc.SaveReportAsMarkdownById(badID); got != "分析结果异常,无法保存。" {
			t.Fatalf("非法 ID %d 应返回错误文案，实际: %q", badID, got)
		}
	}

	// 5. GetReportById：桌面端取行，ID/内容/文件名字段齐全；非法 ID 返回 nil。
	row := svc.GetReportById(int(oldRow.ID))
	if row == nil || row.ID != oldRow.ID || row.Content != oldRow.Content ||
		row.StockCode != code || row.StockName != "按行导出测试股" || row.CreatedAt.IsZero() {
		t.Fatalf("GetReportById 取行异常: %+v", row)
	}
	if svc.GetReportById(0) != nil || svc.GetReportById(-5) != nil {
		t.Fatalf("GetReportById 非法 ID 应返回 nil")
	}
	if missing := svc.GetReportById(999999999); missing == nil || missing.ID != 0 {
		t.Fatalf("GetReportById 不存在的 ID 应返回 ID==0 的记录，实际: %+v", missing)
	}
}
