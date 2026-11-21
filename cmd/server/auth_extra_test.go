package main

// AuthMiddleware / isAllowedOrigin / generateRandomToken 的补充测试。
// 与 server_test.go 的 TestAuthProtection 互补：后者只覆盖
// “无 Bearer → 401 / 有正确 Bearer → 200”两条路，本文件覆盖：
// Cookie 与 ?token= 两种取 token 方式、错误 token、免鉴权端点、
// 未知 /api/ 路径返回 404 而非 401、OPTIONS 预检、Origin 白名单。
// 刻意不走 setupTestServer（它会读真实 data/stock.db 并自动注入
// Authorization 头，测不到 Cookie / query 取 token 的分支），
// 而是直接对 AuthMiddleware + 哑 handler 做 httptest。

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// dummyNext 对所有放行请求返回 200，便于只断言中间件本身的行为。
func dummyNext() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// useTestToken 把全局 token 设为固定值，测试结束恢复成“重新初始化”
// 的状态（与 TestAuthProtection 的 defer 写法一致）。
func useTestToken(t *testing.T, token string) {
	t.Helper()
	t.Setenv("BLADES_AUTH_TOKEN", token)
	initAuth("")
	t.Cleanup(func() {
		_ = os.Unsetenv("BLADES_AUTH_TOKEN")
		initAuth("")
	})
}

func TestGenerateRandomToken_FormatAndUniqueness(t *testing.T) {
	a, err := generateRandomToken()
	if err != nil {
		t.Fatalf("generateRandomToken: %v", err)
	}
	b, err := generateRandomToken()
	if err != nil {
		t.Fatalf("generateRandomToken: %v", err)
	}
	// 32 字节 hex = 64 字符，只含 [0-9a-f]。
	if len(a) != 64 || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a) {
		t.Errorf("generateRandomToken = %q, want 64 hex chars", a)
	}
	if a == b {
		t.Error("two generateRandomToken calls returned the same token")
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	t.Setenv("BLADES_ALLOWED_ORIGINS", "")
	cases := []struct {
		origin string
		host   string
		want   bool
	}{
		{"https://example.com", "example.com", true},       // 同源
		{"https://example.com:8443", "example.com", false}, // 注意：比的是带端口的 Host，同 host 不同端口也不放行
		{"https://evil.com", "example.com", false},         // 任意外站不回显
		{"", "example.com", false},                         // 无 Origin 不放行
		{"https://localhost:34115", "example.com", true},   // 本地开发白名单
		{"http://127.0.0.1:8080", "example.com", true},     // 本地开发白名单
		{"https://kline.woha.men", "other", true},          // 生产域名后缀
		{"https://stock.8501025.xyz", "other", true},       // 生产域名后缀
		{"https://x.instabox.tech", "other", true},         // 生产域名后缀
		{"https://woha.men.evil.com", "other", false},      // 后缀欺骗：woha.men 不在末尾
		{"https://woha.men", "other", true},                // 生产域名本身
		{"https://evil-woha.men", "other", false},          // 后缀欺骗：须以 ".woha.men" 结尾才放行
		{"https://evil-8501025.xyz", "other", false},       // 同理
		{"not a url", "example.com", false},                // 解析不出 host
	}
	for _, c := range cases {
		if got := isAllowedOrigin(c.origin, c.host); got != c.want {
			t.Errorf("isAllowedOrigin(%q, %q) = %v, want %v", c.origin, c.host, got, c.want)
		}
	}
}

func TestIsAllowedOrigin_CustomEnvAllowlist(t *testing.T) {
	t.Setenv("BLADES_ALLOWED_ORIGINS", "https://custom.example.com, custom2.example.org")
	if !isAllowedOrigin("https://custom.example.com", "other") {
		t.Error("env allowlist should allow https://custom.example.com")
	}
	// 条目不带 scheme 时按 originHost 比对。
	if !isAllowedOrigin("https://custom2.example.org", "other") {
		t.Error("env allowlist should allow bare-host entry custom2.example.org")
	}
	if isAllowedOrigin("https://not-listed.example.com", "other") {
		t.Error("env allowlist should not allow unlisted origin")
	}
}

func TestAuthMiddleware_TokenSources(t *testing.T) {
	t.Setenv("BLADES_AUTH_TOKEN", "test-token-abc")
	initAuth("")
	h := AuthMiddleware(dummyNext())
	do := func(method, target string, setup func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		if setup != nil {
			setup(req)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	// 正确 Bearer → 200。
	bearer := func(r *http.Request) { r.Header.Set("Authorization", "Bearer test-token-abc") }
	if w := do("POST", "/api/App/GetConfig", bearer); w.Code != http.StatusOK {
		t.Errorf("valid Bearer: got %d, want 200 body=%s", w.Code, w.Body.String())
	}
	// 无任何凭证 → 401。
	if w := do("POST", "/api/App/GetConfig", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("no credentials: got %d, want 401", w.Code)
	}
	// 错误 Bearer → 401（防“只要带了头就放行”的回归）。
	wrongBearer := func(r *http.Request) { r.Header.Set("Authorization", "Bearer wrong") }
	if w := do("POST", "/api/App/GetConfig", wrongBearer); w.Code != http.StatusUnauthorized {
		t.Errorf("wrong Bearer: got %d, want 401", w.Code)
	}
	// HttpOnly Cookie 取 token → 200。
	cookie := func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "blades_token", Value: "test-token-abc"})
	}
	if w := do("POST", "/api/App/GetConfig", cookie); w.Code != http.StatusOK {
		t.Errorf("valid cookie: got %d, want 200 body=%s", w.Code, w.Body.String())
	}
	// 错误 Cookie → 401。
	wrongCookie := func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "blades_token", Value: "nope"})
	}
	if w := do("POST", "/api/App/GetConfig", wrongCookie); w.Code != http.StatusUnauthorized {
		t.Errorf("wrong cookie: got %d, want 401", w.Code)
	}
	// ?token= 降级取 token → 200。
	if w := do("POST", "/api/App/GetConfig?token=test-token-abc", nil); w.Code != http.StatusOK {
		t.Errorf("valid ?token=: got %d, want 200 body=%s", w.Code, w.Body.String())
	}
	// 错误 ?token= → 401。
	if w := do("POST", "/api/App/GetConfig?token=nope", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("wrong ?token=: got %d, want 401", w.Code)
	}
	// Bearer 前缀缺失（裸 token）不被识别 → 401。
	bare := func(r *http.Request) { r.Header.Set("Authorization", "test-token-abc") }
	if w := do("POST", "/api/App/GetConfig", bare); w.Code != http.StatusUnauthorized {
		t.Errorf("bare Authorization without Bearer prefix: got %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_ExemptAndUnknownPaths(t *testing.T) {
	useTestToken(t, "test-token-abc")
	h := AuthMiddleware(dummyNext())

	// 探针与静态端点免鉴权。
	for _, p := range []string{"/live", "/ready", "/health", "/api/health", "/robots.txt"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("exempt path %s: got %d, want 200", p, w.Code)
		}
	}
	// 未知非 App 的 /api/ 路径直接 404，而不是 401（防客户端把“不存在”误读成“未登录”）。
	req := httptest.NewRequest(http.MethodPost, "/api/does-not-exist", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown /api/ path: got %d, want 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "API endpoint not found") {
		t.Errorf("unknown /api/ path body = %q, want API endpoint not found", w.Body.String())
	}
}

func TestAuthMiddleware_Preflight(t *testing.T) {
	useTestToken(t, "test-token-abc")
	h := AuthMiddleware(dummyNext())

	// 同源预检 → 204 且回显 Origin + 凭据头。
	req := httptest.NewRequest(http.MethodOptions, "/api/App/GetConfig", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Host = "example.com"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("allowed preflight: got %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("allowed preflight ACAO = %q, want origin echo", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("allowed preflight ACAC = %q, want true", got)
	}

	// 非法 Origin 预检 → 403，且绝不回显该 Origin。
	req2 := httptest.NewRequest(http.MethodOptions, "/api/App/GetConfig", nil)
	req2.Header.Set("Origin", "https://evil.com")
	req2.Host = "example.com"
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Errorf("disallowed preflight: got %d, want 403", w2.Code)
	}
	if got := w2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed preflight ACAO = %q, want empty (never echo evil origin)", got)
	}
}
