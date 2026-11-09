package data

import (
	"path/filepath"
	"testing"

	"blades-stock/backend/db"
)

// 固化 v3.0.0 改造：模型选择收敛到 ResolveModel 唯一入口。
//
// 改造前仓库里同时存在 **4 套互不相同的解析行为**：
//  1. agent/agent_api.go        ID → 全局默认 → 第一张非 nil（并直接改写 Thinking）
//  2. data/NewDeepSeekOpenAi     ID → **空配置**（无任何回落）
//  3. agent/llm_summarizer.go   自定义内联解析，额外要求 ApiKey 非空
//  4. agent/price_alert_watcher.go 又一套内联解析
//
// 结果就是同一次请求在不同链路上“实际用的模型”可能不同，
// 而界面上永远只显示**请求的**那个 —— 用户反复问“到底哪个模型在跑”。

func newResolverEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	dir := t.TempDir()
	db.Init(filepath.Join(dir, "resolver.db"))
	t.Cleanup(func() {
		if db.Dao == nil {
			return
		}
		if sqlDB, err := db.Dao.DB(); err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.Dao.AutoMigrate(&Settings{}, &AIConfig{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	_ = db.Dao.Exec("DELETE FROM settings").Error
	_ = db.Dao.Exec("DELETE FROM ai_config").Error
}

func seedResolverCards(t *testing.T, globalDefault uint, cards ...AIConfig) []AIConfig {
	t.Helper()
	created := make([]AIConfig, 0, len(cards))
	for i := range cards {
		c := cards[i]
		if err := db.Dao.Create(&c).Error; err != nil {
			t.Fatalf("写入卡片失败: %v", err)
		}
		created = append(created, c)
	}
	if err := db.Dao.Create(&Settings{AiConfigID: globalDefault, RefreshInterval: 3}).Error; err != nil {
		t.Fatalf("写入 settings 失败: %v", err)
	}
	return created
}

func card(name, model, baseURL string) AIConfig {
	return AIConfig{Name: name, ModelName: model, BaseUrl: baseURL, ApiKey: "sk-test-key-123456"}
}

// 契约一：显式指定的卡片必须被精确使用。
func TestResolveModelExplicit(t *testing.T) {
	newResolverEnv(t)
	cs := seedResolverCards(t, 0,
		card("A", "model-a", "https://a.example.com/v1"),
		card("B", "model-b", "https://b.example.com/v1"),
	)

	res, err := ResolveModel(int(cs[1].ID))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.ResolvedID != cs[1].ID || res.Config.ModelName != "model-b" {
		t.Fatalf("应精确使用 B 卡片，实际 id=%d model=%q", res.ResolvedID, res.Config.ModelName)
	}
	if res.Source != ModelSourceExplicit || res.FallbackUsed {
		t.Fatalf("应标记为 explicit 且未回落，实际 source=%q fallback=%v", res.Source, res.FallbackUsed)
	}
}

// 契约二：id<=0 时使用全局默认（而不是“第一张”）。
func TestResolveModelUsesGlobalDefault(t *testing.T) {
	newResolverEnv(t)
	cs := seedResolverCards(t, 0, // 先占位，稍后回填默认
		card("A", "model-a", "https://a.example.com/v1"),
		card("B", "model-b", "https://b.example.com/v1"),
	)
	// 把全局默认指向 B
	if err := db.Dao.Model(&Settings{}).Where("1 = 1").Update("ai_config_id", cs[1].ID).Error; err != nil {
		t.Fatalf("设置全局默认失败: %v", err)
	}

	res, err := ResolveModel(0)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.ResolvedID != cs[1].ID {
		t.Fatalf("未指定时应采用全局默认 B(%d)，实际 %d", cs[1].ID, res.ResolvedID)
	}
	if res.Source != ModelSourceGlobalDefault {
		t.Fatalf("source 应为 global_default，实际 %q", res.Source)
	}
}

// 契约三：指定了一张**已被删除**的卡片时，必须回落到全局默认，
// 并如实标记 fallbackUsed（这是“界面说 A、实际跑 B”的核心场景）。
func TestResolveModelFallsBackToGlobalDefaultWhenRequestedMissing(t *testing.T) {
	newResolverEnv(t)
	cs := seedResolverCards(t, 0, card("A", "model-a", "https://a.example.com/v1"))
	if err := db.Dao.Model(&Settings{}).Where("1 = 1").Update("ai_config_id", cs[0].ID).Error; err != nil {
		t.Fatalf("设置全局默认失败: %v", err)
	}

	res, err := ResolveModel(99999) // 不存在的卡片
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.ResolvedID != cs[0].ID {
		t.Fatalf("应回落到全局默认 %d，实际 %d", cs[0].ID, res.ResolvedID)
	}
	if !res.FallbackUsed {
		t.Fatal("发生回落时必须如实标记 FallbackUsed")
	}
	if res.RequestedID != 99999 {
		t.Fatalf("RequestedID 应保留原始请求值 99999，实际 %d", res.RequestedID)
	}
	// Reason 必须能解释清楚，供日志/记录/界面直接展示
	if res.Reason == "" {
		t.Fatal("Reason 不能为空：它是运行记录与界面解释的依据")
	}
}

// 契约四：既没有指定、也没有全局默认（或默认也失效）时，回落第一张可用卡片。
func TestResolveModelFallsBackToFirstAvailable(t *testing.T) {
	newResolverEnv(t)
	cs := seedResolverCards(t, 0,
		card("A", "model-a", "https://a.example.com/v1"),
		card("B", "model-b", "https://b.example.com/v1"),
	)

	res, err := ResolveModel(0)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.ResolvedID != cs[0].ID {
		t.Fatalf("应回落到第一张 %d，实际 %d", cs[0].ID, res.ResolvedID)
	}
	if res.Source != ModelSourceFirstAvail || !res.FallbackUsed {
		t.Fatalf("source=%q fallback=%v 不符合预期", res.Source, res.FallbackUsed)
	}
}

// 契约五（关键）：WithThinking 必须只改**副本**，绝不污染存储。
//
// 旧实现是 `aiConfig.Thinking = thinkingMode`，直接写回了从
// GetSettingConfig() 取出的结构体 —— 请求级的界面开关会“粘”在配置上。
func TestWithThinkingDoesNotMutateStoredConfig(t *testing.T) {
	newResolverEnv(t)
	cs := seedResolverCards(t, 0, card("A", "model-a", "https://a.example.com/v1"))
	// 库里把这张卡片设为 thinking=false
	if err := db.Dao.Model(&AIConfig{}).Where("id = ?", cs[0].ID).Update("thinking", false).Error; err != nil {
		t.Fatalf("设置 thinking 失败: %v", err)
	}

	res, err := ResolveModel(int(cs[0].ID))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Config.Thinking {
		t.Fatal("前置假设不成立：库里应为 thinking=false")
	}
	overridden := res.WithThinking(true)
	if !overridden.Config.Thinking {
		t.Fatal("WithThinking(true) 未生效于副本")
	}
	if res.Config.Thinking {
		t.Fatal("WithThinking 污染了原解析结果")
	}

	// 存储必须保持 false
	stored := GetSettingConfig()
	for _, c := range stored.AiConfigs {
		if c.ID == cs[0].ID && c.Thinking {
			t.Fatal("WithThinking 把 thinking 写回了存储 —— 这正是旧实现的缺陷")
		}
	}
}

// 契约六：没有任何卡片时必须明确报错（而不是像 NewDeepSeekOpenAi 那样给个空配置）。
func TestResolveModelErrorsWhenNoCards(t *testing.T) {
	newResolverEnv(t)
	if err := db.Dao.Create(&Settings{RefreshInterval: 3}).Error; err != nil {
		t.Fatalf("写入 settings 失败: %v", err)
	}
	if _, err := ResolveModel(5); err == nil {
		t.Fatal("没有任何卡片时必须返回错误，而不是静默使用空配置")
	}
	if _, err := ResolveModel(0); err == nil {
		t.Fatal("没有任何卡片时 id<=0 也必须返回错误")
	}
}

// 契约七：Usable 必须如实反映“能否发起请求”。
func TestResolutionUsable(t *testing.T) {
	newResolverEnv(t)
	cs := seedResolverCards(t, 0, card("A", "model-a", "https://a.example.com/v1"))
	res, _ := ResolveModel(int(cs[0].ID))
	if !res.Usable() {
		t.Fatal("BaseUrl 与 ModelName 都非空时应为 Usable")
	}
	res.Config.ModelName = "  "
	if res.Usable() {
		t.Fatal("ModelName 为空白时不应为 Usable")
	}
}

// 契约八：ResolveModelForced 对“必须要有 API Key”的链路给出明确错误。
func TestResolveModelForcedRequiresApiKey(t *testing.T) {
	newResolverEnv(t)
	noKey := AIConfig{Name: "无密钥", ModelName: "m", BaseUrl: "https://x.example.com/v1"}
	cs := seedResolverCards(t, 0, noKey)
	if _, err := ResolveModelForced(int(cs[0].ID), true); err == nil {
		t.Fatal("requireApiKey=true 且无密钥时应报错")
	}
	if _, err := ResolveModelForced(int(cs[0].ID), false); err != nil {
		t.Fatalf("requireApiKey=false 时不应报错: %v", err)
	}
}

// 契约九：算法必须确定性 —— 配置不变时多次解析结果完全一致。
func TestResolveModelIsDeterministic(t *testing.T) {
	newResolverEnv(t)
	cs := seedResolverCards(t, 0,
		card("A", "model-a", "https://a.example.com/v1"),
		card("B", "model-b", "https://b.example.com/v1"),
	)
	if err := db.Dao.Model(&Settings{}).Where("1 = 1").Update("ai_config_id", cs[1].ID).Error; err != nil {
		t.Fatalf("设置全局默认失败: %v", err)
	}
	first, err := ResolveModel(0)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := ResolveModel(0)
		if err != nil {
			t.Fatalf("第 %d 次解析失败: %v", i, err)
		}
		if again.ResolvedID != first.ResolvedID || again.Source != first.Source {
			t.Fatalf("解析不确定：第 %d 次得到 id=%d source=%q，首次为 id=%d source=%q",
				i, again.ResolvedID, again.Source, first.ResolvedID, first.Source)
		}
	}
}
