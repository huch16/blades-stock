package data

import (
	"strings"
	"testing"
)

// TestAiConfigTemplatesDeepSeekBaseURL 守卫 DeepSeek 模板的 BaseUrl 必须带 /v1。
//
// 根因（2026-11-23 第三轮 C 组）：后端注册表曾用 https://api.deepseek.com（无 /v1），
// 而前端兜底模板与新建默认值用的是带 /v1 的地址。两处调用都要求带 /v1：
//   - FetchAiModels：resty SetBaseURL(baseUrl) + Get("/models")，
//     无 /v1 会请求到 https://api.deepseek.com/models（404）；
//   - 对话：blades/contrib/openai 把 BaseURL 原样交给 openai-go WithBaseURL，
//     openai-go 以 BaseURL 为基准解析相对路径 chat/completions，
//     无 /v1 会请求到 https://api.deepseek.com/chat/completions（404）。
//
// 修正以后端向前端看齐：模板、种子卡与空地址兜底默认值统一为 /v1。
func TestAiConfigTemplatesDeepSeekBaseURL(t *testing.T) {
	tpls := NewSettingsApi().GetAiConfigTemplates()
	var found *AiConfigTemplate
	for _, tpl := range tpls {
		if strings.Contains(tpl.Label, "DeepSeek") {
			found = tpl
			break
		}
	}
	if found == nil {
		t.Fatalf("GetAiConfigTemplates 未返回 DeepSeek 模板")
	}
	if found.BaseUrl != "https://api.deepseek.com/v1" {
		t.Errorf("DeepSeek 模板 BaseUrl = %q, want %q（缺 /v1 会导致 /models 与 /chat/completions 均 404）",
			found.BaseUrl, "https://api.deepseek.com/v1")
	}
	// 用模板地址拼出的两个实际请求路径必须落在 DeepSeek 文档的 /v1 之下。
	if got := strings.TrimRight(found.BaseUrl, "/") + "/models"; got != "https://api.deepseek.com/v1/models" {
		t.Errorf("FetchAiModels 请求路径 = %q, want %q", got, "https://api.deepseek.com/v1/models")
	}
	if got := strings.TrimRight(found.BaseUrl, "/") + "/chat/completions"; got != "https://api.deepseek.com/v1/chat/completions" {
		t.Errorf("对话请求路径 = %q, want %q", got, "https://api.deepseek.com/v1/chat/completions")
	}
}

// TestAiConfigTemplatesSiliconFlowModel 守卫硅基流动模板模型与种子/前端兜底一致：
// 注册表（本文件）与种子卡（services/migrate.go）均为 Qwen/Qwen2.5-7B-Instruct，
// 前端离线兜底曾用 deepseek-ai/DeepSeek-V3，已统一为同一值。
// Qwen2.5-7B-Instruct 是硅基流动免费层模型，新 Key 无需充值即可调用模板默认值。
func TestAiConfigTemplatesSiliconFlowModel(t *testing.T) {
	tpls := NewSettingsApi().GetAiConfigTemplates()
	var found *AiConfigTemplate
	for _, tpl := range tpls {
		if strings.Contains(tpl.Label, "硅基流动") {
			found = tpl
			break
		}
	}
	if found == nil {
		t.Fatalf("GetAiConfigTemplates 未返回硅基流动模板")
	}
	if found.BaseUrl != "https://api.siliconflow.cn/v1" {
		t.Errorf("硅基流动模板 BaseUrl = %q, want %q", found.BaseUrl, "https://api.siliconflow.cn/v1")
	}
	if found.ModelName != "Qwen/Qwen2.5-7B-Instruct" {
		t.Errorf("硅基流动模板 ModelName = %q, want %q", found.ModelName, "Qwen/Qwen2.5-7B-Instruct")
	}
}
