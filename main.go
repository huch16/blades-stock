package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	log "blades-stock/backend/logger"
	"blades-stock/backend/machineid"
	"blades-stock/backend/models"
	"blades-stock/backend/paths"
	"blades-stock/backend/quant"
	"blades-stock/backend/services"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// crashOutputFile 持有崩溃转储文件句柄，避免被 GC 回收后关闭（运行时不会重新打开它）。
//
// 一旦没有引用，GC 会回收 *os.File 并关闭 fd，致命错误堆栈就写不进 logs/crash.log 了。
// 不能删，也不能改成局部变量。
//
//lint:ignore U1000 故意只赋值不读取 —— 这个变量的存在意义就是**保持文件句柄存活**；
var crashOutputFile *os.File

// setupCrashOutput 把 Go 运行时致命错误（任意 goroutine 的 panic、并发 map 读写、栈溢出等）
// 的完整堆栈转储写入 logs/crash.log，便于定位进程「静默消失」根因。
// 这类错误默认只写 stderr，Wails GUI 程序无控制台时表现为「闪退」。
func setupCrashOutput() {
	dir := paths.LogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "crash.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	crashOutputFile = f
	_ = debug.SetCrashOutput(f, debug.CrashOptions{})
}

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed build/app.ico
var icon2 []byte

//go:embed build/screenshot/alipay.jpg
var alipay []byte

//go:embed build/screenshot/wxpay.jpg
var wxpay []byte

//go:embed docs/blades-stock使用手册.md
var userManual []byte

//go:generate cp -R ./data ./build/bin

var Version string = "3.2.0"
var VersionCommit string = "release"
var OFFICIAL_STATEMENT string
var BuildKey string

func main() {
	// 尽早把 Go 运行时致命错误（任意 goroutine 的 panic、并发 map 读写、栈溢出等）
	// 的完整堆栈转储到 logs/crash.log，便于定位进程「静默消失」的根因。
	setupCrashOutput()

	defer func() {
		if r := recover(); r != nil {
			log.SugaredLogger.Error("panic: ", r)
			log.SugaredLogger.Error("stack: ", string(debug.Stack()))
		}
	}()

	// 锚定基准目录：所有相对路径（data/、logs/、memory/、stock_basic.json 等）以它为准，
	// 使开发模式与本地发布 exe 共用同一份数据，部署包则自包含。
	if err := os.Chdir(paths.BaseDir()); err != nil {
		log.SugaredLogger.Warnf("chdir to base dir failed: %v", err)
	}

	// GUI 程序无 stderr，未被 recover 的 panic 会静默退出。重定向 stderr 到
	// logs/panic.log，使任何未捕获崩溃（含堆栈）可被追溯。
	if err := os.MkdirAll(paths.LogDir(), 0755); err == nil {
		if f, err := os.OpenFile(filepath.Join(paths.LogDir(), "panic.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
			os.Stderr = f
			log.SugaredLogger.Infof("stderr redirected to %s", filepath.Join(paths.LogDir(), "panic.log"))
		}
	}

	// 单实例保护：避免双开抢同一份 SQLite/WAL 导致进程静默闪退。
	if !acquireSingleInstanceLock() {
		showAlreadyRunningMessage()
		log.SugaredLogger.Warn("another instance already running, exit")
		return
	}
	defer releaseSingleInstanceLock()

	checkDir(paths.DataDir())
	machineid.Init(BuildKey)
	data.SetAppIcon(icon)
	db.Init("")
	// P3.2: Skill.ToolGroups 启动期校验。非法 ToolGroup typo 不会静默。
	agent.ValidateSkillToolGroups()
	// P1.1 路径指纹防护：警告但不中断启动，避免静默升级。
	// 当仓库根 data/stock.db 与 build/bin/data/stock.db 同源时记录警告，
	// 让"双库污染"问题在日志里可见。
	if realDB, buildBinDB, fpResult, err := func() (string, string, paths.FingerprintResult, error) {
		realDB := db.ResolveDBPath(paths.BaseDir())
		exe, _ := os.Executable()
		exeDir := filepath.Dir(exe)
		buildBinDB := filepath.Join(exeDir, "data", "stock.db")
		fp, err := paths.ResolveAndFingerprint(realDB, buildBinDB)
		return realDB, buildBinDB, fp, err
	}(); err != nil {
		log.SugaredLogger.Warnf("db fingerprint 路径防护计算失败（非致命）: %v", err)
	} else {
		log.SugaredLogger.Infof("db fingerprint: real=%s fp=%s build_bin=%s fp=%s equal=%v",
			realDB, fpResult.RealFingerprint, buildBinDB, fpResult.BuildBinFingerprint, fpResult.EqualToBuildBin)
		if fpResult.EqualToBuildBin {
			log.SugaredLogger.Warn("检测到自包含部署包与仓库根库同源，请检查是否误拷贝；本地运行应以仓库根库为准")
		}
	}
	data.InitAnalyzeSentiment()
	// Initialize embedding service for semantic memory search
	agent.InitEmbeddingService()
	go data.InitTradingCalendar()
	services.AutoMigrateAll()
	// 初始化 global_stock_index_cache 定时任务（原 main.AutoMigrate() 末尾调用）
	initGlobalStockIndexCacheTask()
	if err := db.RunVersionedMigrations(db.Dao); err != nil {
		log.SugaredLogger.Fatalf("版本化数据库迁移失败: %v", err)
	}
	// 后台启动分析报告回测
	go func() {
		time.Sleep(10 * time.Second) // 等数据库和数据服务就绪
		mgr := quant.NewBacktestManager()
		result, err := mgr.RunAIReportBacktest(context.Background(), 50)
		if err != nil {
			log.SugaredLogger.Errorf("auto backtest failed: %v", err)
		} else if result.Processed > 0 {
			log.SugaredLogger.Infof("auto backtest completed: %d reports processed, %d accurate (%.1f%%)",
				result.Processed, result.Accurate, result.AccuracyRate*100)
		}
	}()

	log.SugaredLogger.Info("starting...")
	log.SugaredLogger.Infof("version: %s  commit: %s", Version, VersionCommit)
	//log.SugaredLogger.Infof("build key: %s", BuildKey)

	// 程序启动时预缓存东财 Cookie
	//go func() {
	//	cacheCookies("https://push2his.eastmoney.com/api/qt/stock/kline/get")
	//}()

	// Create an instance of the app structure
	app := NewApp()
	AppMenu := menu.NewMenu()
	if IsMacOS() {
		AppMenu.Append(menu.EditMenu())
	}
	//FileMenu := AppMenu.AddSubmenu("设置")
	//FileMenu.AddText("窗口全屏", keys.CmdOrCtrl("f"), func(callback *menu.CallbackData) {
	//	runtime.WindowFullscreen(app.ctx)
	//})
	//FileMenu.AddText("窗口还原", keys.Key("Esc"), func(callback *menu.CallbackData) {
	//	runtime.WindowUnfullscreen(app.ctx)
	//})
	//FileMenu.AddText("显示搜索框", keys.CmdOrCtrl("s"), func(callbackData *menu.CallbackData) {
	//	runtime.EventsEmit(app.ctx, "showSearch", 1)
	//})
	//FileMenu.AddText("隐藏搜索框", keys.CmdOrCtrl("d"), func(callbackData *menu.CallbackData) {
	//	runtime.EventsEmit(app.ctx, "showSearch", 0)
	//})
	//FileMenu.AddText("刷新数据", keys.CmdOrCtrl("r"), func(callbackData *menu.CallbackData) {
	//	//runtime.EventsEmit(app.ctx, "refresh", "setting-"+time.Now().Format("2006-01-02 15:04:05"))
	//	runtime.EventsEmit(app.ctx, "refreshFollowList", "refresh-"+time.Now().Format("2006-01-02 15:04:05"))
	//})
	//FileMenu.AddSeparator()

	//if goruntime.GOOS == "windows" {
	//	FileMenu.AddText("隐藏到托盘区", keys.CmdOrCtrl("z"), func(_ *menu.CallbackData) {
	//		runtime.WindowHide(app.ctx)
	//	})
	//}

	//FileMenu.AddText("退出", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
	//	runtime.Quit(app.ctx)
	//})
	log.SugaredLogger.Info("version: " + Version)
	log.SugaredLogger.Info("commit: " + VersionCommit)
	// 根据屏幕分辨率自适应窗口尺寸
	width, height, _, _, err := getScreenResolution()
	if err != nil {
		log.SugaredLogger.Error("get screen resolution error")
		// 获取失败时给一个合理的默认值
		width = 1412
		height = 834
	}

	darkTheme := data.GetSettingConfig().DarkTheme
	backgroundColour := &options.RGBA{R: 255, G: 255, B: 255, A: 1}
	if darkTheme {
		backgroundColour = &options.RGBA{R: 27, G: 38, B: 54, A: 1}
	}

	//frameless := getFrameless()

	// 计算默认窗口大小：优先使用上次保存的用户尺寸，否则自适应
	config := data.GetSettingConfig()

	appWidth := config.WindowWidth
	appHeight := config.WindowHeight

	// 若用户尚未调整过窗口或记录为 0，则按屏幕比例给一个合适默认值
	if appWidth <= 0 || appHeight <= 0 {
		appWidth = width * 5 / 10
		appHeight = height * 5 / 10
	}
	log.SugaredLogger.Info("screen resolution: " + convertor.ToString(width) + "x" + convertor.ToString(height))
	log.SugaredLogger.Info("window size: " + convertor.ToString(appWidth) + "x" + convertor.ToString(appHeight))

	// Start metrics exporter if enabled
	shutdownMetrics, err := agent.StartMetricsExporter(context.Background())
	if err != nil {
		log.SugaredLogger.Warnf("Failed to start metrics exporter: %v", err)
	}
	if shutdownMetrics != nil {
		defer shutdownMetrics(context.Background())
	}

	// Create application with options
	err = wails.Run(&options.App{
		Title: "Blades Stock：AI赋能股票分析✨ " + OFFICIAL_STATEMENT,
		// 默认窗口大小：自适应但保留明显边距
		Width:  appWidth,
		Height: appHeight,
		//MinWidth:  minWidth,
		//MinHeight: minHeight,
		// 限制最大尺寸不超过屏幕
		//MaxWidth:                 width,
		//MaxHeight:                height,
		DisableResize:            false,
		Fullscreen:               false,
		Frameless:                false,
		StartHidden:              false,
		EnableDefaultContextMenu: true,
		BackgroundColour:         backgroundColour,
		Assets:                   assets,
		Menu:                     AppMenu,
		Logger:                   logger.NewFileLogger(filepath.Join(paths.LogDir(), "wails.log")),
		LogLevel:                 logger.DEBUG,
		LogLevelProduction:       logger.INFO,
		OnStartup:                app.startup,
		OnDomReady:               app.domReady,
		OnBeforeClose:            app.beforeClose,
		OnShutdown:               app.shutdown,
		WindowStartState:         options.Normal,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "blades-stock",
			OnSecondInstanceLaunch: OnSecondInstanceLaunch,
		},
		Bind: []interface{}{
			app,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			// DisableFramelessWindowDecorations: false,
			WebviewUserDataPath:  filepath.Join(paths.BaseDir(), "data", "webview2_data"),
			WebviewGpuIsDisabled: false,
		},
		// Mac platform specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 true,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Blades Stock",
				Message: "Blades Stock：AI赋能股票分析✨ ",
				Icon:    icon,
			},
		},
	})

	if err != nil {
		log.SugaredLogger.Fatal(err)
	}

}

// initGlobalStockIndexCacheTask 检查并创建 global_stock_index_cache 定时任务
func initGlobalStockIndexCacheTask() {
	var count int64
	db.Dao.Model(&models.CronTask{}).Where("task_type = ?", "global_stock_index_cache").Count(&count)
	if count == 0 {
		task := &models.CronTask{
			Name:        "全球指数缓存",
			CronExpr:    "0 0/5 * * * *", // 每5分钟执行一次
			TaskType:    "global_stock_index_cache",
			Target:      "",
			Params:      `{"crawlTimeOut": 30}`,
			Enable:      true,
			Status:      "active",
			Description: "自动缓存全球股票指数数据",
		}
		err := db.Dao.Create(task).Error
		if err != nil {
			log.SugaredLogger.Errorf("创建 global_stock_index_cache 定时任务失败：%v", err)
		} else {
			log.SugaredLogger.Info("创建 global_stock_index_cache 定时任务成功")
		}
	}

}

func updateBasicInfo() {
	config := data.GetSettingConfig()
	if config.UpdateBasicInfoOnStart {
		//更新基本信息
		go data.NewStockDataApi().GetStockBaseInfo()
		go data.NewStockDataApi().GetIndexBasic()
	}
}

func checkDir(dir string) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		os.Mkdir(dir, os.ModePerm)
		log.SugaredLogger.Info("create dir: " + dir)
	}
	if BuildKey == "" {
		log.SugaredLogger.Warn("BUILD_KEY not set via ldflags, using dev fallback")
		BuildKey = "dev-fallback-build-key"
	}
}

// PanicHandler 捕获 panic 的包装函数
func PanicHandler() {
	if r := recover(); r != nil {
		fmt.Printf("Recovered from panic: %v\n", r)
		debug.PrintStack()
	}
}
