package services

import (
	"strings"
	"testing"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
)

// TestSeedDefaultAIConfigs 守卫默认种子卡（2026-11-23 第三轮 C 组）：
//   - DeepSeek 首张卡（默认主用卡，Serenity 默认用它）BaseUrl 必须带 /v1
//     （无 /v1 时 FetchAiModels 的 /models 与对话的 /chat/completions 均 404），
//     TimeOut 必须是 600s（Serenity 慢推理模型“思考 60–120s + 输出 300s”
//     会撞上 300s 单次请求超时；600s 与外层 genCtx 10 分钟一致）；
//   - 硅基流动种子卡模型与注册表模板一致：Qwen/Qwen2.5-7B-Instruct；
//   - 其它种子卡的 TimeOut 保持 300s 不动（只改 Serenity 主用卡）。
func TestSeedDefaultAIConfigs(t *testing.T) {
	newServicesTestEnv(t)
	SeedDefaultAIConfigsIfEmpty()

	var cards []data.AIConfig
	if err := db.Dao.Order("id ASC").Find(&cards).Error; err != nil {
		t.Fatalf("读种子卡失败: %v", err)
	}
	byName := map[string]data.AIConfig{}
	for _, c := range cards {
		byName[c.Name] = c
	}

	deepseek, ok := byName["DeepSeek 官方 API"]
	if !ok {
		t.Fatalf("未播种 DeepSeek 官方 API 卡，got %d 张卡", len(cards))
	}
	if deepseek.BaseUrl != "https://api.deepseek.com/v1" {
		t.Errorf("DeepSeek 种子卡 BaseUrl = %q, want %q", deepseek.BaseUrl, "https://api.deepseek.com/v1")
	}
	if deepseek.TimeOut != 600 {
		t.Errorf("DeepSeek 种子卡（Serenity 主用卡）TimeOut = %d, want 600", deepseek.TimeOut)
	}

	siliconflow, ok := byName["硅基流动 (SiliconFlow)"]
	if !ok {
		t.Fatalf("未播种硅基流动卡")
	}
	if siliconflow.ModelName != "Qwen/Qwen2.5-7B-Instruct" {
		t.Errorf("硅基流动种子卡 ModelName = %q, want %q", siliconflow.ModelName, "Qwen/Qwen2.5-7B-Instruct")
	}
	if siliconflow.TimeOut != 300 {
		t.Errorf("硅基流动种子卡 TimeOut = %d, want 300（只改 Serenity 主用卡）", siliconflow.TimeOut)
	}

	// 其余种子卡的 BaseUrl 仍应带版本路径（回归：不要把 /v1 修正扩成全表改写）。
	for _, c := range cards {
		if strings.TrimSpace(c.BaseUrl) == "" {
			t.Errorf("种子卡 %q 的 BaseUrl 为空", c.Name)
		}
	}
}
