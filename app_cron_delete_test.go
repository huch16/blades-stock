package main

import (
	"context"
	"strings"
	"testing"

	"blades-stock/backend/models"
	"blades-stock/backend/services"

	"github.com/duke-git/lancet/v2/convertor"
	cron "github.com/robfig/cron/v3"
)

// TestDeleteCronTask_RemovesSchedulerEntryAndReturnsSuccess 回归测试：
// 旧 DeleteCronTask 先硬删再查已删记录，必然拿到 record not found 而
// 误报“删除失败”，且移除定时条目的代码在 `if err == nil` 分支内永不
// 执行，被删任务会继续按原定时执行。
// （写包内 TestMain 配的临时 SQLite 空库，不触真实 data/stock.db。）
func TestDeleteCronTask_RemovesSchedulerEntryAndReturnsSuccess(t *testing.T) {
	app := &App{
		ctx:          context.Background(),
		cron:         cron.New(),
		cronEntrys:   map[string]cron.EntryID{},
		agentService: services.NewAgentService(),
	}
	task := &models.CronTask{
		Name:     "delete-test-task",
		CronExpr: "@every 1h",
		TaskType: "news_fetch",
		Enable:   true,
	}
	if err := app.agentService.CreateCronTaskRaw(task); err != nil {
		t.Fatalf("CreateCronTaskRaw: %v", err)
	}
	if task.ID == 0 {
		t.Fatalf("expected task ID assigned, got 0")
	}
	// 模拟 CreateCronTask 的登记：定时条目以 "<id>_<name>" 登记。
	entryID, err := app.cron.AddFunc(task.CronExpr, func() {})
	if err != nil {
		t.Fatalf("cron.AddFunc: %v", err)
	}
	key := convertor.ToString(task.ID) + "_" + task.Name
	app.setCronEntry(key, entryID)

	if got := app.DeleteCronTask(task.ID); got != "删除成功" {
		t.Fatalf("DeleteCronTask = %q, want %q", got, "删除成功")
	}
	// DB 行已删。
	if _, err := app.agentService.GetCronTaskRaw(task.ID); err == nil {
		t.Errorf("task %d still exists after DeleteCronTask", task.ID)
	}
	// 定时条目已移除：调度器里查不到，登记表里也没有残留 key。
	if e := app.cron.Entry(entryID); e.ID != 0 {
		t.Errorf("cron entry %d still scheduled after DeleteCronTask", entryID)
	}
	if _, exists := app.getCronEntry(key); exists {
		t.Errorf("cronEntrys still holds key %q after DeleteCronTask", key)
	}
	// 再次删除不存在的任务应明确失败，而不是误报成功。
	if got := app.DeleteCronTask(task.ID); !strings.HasPrefix(got, "删除失败") {
		t.Errorf("second DeleteCronTask = %q, want prefix %q", got, "删除失败")
	}
	// 从未存在过的 ID 同样失败。
	if got := app.DeleteCronTask(999999); !strings.HasPrefix(got, "删除失败") {
		t.Errorf("DeleteCronTask(999999) = %q, want prefix %q", got, "删除失败")
	}
}
