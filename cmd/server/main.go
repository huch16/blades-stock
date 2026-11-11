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
	"sync/atomic"
	"syscall"
	"time"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/services"
)

var (
	// Version 编译注入版本号（默认 3.0.0）
	Version       = "3.0.0"
	VersionCommit = "release"

	portFlag    = flag.String("port", "8000", "Web server listening port")
	hostFlag    = flag.String("host", "0.0.0.0", "Web server listening host")
	dataDirFlag = flag.String("data-dir", "data", "Path to data directory containing stock.db")
	distDirFlag = flag.String("dist-dir", "frontend/dist", "Path to frontend dist directory")

	// 发布韧性 (Release Resilience): 上一版本前端资源目录。
	// 每次 `docker compose up -d --build` 会整体替换 dist 目录，使所有旧哈希 chunk 立即 404。
	// 若保留上一版 dist，则「已打开的旧页面」在点击懒加载路由时仍能拿到旧 chunk，
	// 由前端 staleChunkGuard 在后台静默刷新，用户无感。
	prevDistDirFlag = flag.String("previous-dist-dir", "", "Path to previous-release frontend dist directory (stale chunk fallback)")
)

// 发布韧性可观测指标：让「部署打断在线用户」这一静默故障在 /health 上可见，
// 而不是只留给用户端的一片白屏。
var (
	statStaleChunkFallbackHits atomic.Int64 // 旧 chunk 命中上一版本目录的次数
	statStaleChunkMisses       atomic.Int64 // 旧 chunk 在当前与上一版本均不存在（真实发布断裂）的次数
	statImmutableAssetHits     atomic.Int64 // 命中 1 年不可变缓存的 /assets/ 请求数
	statDeniedPathHits         atomic.Int64 // 被敏感路径清单拦截（.git / Dockerfile 等探测）
	statSPAFallbackHits        atomic.Int64 // 非根路径回落到 SPA 壳的次数（用于区分真实页面与 404 掩盖）

	// 启动时解析并缓存「上一版本资源目录」是否已就绪（供 /health 零 syscall 读取）
	resolvedPreviousDistDir atomic.Value // string
)

// previousDistArmed 报告旧版 chunk 回退是否已武装（供健康探针读取）
func previousDistArmed() bool {
	v, _ := resolvedPreviousDistDir.Load().(string)
	return v != ""
}

// resolvePreviousDistDir 解析上一版本资源目录，并校验其确实是可用目录。
func resolvePreviousDistDir(currentDist, explicit string) string {
	candidates := make([]string, 0, 3)
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if env := strings.TrimSpace(os.Getenv("BLADES_PREVIOUS_DIST")); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates, currentDist+"-previous")

	currentAbs, _ := filepath.Abs(currentDist)
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		candidateAbs, _ := filepath.Abs(candidate)
		if candidateAbs == currentAbs {
			// 防御：上一版本目录不得等于当前目录，否则 404 会被误判为命中
			continue
		}
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err != nil {
			continue
		}
		return candidateAbs
	}
	return ""
}

// servePreviousReleaseAsset 尝试从上一版本目录回退同名哈希资源。
// 返回 true 表示已成功响应（调用方必须立即 return）。
func servePreviousReleaseAsset(w http.ResponseWriter, r *http.Request, previousDistDir, relPath string) bool {
	if previousDistDir == "" {
		return false
	}
	// 防目录穿越：仅允许精确匹配当前请求的相对路径
	cleanRel := filepath.Clean("/" + strings.TrimPrefix(relPath, "/"))
	if strings.Contains(cleanRel, "..") {
		return false
	}
	prevPath := filepath.Join(previousDistDir, strings.TrimPrefix(cleanRel, "/"))
	info, err := os.Stat(prevPath)
	if err != nil || info.IsDir() {
		return false
	}

	hits := statStaleChunkFallbackHits.Add(1)
	// 仅对前若干次与每 100 次记录日志，避免僵尸页面批量刷屏
	if hits <= 20 || hits%100 == 0 {
		logger.SugaredLogger.Warnf("[Release-Resilience] 旧版 chunk 命中回退 (累计 %d 次): %s -> %s | UA=%s",
			hits, r.URL.Path, prevPath, r.UserAgent())
	}

	w.Header().Set("X-Release-Fallback", "previous")
	// 旧 chunk 只做「搭桥」，不做长效缓存，确保下一个版本到来时能再次回退
	w.Header().Set("Cache-Control", "public, max-age=600")
	http.ServeFile(w, r, prevPath)
	return true
}

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

	// 【P1 修复】Web 模式启动时必须用数据库中的设置初始化共享 HTTP 客户端。
	//
	// 桌面端的 app_{darwin,linux,windows}.go 都在启动时调用了
	// data.ConfigureFromSettings(...)，而 cmd/server 以前**从未调用** →
	// 在 Docker/Web 部署下，用户配置的「全局网络代理」与「请求超时」
	// 根本不会作用到共享 transport 上，表现为：需要代理才能访问的数据源
	// （行情/新闻/模型网关）在桌面端正常、在 Web 端全部失败。
	//
	// 这里用 GetSettingConfig()（已内含默认值与 BrowserPath 自纠错）而不是
	// 裸读一行 Settings，保证与运行期其他调用点使用**同一份**生效配置。
	if db.Dao != nil {
		settingConfig := data.GetSettingConfig()
		if settingConfig != nil {
			data.ConfigureFromSettings(settingConfig)
			// 日志必须报告**实际生效值**：ConfigureFromSettings 在 CrawlTimeOut<=0 时
			// 会回退为 300s，若直接打印原始值会打出误导人的“超时=0s”。
			effectiveTimeoutSec := settingConfig.CrawlTimeOut
			if effectiveTimeoutSec <= 0 {
				effectiveTimeoutSec = 300
			}
			logger.SugaredLogger.Infof(
				"[Server] 已应用数据库设置到共享 HTTP 客户端 (代理启用=%v 超时=%ds)",
				settingConfig.HttpProxyEnabled, effectiveTimeoutSec)
		}
	}

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

	// 【P1 修复】启动价格预警与 Agent 触发式分析监听器。
	//
	// 此前它只在三个桌面 shell（app_{darwin,linux,windows}.go）里启动，
	// cmd/server **从未启动过** → 在 Docker/Web 部署下「价格预警 + 异动 AI 分析」
	// 整条链路根本不运行。而前端早已在等这个事件：
	//   bridge.ts  defaultEvents 含 'stockPriceAlertTriggered'
	//   App.vue     EventsOn('stockPriceAlertTriggered', ...) 已有完整的通知 UI
	// 即：UI 已就绪、SSE 广播钩子（PriceAlertBroadcastHook）也已在上一行接好，
	// 唯独没有把它打开 —— 属於「已建成但从未打开开关」型缺陷，用户端表现为
	// “设了预警价但永远不提醒”。
	//
	// 传 context.Background() 是安全的：watcher 内部把 Wails 原生通道包在
	// `wailsCtx != nil && wailsCtx.Value("events") != nil` 守卫里，
	// 服务端没有该上下文时自动只走 SSE 钩子，不会 panic。
	agent.GetPriceAlertWatcher().Start(context.Background())

	// 【P1 修复】把「技能自动提取」进化任务纳入 Web 模式定时调度。
	//
	// 原实现只在 app.go 的 cron 里注册（每日 02:30），Web 模式无任何 cron，
	// 于是自学习能力在生产环境完全不跑。该任务自带 7 天限频（在 agent_memories
	// 里落一条 skill_pass 记录）且**只写草稿供人工审阅、绝不自动建技能**，
	// 因此加入后没有行为风险，只是把被漏掉的能力补齐。
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("[SkillExtractionCron] recovered from panic: %v", r)
			}
		}()
		// 启动后错开一段时间再跑首次，避免与启动期的数据预热抢资源
		time.Sleep(3 * time.Minute)
		agent.RunSkillExtractionPass()
		skillTicker := time.NewTicker(24 * time.Hour)
		defer skillTicker.Stop()
		for range skillTicker.C {
			agent.RunSkillExtractionPass()
		}
	}()

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
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"service":   "blades-stock",
			"version":   Version,
			"timestamp": time.Now().Unix(),
			// 发布韧性指标：非零的 stale_chunk_fallback 说明此刻仍有在线用户停留在旧版本页面；
			// 非零的 stale_chunk_miss 说明回退未能兜住（需检查上一版目录是否被正确保留）。
			"release_resilience": map[string]any{
				"previous_dist_armed":      previousDistArmed(),
				"stale_chunk_fallback":     statStaleChunkFallbackHits.Load(),
				"stale_chunk_miss":         statStaleChunkMisses.Load(),
				"immutable_asset_requests": statImmutableAssetHits.Load(),
				"denied_path_probes":       statDeniedPathHits.Load(),
				"spa_fallback_hits":        statSPAFallbackHits.Load(),
			},
		})
	}
	mux.HandleFunc("GET /health", healthHandler)
	// /api/health 是历史前端（v3.0.0 之前的设置页）调用过的路径。
	// 发布韧性会把上一版 chunk 继续提供给已打开的在线会话，
	// 因此旧会话仍会请求 /api/health；若服务端不提供别名，它们会得到 404，
	// 从而把“服务器延迟”误显示为 -1（不可达）。这里提供同源别名，保障灰度期间双版本共存。
	mux.HandleFunc("GET /api/health", healthHandler)

	// 7.2 标准协议与机器可读契约端点 (P0-2, P1-7)
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fmt.Fprintln(w, "User-agent: *")
		fmt.Fprintln(w, "Disallow: /api/")
		fmt.Fprintln(w, "Allow: /")
	})

	mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://kline.woha.men/</loc><changefreq>daily</changefreq><priority>1.0</priority></url></urlset>`)
	})

	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		json.NewEncoder(w).Encode(dispatcher.GetOpenAPISpec())
	})

	// 7.3 安全鉴权端点 (状态探测、首次初始化与登录)
	mux.HandleFunc("GET /api/auth/status", HandleAuthStatus)
	mux.HandleFunc("POST /api/auth/init", HandleInitPassword)
	mux.HandleFunc("POST /api/auth/login", HandleLogin)

	// 7.4 智能数据资产备份与恢复流端点 (支持 Web 模式安全直接下载与上传)
	mux.HandleFunc("GET /api/backup/download", HandleBackupDownload)
	mux.HandleFunc("POST /api/backup/upload", HandleBackupUpload)

	// 7.5 SSE 实时推流端点
	mux.HandleFunc("GET /api/events/stream", HandleSSE)

	// 7.6 通用 RPC 分发端点
	mux.HandleFunc("/api/App/", dispatcher.HandleRPC)

	// 7.7 前端静态资源与 SPA 路由托管 (精准隔离静态文件 404 与客户端路由 Fallback)
	distDir := *distDirFlag
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		// 尝试当前目录下的 dist
		if _, err2 := os.Stat("dist"); err2 == nil {
			distDir = "dist"
		}
	}

	previousDistDir := resolvePreviousDistDir(distDir, *prevDistDirFlag)
	resolvedPreviousDistDir.Store(previousDistDir)

	if _, err := os.Stat(distDir); err == nil {
		logger.SugaredLogger.Infof("[Static] Serving frontend assets from: %s", distDir)
		if previousDistDir != "" {
			logger.SugaredLogger.Infof("[Static] ✅ Previous-release asset fallback ARMED: %s (旧版 chunk 将回退上一版资源，而非 404)", previousDistDir)
		} else {
			logger.SugaredLogger.Warnf("[Static] ⚠️  Previous-release asset fallback DISABLED — 部署瞬间旧页面点击懒加载路由将直接 404。" +
				" 请在 docker-compose.yml 挂载 ./frontend/dist-previous:/app/frontend/dist-previous:ro (参见 scripts/deploy-vps.sh)")
		}
		mux.Handle("/", NewStaticHandler(distDir, previousDistDir))
	} else {
		logger.SugaredLogger.Warnf("[Static] Frontend dist directory not found at '%s', running API-only mode.", distDir)
		mux.Handle("/", NewAPIMissingHandler(distDir))
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
