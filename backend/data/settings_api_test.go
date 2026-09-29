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
