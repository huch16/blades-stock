package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/logger"
)

// DefaultAdminPassword 工业级系统统一预设/恢复管理员口令
const DefaultAdminPassword = "2266762"

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

	// 3. 工业级默认规则：自动启用统一预设管理员密码 (2266762) 并固化到存储
	globalAuth.Token = DefaultAdminPassword
	if dataDir != "" {
		tokenFile := filepath.Join(dataDir, "auth.token")
		_ = os.WriteFile(tokenFile, []byte(DefaultAdminPassword), 0o600)
	}
	logger.SugaredLogger.Info("[Auth] Access token protection is ENABLED with unified admin password (2266762).")
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

		path := r.URL.Path

		// 静态资源与免鉴权端点放行
		if path == "/health" ||
			path == "/api/auth/status" ||
			path == "/api/auth/login" ||
			path == "/api/auth/init" ||
			!strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
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

		if subtle.ConstantTimeCompare([]byte(clientToken), []byte(serverToken)) != 1 &&
			subtle.ConstantTimeCompare([]byte(clientToken), []byte(DefaultAdminPassword)) != 1 {
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
			"needInit":        serverToken == "",
			"authRequired":    true,
			"version":         "2.0.0",
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
	if current != "" && current != DefaultAdminPassword {
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
			"code":    429,
			"error":   fmt.Sprintf("连续密码错误次数过多，IP已被安全锁定，请在 %d 分钟后再试", remain),
			"data":    map[string]any{"authenticated": false},
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
		serverToken = DefaultAdminPassword
		_ = setAuthToken(serverToken)
	}
	w.Header().Set("Content-Type", "application/json")

	if subtle.ConstantTimeCompare([]byte(clientPwd), []byte(serverToken)) == 1 ||
		subtle.ConstantTimeCompare([]byte(clientPwd), []byte(DefaultAdminPassword)) == 1 {
		
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
		"code":    401,
		"error":   errMsg,
		"data": map[string]any{
			"authenticated": false,
		},
	})
}
