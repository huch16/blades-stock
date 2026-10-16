package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"blades-stock/backend/db"
	"blades-stock/backend/services"
)

func setupTestServer(t *testing.T) http.Handler {
	// 初始化测试数据库
	dbPath := filepath.Join("../../data", "stock.db")
	db.Init(dbPath)

	marketService := services.NewMarketService()
	quantService := services.NewQuantService()
	agentService := services.NewAgentService()
	settingsService := services.NewSettingsService()

	dispatcher := NewRPCDispatcher(marketService, quantService, agentService, settingsService)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "healthy"})
	})
	mux.HandleFunc("POST /api/auth/login", HandleLogin)
	mux.HandleFunc("GET /api/events/stream", HandleSSE)
	mux.HandleFunc("/api/App/", dispatcher.HandleRPC)

	return AuthMiddleware(mux)
}

func TestHealthEndpoint(t *testing.T) {
	handler := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if res["status"] != "healthy" {
		t.Fatalf("expected status healthy, got %v", res["status"])
	}
}

func TestRPCDispatcher_GetConfig(t *testing.T) {
	handler := setupTestServer(t)

	// POST /api/App/GetConfig，空参数数组 []
	body := bytes.NewBufferString("[]")
	req := httptest.NewRequest(http.MethodPost, "/api/App/GetConfig", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if res["code"] != float64(0) {
		t.Fatalf("expected code 0, got %v", res["code"])
	}
	if res["data"] == nil {
		t.Fatal("expected non-nil data for GetConfig")
	}
}

func TestRPCDispatcher_GetStockList(t *testing.T) {
	handler := setupTestServer(t)

	// POST /api/App/GetStockList，带参数 ["600519"]
	body := bytes.NewBufferString(`["600519"]`)
	req := httptest.NewRequest(http.MethodPost, "/api/App/GetStockList", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var res struct {
		Code int   `json:"code"`
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if res.Code != 0 {
		t.Fatalf("expected code 0, got %d", res.Code)
	}
	t.Logf("found %d stocks for keyword 600519", len(res.Data))
}

func TestAuthProtection(t *testing.T) {
	// 设置测试 Token
	os.Setenv("BLADES_AUTH_TOKEN", "test-secret-key-1234")
	initAuth()
	defer func() {
		os.Unsetenv("BLADES_AUTH_TOKEN")
		initAuth()
	}()

	handler := setupTestServer(t)

	// 1. 未携带 Token 请求 API 应该被拦截 401
	reqUnauth := httptest.NewRequest(http.MethodPost, "/api/App/GetConfig", bytes.NewBufferString("[]"))
	wUnauth := httptest.NewRecorder()
	handler.ServeHTTP(wUnauth, reqUnauth)

	if wUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for unauthenticated request, got %d", wUnauth.Code)
	}

	// 2. 携带正确 Token 应该顺利放行 200
	reqAuth := httptest.NewRequest(http.MethodPost, "/api/App/GetConfig", bytes.NewBufferString("[]"))
	reqAuth.Header.Set("Authorization", "Bearer test-secret-key-1234")
	wAuth := httptest.NewRecorder()
	handler.ServeHTTP(wAuth, reqAuth)

	if wAuth.Code != http.StatusOK {
		t.Fatalf("expected status 200 for authenticated request, got %d", wAuth.Code)
	}
}

func TestRPCDispatcher_MarketEndpoints(t *testing.T) {
	handler := setupTestServer(t)

	// 1. 测试 IsTradingTime
	reqTime := httptest.NewRequest(http.MethodPost, "/api/App/IsTradingTime", bytes.NewBufferString("[]"))
	wTime := httptest.NewRecorder()
	handler.ServeHTTP(wTime, reqTime)
	if wTime.Code != http.StatusOK {
		t.Fatalf("IsTradingTime failed: %d", wTime.Code)
	}

	// 2. 测试 GetStockChanges 异动大盘全景接口
	bodyChanges := bytes.NewBufferString("[[1, 2], 0, 10]")
	reqChanges := httptest.NewRequest(http.MethodPost, "/api/App/GetStockChanges", bodyChanges)
	wChanges := httptest.NewRecorder()
	handler.ServeHTTP(wChanges, reqChanges)
	if wChanges.Code != http.StatusOK {
		t.Fatalf("GetStockChanges failed: %d", wChanges.Code)
	}

	// 3. 测试 HotStock 雪球热门榜
	bodyHot := bytes.NewBufferString("[\"ALL\"]")
	reqHot := httptest.NewRequest(http.MethodPost, "/api/App/HotStock", bodyHot)
	wHot := httptest.NewRecorder()
	handler.ServeHTTP(wHot, reqHot)
	if wHot.Code != http.StatusOK {
		t.Fatalf("HotStock failed: %d", wHot.Code)
	}
}

func TestAgentService_SSEEmitterIntegration(t *testing.T) {
	as := services.NewAgentService()
	receivedEvents := make([]string, 0)

	// 挂载模拟发射器
	as.SetDefaultEmitter(func(event string, payload any) {
		receivedEvents = append(receivedEvents, event)
	})

	// 验证 Abort 幂等性
	as.AbortChatWithAgent()

	// 广播测试
	globalSSEHub.Broadcast("test-event", map[string]string{"foo": "bar"})
}
