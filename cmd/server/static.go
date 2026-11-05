package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"blades-stock/backend/logger"
)

// ── 敏感 / 基础设施路径拒绝清单 ─────────────────────────────────────────────
// 本应用使用 hash 路由（createWebHashHistory），浏览器只会在服务端请求真实文件与 `/`，
// 因此服务端看到的路径绝无可能是「合法的 SPA 深链」。但 SPA 兜底会把任何未命中的路径
// 都返回 200 + index.html，导致：
//
//	① 任何安全扫描器都会得到一份高危误报清单（`.git 目录可访问` / `Dockerfile 暴露` …）；
//	② 非 API 路径的真实 404 永远被掩盖，404 监控彻底失效。
//
// 因此在这里显式拒绝「隐藏路径段」与常见基础设施文件，避免被 SPA 壳伪装成 200。
var deniedPathPrefixes = []string{
	"/.git", "/.svn", "/.hg", "/.bzr", "/.env", "/.aws", "/.ssh",
	"/.npmrc", "/.netrc", "/.htpasswd", "/.idea", "/.vscode",
	"/node_modules", "/frontend/", "/backend/", "/cmd/", "/scripts/",
	"/data/", "/logs/", "/dist/", "/vendor/",
}

var deniedBaseNames = map[string]struct{}{
	"dockerfile": {}, "docker-compose.yml": {}, "docker-compose.yaml": {},
	"makefile": {}, "gnumakefile": {}, "procfile": {}, "jenkinsfile": {},
	"package.json": {}, "package-lock.json": {}, "yarn.lock": {}, "pnpm-lock.yaml": {},
	"go.mod": {}, "go.sum": {}, "tsconfig.json": {}, "vite.config.js": {}, "vite.config.ts": {},
	"eslint.config.js": {}, "web.config": {}, "nginx.conf": {}, ".ds_store": {},
	"id_rsa": {}, "id_ed25519": {}, "credentials": {}, "secrets.yml": {}, "secrets.yaml": {},
}

// shouldDenyStaticPath 报告该路径是否属于「绝不能由 SPA 兜底伪装成 200」的敏感路径。
// 注意：`.well-known` 按 RFC 8615 属于必须可服务的标准路径，予以放行。
func shouldDenyStaticPath(reqPath string) bool {
	p := strings.ToLower(strings.TrimSuffix(reqPath, "/"))
	if p == "" || p == "/" {
		return false
	}

	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == ".well-known" {
			continue
		}
		// 任何以点开头的路径段（.git / .env / .DS_Store / .ssh …）都绝不属于 SPA 路由
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}

	for _, prefix := range deniedPathPrefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}

	if _, ok := deniedBaseNames[filepath.Base(p)]; ok {
		return true
	}
	return false
}

// NewStaticHandler 构造前端静态资源与 SPA 客户端路由托管处理器。
//
// 行为契约：
//  1. 未匹配的 /api/* 一律返回 JSON 404，杜绝把 API 错误伪装成 index.html（P0-2 修复）；
//  2. 任何带扩展名或位于 /assets/ 下的文件若不存在，返回 404，不回落 index.html（P0-2 修复）；
//     唯一例外是 /assets/ 下的**旧版本内容哈希 chunk** —— 会回退到 previousDistDir，
//     以保证「部署瞬间仍停留在旧页面的在线用户」不会白屏（发布韧性 / Release Resilience）；
//  3. /assets/ 下真实存在的文件注入 1 年不可变长效缓存（内容哈希文件名，天然安全）；
//  4. 其余路径视为 SPA 客户端路由，回落 index.html 并禁用 HTML 缓存。
//
// previousDistDir 为空字符串时，第 2 条例外退化为一律 404。
func NewStaticHandler(distDir, previousDistDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(distDir))
	indexPath := filepath.Join(distDir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. API 路由未匹配到时严格返回 404 JSON，杜绝返回 HTML
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":  404,
				"error": "接口不存在",
				"path":  r.URL.Path,
			})
			return
		}

		reqPath := r.URL.Path
		relPath := strings.TrimPrefix(reqPath, "/")
		targetPath := filepath.Join(distDir, relPath)
		isAsset := strings.HasPrefix(reqPath, "/assets/")

		// 1.5 敏感 / 基础设施路径一律 404，绝不回落到 index.html。
		//     目的：消除安全扫描器误报，并让这类探测产生真实 404 可被监控。
		if shouldDenyStaticPath(reqPath) {
			statDeniedPathHits.Add(1)
			http.NotFound(w, r)
			return
		}

		// 2. 静态资源 / 协议文件 / 带扩展名文件缺失时严格 404（旧版 chunk 例外回退）
		if isAsset || strings.Contains(filepath.Base(reqPath), ".") {
			if fi, err := os.Stat(targetPath); err != nil || fi.IsDir() {
				if isAsset && servePreviousReleaseAsset(w, r, previousDistDir, relPath) {
					return
				}
				misses := statStaleChunkMisses.Add(1)
				if misses <= 20 || misses%100 == 0 {
					logger.SugaredLogger.Errorf("[Release-Resilience] 静态资源真实缺失 (累计 %d 次): %s | previousDir=%q — 旧页面将白屏，请检查部署是否保留了上一版 dist",
						misses, reqPath, previousDistDir)
				}
				http.NotFound(w, r)
				return
			}
		}

		// 3. 真实存在的静态文件：/assets/ 注入 1 年不可变缓存
		if fi, err := os.Stat(targetPath); err == nil && !fi.IsDir() {
			if isAsset {
				statImmutableAssetHits.Add(1)
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// 4. 纯 SPA 客户端路由导航，回退 index.html。
		//    显式标记 X-SPA-Fallback，让运维/监控能区分「真实文件」与「SPA 壳兜底」，
		//    否则非 API 路径永远 200，真实 404 无法被观测。
		w.Header().Set("Cache-Control", "no-cache")
		if reqPath != "/" {
			statSPAFallbackHits.Add(1)
			w.Header().Set("X-SPA-Fallback", "1")
		}
		http.ServeFile(w, r, indexPath)
	})
}

// NewAPIMissingHandler 在未构建前端产物时使用的降级处理器（API-only 模式）
func NewAPIMissingHandler(distDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 404, "error": "接口不存在"})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>Blades Stock Web API Server is running.</h2><p>Frontend dist not found at %s. Please build frontend first.</p>", distDir)
	})
}
