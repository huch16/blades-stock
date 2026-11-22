package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"blades-stock/backend/models"
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
