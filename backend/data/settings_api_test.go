package data

import (
	"testing"
	"blades-stock/backend/db"
)

func TestUpdateConfigAndAIConfig(t *testing.T) {
	t.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	db.Init("file::memory:?mode=memory&cache=shared")
	db.Dao.AutoMigrate(&Settings{}, &AIConfig{})

	db.Dao.Create(&Settings{
		TushareToken: "test",
	})

	settingConfig := GetSettingConfig()
	if settingConfig == nil {
		t.Fatalf("GetSettingConfig returned nil")
	}

	// 测试 1: 本地 API (ApiKey 为空) 应该允许保存，自动填充默认值不应被阻断
	localAIConfig := &AIConfig{
		ID:        0,
		Name:      "Ollama Local",
		BaseUrl:   "http://localhost:11434",
		ApiKey:    "", // 本地模型常为空
		ModelName: "qwen2.5:7b",
		TimeOut:   0,  // 为 0 时自动填充 300
	}

	settingConfig.AiConfigs = append(settingConfig.AiConfigs, localAIConfig)
	res := UpdateConfig(settingConfig)
	if res != "保存成功！" {
		t.Errorf("UpdateConfig with empty ApiKey failed: %s", res)
	}

	// 重新加载配置验证
	reloadConfig := GetSettingConfig()
	if len(reloadConfig.AiConfigs) == 0 {
		t.Fatalf("expected saved aiConfigs, got 0")
	}

	if reloadConfig.AiConfigs[0].ApiKey != "none" {
		t.Errorf("expected ApiKey fallback to 'none', got %q", reloadConfig.AiConfigs[0].ApiKey)
	}
	if reloadConfig.AiConfigs[0].TimeOut != 300 {
		t.Errorf("expected TimeOut fallback to 300, got %d", reloadConfig.AiConfigs[0].TimeOut)
	}

	// 测试 2: 修改配置并添加第二个配置
	savedConfig := reloadConfig.AiConfigs[0]
	savedConfig.Name = "Ollama Modified"
	
	newConfig2 := &AIConfig{
		ID:        999, // 前端传来的临时ID
		Name:      "DeepSeek API",
		BaseUrl:   "https://api.deepseek.com",
		ApiKey:    "sk-xxx",
		ModelName: "deepseek-chat",
		TimeOut:   120,
	}

	reloadConfig.AiConfigs = []*AIConfig{savedConfig, newConfig2}
	res2 := UpdateConfig(reloadConfig)
	if res2 != "保存成功！" {
		t.Fatalf("UpdateConfig phase 2 failed: %s", res2)
	}

	reloadConfig2 := GetSettingConfig()
	if len(reloadConfig2.AiConfigs) != 2 {
		t.Fatalf("expected 2 aiConfigs, got %d", len(reloadConfig2.AiConfigs))
	}
	if reloadConfig2.AiConfigs[0].Name != "Ollama Modified" {
		t.Errorf("expected Name 'Ollama Modified', got %q", reloadConfig2.AiConfigs[0].Name)
	}
	if reloadConfig2.AiConfigs[1].Name != "DeepSeek API" {
		t.Errorf("expected Name 'DeepSeek API', got %q", reloadConfig2.AiConfigs[1].Name)
	}

	// 测试 3: 删除配置 1，仅保留配置 2
	reloadConfig2.AiConfigs = []*AIConfig{reloadConfig2.AiConfigs[1]}
	res3 := UpdateConfig(reloadConfig2)
	if res3 != "保存成功！" {
		t.Fatalf("UpdateConfig phase 3 failed: %s", res3)
	}

	reloadConfig3 := GetSettingConfig()
	if len(reloadConfig3.AiConfigs) != 1 {
		t.Fatalf("expected 1 aiConfig after deletion, got %d", len(reloadConfig3.AiConfigs))
	}
	if reloadConfig3.AiConfigs[0].Name != "DeepSeek API" {
		t.Errorf("expected remaining config 'DeepSeek API', got %q", reloadConfig3.AiConfigs[0].Name)
	}
}

func TestMaskedSettingConfigAndPreserve(t *testing.T) {
	t.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	db.Init("file::memory:?mode=memory&cache=shared")
	db.Dao.AutoMigrate(&Settings{}, &AIConfig{})

	// 1. 验证辅助函数 MaskSecret 与 IsMaskedSecret
	realKey := "sk-2897194fdd7c13d7d16934383d"
	masked := MaskSecret(realKey)
	if masked != "sk-2****383d" {
		t.Fatalf("MaskSecret(%q) = %q, expected 'sk-2****383d'", realKey, masked)
	}
	if !IsMaskedSecret(masked) {
		t.Fatalf("IsMaskedSecret(%q) should be true", masked)
	}
	if IsMaskedSecret(realKey) {
		t.Fatalf("IsMaskedSecret(%q) should be false", realKey)
	}

	// 2. 初始化真实凭据
	realTushare := "dca1afb47b186d365ea451cd07e6180abcdef"
	realEmbedding := "nvapi-KK3sXw6AoPk3xw97m_P934AE4XKjabcdef"
	realTickFlow := "tk_c2d9a98e5da24c14bb433b1f1101abcdef"
	realEm := "em_1n3BvRz7m5v4wAJU2rYDKI90abcdef"

	cfg := GetSettingConfig()
	cfg.Settings.TushareToken = realTushare
	cfg.Settings.EmbeddingApiKey = realEmbedding
	cfg.Settings.TickFlowAPIKey = realTickFlow
	cfg.Settings.EmApiKey = realEm
	cfg.AiConfigs = []*AIConfig{
		{
			ID:        0,
			Name:      "Gemini Pro",
			BaseUrl:   "https://generativelanguage.googleapis.com",
			ApiKey:    realKey,
			ModelName: "gemini-1.5-pro",
		},
	}
	saveRes := UpdateConfig(cfg)
	if saveRes != "保存成功！" {
		t.Fatalf("初始保存配置失败: %s", saveRes)
	}

	// 3. 验证 GetMaskedSettingConfig 成功将凭据打码下发
	maskedCfg := GetMaskedSettingConfig()
	if maskedCfg.Settings.TushareToken == realTushare {
		t.Fatalf("TushareToken 未被脱敏: %s", maskedCfg.Settings.TushareToken)
	}
	if maskedCfg.Settings.EmbeddingApiKey == realEmbedding {
		t.Fatalf("EmbeddingApiKey 未被脱敏: %s", maskedCfg.Settings.EmbeddingApiKey)
	}
	if maskedCfg.Settings.TickFlowAPIKey == realTickFlow {
		t.Fatalf("TickFlowAPIKey 未被脱敏: %s", maskedCfg.Settings.TickFlowAPIKey)
	}
	if maskedCfg.Settings.EmApiKey == realEm {
		t.Fatalf("EmApiKey 未被脱敏: %s", maskedCfg.Settings.EmApiKey)
	}
	if maskedCfg.AiConfigs[0].ApiKey == realKey {
		t.Fatalf("AiConfig ApiKey 未被脱敏: %s", maskedCfg.AiConfigs[0].ApiKey)
	}

	// 4. 模拟前端带着掩码数据修改其它字段（如修改 Prompt 与超时时间）并回传保存
	maskedCfg.Settings.Prompt = "新的买方研报提示词"
	maskedCfg.AiConfigs[0].TimeOut = 120
	// 前端提交的 ApiKey 仍是掩码串
	resUpdate := UpdateConfig(maskedCfg)
	if resUpdate != "保存成功！" {
		t.Fatalf("带着掩码保存配置失败: %s", resUpdate)
	}

	// 5. 验证后端数据库中的真实凭据没有被掩码覆盖冲掉
	rawFromDB := GetSettingConfig()
	if rawFromDB.Settings.TushareToken != realTushare {
		t.Errorf("TushareToken 被掩码破坏，当前值: %s, 预期原值: %s", rawFromDB.Settings.TushareToken, realTushare)
	}
	if rawFromDB.Settings.EmbeddingApiKey != realEmbedding {
		t.Errorf("EmbeddingApiKey 被掩码破坏，当前值: %s, 预期原值: %s", rawFromDB.Settings.EmbeddingApiKey, realEmbedding)
	}
	if rawFromDB.Settings.TickFlowAPIKey != realTickFlow {
		t.Errorf("TickFlowAPIKey 被掩码破坏，当前值: %s, 预期原值: %s", rawFromDB.Settings.TickFlowAPIKey, realTickFlow)
	}
	if rawFromDB.Settings.EmApiKey != realEm {
		t.Errorf("EmApiKey 被掩码破坏，当前值: %s, 预期原值: %s", rawFromDB.Settings.EmApiKey, realEm)
	}
	if rawFromDB.AiConfigs[0].ApiKey != realKey {
		t.Errorf("AiConfig ApiKey 被掩码破坏，当前值: %s, 预期原值: %s", rawFromDB.AiConfigs[0].ApiKey, realKey)
	}
	if rawFromDB.Settings.Prompt != "新的买方研报提示词" {
		t.Errorf("Prompt 未更新成功: %s", rawFromDB.Settings.Prompt)
	}
	if rawFromDB.AiConfigs[0].TimeOut != 120 {
		t.Errorf("TimeOut 未更新成功: %d", rawFromDB.AiConfigs[0].TimeOut)
	}
}
