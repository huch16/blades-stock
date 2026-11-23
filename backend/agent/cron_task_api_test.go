package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestExecuteFundAnalysis_NotImplemented 验证未实现的任务类型
// 返回错误而不是静默记“成功”：旧实现只打几条日志就返回 nil，
// 定时任务被记为执行成功，基金分析从未真正跑过也无人知晓。
func TestExecuteFundAnalysis_NotImplemented(t *testing.T) {
	api := &CronTaskApi{}
	task := &models.CronTask{Params: `{"fund_codes":["000001"]}`}
	err := api.executeFundAnalysis(context.Background(), task)
	if err == nil {
		t.Fatal("executeFundAnalysis should return a not-implemented error")
	}
	if !strings.Contains(err.Error(), "未实现") {
		t.Errorf("error should say not implemented, got: %v", err)
	}
}

// TestExecuteStockMonitor_NotImplemented 同上：股票监控同样未实现。
func TestExecuteStockMonitor_NotImplemented(t *testing.T) {
	api := &CronTaskApi{}
	task := &models.CronTask{Params: `{"codes":["600519"]}`}
	err := api.executeStockMonitor(context.Background(), task)
	if err == nil {
		t.Fatal("executeStockMonitor should return a not-implemented error")
	}
	if !strings.Contains(err.Error(), "未实现") {
		t.Errorf("error should say not implemented, got: %v", err)
	}
}

// TestExecutePreMarketBriefing_InvalidParams 验证非法 JSON 参数
// 直接返回错误：旧实现忽略解析错误，静默以 AiConfigId=0 回落到
// 默认模型执行，参数写错了也无人知晓。
func TestExecutePreMarketBriefing_InvalidParams(t *testing.T) {
	api := &CronTaskApi{}
	task := &models.CronTask{Params: `{invalid json`}
	err := api.executePreMarketBriefing(context.Background(), task)
	if err == nil {
		t.Fatal("executePreMarketBriefing with invalid JSON params should fail")
	}
}

// TestExecutePostMarketReview_InvalidParams 同上。
func TestExecutePostMarketReview_InvalidParams(t *testing.T) {
	api := &CronTaskApi{}
	task := &models.CronTask{Params: `{invalid json`}
	err := api.executePostMarketReview(context.Background(), task)
	if err == nil {
		t.Fatal("executePostMarketReview with invalid JSON params should fail")
	}
}

// TestCalculateNextRunTime_InvalidExpression 验证非法 cron 表达式
// 仍返回约 1 小时后的时间（fallback 语义保留），便于后续按需告警。
func TestCalculateNextRunTime_InvalidExpression(t *testing.T) {
	api := &CronTaskApi{}
	before := time.Now()
	next := api.CalculateNextRunTime("not a cron expression")
	if next.Before(before.Add(50*time.Minute)) || next.After(before.Add(70*time.Minute)) {
		t.Errorf("fallback should be ~1h later, got %v (now %v)", next, before)
	}
}

// TestCalculateNextRunTime_ValidExpression 验证合法表达式按 cron 计算。
// 本项目 cron 解析器要求 6 字段（含秒）。
func TestCalculateNextRunTime_ValidExpression(t *testing.T) {
	api := &CronTaskApi{}
	next := api.CalculateNextRunTime("0 0 9 * * *")
	if next.Hour() != 9 || next.Minute() != 0 {
		t.Errorf("next run should be 09:00, got %v", next)
	}
	if !next.After(time.Now()) {
		t.Errorf("next run should be in the future, got %v", next)
	}
}

// TestExecuteCustomTask_NotImplemented 验证自定义任务未实现时返回
// 错误而不是静默记“成功”（与 fund_analysis / stock_monitor 同类）。
func TestExecuteCustomTask_NotImplemented(t *testing.T) {
	api := &CronTaskApi{}
	task := &models.CronTask{Params: `{}`}
	err := api.executeCustomTask(context.Background(), task)
	if err == nil {
		t.Fatal("executeCustomTask should return a not-implemented error")
	}
	if !strings.Contains(err.Error(), "未实现") {
		t.Errorf("error should say not implemented, got: %v", err)
	}
}

func chanOfCronMsgs(msgs ...*StreamMessage) chan *StreamMessage {
	ch := make(chan *StreamMessage, len(msgs))
	for _, m := range msgs {
		ch <- m
	}
	close(ch)
	return ch
}

// TestCollectCronAgentStream_Markers 验证流错误标记与 collectReportStream
// 一致：❌ 开头或含“Agent 调用失败”的内容记为流错误、不进正文；
// nil 消息跳过；思考内容照常汇总。
func TestCollectCronAgentStream_Markers(t *testing.T) {
	content, streamErr := collectCronAgentStream(chanOfCronMsgs(
		&StreamMessage{ReasoningContent: "思考…"},
		&StreamMessage{Content: "正文第一段"},
		nil,
		&StreamMessage{Content: "❌ 尚未配置任何 AI 模型"},
		&StreamMessage{Content: "Agent 调用失败：上游 500"},
		&StreamMessage{Content: "正文第二段"},
	))
	if streamErr == "" || !strings.Contains(streamErr, "尚未配置任何 AI 模型") {
		t.Errorf("first stream error should be kept, got %q", streamErr)
	}
	if content != "思考…正文第一段正文第二段" {
		t.Errorf("error text must not be saved as content, got %q", content)
	}

	content, streamErr = collectCronAgentStream(chanOfCronMsgs(
		&StreamMessage{Content: "正常正文"},
	))
	if streamErr != "" || content != "正常正文" {
		t.Errorf("normal stream should pass through, got content=%q err=%q", content, streamErr)
	}
}

// TestCheckCronAgentContent 验证流错误或空内容都拒绝落库。
func TestCheckCronAgentContent(t *testing.T) {
	if err := checkCronAgentContent("正常报告", "", "股票分析任务"); err != nil {
		t.Errorf("normal content should pass, got %v", err)
	}
	if err := checkCronAgentContent("半份正文", "❌ 流中断", "股票分析任务"); err == nil {
		t.Error("stream error should fail")
	}
	if err := checkCronAgentContent("   ", "", "股票分析任务"); err == nil {
		t.Error("empty content should fail")
	}
	if err := checkCronAgentContent("", "", "市场分析任务"); err == nil {
		t.Error("empty content should fail")
	}
}

// TestExecuteStockAnalysis_NoConfigReturnsError 回归测试：无 AI 配置时
// agent 流产出 ❌ 错误文本，旧实现会把它原样存为分析报告并返回 nil
// 记“成功”；修复后应返回流错误、不落库。
func TestExecuteStockAnalysis_NoConfigReturnsError(t *testing.T) {
	withEmptyModelRegistry(t)
	api := &CronTaskApi{}
	task := &models.CronTask{Params: `{"stockCode":"600519","stockName":"贵州茅台","aiConfigId":0}`}
	err := api.executeStockAnalysis(context.Background(), task)
	if err == nil {
		t.Fatal("executeStockAnalysis with no AI config should return a stream error, not nil")
	}
	if !strings.Contains(err.Error(), "流错误") || !strings.Contains(err.Error(), "尚未配置任何 AI 模型") {
		t.Errorf("error should carry the stream error, got: %v", err)
	}
}

// TestExecuteMarketAnalysis_NoConfigReturnsError 同上：市场分析执行器
// 的守卫接线也必须生效。
func TestExecuteMarketAnalysis_NoConfigReturnsError(t *testing.T) {
	withEmptyModelRegistry(t)
	api := &CronTaskApi{}
	task := &models.CronTask{Params: `{"aiConfigId":0}`}
	err := api.executeMarketAnalysis(context.Background(), task)
	if err == nil {
		t.Fatal("executeMarketAnalysis with no AI config should return a stream error, not nil")
	}
	if !strings.Contains(err.Error(), "流错误") || !strings.Contains(err.Error(), "尚未配置任何 AI 模型") {
		t.Errorf("error should carry the stream error, got: %v", err)
	}
}

// TestGetTaskTypesIncludesAllExecutableTypes 验证任务类型列表与
// executeTaskByType 的 switch 一致：第五轮前漏了 fund_analysis /
// news_fetch / stock_monitor / custom，前端下拉（GetCronTaskTypes
// 调本函数）里选不到这四种，但后端执行分支其实认它们。
// 标签为空或类型重复都会让下拉出现空白/重复项，一并断言。
func TestGetTaskTypesIncludesAllExecutableTypes(t *testing.T) {
	got := NewCronTaskApi().GetTaskTypes()
	byType := make(map[string]string, len(got))
	for _, tp := range got {
		if tp.A == "" {
			t.Errorf("任务类型键不应为空（标签 %q）", tp.B)
		}
		if tp.B == "" {
			t.Errorf("任务类型 %q 的标签不应为空", tp.A)
		}
		if _, dup := byType[tp.A]; dup {
			t.Errorf("任务类型 %q 重复出现", tp.A)
		}
		byType[tp.A] = tp.B
	}
	for typ, label := range map[string]string{
		"stock_analysis":           "股票分析",
		"market_analysis":          "市场分析",
		"pre_market_briefing":      "盘前智能前瞻晨报",
		"post_market_review":       "盘后收盘深度复盘",
		"global_stock_index_cache": "全球指数缓存",
		"stock_change_save":        "异动数据保存",
		"fund_analysis":            "基金分析",
		"news_fetch":               "新闻抓取",
		"stock_monitor":            "股票监控",
		"custom":                   "自定义任务",
	} {
		if gotLabel, ok := byType[typ]; !ok {
			t.Errorf("GetTaskTypes 缺少 %q（执行分支认、列表不认）", typ)
		} else if gotLabel != label {
			t.Errorf("任务类型 %q 标签应为 %q，实际 %q", typ, label, gotLabel)
		}
	}
}

// withEmptyModelRegistry 把 db.Dao 换成一个只有空 settings/ai_config
// 表的隔离临时库（附 agent_run_records 表供运行记录落库），测试结束
// 恢复。必须隔离——同包其它测试（如 agent_run_record_test.go 的
// seedRunCards）会往共享临时库里插 AI 卡片且不清理；直接用共享库
// 时 AiConfigId=0 会解析到那张卡并真去调不可达端点（9 秒重试后才
// 失败），测试结果依赖执行顺序。
func withEmptyModelRegistry(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "empty-registry.db")
	isoDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open isolated db: %v", err)
	}
	if err := isoDB.AutoMigrate(&data.Settings{}, &data.AIConfig{}, &models.AgentRunRecord{}); err != nil {
		t.Fatalf("migrate isolated db: %v", err)
	}
	origDao := db.Dao
	db.Dao = isoDB
	t.Cleanup(func() { db.Dao = origDao })
}
