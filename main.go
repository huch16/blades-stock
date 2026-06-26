package main

import (
	"embed"
	"fmt"
	"time"

	assistantweb "go-stock/ai-assistant-web"
	"go-stock/backend/agent"
	"go-stock/backend/data"
	"go-stock/backend/db"
	log "go-stock/backend/logger"
	"go-stock/backend/machineid"
	"go-stock/backend/models"
	"os"
	"runtime/debug"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

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

//go:embed build/screenshot/扫码_搜索联合传播样式-白色版.png
var wxgzh []byte

//go:embed docs/go-stock使用手册.md
var userManual []byte

//go:generate cp -R ./data ./build/bin

var Version string
var VersionCommit string
var OFFICIAL_STATEMENT string
var BuildKey string

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.SugaredLogger.Error("panic: ", r)
			log.SugaredLogger.Error("stack: ", string(debug.Stack()))
		}
	}()

	checkDir("data")
	machineid.Init(BuildKey)
	data.SetAppIcon(icon)
	db.Init("")
	data.InitAnalyzeSentiment()
	go AutoMigrate()
	// 后台启动分析报告回测
	go func() {
		time.Sleep(10 * time.Second) // 等数据库和数据服务就绪
		count, err := agent.NewBacktestService().RunAllBacktests()
		if err != nil {
			log.SugaredLogger.Errorf("auto backtest failed: %v", err)
		} else if count > 0 {
			log.SugaredLogger.Infof("auto backtest completed: %d reports", count)
		}
	}()

	//db.Dao.Model(&data.Group{}).Where("id = ?", 0).FirstOrCreate(&data.Group{
	//	Name: "默认分组",
	//	Sort: 0,
	//})

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

	// 作为 go-stock 子组件启动独立 Web 服务
	// 端口默认由 AI_ASSISTANT_WEB_ADDR 决定。
	go func() {
		if err := assistantweb.Start(); err != nil {
			log.SugaredLogger.Errorf("ai-assistant-web start error: %v", err)
		}
	}()

	// Create application with options
	err = wails.Run(&options.App{
		Title: "go-stock：AI赋能股票分析✨ " + OFFICIAL_STATEMENT,
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
		Logger:                   logger.NewFileLogger("./logs/wails.log"),
		LogLevel:                 logger.DEBUG,
		LogLevelProduction:       logger.INFO,
		OnStartup:                app.startup,
		OnDomReady:               app.domReady,
		OnBeforeClose:            app.beforeClose,
		OnShutdown:               app.shutdown,
		WindowStartState:         options.Normal,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "go-stock",
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
			WebviewUserDataPath: "",
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
				Title:   "go-stock",
				Message: "go-stock：AI赋能股票分析✨ ",
				Icon:    icon,
			},
		},
	})

	if err != nil {
		log.SugaredLogger.Fatal(err)
	}

}

func AutoMigrate() {
	db.Dao.AutoMigrate(&data.StockInfo{})
	db.Dao.AutoMigrate(&data.StockBasic{})
	db.Dao.AutoMigrate(&data.FollowedStock{})
	db.Dao.AutoMigrate(&data.IndexBasic{})
	db.Dao.AutoMigrate(&data.Settings{})
	db.Dao.AutoMigrate(&models.AIResponseResult{})
	db.Dao.AutoMigrate(&models.StockInfoHK{})
	db.Dao.AutoMigrate(&models.StockInfoUS{})
	db.Dao.AutoMigrate(&data.FollowedFund{})
	db.Dao.AutoMigrate(&data.FollowedStock{})
	db.Dao.AutoMigrate(&data.FundBasic{})
	db.Dao.AutoMigrate(&data.Group{})
	db.Dao.AutoMigrate(&data.GroupStock{})
	db.Dao.AutoMigrate(&models.Tags{})
	db.Dao.AutoMigrate(&models.Telegraph{})
	db.Dao.AutoMigrate(&models.TelegraphTags{})
	db.Dao.AutoMigrate(&models.LongTigerRankData{})
	db.Dao.AutoMigrate(&data.AIConfig{})
	db.Dao.AutoMigrate(&models.BKDict{})
	db.Dao.AutoMigrate(&models.WordAnalyze{})
	db.Dao.AutoMigrate(&models.SentimentResultAnalyze{})
	db.Dao.AutoMigrate(&models.AiRecommendStocks{})
	db.Dao.AutoMigrate(&models.AllStockInfo{})
	db.Dao.AutoMigrate(&models.CronTask{})
	db.Dao.AutoMigrate(&models.AiAssistantSession{})
	db.Dao.AutoMigrate(&models.GlobalStockIndex{})
	db.Dao.AutoMigrate(&data.TradingRecord{})
	db.Dao.AutoMigrate(&models.MCPServer{})
	db.Dao.AutoMigrate(&models.MCPServerTool{})
	db.Dao.AutoMigrate(&models.Skill{})
	db.Dao.AutoMigrate(&models.CustomStrategy{})
	db.Dao.AutoMigrate(&models.BKFundFlow{})
	db.Dao.AutoMigrate(&models.ConceptFundFlow{})
	db.Dao.AutoMigrate(&models.ProjectAnalysis{})
	db.Dao.AutoMigrate(&models.CodeChunk{})
	db.Dao.AutoMigrate(&models.BacktestResult{})

	//updateMultipleModel()

	// 初始化 global_stock_index_cache 定时任务
	initGlobalStockIndexCacheTask()
}

// initGlobalStockIndexCacheTask 检查并创建 global_stock_index_cache 定时任务
func initGlobalStockIndexCacheTask() {
	var count int64
	db.Dao.Model(&models.CronTask{}).Where("task_type = ?", "global_stock_index_cache").Count(&count)
	if count == 0 {
		task := &models.CronTask{
			Name:        "全球指数缓存",
			CronExpr:    "0 0/5 * * * *", // 每分钟执行一次
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
