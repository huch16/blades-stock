package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"blades-stock/backend/logger"
)

// AuthConfig 访问鉴权配置
type AuthConfig struct {
	Token string
}

var globalAuth = &AuthConfig{}

func initAuth() {
	globalAuth.Token = strings.TrimSpace(os.Getenv("BLADES_AUTH_TOKEN"))
	if globalAuth.Token == "" {
		logger.SugaredLogger.Warn("[Auth] BLADES_AUTH_TOKEN is not set. Server is running in OPEN mode (recommended only for trusted/private network).")
	} else {
		logger.SugaredLogger.Info("[Auth] Access token protection is ENABLED.")
	}
}

// AuthMiddleware 校验公网访问 Token
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 跨域预检请求直接放行
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 未配置 Token 则处于开放模式，直接通过
		if globalAuth.Token == "" {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// 静态资源与免鉴权端点放行
		if path == "/health" ||
			path == "/api/auth/login" ||
			!strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// 检查 Authorization 标头
		authHeader := r.Header.Get("Authorization")
		clientToken := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			clientToken = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}

		// 降级检查 URL Query ?token=... (针对 SSE EventSource 等无法自定义 Header 的场景)
		if clientToken == "" {
			clientToken = strings.TrimSpace(r.URL.Query().Get("token"))
		}

		if clientToken != globalAuth.Token {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"code":  401,
				"error": "未授权访问：请提供正确的访问令牌 (Authorization: Bearer <token>)",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HandleLogin 提供口令验证端点
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	w.Header().Set("Content-Type", "application/json")

	// 若服务端未配置 Token，则任何口令均放行
	if globalAuth.Token == "" || req.Token == globalAuth.Token {
		json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "登录验证成功",
			"data": map[string]any{
				"authenticated": true,
			},
		})
		return
	}

	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{
		"code":    401,
		"error":   "访问口令错误，请重新输入",
		"data": map[string]any{
			"authenticated": false,
		},
	})
}
