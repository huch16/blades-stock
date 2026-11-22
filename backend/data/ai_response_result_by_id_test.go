package data

import (
	"context"
	"testing"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// 研报导出按行（B 组任务一）回归测试。
//
// 背景：SaveAsMarkdown(code, name) 两个后端实现都是按 stockCode 取最新一条，
// 用户在列表里打开一条旧报告点导出，拿到的是最新报告的内容。
// 新接口 GetAIResponseResultById 按主键取行，导出旧行必须拿到旧内容。
//
// 本包 TestMain 已把 db.Dao 指向临时 SQLite 文件，不碰真实 DB。
func TestGetAIResponseResultByIdReturnsSpecifiedRow(t *testing.T) {
	if err := db.Dao.AutoMigrate(&models.AIResponseResult{}); err != nil {
		t.Fatalf("AutoMigrate ai_response_result 失败: %v", err)
	}
	// 同一标的先后两条报告：旧行与新行内容必须不同，否则测不出“取错行”。
	const code = "TESTBYID000001"
	oldRow := models.AIResponseResult{
		StockCode: code,
		StockName: "按行导出测试股",
		ModelName: "test-model",
		Content:   "# 旧报告\n这是 2026-01-01 的旧报告内容。",
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
		Content:   "# 新报告\n这是 2026-01-02 的新报告内容，与旧报告完全不同。",
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
	if oldRow.ID == 0 || newRow.ID == 0 || oldRow.ID == newRow.ID {
		t.Fatalf("测试行 ID 异常: old=%d new=%d", oldRow.ID, newRow.ID)
	}

	api := NewDeepSeekOpenAi(context.Background(), 0)

	// 1. 按 ID 导出旧行：必须拿到旧内容，绝不能是新内容。
	gotOld := api.GetAIResponseResultById(oldRow.ID)
	if gotOld == nil || gotOld.ID != oldRow.ID {
		t.Fatalf("按 ID 取旧行失败: got=%+v want ID=%d", gotOld, oldRow.ID)
	}
	if gotOld.Content != oldRow.Content {
		t.Fatalf("按 ID 取旧行内容错误:\n got: %q\nwant: %q", gotOld.Content, oldRow.Content)
	}

	// 2. 按 ID 导出新行：同样精确命中。
	gotNew := api.GetAIResponseResultById(newRow.ID)
	if gotNew == nil || gotNew.Content != newRow.Content {
		t.Fatalf("按 ID 取新行内容错误: got=%+v want content=%q", gotNew, newRow.Content)
	}

	// 3. 对照：按 code 取仍是最新一条（旧接口语义不变，别处仍在用）。
	gotLatest := api.GetAIResponseResult(code)
	if gotLatest == nil || gotLatest.Content != newRow.Content {
		t.Fatalf("按 code 取最新语义被破坏: got=%+v want content=%q", gotLatest, newRow.Content)
	}

	// 4. 不存在的 ID：返回零值记录（ID==0），与 GetAIResponseResult 的约定一致，
	// 调用方凭 ID==0 / 内容为空识别“无此报告”。
	gotMissing := api.GetAIResponseResultById(999999999)
	if gotMissing == nil || gotMissing.ID != 0 || gotMissing.Content != "" {
		t.Fatalf("不存在的 ID 应返回零值记录，实际: %+v", gotMissing)
	}
}
