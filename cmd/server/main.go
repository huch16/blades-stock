package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/services"
)

var (
	portFlag    = flag.String("port", "8000", "Web server listening port")
	hostFlag    = flag.String("host", "0.0.0.0", "Web server listening host")
	dataDirFlag = flag.String("data-dir", "data", "Path to data directory containing stock.db")
	distDirFlag = flag.String("dist-dir", "frontend/dist", "Path to frontend dist directory")
)

func main() {
	flag.Parse()

	// 1. 初始化日志
	logger.InitLogger()
	logger.SugaredLogger.Info("==================================================================")
	logger.SugaredLogger.Info("   Blades Stock — Cloud-Native Web/SaaS Server Starting...        ")
	logger.SugaredLogger.Info("==================================================================")

	// 2. 确定数据目录并初始化 SQLite
	dataDir := *dataDirFlag
	if envData := os.Getenv("BLADES_DATA_DIR"); envData != "" {
		dataDir = envData
	}
	_ = os.MkdirAll(dataDir, 0o755)
	dbPath := filepath.Join(dataDir, "stock.db")
	logger.SugaredLogger.Infof("[Database] Initializing SQLite at: %s", dbPath)
	db.Init(dbPath)

	// 执行全量数据表自动迁移与默认种子数据初始化 (确保全新安装具备完整表结构)
	services.AutoMigrateAll()

	// 3. 初始化分词词典与舆情情感分析器 (防止新闻切词 nil 指针异常)
	data.InitAnalyzeSentiment()

	// 4. 启动市场资讯后台定时采集与实时 SSE 广播
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("[NewsCron] recovered from panic: %v", r)
			}
		}()
		// 启动时立即预加载最新市场资讯入库
		go data.NewMarketNewsApi().TelegraphList(30)
		go data.NewMarketNewsApi().GetSinaNews(30)
		go data.NewMarketNewsApi().TradingViewNews()

		// 定时轮询并向 Web 客户端广播最新快讯 (财联社电报、新浪财经、外媒)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if news := data.NewMarketNewsApi().TelegraphList(20); news != nil && len(*news) > 0 {
				globalSSEHub.Broadcast("newTelegraph", news)
			}
			if sina := data.NewMarketNewsApi().GetSinaNews(20); sina != nil && len(*sina) > 0 {
				globalSSEHub.Broadcast("newSinaNews", sina)
			}
			if tv := data.NewMarketNewsApi().TradingViewNews(); tv != nil && len(*tv) > 0 {
				globalSSEHub.Broadcast("tradingViewNews", tv)
			}
		}
	}()

	// 5. 启动盘面多空统计、板块资金流向、概念热度后台定时采集与自愈
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("[MarketStatsCron] recovered from panic: %v", r)
			}
		}()
		// 启动时立即主动抓取一次，确保全新部署开箱即有盘面图表数据
		go func() {
			_ = data.NewMarketStatisticApi().FetchAndSave()
			_, _ = data.NewBKFundFlowApi().FetchAndSave()
			_, _ = data.NewConceptFundFlowApi().FetchAndSave()
			if changes := data.NewStockChangesApi().GetStockAllChanges(0, 100); changes != nil && len(changes.Data) > 0 {
				_, _ = data.NewStockChangeHistoryService().SaveStockChangesWithDedup(changes.Data)
			}
		}()

		// 每 3 分钟定时轮询更新盘面统计与资金流向
		statTicker := time.NewTicker(3 * time.Minute)
		defer statTicker.Stop()
		for range statTicker.C {
			_ = data.NewMarketStatisticApi().FetchAndSave()
			_, _ = data.NewBKFundFlowApi().FetchAndSave()
			_, _ = data.NewConceptFundFlowApi().FetchAndSave()
		}
	}()

	// 6. 初始化安全鉴权
	initAuth(dataDir)

	// 7. 初始化核心业务微服务
	marketService := services.NewMarketService()
	quantService := services.NewQuantService()
	agentService := services.NewAgentService()
	settingsService := services.NewSettingsService()

	// 挂载云端 SSE 广播作为 Agent 默认流式消息发射器与智能研报广播钩子
	agentService.SetDefaultEmitter(globalSSEHub.Broadcast)
	agent.MarketReportBroadcastHook = globalSSEHub.Broadcast

	// 5. 初始化通用 RPC 反射分发器
	dispatcher := NewRPCDispatcher(marketService, quantService, agentService, settingsService)

	// 6. 确定监听地址
	port := *portFlag
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	host := *hostFlag
	if envHost := os.Getenv("HOST"); envHost != "" {
		host = envHost
	}
	addr := fmt.Sprintf("%s:%s", host, port)

	// 7. 注册路由 (http.ServeMux)
	mux := http.NewServeMux()

	// 7.1 健康检查端点
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.4.0",
		})
	})

	// 7.2 安全鉴权端点 (状态探测、首次初始化与登录)
	mux.HandleFunc("GET /api/auth/status", HandleAuthStatus)
	mux.HandleFunc("POST /api/auth/init", HandleInitPassword)
	mux.HandleFunc("POST /api/auth/login", HandleLogin)

	// 7.3 SSE 实时推流端点
	mux.HandleFunc("GET /api/events/stream", HandleSSE)

	// 7.4 通用 RPC 分发端点
	mux.HandleFunc("/api/App/", dispatcher.HandleRPC)

	// 7.5 前端静态资源与 SPA Fallback 托管
	distDir := *distDirFlag
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		// 尝试当前目录下的 dist
		if _, err2 := os.Stat("dist"); err2 == nil {
			distDir = "dist"
		}
	}

	if _, err := os.Stat(distDir); err == nil {
		logger.SugaredLogger.Infof("[Static] Serving frontend assets from: %s", distDir)
		fileServer := http.FileServer(http.Dir(distDir))
		indexPath := filepath.Join(distDir, "index.html")

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// API 路由未匹配到时的 404
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}

			// 检查请求的文件是否存在
			relPath := strings.TrimPrefix(r.URL.Path, "/")
			targetPath := filepath.Join(distDir, relPath)
			fi, err := os.Stat(targetPath)
			if err == nil && !fi.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}

			// SPA 客户端路由 Fallback 到 index.html
			http.ServeFile(w, r, indexPath)
		})
	} else {
		logger.SugaredLogger.Warnf("[Static] Frontend dist directory not found at '%s', running API-only mode.", distDir)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, "<h2>Blades Stock Web API Server is running.</h2><p>Frontend dist not found at %s. Please build frontend first.</p>", distDir)
		})
	}

	// 8. 挂载中间件并启动服务器
	handler := AuthMiddleware(mux)

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 优雅停机信号捕获
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.SugaredLogger.Infof(">>> Server listening at http://%s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.SugaredLogger.Fatalf("HTTP server ListenAndServe failed: %v", err)
		}
	}()

	// 阻塞等待退出信号
	sig := <-stopChan
	logger.SugaredLogger.Infof("[Shutdown] Received signal %v, shutting down gracefully...", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.SugaredLogger.Errorf("[Shutdown] HTTP server shutdown error: %v", err)
	}

	// 执行 SQLite WAL 检查点确保退出前数据完全落盘
	if db.Dao != nil {
		if err := db.Dao.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
			logger.SugaredLogger.Warnf("[Shutdown] WAL checkpoint error: %v", err)
		} else {
			logger.SugaredLogger.Info("[Shutdown] WAL checkpoint complete.")
		}
	}

	logger.SugaredLogger.Info("[Shutdown] Blades Stock Web Server exited cleanly.")
}
