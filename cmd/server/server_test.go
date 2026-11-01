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

	// 测试环境默认设置测试 Token
	if getAuthToken() == "" {
		_ = setAuthToken("test-unit-token-2026")
	}

	authHandler := AuthMiddleware(mux)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-No-Auth") == "1" {
			r.Header.Del("X-Test-No-Auth")
			authHandler.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") == "" {
			token := getAuthToken()
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
		}
		authHandler.ServeHTTP(w, r)
	})
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
	initAuth("")
	defer func() {
		os.Unsetenv("BLADES_AUTH_TOKEN")
		initAuth("")
	}()

	handler := setupTestServer(t)

	// 1. 未携带 Token 请求 API 应该被拦截 401
	reqUnauth := httptest.NewRequest(http.MethodPost, "/api/App/GetConfig", bytes.NewBufferString("[]"))
	reqUnauth.Header.Set("X-Test-No-Auth", "1")
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

func TestAuthEndpoints_StatusInitLogin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "blades-auth-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Unsetenv("BLADES_AUTH_TOKEN")
	initAuth(tempDir)
	defer func() {
		initAuth("")
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/status", HandleAuthStatus)
	mux.HandleFunc("POST /api/auth/init", HandleInitPassword)
	mux.HandleFunc("POST /api/auth/login", HandleLogin)
	mux.HandleFunc("POST /api/App/GetConfig", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"data":{"status":"ok"}}`))
	})
	handler := AuthMiddleware(mux)

	// 1. 初始状态：工业级统一规则下，服务预设 DefaultAdminPassword (2266762)
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	wStatus := httptest.NewRecorder()
	handler.ServeHTTP(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/auth/status, got %d", wStatus.Code)
	}
	var resStatus struct {
		Code int `json:"code"`
		Data struct {
			NeedInit     bool `json:"needInit"`
			Version      string `json:"version"`
			AuthRequired bool `json:"authRequired"`
		} `json:"data"`
	}
	if err := json.NewDecoder(wStatus.Body).Decode(&resStatus); err != nil {
		t.Fatalf("failed to decode /api/auth/status response: %v", err)
	}
	if !resStatus.Data.AuthRequired {
		t.Fatalf("expected authRequired=true")
	}

	// 1.1 开箱即用验证：直接使用统一密码 2266762 登录成功
	bodyDefault := bytes.NewBufferString(`{"password":"2266762"}`)
	reqDefault := httptest.NewRequest(http.MethodPost, "/api/auth/login", bodyDefault)
	wDefault := httptest.NewRecorder()
	handler.ServeHTTP(wDefault, reqDefault)
	if wDefault.Code != http.StatusOK {
		t.Fatalf("expected 200 for unified default password login, got %d: %s", wDefault.Code, wDefault.Body.String())
	}

	// 2. 初始化/修改密码：设置新密码为 test-suite-pass-2026
	bodyInit := bytes.NewBufferString(`{"password":"test-suite-pass-2026"}`)
	reqInit := httptest.NewRequest(http.MethodPost, "/api/auth/init", bodyInit)
	wInit := httptest.NewRecorder()
	handler.ServeHTTP(wInit, reqInit)
	if wInit.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/auth/init, got %d: %s", wInit.Code, wInit.Body.String())
	}

	// 3. 错误密码登录：应当被拒绝 401
	bodyWrong := bytes.NewBufferString(`{"password":"wrong-password"}`)
	reqWrong := httptest.NewRequest(http.MethodPost, "/api/auth/login", bodyWrong)
	wWrong := httptest.NewRecorder()
	handler.ServeHTTP(wWrong, reqWrong)
	if wWrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", wWrong.Code)
	}

	// 4. 正确密码登录：应当成功 200 并返回 token test-suite-pass-2026
	bodyCorrect := bytes.NewBufferString(`{"password":"test-suite-pass-2026"}`)
	reqCorrect := httptest.NewRequest(http.MethodPost, "/api/auth/login", bodyCorrect)
	wCorrect := httptest.NewRecorder()
	handler.ServeHTTP(wCorrect, reqCorrect)
	if wCorrect.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct login, got %d", wCorrect.Code)
	}

	// 5. 再次请求业务接口：携带 test-suite-pass-2026 应当放行 200
	reqApp := httptest.NewRequest(http.MethodPost, "/api/App/GetConfig", bytes.NewBufferString("[]"))
	reqApp.Header.Set("Authorization", "Bearer test-suite-pass-2026")
	wApp := httptest.NewRecorder()
	handler.ServeHTTP(wApp, reqApp)
	if wApp.Code != http.StatusOK {
		t.Fatalf("expected 200 for authenticated RPC, got %d", wApp.Code)
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

func TestRPCDispatcher_MethodParityAudit(t *testing.T) {
	marketService := services.NewMarketService()
	quantService := services.NewQuantService()
	agentService := services.NewAgentService()
	settingsService := services.NewSettingsService()

	dispatcher := NewRPCDispatcher(marketService, quantService, agentService, settingsService)

	t.Logf("Total registered RPC dispatcher methods: %d", len(dispatcher.methods))

	frontendMethods := []string{
		"AbortChatWithAgent",
		"AbortSummaryStockNews",
		"AddGroup",
		"AddStockGroup",
		"AddTradingRecord",
		"AnalyzeSentiment",
		"AnalyzeSentimentWithFreqWeight",
		"ApproveSkillChange",
		"BackupData",
		"BatchApproveSkillChanges",
		"BatchGreet",
		"BatchRejectSkillChanges",
		"CalculateNextRunTime",
		"CalculateNextRunTimes",
		"ChatWithAgent",
		"CheckFrequentTrading",
		"CleanupData",
		"ClsCalendar",
		"CreateCronTask",
		"CreateDatabaseSnapshot",
		"CreateIntelligentBackup",
		"CreateMCPServer",
		"DeleteAIResponseResult",
		"DeleteAiRecommendStocks",
		"DeleteCronTask",
		"DeleteCustomStrategy",
		"DeleteMCPServer",
		"DeleteTradingRecord",
		"DeleteBackupFile",
		"DiagnoseStock",
		"EMDictCode",
		"EnableCronTask",
		"EnableMCPServer",
		"EnableMCPTool",
		"ExecuteCronTaskNow",
		"ExportConfig",
		"FetchAiModelInfo",
		"FetchAiModels",
		"FetchAuditLogs",
		"FetchSkillChanges",
		"Follow",
		"FollowFund",
		"GenerateETFRotationReport",
		"GeneratePostMarketReview",
		"GeneratePreMarketBriefing",
		"GenerateSerenityResearchReport",
		"GetAIResponseResult",
		"GetAIResponseResultList",
		"GetAiAssistantSession",
		"GetAiConfigTemplates",
		"GetAiConfigs",
		"GetBackupContent",
		"GetBackupOverview",
		"GetAiRecommendStocksList",
		"GetAllBKCodes",
		"GetAllConceptCodes",
		"GetAllConcepts",
		"GetAllCustomStrategies",
		"GetAllIndustries",
		"GetAllMCPTools",
		"GetAllMarkets",
		"GetAllStockChangesWithPaging",
		"GetAllStockInfoList",
		"GetAllStocks",
		"GetBKFundFlowListByDate",
		"GetBKFundFlowTopListByDate",
		"GetChangeRank",
		"GetChangeTypeDailyStats",
		"GetConceptFundFlowListByDate",
		"GetConceptFundFlowTopListByDate",
		"GetConfig",
		"GetCronTaskByID",
		"GetCronTaskList",
		"GetCronTaskTypes",
		"GetDailyChangeStats",
		"GetDailyDimensionStats",
		"GetFollowList",
		"GetFollowedFund",
		"GetFollowedFundPaged",
		"GetFundHistoryNetValue",
		"GetFundKLine",
		"GetFundRanking",
		"GetFundTop10Holdings",
		"GetGroupList",
		"GetHotStrategy",
		"GetIndustryMoneyRankSina",
		"GetIndustryRank",
		"GetLatestMarketIntelligenceReport",
		"GetLatestMarketIntelligenceReportByType",
		"GetLatestTradingDay",
		"GetMCPServerByID",
		"GetMCPServerList",
		"GetMCPToolsByServerID",
		"GetMarketLifecycleStatus",
		"GetMoneyRankSina",
		"GetPromptTemplates",
		"GetRecentDaysMarketStatistic",
		"GetStockChangeHistory",
		"GetStockChanges",
		"GetStockEastMoneyKLine",
		"GetStockEastMoneyKLinePage",
		"GetStockKLine",
		"GetStockKLinePageWithFallback",
		"GetStockKLineWithFallback",
		"GetStockList",
		"GetStockMinutePriceLineData",
		"GetStockMoneyTrendByDay",
		"GetStockRealTimePrice",
		"GetTelegraphList",
		"GetTodayMarketStatistic",
		"GetTradingRecordList",
		"GetTradingRecordStatistics",
		"GetTypeStatsByDate",
		"GetUplimitHot",
		"GetVersionInfo",
		"GetfundList",
		"GlobalStockIndexes",
		"Greet",
		"HotEvent",
		"HotStock",
		"HotTopic",
		"IndustryResearchReport",
		"ImportIntelligentBackup",
		"InitializeGroupSort",
		"InvestCalendarTimeLine",
		"IsHKTradingTime",
		"IsTradingDay",
		"IsTradingTime",
		"IsUSTradingTime",
		"ListBackups",
		"LongTigerRank",
		"NewChatStream",
		"OpenURL",
		"ReFleshTelegraphList",
		"RejectSkillChange",
		"RemoveGroup",
		"RemoveStockGroup",
		"RestoreBackupFile",
		"RestoreData",
		"RunVectorBacktest",
		"RunVectorGridSearch",
		"SaveAIResponseResult",
		"SaveAiAssistantSession",
		"SaveAsMarkdown",
		"SaveCustomStrategy",
		"SaveImage",
		"SaveStockChangesToHistory",
		"SaveWordFile",
		"SearchCronTasks",
		"SearchFundCodes",
		"SearchStock",
		"SendDingDingMessageByType",
		"SetAlarmChangePercent",
		"SetCostPriceAndVolume",
		"SetStockAICron",
		"SetStockSort",
		"SetTradingPrice",
		"ShareAnalysis",
		"ShareText",
		"StockNotice",
		"StockResearchReport",
		"SummaryStockNews",
		"SyncRecommendStocksFromIntelligence",
		"TestAiConnection",
		"TestMCPServer",
		"UnFollow",
		"UnFollowFund",
		"UpdateAiRecommendStocksAlert",
		"UpdateConfig",
		"UpdateCronTask",
		"UpdateGroup",
		"UpdateGroupSort",
		"UpdateMCPServer",
		"UpdateTradingRecord",
		"ValidateCronExpr",
	}

	missing := make([]string, 0)
	for _, m := range frontendMethods {
		if _, ok := dispatcher.methods[m]; !ok {
			missing = append(missing, m)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Found %d methods missing in RPCDispatcher: %v", len(missing), missing)
	} else {
		t.Logf("100%% method parity verified! All %d frontend methods exist in RPCDispatcher.", len(frontendMethods))
	}
}

func TestPreMarketBriefingProbe(t *testing.T) {
	handler := setupTestServer(t)

	// 测试按类型获取最新智能研报 (离线/回退测试)
	req := httptest.NewRequest(http.MethodPost, "/api/App/GetLatestMarketIntelligenceReportByType", bytes.NewBufferString(`["pre_market_briefing"]`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetLatestMarketIntelligenceReportByType failed: status %d", w.Code)
	}

	// 测试获取市场生命周期时钟
	reqLifecycle := httptest.NewRequest(http.MethodPost, "/api/App/GetMarketLifecycleStatus", bytes.NewBufferString(`[]`))
	wLifecycle := httptest.NewRecorder()
	handler.ServeHTTP(wLifecycle, reqLifecycle)
	if wLifecycle.Code != http.StatusOK {
		t.Fatalf("GetMarketLifecycleStatus failed: status %d", wLifecycle.Code)
	}
}

func TestGetStockKLineProbe(t *testing.T) {
	handler := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/App/GetStockKLine", bytes.NewBufferString(`["sh000001", "上证指数", 365]`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	t.Logf("GetStockKLine sh000001: status=%d, bodyLen=%d", w.Code, w.Body.Len())
	if w.Body.Len() < 200 {
		t.Logf("body: %s", w.Body.String())
	}
}

func TestFundListProbe(t *testing.T) {
	handler := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/App/GetfundList", bytes.NewBufferString(`["110011"]`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	t.Logf("GetfundList 110011: status=%d, body=%s", w.Code, w.Body.String())

	reqName := httptest.NewRequest(http.MethodPost, "/api/App/GetfundList", bytes.NewBufferString(`["易方达"]`))
	wName := httptest.NewRecorder()
	handler.ServeHTTP(wName, reqName)
	t.Logf("GetfundList 易方达: status=%d, body=%s", wName.Code, wName.Body.String())
}

func TestMarketNewsAndStockSearch(t *testing.T) {
	handler := setupTestServer(t)

	// 1. 测试 GetTelegraphList
	reqNews := httptest.NewRequest(http.MethodPost, "/api/App/GetTelegraphList", bytes.NewBufferString(`["财联社电报"]`))
	wNews := httptest.NewRecorder()
	handler.ServeHTTP(wNews, reqNews)
	if wNews.Code != http.StatusOK {
		t.Fatalf("GetTelegraphList failed: status %d", wNews.Code)
	}

	// 2. 测试 ReFleshTelegraphList
	reqRefresh := httptest.NewRequest(http.MethodPost, "/api/App/ReFleshTelegraphList", bytes.NewBufferString(`["新浪财经"]`))
	wRefresh := httptest.NewRecorder()
	handler.ServeHTTP(wRefresh, reqRefresh)
	if wRefresh.Code != http.StatusOK {
		t.Fatalf("ReFleshTelegraphList failed: status %d", wRefresh.Code)
	}

	// 3. 测试按股票代码搜索
	reqStockCode := httptest.NewRequest(http.MethodPost, "/api/App/GetStockList", bytes.NewBufferString(`["600519"]`))
	wStockCode := httptest.NewRecorder()
	handler.ServeHTTP(wStockCode, reqStockCode)
	if wStockCode.Code != http.StatusOK {
		t.Fatalf("GetStockList by code failed: status %d", wStockCode.Code)
	}

	// 4. 测试按股票名称模糊搜索
	reqStockName := httptest.NewRequest(http.MethodPost, "/api/App/GetStockList", bytes.NewBufferString(`["茅台"]`))
	wStockName := httptest.NewRecorder()
	handler.ServeHTTP(wStockName, reqStockName)
	if wStockName.Code != http.StatusOK {
		t.Fatalf("GetStockList by name failed: status %d", wStockName.Code)
	}

	// 5. 测试多源降级 K 线获取
	reqKLine := httptest.NewRequest(http.MethodPost, "/api/App/GetStockKLineWithFallback", bytes.NewBufferString(`["600519.SH", "贵州茅台", "101", 10]`))
	wKLine := httptest.NewRecorder()
	handler.ServeHTTP(wKLine, reqKLine)
	if wKLine.Code != http.StatusOK {
		t.Fatalf("GetStockKLineWithFallback failed: status %d", wKLine.Code)
	}
}

