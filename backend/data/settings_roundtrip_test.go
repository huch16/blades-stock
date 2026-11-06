package data

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"blades-stock/backend/db"
)

// 本文件固化 v3.0.0 修复的「控制中枢 · AI模型注册表」数据丢失/保存失败缺陷。
//
// 事故背景（用户反馈：“AI输入、配置、保存都成问题，还不如原来的配置”）：
//
//	后端 `SettingConfig` 是**匿名嵌入** `*Settings` 的结构体，其 JSON 契约是**扁平**的：
//	    { "ID":1, "prompt":"...", "aiConfigs":[ ... ] }
//	但 6a33fd1 重构后，前端改成了嵌套形态：
//	    { "settings": { ... }, "aiConfigs": [ ... ] }
//	后果一：加载时 res.settings 为 undefined → 基础设置表单完全无法回显。
//	后果二：保存时 s.Settings == nil → 后端直接返回「保存失败: 配置数据为空」，任何修改都存不进去。
//
//	同一重构还把模型配置的提交字段从“完整回传”改成“只回传 6 个核心字段”，
//	而后端 updateAiConfigsTx 使用 GORM Updates(map) **显式写入每一列**，
//	于是未提交的 maxTokens / embedding / httpProxy / sessionId / 配额等全部被清零。
//
// 本测试覆盖两条契约：① 扁平形态必须能正常保存；② 未提交的字段必须被保留。

func newRoundTripEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	// 必用 t.TempDir()：SQLite 的 "file:<name>?mode=memory" 形态在本仓库的驱动下
	// 会**真的落盘**成包目录里的文件（roundtrip_v300 / -shm / -wal），并污染仓库。
	// t.TempDir() 由测试框架自动清理，绝不会写进工作区。
	//
	// 注意 t.Cleanup 的执行顺序是 LIFO：t.TempDir() 先注册清理，
	// 因此这个后注册的“关闭连接”会**先**执行，否则 Windows 上会因文件占用而清理失败。
	dir := t.TempDir()
	db.Init(filepath.Join(dir, "roundtrip.db"))
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
	if err := db.Dao.Exec("DELETE FROM settings").Error; err != nil {
		t.Fatalf("清理 settings 失败: %v", err)
	}
	if err := db.Dao.Exec("DELETE FROM ai_config").Error; err != nil {
		t.Fatalf("清理 ai_config 失败: %v", err)
	}

	// 写入一份“用户精心配置过”的富配置，作为被误伤的对象
	if err := db.Dao.Create(&Settings{
		TushareToken:              "tk-realtoken-123456",
		Prompt:                    "原始系统提示词",
		RefreshInterval:           5,
		EmbeddingEnabled:          true,
		EmbeddingModelName:        "text-embedding-3-small",
		EmbeddingBaseUrl:          "https://embed.example.com/v1",
		EmbeddingApiKey:           "ek-realkey-abcdefgh",
		BacktestEnabled:           true,
		BacktestVerifyDays:        5,
		BacktestAccuracyThreshold: 7.5,
		McpTimeout:                12,
		ToolRunBudget:             33,
		ToolDailyBudget:           400,
		ToolDailyCostUSD:          3.5,
		PromptPlazaApiBase:        "https://plaza.example.com",
		UpdateChannel:             "beta",
	}).Error; err != nil {
		t.Fatalf("写入 settings 失败: %v", err)
	}

	if err := db.Dao.Create(&AIConfig{
		Name:                "富配置模型",
		BaseUrl:             "https://api.deepseek.com/v1",
		ApiKey:              "sk-realsecretkey-1234",
		ModelName:           "deepseek-chat",
		MaxTokens:           64000, // 关键：远大于前端硬编码的 4096
		Temperature:         1.3,
		TimeOut:             600,
		HttpProxy:           "http://proxy.internal:8080",
		HttpProxyEnabled:    true,
		SessionId:           "sess-abc-123",
		Thinking:            true,
		LLMConcurrencyLimit: 7,
		SubAgentTimeout:     45,
		RateLimitPerSecond:  3.5,
		Burst:               9,
		DailyRequestQuota:   500,
		DailyCostQuota:      12.5,
	}).Error; err != nil {
		t.Fatalf("写入 ai_config 失败: %v", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	return b
}

// 契约一：后端下发的是**扁平**形态（不是 { settings: {...} }）
func TestGetConfigPayloadIsFlatNotNested(t *testing.T) {
	newRoundTripEnv(t)

	raw := GetMaskedSettingConfig()
	if raw == nil {
		t.Fatal("GetMaskedSettingConfig 返回 nil")
	}
	b := mustJSON(t, raw)

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if _, nested := probe["settings"]; nested {
		t.Fatal("下发载荷含嵌套 settings 键：前端 res.settings 会取到 undefined")
	}
	for _, key := range []string{"prompt", "aiConfigs", "embeddingEnabled", "refreshInterval"} {
		if _, ok := probe[key]; !ok {
			t.Fatalf("下发载荷缺少顶层键 %q（应为扁平形态）", key)
		}
	}
}

// 契约二（事故复现）：嵌套形态必然保存失败
func TestNestedPayloadIsRejected(t *testing.T) {
	newRoundTripEnv(t)

	masked := GetMaskedSettingConfig()
	nested := map[string]any{
		"settings":  masked, // 前端在 6a33fd1 之后提交的形态
		"aiConfigs": masked.AiConfigs,
	}

	var sc SettingConfig
	if err := json.Unmarshal(mustJSON(t, nested), &sc); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if sc.Settings != nil {
		t.Fatal("前置假设不成立：嵌套形态竟然绑定了 Settings")
	}
	if res := UpdateConfig(&sc); res != "保存失败: 配置数据为空" {
		t.Fatalf("嵌套形态应被拒绝，实际返回: %q", res)
	}
}

// 契约三（用户实际流程）：把 GetConfig 的响应原样扁平回传，必须保存成功且字段零丢失
func TestFlatRoundTripPreservesEverything(t *testing.T) {
	newRoundTripEnv(t)

	// 完整模拟前端：GetConfig → （用户不改动）→ 原样回传
	masked := GetMaskedSettingConfig()
	var sc SettingConfig
	if err := json.Unmarshal(mustJSON(t, masked), &sc); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if res := UpdateConfig(&sc); res != "保存成功！" {
		t.Fatalf("扁平完整回传应保存成功，实际: %q", res)
	}

	after := GetSettingConfig()
	// 敏感凭据必须被掩码恢复（而不是被 sk-**** 覆盖）
	if after.TushareToken != "tk-realtoken-123456" {
		t.Errorf("tushareToken 被掩码覆盖: %q", after.TushareToken)
	}
	if after.EmbeddingApiKey != "ek-realkey-abcdefgh" {
		t.Errorf("embeddingApiKey 被掩码覆盖: %q", after.EmbeddingApiKey)
	}
	// 透传的 Settings 字段一个都不能丢
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"embeddingEnabled", after.EmbeddingEnabled, true},
		{"embeddingModelName", after.EmbeddingModelName, "text-embedding-3-small"},
		{"embeddingBaseUrl", after.EmbeddingBaseUrl, "https://embed.example.com/v1"},
		{"backtestEnabled", after.BacktestEnabled, true},
		{"backtestVerifyDays", after.BacktestVerifyDays, int64(5)},
		{"backtestAccuracyThreshold", after.BacktestAccuracyThreshold, 7.5},
		{"mcpTimeout", after.McpTimeout, int64(12)},
		{"toolRunBudget", after.ToolRunBudget, int64(33)},
		{"toolDailyBudget", after.ToolDailyBudget, int64(400)},
		{"toolDailyCostUSD", after.ToolDailyCostUSD, 3.5},
		{"promptPlazaApiBase", after.PromptPlazaApiBase, "https://plaza.example.com"},
		{"updateChannel", after.UpdateChannel, "beta"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("Settings.%s 丢失/被改写: got=%v want=%v", c.name, c.got, c.want)
		}
	}

	if len(after.AiConfigs) != 1 {
		t.Fatalf("AI 配置数量异常: %d", len(after.AiConfigs))
	}
	ai := after.AiConfigs[0]
	if ai.ApiKey != "sk-realsecretkey-1234" {
		t.Errorf("AI ApiKey 被掩码覆盖: %q", ai.ApiKey)
	}
	aiChecks := []struct {
		name string
		got  any
		want any
	}{
		{"MaxTokens", ai.MaxTokens, 64000},
		{"Temperature", ai.Temperature, 1.3},
		{"TimeOut", ai.TimeOut, 600},
		{"HttpProxy", ai.HttpProxy, "http://proxy.internal:8080"},
		{"HttpProxyEnabled", ai.HttpProxyEnabled, true},
		{"SessionId", ai.SessionId, "sess-abc-123"},
		{"Thinking", ai.Thinking, true},
		{"LLMConcurrencyLimit", ai.LLMConcurrencyLimit, 7},
		{"SubAgentTimeout", ai.SubAgentTimeout, 45},
		{"RateLimitPerSecond", ai.RateLimitPerSecond, 3.5},
		{"Burst", ai.Burst, 9},
		{"DailyRequestQuota", ai.DailyRequestQuota, 500},
		{"DailyCostQuota", ai.DailyCostQuota, 12.5},
	}
	for _, c := range aiChecks {
		if c.got != c.want {
			t.Errorf("AIConfig.%s 丢失/被改写: got=%v want=%v", c.name, c.got, c.want)
		}
	}
}

// 契约四（回归防线）：即使调用方只提交 5 项核心身份字段，
// 后端也必须保留未提交的调度/扩展字段，而不是清零。
func TestPartialAiConfigPayloadDoesNotWipeFields(t *testing.T) {
	newRoundTripEnv(t)

	var existing AIConfig
	if err := db.Dao.First(&existing).Error; err != nil {
		t.Fatalf("读取现有 ai_config 失败: %v", err)
	}

	// 模拟“精简版前端”只发送 6 个字段
	partial := map[string]any{
		"prompt": "只改了提示词",
		"aiConfigs": []map[string]any{{
			"ID":        existing.ID,
			"name":      "改名后的模型",
			"baseUrl":   existing.BaseUrl,
			"apiKey":    "sk-****", // 掩码，必须被还原为真实密钥
			"modelName": existing.ModelName,
			"thinking":  existing.Thinking,
		}},
	}

	var sc SettingConfig
	if err := json.Unmarshal(mustJSON(t, partial), &sc); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if res := UpdateConfig(&sc); res != "保存成功！" {
		t.Fatalf("部分字段提交应保存成功，实际: %q", res)
	}

	after := GetSettingConfig()
	if after.Prompt != "只改了提示词" {
		t.Errorf("显式提交的 prompt 未生效: %q", after.Prompt)
	}
	// Settings 侧：无界面入口的字段必须被保留
	if !after.EmbeddingEnabled || after.EmbeddingModelName != "text-embedding-3-small" {
		t.Errorf("Settings 侧 embedding 配置被清零: enabled=%v model=%q",
			after.EmbeddingEnabled, after.EmbeddingModelName)
	}
	if after.McpTimeout != 12 || after.ToolRunBudget != 33 || after.ToolDailyCostUSD != 3.5 {
		t.Errorf("Settings 侧 mcp/tool 预算被清零: mcp=%d run=%d cost=%v",
			after.McpTimeout, after.ToolRunBudget, after.ToolDailyCostUSD)
	}

	if len(after.AiConfigs) != 1 {
		t.Fatalf("AI 配置数量异常: %d", len(after.AiConfigs))
	}
	ai := after.AiConfigs[0]
	if ai.Name != "改名后的模型" {
		t.Errorf("显式提交的 name 未生效: %q", ai.Name)
	}
	if ai.ApiKey != "sk-realsecretkey-1234" {
		t.Errorf("掩码 ApiKey 未被还原为真实密钥: %q", ai.ApiKey)
	}
	// 关键回归点：这些字段没被提交，必须保持原值
	if ai.MaxTokens != 64000 {
		t.Errorf("MaxTokens 被清零/降级: got=%d want=64000", ai.MaxTokens)
	}
	if ai.Temperature != 1.3 || ai.TimeOut != 600 {
		t.Errorf("Temperature/TimeOut 被清零: temp=%v timeout=%d", ai.Temperature, ai.TimeOut)
	}
	if ai.HttpProxy != "http://proxy.internal:8080" || !ai.HttpProxyEnabled {
		t.Errorf("httpProxy 配置被清零: proxy=%q enabled=%v", ai.HttpProxy, ai.HttpProxyEnabled)
	}
	if ai.SessionId != "sess-abc-123" {
		t.Errorf("SessionId 被清零: %q", ai.SessionId)
	}
	if ai.LLMConcurrencyLimit != 7 || ai.SubAgentTimeout != 45 {
		t.Errorf("并发/子Agent超时被清零: limit=%d subTimeout=%d", ai.LLMConcurrencyLimit, ai.SubAgentTimeout)
	}
	if ai.Burst != 9 || ai.DailyRequestQuota != 500 || ai.DailyCostQuota != 12.5 {
		t.Errorf("配额配置被清零: burst=%d reqQuota=%d costQuota=%v", ai.Burst, ai.DailyRequestQuota, ai.DailyCostQuota)
	}
}

// 契约五：新增配置（ID<=0）必须自动补齐调度默认值，不能落库为零值
func TestNewAiConfigGetsDefaults(t *testing.T) {
	newRoundTripEnv(t)

	partial := map[string]any{
		"prompt": "p",
		"aiConfigs": []map[string]any{{
			"ID":        0,
			"name":      "全新模型",
			"baseUrl":   "http://localhost:11434",
			"apiKey":    "",
			"modelName": "qwen2.5:7b",
			"thinking":  false,
		}},
	}
	var sc SettingConfig
	if err := json.Unmarshal(mustJSON(t, partial), &sc); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if res := UpdateConfig(&sc); res != "保存成功！" {
		t.Fatalf("新增配置应保存成功，实际: %q", res)
	}

	after := GetSettingConfig()
	if len(after.AiConfigs) != 1 {
		t.Fatalf("期望仅剩 1 个新增配置，实际 %d", len(after.AiConfigs))
	}
	ai := after.AiConfigs[0]
	if ai.MaxTokens <= 0 {
		t.Errorf("新增配置 MaxTokens 未补默认值: %d", ai.MaxTokens)
	}
	if ai.TimeOut <= 0 {
		t.Errorf("新增配置 TimeOut 未补默认值: %d", ai.TimeOut)
	}
	if ai.LLMConcurrencyLimit <= 0 {
		t.Errorf("新增配置 LLMConcurrencyLimit 未补默认值: %d", ai.LLMConcurrencyLimit)
	}
	if ai.SubAgentTimeout <= 0 {
		t.Errorf("新增配置 SubAgentTimeout 未补默认值: %d", ai.SubAgentTimeout)
	}
}
