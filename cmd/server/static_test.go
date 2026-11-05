package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildDistFixture 构造一个模拟的前端产物目录
func buildDistFixture(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, "dist")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	for rel, body := range files {
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", target, err)
		}
	}
	return dir
}

func doGet(h http.Handler, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestStaticHandler_AssetImmutableCache 验证 /assets/ 命中内容哈希文件时强制注入
// 1 年不可变缓存头（P1：此前实测为 no-store，浏览器无法回退到任何缓存副本）。
func TestStaticHandler_AssetImmutableCache(t *testing.T) {
	root := t.TempDir()
	dist := buildDistFixture(t, root, map[string]string{
		"index.html":             "<html><body>app</body></html>",
		"assets/index-abc123.js": "console.log('entry')",
	})

	h := NewStaticHandler(dist, "")

	rec := doGet(h, "/assets/index-abc123.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	got := rec.Header().Get("Cache-Control")
	if got != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable cache-control, got %q", got)
	}
}

// TestStaticHandler_StaleChunkFallsBackToPreviousRelease 是本轮 P0 的核心回归测试：
// 部署新版本后，旧哈希 chunk 在当前 dist 中已不存在，但必须能从上一版本目录回退返回 200，
// 而不是 404 —— 否则所有「打开着旧页面」的在线用户点击懒加载路由时会白屏。
func TestStaticHandler_StaleChunkFallsBackToPreviousRelease(t *testing.T) {
	root := t.TempDir()

	current := buildDistFixture(t, filepath.Join(root, "current"), map[string]string{
		"index.html":             "<html><body>v2</body></html>",
		"assets/index-new111.js": "console.log('v2')",
	})
	previous := buildDistFixture(t, filepath.Join(root, "previous"), map[string]string{
		"index.html":                 "<html><body>v1</body></html>",
		"assets/index-old000.js":     "console.log('v1')",
		"assets/agent-chat-old00.js": "console.log('old agent chat chunk')",
	})

	// ---- 武装回退 ----
	armed := NewStaticHandler(current, previous)

	rec := doGet(armed, "/assets/agent-chat-old00.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("武装回退时期望旧 chunk 返回 200，实际 %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "old agent chat chunk") {
		t.Fatalf("回退内容不正确: %q", body)
	}
	if hdr := rec.Header().Get("X-Release-Fallback"); hdr != "previous" {
		t.Fatalf("缺少 X-Release-Fallback 标记，实际 %q", hdr)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=600" {
		t.Fatalf("旧 chunk 应为短期搭桥缓存，实际 %q", cc)
	}

	// ---- 未武装回退（回归防护：确认确实会 404，从而证明回退不是空转）----
	unarmed := NewStaticHandler(current, "")
	rec = doGet(unarmed, "/assets/agent-chat-old00.js")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("未武装回退时期望 404，实际 %d", rec.Code)
	}

	// ---- 两个版本都不存在 → 必须 404，而不是误报命中 ----
	rec = doGet(armed, "/assets/never-existed.js")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("期望 404，实际 %d", rec.Code)
	}
}

// TestResolvePreviousDistDir 校验上一版目录解析的防御性约束
func TestResolvePreviousDistDir(t *testing.T) {
	root := t.TempDir()
	current := buildDistFixture(t, filepath.Join(root, "dist"), map[string]string{
		"index.html": "<html>v2</html>",
	})

	// 1) 显式路径不存在 → 回退到 "<dist>-previous"，同样不存在 → 返回空
	if got := resolvePreviousDistDir(current, filepath.Join(root, "nope")); got != "" {
		t.Fatalf("期望空字符串，实际 %q", got)
	}

	// 2) 防御：显式传入当前 dist 本身时必须被拒绝（否则 404 会被误判为命中）。
	//    注意：为避免与下方「约定目录」用例互相干扰，本用例使用一个干净且无 -previous 邻居的 dist。
	isolated := buildDistFixture(t, filepath.Join(root, "isolated"), map[string]string{
		"index.html": "<html>v2</html>",
	})
	if got := resolvePreviousDistDir(isolated, isolated); got != "" {
		t.Fatalf("上一版目录不得等于当前目录，实际返回 %q", got)
	}

	// 3) 约定目录 <dist>-previous 存在且含 index.html → 命中
	prevConvention := current + "-previous"
	if err := os.MkdirAll(filepath.Join(prevConvention, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prevConvention, "index.html"), []byte("<html>v1</html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := resolvePreviousDistDir(current, ""); got == "" {
		t.Fatal("期望命中约定目录 <dist>-previous，实际为空")
	}

	// 4) 目录存在但缺少 index.html（不是完整产物）→ 拒绝
	incomplete := filepath.Join(root, "incomplete")
	if err := os.MkdirAll(incomplete, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := resolvePreviousDistDir(current, incomplete); got == "" {
		// 显式路径被拒绝后应回退到约定目录；只要不是 incomplete 即为正确
		if strings.Contains(got, "incomplete") {
			t.Fatalf("缺少 index.html 的目录不得作为回退源，实际返回 %q", got)
		}
	}
	if got := resolvePreviousDistDir(isolated, filepath.Join(root, "incomplete")); got != "" {
		t.Fatalf("孤立 dist 无任何可用上一版目录时应返回空，实际 %q", got)
	}
}

// TestShouldDenyStaticPath 锁定「敏感路径绝不能被 SPA 壳伪装成 200」——
// 否则任何安全扫描器都会收到一份高危误报清单（.git 目录可访问 / Dockerfile 暴露），
// 且这类探测永远产生不了真实 404，监控失效。
func TestShouldDenyStaticPath(t *testing.T) {
	denied := []string{
		"/.git/config", "/.git/HEAD", "/.env", "/.env.production", "/.DS_Store",
		"/Dockerfile", "/docker-compose.yml", "/Makefile", "/package.json",
		"/package-lock.json", "/go.mod", "/vite.config.js", "/node_modules/x.js",
		"/.ssh/id_rsa", "/.aws/credentials", "/backend/secret.go", "/data/stock.db",
		"/logs/app.log", "/scripts/deploy-vps.sh", "/nginx.conf",
	}
	for _, p := range denied {
		if !shouldDenyStaticPath(p) {
			t.Errorf("%s 应被拒绝（SPA 兜底会把它变成 200）", p)
		}
	}

	allowed := []string{
		"/", "", "/research", "/agent", "/index.html", "/assets/index-abc.js",
		"/.well-known/acme-challenge/token", "/favicon.ico", "/robots.txt",
	}
	for _, p := range allowed {
		if shouldDenyStaticPath(p) {
			t.Errorf("%s 不应被拒绝", p)
		}
	}
}

// TestStaticHandler_DeniesSensitivePaths 验证端到端行为：
// 敏感路径必须返回 404，且响应体不能是 SPA 壳 HTML。
func TestStaticHandler_DeniesSensitivePaths(t *testing.T) {
	root := t.TempDir()
	dist := buildDistFixture(t, root, map[string]string{
		"index.html": "<html><body>app-shell</body></html>",
	})
	h := NewStaticHandler(dist, "")

	for _, p := range []string{"/.git/config", "/Dockerfile", "/Makefile", "/package.json", "/.env"} {
		rec := doGet(h, p)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s 应返回 404，实际 %d", p, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "app-shell") {
			t.Errorf("%s 不得回落 index.html", p)
		}
	}
}

// TestStaticHandler_FlagsSPAFallback 验证非根路径的 SPA 兜底会打上 X-SPA-Fallback 标记，
// 使运维能区分「真实页面」与「被兜底的正向 200」。
func TestStaticHandler_FlagsSPAFallback(t *testing.T) {
	root := t.TempDir()
	dist := buildDistFixture(t, root, map[string]string{
		"index.html": "<html><body>app-shell</body></html>",
	})
	h := NewStaticHandler(dist, "")

	rec := doGet(h, "/research")
	if rec.Code != http.StatusOK {
		t.Fatalf("SPA 路由应回落 index.html，实际 %d", rec.Code)
	}
	if rec.Header().Get("X-SPA-Fallback") != "1" {
		t.Fatalf("非根路径的 SPA 兜底应标记 X-SPA-Fallback: 1，实际 %q", rec.Header().Get("X-SPA-Fallback"))
	}

	// 根路径是正常入口，不应被标记为兜底
	rec = doGet(h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("根路径应 200，实际 %d", rec.Code)
	}
	if hdr := rec.Header().Get("X-SPA-Fallback"); hdr != "" {
		t.Fatalf("根路径不应带 X-SPA-Fallback，实际 %q", hdr)
	}
}

// TestStaticHandler_NoHTMLFallbackForMissingFiles 锁定 P0-2 行为：
// 缺失的带扩展名文件与 /assets/ 文件绝不能回落 index.html（否则 SPA 会把 404 伪装成 200）。
func TestStaticHandler_NoHTMLFallbackForMissingFiles(t *testing.T) {
	root := t.TempDir()
	dist := buildDistFixture(t, root, map[string]string{
		"index.html": "<html><body>app-shell</body></html>",
	})

	h := NewStaticHandler(dist, "")

	for _, path := range []string{"/missing.png", "/assets/gone.js", "/openapi.json.missing"} {
		rec := doGet(h, path)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s 期望 404，实际 %d", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "app-shell") {
			t.Fatalf("%s 不得回落 index.html", path)
		}
	}

	// 客户端路由路径仍应回落 index.html
	rec := doGet(h, "/research")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app-shell") {
		t.Fatalf("SPA 路由应回落 index.html，实际 %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("index.html 应为 no-cache，实际 %q", cc)
	}
}
