package main

import (
	"context"
	"embed"
	"fmt"
	"time"

	assistantweb "blades-stock/ai-assistant-web"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	log "blades-stock/backend/logger"
	"blades-stock/backend/machineid"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"
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

//go:embed docs/blades-stock使用手册.md
var userManual []byte

//go:embed cmd/insert_etf_skill/etf_prompt.txt
var defaultETFBackup string

//go:embed cmd/insert_serenity_skill/serenity_prompt.txt
var defaultSerenityBackup string

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
	// 初始化交易日历（Baostock，含法定节假日）
	data.InitTradingCalendar()
	AutoMigrate()
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

	// 作为 blades-stock 子组件启动独立 Web 服务
	// 端口默认由 AI_ASSISTANT_WEB_ADDR 决定。
	go func() {
		if err := assistantweb.Start(); err != nil {
			log.SugaredLogger.Errorf("ai-assistant-web start error: %v", err)
		}
	}()

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
		Logger:                   logger.NewFileLogger("./logs/wails.log"),
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
	// AutoMigrate with timeout guard. GORM's AutoMigrate does not accept a
	// context, so we run it in a goroutine and abort on timeout to prevent the
	// app from hanging on a locked or corrupted database.
	migrateDone := make(chan struct{}, 1)
	go func() {
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
		SeedDefaultSkills()
		db.Dao.AutoMigrate(&models.CustomStrategy{})
		db.Dao.AutoMigrate(&models.BKFundFlow{})
		db.Dao.AutoMigrate(&models.ConceptFundFlow{})
		db.Dao.AutoMigrate(&models.ProjectAnalysis{})
		db.Dao.AutoMigrate(&models.CodeChunk{})
		db.Dao.AutoMigrate(&models.BacktestResult{})
		migrateDone <- struct{}{}
	}()
	select {
	case <-migrateDone:
		log.SugaredLogger.Info("数据库迁移完成")
	case <-time.After(30 * time.Second):
		log.SugaredLogger.Fatal("数据库迁移超时（30秒），请检查数据库文件是否损坏或已被锁定")
	}

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

func SeedDefaultSkills() {
	var count int64
	db.Dao.Model(&models.Skill{}).Count(&count)
	if count > 0 {
		return
	}
	skills := []models.Skill{
		{
			Name: "ETF轮动多Agent分析",
			Description: "执行完整的多Agent ETF轮动分析流程：21日加权动量筛选→6路因子并发分析(fan-out)→投委会5视角加权融合→输出开盘前交易决策",
			Category: "量化策略",
			SystemPrompt: defaultETFBackup,
			Directive: `# ETF轮动多Agent分析 — 强制执行规则

你必须严格按以下流程执行ETF开盘前分析：

## 第1步：动量筛选
用 SearchStockByIndicators 获取ETF池，按21日加权动量排序，剔除回撤>冷却阈值或盈亏比<1的标的。

## 第2步：6路因子并发分析
对前5只候选ETF，用工具fan-out并行分析以下维度：
- 技术面（均线/KDJ/MACD/布林带）
- 资金面（主力/北向资金流向）
- 消息面（新闻/公告）
- 海外联动（隔夜外围表现）
- 宏观/行业周期
- 申赎/溢价率

## 第3步：投委会5视角加权融合
从趋势、风控、价值、筹码、情绪5个视角评分，加权输出综合得分和决策建议。

## 输出格式
必须包含：ETF池概览→动量排名→各维度分析摘要→投委会评分表→最终交易决策（买入/持有/卖出/观望+仓位建议）`,
			TriggerKeywords: "ETF,轮动,动量,板块轮动,资金流向,北向资金,技术面,消息面,开盘前决策,集合竞价,ETF池,动量轮动,因子分析,投委会,持仓评审,海外联动,宏观,资金流,申赎,主力资金,溢价率,双动量,回撤冷却,盈亏比,仓位管理,集合竞价复核,投委会精选",
			Examples: "用户：今天ETF开盘前有什么建议？我持有159915和512660\n助手：必须输出完整的ETF轮动分析报告结构，从动量筛选开始逐步分析到最终决策，包含持仓评审。",
			Enable: true, SortOrder: 1,
		},
		{
			Name: "Serenity式产业分析",
			Description: "强制使用Serenity产业分析框架：产业链拆解→稀缺层识别→优先级排序→证据验证→证伪条件",
			Category: "资讯研究",
			SystemPrompt: defaultSerenityBackup,
			Directive: `# Serenity式产业分析 — 强制执行规则

你必须严格按Serenity研究框架执行产业分析：

## 第1步：产业链全景拆解
识别目标公司在产业链中的位置（上游/中游/下游），列出关键环节和参与方。

## 第2步：稀缺层识别
找出产业链中技术壁垒最高、替代难度最大的环节。关注：国产替代率<30%、市占率集中度CR5>60%、毛利率>40%的环节。

## 第3步：优先级排序
按稀缺性、可投资性、确定性三维度评分，锁定高优先级方向。

## 第4步：证据链验证
用数据验证判断：营收/利润增速、研发投入占比、客户集中度、产能利用率。必须调用工具获取最新数据，严禁编造。

## 第5步：证伪条件
明确列出什么情况下分析结论会失效（如技术路线变化、政策转向、竞争格局突变）。

## 输出格式
必须包含：产业链全景图→稀缺层分析→优先级评分→证据链→证伪条件→最终判断`,
			TriggerKeywords: "产业分析,产业链,稀缺性,瓶颈,行业研究,赛道分析,上下游,供需格局,渗透率,国产替代,市场份额,竞争格局,华阳集团,投资机会,细分龙头",
			Examples: "用户：分析华阳集团在汽车电子产业链中的位置\n助手：必须输出Serenity报告结构：产业链全景图→稀缺层识别→优先级排序→证据链→证伪条件",
			Enable: true, SortOrder: 2,
		},
	}
	for _, s := range skills {
		if err := db.Dao.Create(&s).Error; err != nil {
			log.SugaredLogger.Errorf("seed skill <%s> failed: %v", s.Name, err)
		} else {
			log.SugaredLogger.Infof("seed skill <%s> ID=%d", s.Name, s.ID)
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
