package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
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
	// Version 编译注入版本号（默认 2.0.0）
	Version       = "2.0.0"
	VersionCommit = "release"

	portFlag    = flag.String("port", "8000", "Web server listening port")
	hostFlag    = flag.String("host", "0.0.0.0", "Web server listening host")
	dataDirFlag = flag.String("data-dir", "data", "Path to data directory containing stock.db")
	distDirFlag = flag.String("dist-dir", "frontend/dist", "Path to frontend dist directory")
)

// SecurityHeadersMiddleware 为所有 HTTP 响应注入标准安全响应头 (HSTS、点击劫持、MIME 混淆、Referrer 泄露及 XSS 防御)
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		// 生产级 CSP 加固：彻底移除 'unsafe-eval'，杜绝 eval/Function 代码注入与 XSS 风险
		w.Header().Set("Content-Security-Policy", "default-src 'self' 'unsafe-inline' data: blob: https:; img-src 'self' data: blob: https:; font-src 'self' data: https:; frame-ancestors 'none';")
		next.ServeHTTP(w, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// GzipMiddleware 为符合条件的 API 请求与静态资源启用 Gzip 传输压缩 (针对 1.15MB GetStockList 等巨型载荷降低 80%+ 传输带宽)
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SSE 实时推流跳过 Gzip 压缩，确保流式推流即时 flush
		if strings.HasPrefix(r.URL.Path, "/api/events/stream") || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzip.NewWriter(w)
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gzw := gzipResponseWriter{ResponseWriter: w, Writer: gz}
		next.ServeHTTP(gzw, r)
	})
}

// RequestTracingMiddleware 注入并透传全链路分布式追踪 X-Request-ID
func RequestTracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("req-%d-%06d", time.Now().UnixMilli(), time.Now().Nanosecond()%1000000)
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), "request_id", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func main() {
	flag.Parse()

	// 1. 初始化日志
	logger.InitLogger()
	logger.SugaredLogger.Info("==================================================================")
	logger.SugaredLogger.Infof("   Blades Stock — Cloud-Native Web/SaaS Server Starting (v%s)...", Version)
	logger.SugaredLogger.Infof("   [Runtime Profile] Host Cores: %d | GOMAXPROCS: %d | Commit: %s", runtime.NumCPU(), runtime.GOMAXPROCS(0), VersionCommit)
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

	// 开启 SQLite 工业级并发与内存缓存优化 (适应 2C/12G, 4C/8G 等不同 VPS)
	if db.Dao != nil {
		db.Dao.Exec("PRAGMA journal_mode = WAL;")
		db.Dao.Exec("PRAGMA synchronous = NORMAL;")
		db.Dao.Exec("PRAGMA busy_timeout = 10000;")
		db.Dao.Exec("PRAGMA cache_size = -64000;") // 64MB 内存缓存加速热点数据查询
		db.Dao.Exec("PRAGMA temp_store = MEMORY;")
	}

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
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Warnf("[NewsCron:Preload] recovered from panic: %v", r)
				}
			}()
			data.NewMarketNewsApi().TelegraphList(30)
			data.NewMarketNewsApi().GetSinaNews(30)
			data.NewMarketNewsApi().TradingViewNews()
		}()

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
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Warnf("[MarketStatsCron:Preload] recovered from panic: %v", r)
				}
			}()
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

	// 5.2 启动静默后台定时灾备守护协程 (灾备中心降为后端静默定时任务：每日自动执行无锁快照与自动生命周期修剪)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("[SilentBackupCron] recovered from panic: %v", r)
			}
		}()

		// 启动后延迟 60 秒执行首次基准检查与生命周期修剪
		time.Sleep(60 * time.Second)
		bm := data.NewBackupManager()
		bm.EnsureBaselineBackupIfEmpty()
		_, _ = bm.PruneOldBackups(7, 5, 14)

		// 每 24 小时静默自动执行一次数据库原子快照与修剪
		backupTicker := time.NewTicker(24 * time.Hour)
		defer backupTicker.Stop()
		for range backupTicker.C {
			logger.SugaredLogger.Info("[SilentBackupCron] 执行每日后台静默数据库快照与数据资产保护...")
			if _, err := bm.CreateDatabaseSnapshot(); err != nil {
				logger.SugaredLogger.Warnf("[SilentBackupCron] 创建静默快照告警: %v", err)
			}
			_, _ = bm.PruneOldBackups(7, 5, 14)
		}
	}()

	// 6. 初始化安全鉴权
	initAuth(dataDir)

	// 7. 初始化核心业务微服务
	marketService := services.NewMarketService()
	quantService := services.NewQuantService()
	agentService := services.NewAgentService()
	settingsService := services.NewSettingsService()

	// 挂载云端 SSE 广播作为 Agent 默认流式消息发射器与智能研报/价格告警广播钩子
	agentService.SetDefaultEmitter(globalSSEHub.Broadcast)
	agent.MarketReportBroadcastHook = globalSSEHub.Broadcast
	agent.PriceAlertBroadcastHook = globalSSEHub.Broadcast

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

	// 7.1 标准容器与云原生探针 (Kubernetes Liveness & Readiness Probes)
	// 存活探针 (Liveness): 仅返回服务进程存活状态，杜绝泄露版本或内核配置
	mux.HandleFunc("GET /live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})

	// 就绪探针 (Readiness): 深度探测底层持久化 SQLite 数据库活性，失败时返回 503
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if db.Dao == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]any{"status": "not ready", "error": "db not initialized"})
			return
		}
		var check int
		if err := db.Dao.Raw("SELECT 1").Scan(&check).Error; err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]any{"status": "not ready", "error": "db ping failed"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
	})

	// 综合健康检查端点 (收敛脱敏：保留自动化测试所需的 version 字段，移除 commit/cores 等主机敏感指纹)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"service":   "blades-stock",
			"version":   Version,
			"timestamp": time.Now().Unix(),
		})
	})

	// 7.2 安全鉴权端点 (状态探测、首次初始化与登录)
	mux.HandleFunc("GET /api/auth/status", HandleAuthStatus)
	mux.HandleFunc("POST /api/auth/init", HandleInitPassword)
	mux.HandleFunc("POST /api/auth/login", HandleLogin)

	// 7.3 智能数据资产备份与恢复流端点 (支持 Web 模式安全直接下载与上传)
	mux.HandleFunc("GET /api/backup/download", HandleBackupDownload)
	mux.HandleFunc("POST /api/backup/upload", HandleBackupUpload)

	// 7.4 SSE 实时推流端点
	mux.HandleFunc("GET /api/events/stream", HandleSSE)

	// 7.5 通用 RPC 分发端点
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

	// 8. 挂载中间件并启动服务器 (链路追踪 + 传输压缩 + 安全响应头 + 鉴权拦截)
	handler := RequestTracingMiddleware(GzipMiddleware(SecurityHeadersMiddleware(AuthMiddleware(mux))))

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout 设置为 0，防止 SSE 长连接与慢速大模型推演被 net/http 强制切断
		WriteTimeout: 0,
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

	// 优雅关闭 SSE 心跳与客户端连接
	globalSSEHub.Shutdown()

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
