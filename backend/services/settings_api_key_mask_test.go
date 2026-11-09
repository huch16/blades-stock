package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
)

// 固化 v3.0.0 修复：连通性测试 / 拉取模型列表时，前端提交的是**掩码**密钥。
//
// 事故背景（用户反馈「测试连通性还是不成功？？？」）：
//
//	GetMaskedSettingConfig 下发时会把 ApiKey 打码为 sk-a****1234；
//	设置页在用户未改动密钥时，表单里保存的就是这个掩码；
//	而 testAiConfig() / fetchAiModels() 直接把表单里的 apiKey 原样送到后端。
//	后端再拿它去请求真正的供应商 → 供应商返回 401（密钥无效），
//	用户看到的却是一句与真实原因无关的报错。
//
// 因为 RPC 签名（baseUrl, apiKey, modelName）不能改（Wails 绑定兼容），
// 后端必须在收到掩码时，按 baseUrl（+ modelName）从已保存配置中还原真实密钥。

func newServicesTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	dir := t.TempDir()
	db.Init(filepath.Join(dir, "svc.db"))
	t.Cleanup(func() {
		if db.Dao == nil {
			return
		}
		if sqlDB, err := db.Dao.DB(); err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.Dao.AutoMigrate(&data.Settings{}, &data.AIConfig{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	_ = db.Dao.Exec("DELETE FROM settings").Error
	_ = db.Dao.Exec("DELETE FROM ai_config").Error
}

// strictProvider 模拟真实供应商：Bearer 密钥不对就 401，且只在密钥正确时返回模型列表。
func strictProvider(t *testing.T, realKey string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+realKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key provided","type":"invalid_request_error"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/models"):
			_, _ = w.Write([]byte(`{"data":[{"id":"model-alpha"},{"id":"model-beta"}]}`))
		default:
			_, _ = w.Write([]byte(`{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"pong"}}]}`))
		}
	}))
}

func seedCard(t *testing.T, baseURL, realKey, model string) {
	t.Helper()
	if err := db.Dao.Create(&data.Settings{RefreshInterval: 3}).Error; err != nil {
		t.Fatalf("写入 settings 失败: %v", err)
	}
	if err := db.Dao.Create(&data.AIConfig{
		Name: "严格校验供应商", BaseUrl: baseURL, ApiKey: realKey, ModelName: model,
	}).Error; err != nil {
		t.Fatalf("写入 ai_config 失败: %v", err)
	}
}

// 契约一：把 GetMaskedSettingConfig 下发的掩码原样送回来，测试必须**成功**。
// （旧实现会把它直接当密钥发给供应商 → 401。）
func TestTestAiConnectionResolvesMaskedApiKey(t *testing.T) {
	newServicesTestEnv(t)
	const realKey = "sk-live-secret-key-abcdef"
	srv := strictProvider(t, realKey)
	defer srv.Close()
	seedCard(t, srv.URL, realKey, "model-alpha")

	masked := data.GetMaskedSettingConfig()
	if len(masked.AiConfigs) != 1 {
		t.Fatalf("期望 1 张卡片，实际 %d", len(masked.AiConfigs))
	}
	sentKey := masked.AiConfigs[0].ApiKey
	if !data.IsMaskedSecret(sentKey) {
		t.Fatalf("前置假设不成立：下发的密钥不是掩码: %q", sentKey)
	}
	if sentKey == realKey {
		t.Fatal("前置假设不成立：下发的密钥竟然是明文")
	}

	svc := NewSettingsService()
	msg := svc.TestAiConnection(srv.URL, sentKey, "model-alpha")
	if msg != "" {
		t.Fatalf("提交掩码密钥时测试应成功（后端须还原真实密钥），实际返回: %q", msg)
	}
}

// 契约二：拉取模型列表同样必须能用掩码密钥成功。
func TestFetchAiModelsResolvesMaskedApiKey(t *testing.T) {
	newServicesTestEnv(t)
	const realKey = "sk-live-secret-key-abcdef"
	srv := strictProvider(t, realKey)
	defer srv.Close()
	seedCard(t, srv.URL, realKey, "model-alpha")

	masked := data.GetMaskedSettingConfig()
	sentKey := masked.AiConfigs[0].ApiKey

	svc := NewSettingsService()
	got := svc.FetchAiModels(srv.URL, sentKey)
	if len(got) != 2 || got[0] != "model-alpha" {
		t.Fatalf("提交掩码密钥时拉取列表应成功，实际返回: %v", got)
	}
}

// 契约三：掩码无法匹配到任何已保存配置时，必须给出**可行动**的解释，
// 而不是把掩码当密钥发出去换来一句莫名其妙的 401。
func TestMaskedApiKeyWithoutMatchGivesActionableMessage(t *testing.T) {
	newServicesTestEnv(t)
	const realKey = "sk-live-secret-key-abcdef"
	srv := strictProvider(t, realKey)
	defer srv.Close()
	// 库中配置的 baseUrl 与要测试的地址不一致 → 无法按 baseUrl 还原
	seedCard(t, "https://somewhere-else.example.com/v1", realKey, "model-alpha")

	svc := NewSettingsService()
	msg := svc.TestAiConnection(srv.URL, "sk-o****9999", "model-alpha")
	if msg == "" {
		t.Fatal("无法还原真实密钥时应明确报错，而不是静默发一个无效密钥")
	}
	for _, want := range []string{"掩码", "重新", "密钥"} {
		if !strings.Contains(msg, want) {
			t.Errorf("错误信息应包含 %q 以指引用户，实际: %q", want, msg)
		}
	}
	// 绝不应把「密钥无效」这类误导性结论抛给用户
	if strings.Contains(msg, "401") || strings.Contains(msg, "Invalid API key") {
		t.Errorf("不应把上游 401 原文当成结论: %q", msg)
	}
}

// 契约四：明文密钥必须原样使用（不能被还原逻辑改写或丢弃）。
func TestPlainApiKeyStillWorks(t *testing.T) {
	newServicesTestEnv(t)
	const realKey = "sk-live-secret-key-abcdef"
	srv := strictProvider(t, realKey)
	defer srv.Close()
	seedCard(t, srv.URL, realKey, "model-alpha")

	svc := NewSettingsService()
	if msg := svc.TestAiConnection(srv.URL, realKey, "model-alpha"); msg != "" {
		t.Fatalf("明文密钥应直接可用，实际: %q", msg)
	}
	// 相反的密钥必须失败（证明上面不是"永远返回成功"）
	if msg := svc.TestAiConnection(srv.URL, "sk-totally-wrong-key-999999", "model-alpha"); msg == "" {
		t.Fatal("错误的明文密钥必须失败")
	}
}

// 契约五：多卡片场景下，掩码必须按 baseUrl + modelName 精确还原到**同一张**卡片，
// 绝不能张冠李戴用别人卡片的密钥。
func TestMaskedKeyResolvesToCorrectCardAmongMany(t *testing.T) {
	newServicesTestEnv(t)
	const keyA = "sk-key-aaaa-11111111"
	const keyB = "sk-key-bbbb-22222222"
	srvA := strictProvider(t, keyA)
	defer srvA.Close()
	srvB := strictProvider(t, keyB)
	defer srvB.Close()

	if err := db.Dao.Create(&data.Settings{RefreshInterval: 3}).Error; err != nil {
		t.Fatalf("settings: %v", err)
	}
	for _, c := range []data.AIConfig{
		{Name: "A1", BaseUrl: srvA.URL, ApiKey: keyA, ModelName: "model-alpha"},
		{Name: "B1", BaseUrl: srvB.URL, ApiKey: keyB, ModelName: "model-beta"},
	} {
		if err := db.Dao.Create(&c).Error; err != nil {
			t.Fatalf("ai_config: %v", err)
		}
	}

	svc := NewSettingsService()
	// srvB 只接受 keyB；若把 keyA 用在这里就会 401
	if msg := svc.TestAiConnection(srvB.URL, "sk-k****2222", "model-beta"); msg != "" {
		t.Fatalf("应按 baseUrl+modelName 还原到 B 卡片的密钥，实际: %q", msg)
	}
	if msg := svc.TestAiConnection(srvA.URL, "sk-k****1111", "model-alpha"); msg != "" {
		t.Fatalf("应按 baseUrl+modelName 还原到 A 卡片的密钥，实际: %q", msg)
	}
}

// 契约六：resolved 标志必须如实反映"是否还原成功"，供调用方决定是否继续。
func TestResolveRealApiKeyReportsResolution(t *testing.T) {
	newServicesTestEnv(t)
	const realKey = "sk-live-secret-key-abcdef"
	seedCard(t, "https://provider.example.com/v1", realKey, "model-alpha")

	// 明文：直接返回，视为已就绪
	got, ok := resolveRealApiKey("https://provider.example.com/v1", realKey, "model-alpha")
	if !ok || got != realKey {
		t.Errorf("明文密钥应原样返回且 ok=true, got=%q ok=%v", got, ok)
	}
	// 掩码 + 地址匹配：还原
	got, ok = resolveRealApiKey("https://provider.example.com/v1/", "sk-l****cdef", "model-alpha")
	if !ok || got != realKey {
		t.Errorf("掩码应被还原, got=%q ok=%v", got, ok)
	}
	// 掩码 + 地址不匹配：必须报告未还原
	if _, ok = resolveRealApiKey("https://unknown.example.com/v1", "sk-l****cdef", "model-alpha"); ok {
		t.Error("地址不匹配时应报告 ok=false，避免调用方拿掩码去请求上游")
	}
}

// 契约七：本地模型常用的占位密钥 "none" 不是掩码，不得被误当作掩码处理。
func TestPlaceholderNoneKeyIsNotTreatedAsMask(t *testing.T) {
	newServicesTestEnv(t)
	if got, ok := resolveRealApiKey("http://127.0.0.1:11434/v1", "none", "qwen2.5:7b"); !ok || got != "none" {
		t.Errorf("占位密钥 none 应原样保留, got=%q ok=%v", got, ok)
	}
}

// 保证测试环境与线上保持同一 JSON 契约（防止 GetMaskedSettingConfig 未来改成嵌套形态）
func TestMaskedConfigPayloadShapeIsFlat(t *testing.T) {
	newServicesTestEnv(t)
	seedCard(t, "https://provider.example.com/v1", "sk-live-secret-key-abcdef", "model-alpha")
	b, err := json.Marshal(data.GetMaskedSettingConfig())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, nested := probe["settings"]; nested {
		t.Fatal("下发载荷不应含嵌套 settings 键")
	}
	if !strings.Contains(string(b), "aiConfigs") {
		t.Fatal("下发载荷应含 aiConfigs")
	}

}
