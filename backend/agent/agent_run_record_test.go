package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// 固化 v3.0.0 改造：每次运行都必须落一条可查询的**运行记录**，
// 且记录里的模型必须是**实际**使用的那一个（含解析依据与回落标记）。
//
// 用户反复遇到「到底哪个模型在跑」：界面显示的是请求的模型，
// 实际可能因卡片被删/全局默认/回落第一张而换成另一个。
// 这里验证“事实”确实被固化了。

func resetRunRecords() {
	if db.Dao != nil {
		db.Dao.Where("1 = 1").Delete(&models.AgentRunRecord{})
	}
}

func seedRunCards(t *testing.T, globalDefault uint, cards ...data.AIConfig) []data.AIConfig {
	t.Helper()
	if err := db.Dao.Exec("DELETE FROM ai_config").Error; err != nil {
		t.Fatalf("清理 ai_config: %v", err)
	}
	if err := db.Dao.Exec("DELETE FROM settings").Error; err != nil {
		t.Fatalf("清理 settings: %v", err)
	}
	out := make([]data.AIConfig, 0, len(cards))
	for i := range cards {
		c := cards[i]
		if err := db.Dao.Create(&c).Error; err != nil {
			t.Fatalf("写入卡片: %v", err)
		}
		out = append(out, c)
	}
	if err := db.Dao.Create(&data.Settings{OpenAiEnable: true, AiConfigID: globalDefault, Prompt: "test"}).Error; err != nil {
		t.Fatalf("写入 settings: %v", err)
	}
	return out
}

func unreachableCard(name, model string) data.AIConfig {
	c := data.AIConfig{
		Name: name, ModelName: model,
		BaseUrl: "http://127.0.0.1:1/v1", ApiKey: "sk-unreachable-run-test",
		MaxTokens: 512, TimeOut: 10,
	}
	c.SetDefaults()
	return c
}

// 契约一：即使运行失败（端点不可达），也必须留下一条记录，
// 且记录里的“实际模型”与解析依据都要如实写入。
func TestRunRecordCapturesResolvedModelOnFailure(t *testing.T) {
	resetRunRecords()
	cs := seedRunCards(t, 0, unreachableCard("卡片甲", "model-alpha"))
	if err := db.Dao.Model(&data.Settings{}).Where("1 = 1").Update("ai_config_id", cs[0].ID).Error; err != nil {
		t.Fatalf("设置全局默认: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// 故意请求一个不存在的卡片 ID → 必须回落到全局默认
	ch := NewStockAiAgentApi().ChatWithContext(ctx, "ping", 99999, nil, false, 0, false, "direct")
	drainUntilClosed(t, ch, 50*time.Second)

	runs := data.GetRecentAgentRuns(10)
	if len(runs) == 0 {
		t.Fatal("运行结束后必须留下至少一条运行记录")
	}
	r := runs[0]

	if r.Kind != RunKindChat {
		t.Errorf("未标注时 Kind 应为 chat，实际 %q", r.Kind)
	}
	if r.RequestedID != 99999 {
		t.Errorf("RequestedID 应为 99999，实际 %d", r.RequestedID)
	}
	if r.ResolvedID != cs[0].ID {
		t.Errorf("ResolvedID 应为实际使用的卡片 %d，实际 %d", cs[0].ID, r.ResolvedID)
	}
	if r.ModelName != "model-alpha" {
		t.Errorf("ModelName 应为实际模型 model-alpha，实际 %q", r.ModelName)
	}
	if !r.FallbackUsed {
		t.Error("请求了不存在的卡片 → 必须标记 FallbackUsed=true")
	}
	if r.Source == "" || r.Resolution == "" {
		t.Errorf("必须记录解析依据，实际 source=%q resolution=%q", r.Source, r.Resolution)
	}
	if r.Success {
		t.Error("端点不可达的运行不应被标记为成功")
	}
	if r.ErrorMsg == "" {
		t.Error("失败的运行必须记录错误原因")
	}
	if r.DurationMs <= 0 {
		t.Errorf("必须记录耗时，实际 %d", r.DurationMs)
	}
}

// 契约二：RunKind 必须随调用上下文被正确标注并落库。
func TestRunRecordCapturesRunKind(t *testing.T) {
	resetRunRecords()
	cs := seedRunCards(t, 0, unreachableCard("卡片乙", "model-beta"))
	if err := db.Dao.Model(&data.Settings{}).Where("1 = 1").Update("ai_config_id", cs[0].ID).Error; err != nil {
		t.Fatalf("设置全局默认: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = WithRunKind(ctx, RunKindPreMarket)
	ch := NewStockAiAgentApi().ChatWithContext(ctx, "ping", int(cs[0].ID), nil, false, 0, false, "direct")
	drainUntilClosed(t, ch, 50*time.Second)

	runs := data.GetRecentAgentRuns(10)
	if len(runs) == 0 {
		t.Fatal("必须留下运行记录")
	}
	if runs[0].Kind != RunKindPreMarket {
		t.Errorf("Kind 应为 %q，实际 %q", RunKindPreMarket, runs[0].Kind)
	}
	// 精确指定时不应标记回落
	if runs[0].FallbackUsed {
		t.Error("精确命中指定卡片时不应标记回落")
	}
	if runs[0].Source != data.ModelSourceExplicit {
		t.Errorf("Source 应为 explicit，实际 %q", runs[0].Source)
	}
}

// 契约三：WithRunKindIfUnset 让外层更精确的标注优先。
func TestWithRunKindIfUnsetPrefersExplicit(t *testing.T) {
	base := context.Background()
	serenity := WithRunKind(base, RunKindSerenity)
	// 内部通用入口不应覆盖外层标注
	if got := RunKindFrom(WithRunKindIfUnset(serenity, RunKindSectorAnomaly)); got != RunKindSerenity {
		t.Errorf("外层标注应优先，期望 %q 实际 %q", RunKindSerenity, got)
	}
	// 未标注时才写入
	if got := RunKindFrom(WithRunKindIfUnset(base, RunKindCron)); got != RunKindCron {
		t.Errorf("未标注时应写入 %q，实际 %q", RunKindCron, got)
	}
	// 默认值
	if got := RunKindFrom(base); got != RunKindChat {
		t.Errorf("未标注且未设置时默认应为 chat，实际 %q", got)
	}
}

// 契约四：运行记录是纯观测 —— 没有任何卡片时运行应报错，
// 但仍然留下一条说明“为什么没跑起来”的记录。
func TestRunRecordCapturesMissingConfig(t *testing.T) {
	resetRunRecords()
	if err := db.Dao.Exec("DELETE FROM ai_config").Error; err != nil {
		t.Fatalf("清理 ai_config: %v", err)
	}
	if err := db.Dao.Exec("DELETE FROM settings").Error; err != nil {
		t.Fatalf("清理 settings: %v", err)
	}
	if err := db.Dao.Create(&data.Settings{OpenAiEnable: true, Prompt: "test"}).Error; err != nil {
		t.Fatalf("写入 settings: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ch := NewStockAiAgentApi().ChatWithContext(ctx, "ping", 0, nil, false, 0, false, "direct")
	drainUntilClosed(t, ch, 20*time.Second)

	runs := data.GetRecentAgentRuns(10)
	if len(runs) == 0 {
		t.Fatal("即使没有可用模型，也必须留下运行记录（否则用户无从判断为什么没跑）")
	}
	r := runs[0]
	if r.Success {
		t.Error("无可用模型时不应标记成功")
	}
	if r.ErrorMsg == "" {
		t.Error("必须记录失败原因")
	}
}

// 契约五：GetRecentAgentRuns 必须倒序，且 limit 生效。
func TestGetRecentAgentRunsOrderingAndLimit(t *testing.T) {
	resetRunRecords()
	for i := 0; i < 5; i++ {
		data.RecordAgentRun(&models.AgentRunRecord{
			Kind: "chat", ModelName: "m", Source: data.ModelSourceExplicit,
			RequestedID: i, ResolvedID: uint(i + 1), FinishReason: "stop",
		})
	}
	all := data.GetRecentAgentRuns(10)
	if len(all) != 5 {
		t.Fatalf("应有 5 条记录，实际 %d", len(all))
	}
	if all[0].RequestedID != 4 {
		t.Errorf("必须倒序（最新在前），实际首条 RequestedID=%d", all[0].RequestedID)
	}
	limited := data.GetRecentAgentRuns(2)
	if len(limited) != 2 {
		t.Fatalf("limit=2 应返回 2 条，实际 %d", len(limited))
	}
	// 清空
	if n := data.ClearAgentRuns(); n != 5 {
		t.Errorf("清空应返回 5，实际 %d", n)
	}
	if len(data.GetRecentAgentRuns(10)) != 0 {
		t.Error("清空后应为空")
	}
}

// 契约六：`context deadline exceeded` 必须被判定为「我们自己的单次请求超时」，
// 而不是「服务端繁忙」—— 两者混淆会把用户引向完全错误的排查方向。
func TestRequestTimeoutIsNotClassifiedAsServerBusy(t *testing.T) {
	cases := []string{
		"context deadline exceeded",
		"Post \"https://x/v1/chat/completions\": context deadline exceeded",
		"request timeout",
		"timeout awaiting response headers",
		"i/o timeout",
	}
	for _, c := range cases {
		err := errors.New(c)
		if !isRequestTimeoutError(err) {
			t.Errorf("%q 应被判定为请求超时", c)
		}
	}
	// 真正的上游繁忙不应被判成请求超时（否则提示语又会反转错方向）
	for _, c := range []string{"429 Too Many Requests", "503 Service Unavailable", "overloaded"} {
		if isRequestTimeoutError(errors.New(c)) {
			t.Errorf("%q 是上游繁忙，不应判为请求超时", c)
		}
	}
	if isRequestTimeoutError(nil) {
		t.Error("nil 不应判为请求超时")
	}
}

// 契约七：effectiveRequestTimeoutSec 必须如实反映卡片的 TimeOut，
// 让错误提示能明确告诉用户"该改哪一项、改成多少"。
func TestEffectiveRequestTimeoutReflectsCard(t *testing.T) {
	resetRunRecords()
	cs := seedRunCards(t, 0, unreachableCard("超时卡", "timeout-model"))
	if err := db.Dao.Model(&data.AIConfig{}).Where("id = ?", cs[0].ID).Update("time_out", 120).Error; err != nil {
		t.Fatalf("设置 time_out 失败: %v", err)
	}
	if got := effectiveRequestTimeoutSec(int(cs[0].ID)); got != 120 {
		t.Errorf("应读到卡片配置的 120，实际 %d", got)
	}
	if got := effectiveRequestTimeoutSec(999999); got != 0 {
		t.Errorf("不存在的卡片应返回 0，实际 %d", got)
	}
}
