package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/logger"
)

// generateRandomToken 生成一个 256 位随机访问口令（32 字节，hex 编码后 64 字符）。
// 仅在既无环境变量又无持久化口令的首次启动时调用一次，调用方负责持久化并
// 在启动日志里打印一次，之后重启均从持久化文件读取，不再重新生成。
func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// AuthConfig 访问鉴权配置
type AuthConfig struct {
	mu      sync.RWMutex
	Token   string
	DataDir string
}

var globalAuth = &AuthConfig{}

// initAuth 初始化访问鉴权配置
func initAuth(dataDir string) {
	globalAuth.mu.Lock()
	defer globalAuth.mu.Unlock()

	globalAuth.DataDir = dataDir
	globalAuth.Token = ""

	// 1. 优先读取环境变量 BLADES_AUTH_TOKEN
	envToken := strings.TrimSpace(os.Getenv("BLADES_AUTH_TOKEN"))
	if envToken != "" {
		globalAuth.Token = envToken
		logger.SugaredLogger.Info("[Auth] Access token protection is ENABLED (from environment).")
		return
	}

	// 2. 次优先从本地持久化数据目录读取 auth.token
	if dataDir != "" {
		tokenFile := filepath.Join(dataDir, "auth.token")
		if b, err := os.ReadFile(tokenFile); err == nil {
			persistedToken := strings.TrimSpace(string(b))
			if persistedToken != "" {
				globalAuth.Token = persistedToken
				logger.SugaredLogger.Info("[Auth] Access token protection is ENABLED (from persistent storage).")
				return
			}
		}
	}

	// 3. 既无环境变量又无持久化口令：生成一个随机口令并固化到存储，
	//    仅在启动日志里打印一次。仓库与脚本中不得再出现任何硬编码默认口令：
	//    公开的默认口令等于没有口令。
	token, err := generateRandomToken()
	if err != nil {
		logger.SugaredLogger.Errorf("[Auth] Failed to generate random access token: %v", err)
		return
	}
	globalAuth.Token = token
	if dataDir != "" {
		tokenFile := filepath.Join(dataDir, "auth.token")
		if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
			logger.SugaredLogger.Errorf("[Auth] Failed to persist generated access token: %v", err)
		}
	}
	// 口令只在本次启动日志里打印一次：请立即妥善保存，之后请改用环境变量
	// BLADES_AUTH_TOKEN 或首次登录后修改口令；日志本身也应妥善保管。
	logger.SugaredLogger.Info("[Auth] Access token protection is ENABLED (random token generated for first boot, printed once below).")
	logger.SugaredLogger.Infof("[Auth] Generated access token (first boot only, save it now): %s", token)
}

func getAuthToken() string {
	globalAuth.mu.RLock()
	defer globalAuth.mu.RUnlock()
	return globalAuth.Token
}

func setAuthToken(token string) error {
	globalAuth.mu.Lock()
	defer globalAuth.mu.Unlock()

	token = strings.TrimSpace(token)
	globalAuth.Token = token

	if globalAuth.DataDir != "" {
		tokenFile := filepath.Join(globalAuth.DataDir, "auth.token")
		return os.WriteFile(tokenFile, []byte(token), 0o600)
	}
	return nil
}

// isAllowedOrigin 严格校验跨域请求 Origin，杜绝恶意站点凭据跨域窃取 (P1-1 修复)
func isAllowedOrigin(origin, host string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Host
	if originHost == host {
		return true
	}
	// 本地开发与内网调试白名单放行
	if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" {
		return true
	}
	// 环境变量自定义放行域名 (逗号分隔)
	if custom := os.Getenv("BLADES_ALLOWED_ORIGINS"); custom != "" {
		for _, o := range strings.Split(custom, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed == origin || trimmed == originHost {
				return true
			}
		}
	}
	// 默认投研生产域名与远程沙箱放行
	if strings.HasSuffix(u.Hostname(), "woha.men") ||
		strings.HasSuffix(u.Hostname(), "8501025.xyz") ||
		strings.HasSuffix(u.Hostname(), "instabox.tech") {
		return true
	}
	return false
}

// AuthMiddleware 校验公网访问 Token
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		reqHost := r.Host

		// 修复 P1-1: 严禁将 Access-Control-Allow-Credentials: true 与通配符 "*" 混合使用
		// 严禁将任意外部 Origin (如 https://evil.com) 无条件回显并开启凭据跨域
		if origin != "" && isAllowedOrigin(origin, reqHost) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		} else if origin == "" {
			// 同源或无 Origin 请求，保持默认
			w.Header().Set("Vary", "Origin")
		}

		// 跨域预检请求处理
		if r.Method == http.MethodOptions {
			if origin != "" && isAllowedOrigin(origin, reqHost) {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Last-Event-ID")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			// 未经允许的非法跨域预检直接拒绝
			w.WriteHeader(http.StatusForbidden)
			return
		}

		path := r.URL.Path

		// 静态资源、协议文件与免鉴权端点放行 (含标准容器探针 /live, /ready, /health, /robots.txt, /sitemap.xml, /openapi.json)
		// /api/health 是 /health 的历史别名（供旧版缓存分块使用），同样免鉴权。
		if path == "/live" ||
			path == "/ready" ||
			path == "/health" ||
			path == "/api/health" ||
			path == "/robots.txt" ||
			path == "/sitemap.xml" ||
			path == "/openapi.json" ||
			path == "/api/auth/status" ||
			path == "/api/auth/login" ||
			path == "/api/auth/init" ||
			!strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// 路由存在性预检 (P0-2 修复)：若请求的方法根本不存在，严格返回 404 而非 401！杜绝客户端混淆
		if strings.HasPrefix(path, "/api/App/") {
			methodName := strings.TrimPrefix(path, "/api/App/")
			methodName = strings.Trim(methodName, "/")
			if globalDispatcher != nil && !globalDispatcher.HasMethod(methodName) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]any{
					"code":  404,
					"error": fmt.Sprintf("RPC method '%s' not found", methodName),
				})
				return
			}
		} else if !strings.HasPrefix(path, "/api/auth/") &&
			!strings.HasPrefix(path, "/api/backup/") &&
			path != "/api/events/stream" {
			// 未知非 App RPC 路径直接返回 404
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{
				"code":  404,
				"error": "API endpoint not found",
				"path":  path,
			})
			return
		}

		serverToken := getAuthToken()

		// 若服务端未配置任何 Token，则拒绝 RPC 调用并引导进行首次密码初始化
		if serverToken == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"code":     401,
				"needInit": true,
				"error":    "系统尚未初始化访问密码，请在首次访问界面设置密码",
			})
			return
		}

		// 检查 Authorization 标头
		authHeader := r.Header.Get("Authorization")
		// 优先从 Authorization 标头中提取
		clientToken := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			clientToken = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}

		// 次优先从安全 HttpOnly Cookie 中提取 (杜绝 URL Query 暴露风险)
		if clientToken == "" {
			if cookie, err := r.Cookie("blades_token"); err == nil && cookie != nil {
				clientToken = strings.TrimSpace(cookie.Value)
			}
		}

		// 降级检查 URL Query ?token=... (针对无 Cookie 环境或离线脚本)
		if clientToken == "" {
			clientToken = strings.TrimSpace(r.URL.Query().Get("token"))
		}

		if subtle.ConstantTimeCompare([]byte(clientToken), []byte(serverToken)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"code":     401,
				"needInit": false,
				"error":    "未授权访问：请提供正确的访问令牌 (Authorization: Bearer <token>)",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HandleAuthStatus 查询当前鉴权状态与是否需要首次初始化
func HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	serverToken := getAuthToken()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"code": 0,
		"data": map[string]any{
			"needInit":     serverToken == "",
			"authRequired": true,
			"version":      "3.0.0",
		},
	})
}

// HandleInitPassword 提供设置访问密码端点
func HandleInitPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	current := getAuthToken()
	if current != "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"code":  400,
			"error": "系统访问密码已设定，请直接输入密码登录",
		})
		return
	}

	var req struct {
		Password string `json:"password"`
		Token    string `json:"token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	pwd := strings.TrimSpace(req.Password)
	if pwd == "" {
		pwd = strings.TrimSpace(req.Token)
	}

	if len(pwd) < 4 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"code":  400,
			"error": "密码长度不能少于4位，请重新输入",
		})
		return
	}

	if err := setAuthToken(pwd); err != nil {
		logger.SugaredLogger.Errorf("[Auth] Failed to persist auth.token: %v", err)
	}
	logger.SugaredLogger.Info("[Auth] Initial admin password has been set successfully.")

	// 下发安全 HttpOnly Cookie (有效期30天，彻底消除 URL 泄露 Token 的安全隐患)
	http.SetCookie(w, &http.Cookie{
		Name:     "blades_token",
		Value:    pwd,
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	json.NewEncoder(w).Encode(map[string]any{
		"code":    0,
		"message": "访问密码初始化成功！",
		"data": map[string]any{
			"authenticated": true,
		},
	})
}

type loginAttemptTracker struct {
	mu       sync.Mutex
	failures map[string]*ipAttempt
}

type ipAttempt struct {
	failCount   int
	lockedUntil time.Time
	lastAttempt time.Time
}

var loginTracker = &loginAttemptTracker{
	failures: make(map[string]*ipAttempt),
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// HandleLogin 提供口令验证端点（集成爆破防御：错误延时 + 5次锁定15分钟）
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := getClientIP(r)
	now := time.Now()

	loginTracker.mu.Lock()
	attempt, exists := loginTracker.failures[ip]
	if !exists {
		attempt = &ipAttempt{}
		loginTracker.failures[ip] = attempt
	}
	if now.Before(attempt.lockedUntil) {
		remain := int(time.Until(attempt.lockedUntil).Minutes()) + 1
		loginTracker.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"code":  429,
			"error": fmt.Sprintf("连续密码错误次数过多，IP已被安全锁定，请在 %d 分钟后再试", remain),
			"data":  map[string]any{"authenticated": false},
		})
		return
	}
	loginTracker.mu.Unlock()

	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	clientPwd := strings.TrimSpace(req.Token)
	if clientPwd == "" {
		clientPwd = strings.TrimSpace(req.Password)
	}

	serverToken := getAuthToken()
	if serverToken == "" {
		// 未配置任何口令：不设任何默认口令，直接引导走首次初始化流程。
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"code":     401,
			"needInit": true,
			"error":    "系统尚未初始化访问密码，请在首次访问界面设置密码",
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if subtle.ConstantTimeCompare([]byte(clientPwd), []byte(serverToken)) == 1 {

		// 登录成功：清除此 IP 的失败计数
		loginTracker.mu.Lock()
		delete(loginTracker.failures, ip)
		loginTracker.mu.Unlock()

		// 下发安全 HttpOnly Cookie (有效期30天，彻底消除 URL 泄露 Token 的安全隐患)
		http.SetCookie(w, &http.Cookie{
			Name:     "blades_token",
			Value:    clientPwd,
			Path:     "/",
			MaxAge:   86400 * 30,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "登录验证成功",
			"data": map[string]any{
				"authenticated": true,
			},
		})
		return
	}

	// 密码错误：注入 1 秒防御性时延，强行打断自动化爆破攻击脚本
	time.Sleep(1 * time.Second)

	loginTracker.mu.Lock()
	attempt.failCount++
	attempt.lastAttempt = time.Now()
	remainingTries := 5 - attempt.failCount
	if attempt.failCount >= 5 {
		attempt.lockedUntil = time.Now().Add(15 * time.Minute)
		logger.SugaredLogger.Warnf("[Auth] IP %s 连续 5 次输入错误口令，触发安全锁定 15 分钟", ip)
	}
	loginTracker.mu.Unlock()

	w.WriteHeader(http.StatusUnauthorized)
	errMsg := "访问密码错误，请重新输入"
	if remainingTries > 0 {
		errMsg = fmt.Sprintf("访问密码错误，请重新输入（剩余尝试次数：%d）", remainingTries)
	} else {
		errMsg = "连续密码错误达 5 次，IP 已被安全锁定 15 分钟"
	}

	json.NewEncoder(w).Encode(map[string]any{
		"code":  401,
		"error": errMsg,
		"data": map[string]any{
			"authenticated": false,
		},
	})
}
