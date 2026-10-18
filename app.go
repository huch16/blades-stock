package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"blades-stock/backend/agent"
	"blades-stock/backend/agent/tools"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"
	"blades-stock/backend/services"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"golang.org/x/exp/slices"

	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/robfig/cron/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

// App struct
type App struct {
	ctx                context.Context
	cache              *freecache.Cache
	cron               *cron.Cron
	cronEntrys         map[string]cron.EntryID
	cronEntrysMu       sync.Mutex
	AiTools            []data.Tool
	summaryMu          sync.Mutex
	summaryCancel      context.CancelFunc
	agentMu            sync.Mutex
	agentCancel        context.CancelFunc
	stockAlertMu       sync.Mutex
	stockAlertLastSent map[string]time.Time
	priceAtAlertReset  map[string]float64
	memoryMgr          *agent.MemoryManager
	evaluator          *agent.Evaluator
	evolver            *agent.Evolver

	// [阶段 1 解耦服务委托层]
	marketService   *services.MarketService
	quantService    *services.QuantService
	agentService    *services.AgentService
	settingsService *services.SettingsService
}

// NewApp creates a new App application struct
func NewApp() *App {
	cacheSize := 512 * 1024
	cache := freecache.NewCache(cacheSize)
	c := cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	c.Start()
	var tools []data.Tool
	tools = data.Tools(tools)
	return &App{
		cache:              cache,
		cron:               c,
		cronEntrys:         make(map[string]cron.EntryID),
		AiTools:            tools,
		stockAlertLastSent: make(map[string]time.Time),
		priceAtAlertReset:  make(map[string]float64),
		memoryMgr:          agent.NewMemoryManager(20),
		evaluator:          agent.NewEvaluator(),
		evolver:            agent.NewEvolver(),
		marketService:      services.NewMarketService(),
		quantService:       services.NewQuantService(),
		agentService:       services.NewAgentService(),
		settingsService:    services.NewSettingsService(),
	}
}

func (a *App) setCronEntry(key string, id cron.EntryID) {
	a.cronEntrysMu.Lock()
	a.cronEntrys[key] = id
	a.cronEntrysMu.Unlock()
}

func (a *App) getCronEntry(key string) (cron.EntryID, bool) {
	a.cronEntrysMu.Lock()
	id, exists := a.cronEntrys[key]
	a.cronEntrysMu.Unlock()
	return id, exists
}

func (a *App) GetMachineId() string {
	return a.settingsService.GetMachineId()
}

func (a *App) CheckDeviceBinding(token string, apiBase string) map[string]any {
	return a.settingsService.CheckDeviceBinding(token, apiBase)
}

func (a *App) QuitApp() {
	if a.ctx != nil {
		if a.cron != nil {
			a.cron.Stop()
		}
		runtime.Quit(a.ctx)
	}
}

func GetSource(tags []string) string {
	if slice.ContainAny(tags, []string{"外媒简讯", "外媒资讯", "外媒"}) {
		return "外媒"
	}
	if slices.Contains(tags, "财联社电报") {
		return "财联社电报"
	}
	if slices.Contains(tags, "新浪财经") {
		return "新浪财经"
	}
	return ""
}

// domReady is called after front-end resources have been loaded
func (a *App) domReady(ctx context.Context) {
	defer PanicHandler()

	// 初始化 VPS 数据服务
	go func() {
		// 交易日历（AData → Baostock → 周末判断）
		data.InitTradingCalendar()

		// AData 健康检查
		if err := data.NewAdataClient().Ping(); err != nil {
			logger.SugaredLogger.Warnf("AData 服务连接失败: %v", err)
		}

		// AKShare 健康检查
		if err := data.NewAkshareClient().Ping(); err != nil {
			logger.SugaredLogger.Warnf("AKShare 服务连接失败: %v", err)
		}
	}()

	defer func() {
		// 增加延迟确保前端已准备好接收事件
		go func() {
			time.Sleep(2 * time.Second)
			runtime.EventsEmit(a.ctx, "loadingMsg", "done")
		}()
	}()

	//if stocksBin != nil && len(stocksBin) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查A股基础信息...")
	//	go initStockData(a.ctx)
	//}
	//
	//if stocksBinHK != nil && len(stocksBinHK) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查港股基础信息...")
	//	go initStockDataHK(a.ctx)
	//}
	//
	//if stocksBinUS != nil && len(stocksBinUS) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查美股基础信息...")
	//	go initStockDataUS(a.ctx)
	//}
	updateBasicInfo()

	// Add your action here
	//定时更新数据
	config := data.GetSettingConfig()
	go func() {
		go data.NewMarketNewsApi().TelegraphList(30)
		go data.NewMarketNewsApi().GetSinaNews(30)
		go data.NewMarketNewsApi().TradingViewNews()

		interval := config.RefreshInterval
		if interval <= 0 {
			interval = 1
		}
		//ticker := time.NewTicker(time.Second * time.Duration(interval))
		//defer ticker.Stop()
		//for range ticker.C {
		//	MonitorStockPrices(a)
		//}
		id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval), func() {
			MonitorStockPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorStockPrices", id)
		}
		entryID, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			//news := data.NewMarketNewsApi().GetNewTelegraph(30)
			news := data.NewMarketNewsApi().TelegraphList(30)
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "newTelegraph", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("GetNewTelegraph", entryID)
		}

		entryIDSina, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			news := data.NewMarketNewsApi().GetSinaNews(30)
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "newSinaNews", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("newSinaNews", entryIDSina)
		}

		entryIDTradingViewNews, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			news := data.NewMarketNewsApi().TradingViewNews()
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "tradingViewNews", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("tradingViewNews", entryIDTradingViewNews)
		}
	}()

	//刷新基金净值信息
	go func() {
		//ticker := time.NewTicker(time.Second * time.Duration(60))
		//defer ticker.Stop()
		//for range ticker.C {
		//	MonitorFundPrices(a)
		//}
		if config.EnableFund {
			id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
				MonitorFundPrices(a)
			})
			if err != nil {
				logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
			} else {
				a.setCronEntry("MonitorFundPrices", id)
			}
		}

		// AI 推荐股票价格监控定时器
		idAiStock, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			MonitorAiRecommendStockPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc MonitorAiRecommendStockPrices error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorAiRecommendStockPrices", idAiStock)
		}

		// 自选股成本价监控定时器
		idCostPrice, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			MonitorFollowedStockCostPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc MonitorFollowedStockCostPrices error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorFollowedStockCostPrices", idCostPrice)
		}

	}()

	if config.EnableNews {
		//go func() {
		//	ticker := time.NewTicker(time.Second * time.Duration(60))
		//	defer ticker.Stop()
		//	for range ticker.C {
		//		telegraph := refreshTelegraphList()
		//		if telegraph != nil {
		//			go runtime.EventsEmit(a.ctx, "telegraph", telegraph)
		//		}
		//	}
		//
		//}()

		id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			telegraph := refreshTelegraphList()
			if telegraph != nil {
				go runtime.EventsEmit(a.ctx, "telegraph", telegraph)
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("refreshTelegraphList", id)
		}

		go runtime.EventsEmit(a.ctx, "telegraph", refreshTelegraphList())
	}
	go MonitorStockPrices(a)
	if config.EnableFund {
		go MonitorFundPrices(a)
		go data.NewFundApi().AllFund()
	}
	// AI 推荐股票价格监控
	go MonitorAiRecommendStockPrices(a)
	// 自选股成本价监控
	go MonitorFollowedStockCostPrices(a)
	// 市场统计数据采集（交易日每5分钟）
	go func() {
		a.FetchAndSaveMarketStatistic()
		idMarketStat, err := a.cron.AddFunc("0 */5 9-15 * * 1-5", func() {
			a.FetchAndSaveMarketStatistic()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc FetchAndSaveMarketStatistic error:%s", err.Error())
		} else {
			a.setCronEntry("FetchAndSaveMarketStatistic", idMarketStat)
		}
	}()
	// 板块资金流向数据采集（交易日每60秒）
	go func() {
		data.NewBKFundFlowApi().FetchAndSave()
		idBKFundFlow, err := a.cron.AddFunc("@every 60s", func() {
			if a.IsTradingTime() {
				data.NewBKFundFlowApi().FetchAndSave()
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc BKFundFlowFetchAndSave error:%s", err.Error())
		} else {
			a.setCronEntry("BKFundFlowFetchAndSave", idBKFundFlow)
		}
	}()
	// 概念资金流向数据采集（交易日每60秒）
	go func() {
		data.NewConceptFundFlowApi().FetchAndSave()
		idConceptFundFlow, err := a.cron.AddFunc("@every 60s", func() {
			if a.IsTradingTime() {
				data.NewConceptFundFlowApi().FetchAndSave()
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc ConceptFundFlowFetchAndSave error:%s", err.Error())
		} else {
			a.setCronEntry("ConceptFundFlowFetchAndSave", idConceptFundFlow)
		}
	}()
	//检查股票基础信息
	go func() {
		cfg := a.GetConfig()
		var count int64
		db.Dao.Model(&data.StockBasic{}).Count(&count)

		// 仅在首次启动无数据（count == 0）或用户开启「启动时更新基础信息」时，才在启动期拉取全量
		if count == 0 || (cfg != nil && cfg.UpdateBasicInfoOnStart) {
			logger.SugaredLogger.Infof("启动期按策略同步全市场标的基础数据 (count=%d, updateOnStart=%v)...", count, cfg != nil && cfg.UpdateBasicInfoOnStart)
			// 串行执行，杜绝多个后台协程同时执行全量清空与批量写入引发 SQLite BUSY 死锁
			a.CheckStockBaseInfo(a.ctx)
			syncAllStockInfo(a.ctx)
		} else {
			logger.SugaredLogger.Infof("本地已有股票基础数据 (%d 条)，跳过启动期重复全量同步", count)
		}

		a.cron.AddFunc("0 0 2 * * *", func() {
			logger.SugaredLogger.Infof("定时维护：更新股票基础数据...")
			a.CheckStockBaseInfo(a.ctx)
		})
		a.cron.AddFunc("30 05 8,12,20 * * *", func() {
			syncAllStockInfo(a.ctx)
		})
		// 进化闭环：每日从高质量轨迹提取技能草稿（7 天限频，仅记录供人工审阅）。
		a.cron.AddFunc("30 2 * * *", agent.RunSkillExtractionPass)
		// 历史回测与战绩复盘闭环：每个交易日收盘后自动回测 AI 报告的预测准确度与收益率
		a.cron.AddFunc("30 15 * * 1-5", func() {
			logger.SugaredLogger.Infof("盘后触发 AI 报告历史回测与胜率检验...")
			mgr := quant.NewBacktestManager()
			if res, err := mgr.RunAIReportBacktest(context.Background(), 100); err == nil && res.Processed > 0 {
				logger.SugaredLogger.Infof("盘后 AI 报告回测完成: 处理 %d 份，准确 %d (%.1f%%)",
					res.Processed, res.Accurate, res.AccuracyRate*100)
			}
		})
	}()

	//检查谷歌浏览器
	//go func() {
	//	f := checkChromeOnWindows()
	//	if !f {
	//		go runtime.EventsEmit(a.ctx, "warnMsg", "谷歌浏览器未安装,ai分析功能可能无法使用")
	//	}
	//}()

	//检查Edge浏览器
	//go func() {
	//	path, e := checkEdgeOnWindows()
	//	if !e {
	//		go runtime.EventsEmit(a.ctx, "warnMsg", "Edge浏览器未安装,ai分析功能可能无法使用")
	//	} else {
	//		logger.SugaredLogger.Infof("Edge浏览器已安装，路径为: %s", path)
	//	}
	//}()
	followList := data.NewStockDataApi().GetFollowList(0)
	for _, follow := range *followList {
		if follow.Cron == nil || *follow.Cron == "" {
			continue
		}
		entryID, err := a.cron.AddFunc(*follow.Cron, a.AddCronTask(follow))
		if err != nil {
			logger.SugaredLogger.Errorf("添加自动分析任务失败:%s cron=%s entryID:%v", follow.Name, *follow.Cron, entryID)
			continue
		}
		a.setCronEntry(follow.StockCode, entryID)
	}
	//logger.SugaredLogger.Infof("domReady-cronEntrys:%+v", a.cronEntrys)

}

func syncAllStockInfo(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	var datas []models.AllStockInfo
	for page := 1; page < 3; page++ {
		res := data.NewStockDataApi().GetAllStocks(page, 3000, "", models.TechnicalIndicators{})
		if res != nil && (*res).Result.Data != nil {
			for _, d := range (*res).Result.Data {
				datas = append(datas, d.ToAllStockInfo())
			}
		}
	}
	if len(datas) > 0 {
		err := db.Dao.Transaction(func(tx *gorm.DB) error {
			if e := tx.Unscoped().Model(&models.AllStockInfo{}).Where("1=1").Delete(&models.AllStockInfo{}).Error; e != nil {
				return e
			}
			return tx.CreateInBatches(&datas, 500).Error
		})
		if err != nil {
			logger.SugaredLogger.Errorf("db.Dao.CreateInBatches error:%s", err.Error())
		}
	}
}
func (a *App) CheckStockBaseInfo(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	stockBasics := &[]data.StockBasic{}
	data.SharedHTTPClient.R().
		SetHeader("user", "blades-stock").
		SetResult(stockBasics).
		Get(data.StaticDataServiceURL() + "/go-stock/stock_basic.json")

	if stockBasics != nil && len(*stockBasics) > 0 {
		err := db.Dao.Transaction(func(tx *gorm.DB) error {
			if e := tx.Unscoped().Model(&data.StockBasic{}).Where("1=1").Delete(&data.StockBasic{}).Error; e != nil {
				return e
			}
			return tx.CreateInBatches(stockBasics, 500).Error
		})
		if err != nil {
			logger.SugaredLogger.Errorf("保存StockBasic股票基础信息失败:%s", err.Error())
		}
	}
	// 全量覆盖完成后，用通达信即时数据对 A 股做增量校准（单事务内存比对，耗时极短，串行执行避免竞争）
	a.syncStockBasicFromTdx()

	stockHKBasics := &[]models.StockInfoHK{}
	data.SharedHTTPClient.R().
		SetHeader("user", "blades-stock").
		SetResult(stockHKBasics).
		Get(data.StaticDataServiceURL() + "/go-stock/stock_base_info_hk.json")

	if stockHKBasics != nil && len(*stockHKBasics) > 0 {
		err := db.Dao.Transaction(func(tx *gorm.DB) error {
			if e := tx.Unscoped().Model(&models.StockInfoHK{}).Where("1=1").Delete(&models.StockInfoHK{}).Error; e != nil {
				return e
			}
			return tx.CreateInBatches(stockHKBasics, 500).Error
		})
		if err != nil {
			logger.SugaredLogger.Errorf("保存StockInfoHK股票基础信息失败:%s", err.Error())
		}
	}

	stockUSBasics := &[]models.StockInfoUS{}
	data.SharedHTTPClient.R().
		SetHeader("user", "blades-stock").
		SetResult(stockUSBasics).
		Get(data.StaticDataServiceURL() + "/go-stock/stock_base_info_us.json")

	if stockUSBasics != nil && len(*stockUSBasics) > 0 {
		err := db.Dao.Transaction(func(tx *gorm.DB) error {
			if e := tx.Unscoped().Model(&models.StockInfoUS{}).Where("1=1").Delete(&models.StockInfoUS{}).Error; e != nil {
				return e
			}
			return tx.CreateInBatches(stockUSBasics, 500).Error
		})
		if err != nil {
			logger.SugaredLogger.Errorf("保存StockInfoUS股票基础信息失败:%s", err.Error())
		}
	}
	// 港股/美股全量覆盖完成后，用通达信扩展行情即时数据做增量校准
	a.syncHKUSStockBasicFromTdx()
}

// syncStockBasicFromTdx 用通达信即时数据对 A 股基础信息做增量校准。
// 在 CheckStockBaseInfo 全量覆盖之后调用：通达信本地证券列表新股上市当天即可见，
// 以 upsert 方式补充全量 JSON 未及时覆盖的新上市/改名/退市股票。
func (a *App) syncStockBasicFromTdx() {
	defer PanicHandler()
	added, updated, err := data.NewTdxKLineApi().SyncStockBasicToDB()
	if err != nil {
		logger.SugaredLogger.Warnf("通达信同步股票基础信息失败:%s", err.Error())
		return
	}
	logger.SugaredLogger.Infof("通达信同步股票基础信息完成：新增 %d 条，更新 %d 条", added, updated)
}

// syncHKUSStockBasicFromTdx 用通达信扩展行情即时数据对港股/美股基础信息做增量校准。
func (a *App) syncHKUSStockBasicFromTdx() {
	defer PanicHandler()
	hkAdded, hkUpdated, usAdded, usUpdated, err := data.NewTdxKLineApi().SyncHKUSStockBasicToDB()
	if err != nil {
		logger.SugaredLogger.Warnf("通达信同步港美股基础信息失败:%s", err.Error())
		return
	}
	logger.SugaredLogger.Infof("通达信同步港美股基础信息完成：港股新增 %d 更新 %d，美股新增 %d 更新 %d",
		hkAdded, hkUpdated, usAdded, usUpdated)
}
func (a *App) NewsPush(news *[]models.Telegraph) {

	follows := data.NewStockDataApi().GetFollowList(0)
	stockNames := slice.Map(*follows, func(index int, item data.FollowedStock) string {
		return item.Name
	})

	for _, telegraph := range *news {
		if a.GetConfig().EnableOnlyPushRedNews {
			if telegraph.IsRed || strutil.ContainsAny(telegraph.Content, stockNames) {
				go runtime.EventsEmit(a.ctx, "newsPush", telegraph)
			}
		} else {
			go runtime.EventsEmit(a.ctx, "newsPush", telegraph)
		}
		//go data.NewAlertWindowsApi("go-stock", telegraph.Source+" "+telegraph.Time, telegraph.Content, string(icon)).SendNotification()
		//}
	}
}

func (a *App) AddCronTask(follow data.FollowedStock) func() {
	return func() {
		go runtime.EventsEmit(a.ctx, "warnMsg", "开始自动分析"+follow.Name+"_"+follow.StockCode)
		question := fmt.Sprintf("全面分析%s(%s)的技术面、基本面、消息面和资金面", follow.Name, follow.StockCode)
		var res strings.Builder
		thinking := data.GetSettingConfig().GetAIConfigThinking(follow.AiConfigId)
		for msg := range agent.NewStockAiAgentApi().ChatWithContext(a.ctx, question, follow.AiConfigId, nil, false, 0, thinking, "") {
			if msg == nil {
				continue
			}
			if msg.ReasoningContent != "" {
				res.WriteString(msg.ReasoningContent)
			}
			res.WriteString(msg.Content)
		}
		data.NewDeepSeekOpenAi(a.ctx, follow.AiConfigId).SaveAIResponseResult(follow.StockCode, follow.Name, res.String(), "", question)
		go runtime.EventsEmit(a.ctx, "warnMsg", "AI分析完成："+follow.Name+"_"+follow.StockCode)
	}
}

func refreshTelegraphList() *[]string {
	clsURL := "https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraph&os=web&sv=8.7.9"
	response, err := data.SharedHTTPClient.R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		Get(clsURL)
	if err != nil {
		return &[]string{}
	}
	res := map[string]any{}
	if err := json.Unmarshal(response.Body(), &res); err != nil {
		return &[]string{}
	}
	var telegraph []string
	if v, _ := convertor.ToInt(res["errno"]); v == 0 {
		if res["data"] == nil {
			return &[]string{}
		}
		dataMap, ok := res["data"].(map[string]any)
		if !ok {
			return &[]string{}
		}
		rollData, ok := dataMap["roll_data"].([]any)
		if !ok {
			return &[]string{}
		}
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			content, _ := news["content"].(string)
			if content != "" {
				telegraph = append(telegraph, content)
			}
		}
	}
	return &telegraph
}

// isTradingDay 判断是否是交易日
var tradingDayCache = freecache.NewCache(64 * 1024)

func isTradingDay(date time.Time) bool {
	weekday := date.Weekday()
	dateStr := date.Format("2006-01-02")

	cacheKey := []byte(dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type holidayResp struct {
		Code    int `json:"code"`
		Holiday struct {
			Holiday bool   `json:"holiday"`
			Name    string `json:"name"`
		} `json:"holiday"`
	}
	var result holidayResp
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(fmt.Sprintf("https://timor.tech/api/holiday/info/%s", date))
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	if result.Code == 0 && result.Holiday.Holiday {
		return true, true
	}
	return false, true
}

func preCacheTradingDays() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = ShanghaiTimezone
	}
	now := time.Now().In(loc)
	go func() {
		for i := -7; i <= 7; i++ {
			d := now.AddDate(0, 0, i)
			isTradingDay(d)
		}
	}()
	go func() {
		for i := -7; i <= 7; i++ {
			d := now.AddDate(0, 0, i)
			isHKTradingDay(d)
		}
	}()
	go func() {
		est, _ := time.LoadLocation("America/New_York")
		for i := -7; i <= 7; i++ {
			d := now.AddDate(0, 0, i)
			if est != nil {
				d = d.In(est)
			}
			isUSTradingDay(d)
		}
	}()
}

// isTradingTime 判断是否是交易时间
func isTradingTime(date time.Time) bool {
	if !isTradingDay(date) {
		return false
	}

	hour, minute, _ := date.Clock()

	// 判断是否在9:15到11:30之间
	if (hour == 9 && minute >= 15) || (hour == 10) || (hour == 11 && minute <= 30) {
		return true
	}

	// 判断是否在13:00到15:00之间
	if (hour == 13) || (hour == 14) || (hour == 15 && minute <= 0) {
		return true
	}

	return false
}

// IsHKTradingTime 判断当前时间是否在港股交易时间内
func IsHKTradingTime(date time.Time) bool {
	if !isHKTradingDay(date) {
		return false
	}

	hour, minute, _ := date.Clock()

	if (hour == 9 && minute >= 0) || (hour == 9 && minute <= 30) {
		return true
	}

	if (hour == 9 && minute > 30) || (hour >= 10 && hour < 12) || (hour == 12 && minute == 0) {
		return true
	}

	if (hour == 13 && minute >= 0) || (hour >= 14 && hour < 16) || (hour == 16 && minute == 0) {
		return true
	}

	if (hour == 16 && minute >= 0) || (hour == 16 && minute <= 10) {
		return true
	}
	return false
}

func isHKTradingDay(date time.Time) bool {
	weekday := date.Weekday()
	dateStr := date.Format("2006-01-02")

	cacheKey := []byte("hk:" + dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkHKHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkHKHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type klineResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var result klineResp
	dateClean := strings.ReplaceAll(date, "-", "")
	apiURL := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=100.HSI&fields1=f1&fields2=f51&klt=101&fqt=0&beg=%s&end=%s", dateClean, dateClean)
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(apiURL)
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	if len(result.Data.Klines) > 0 {
		return false, true
	}
	return true, true
}

// IsUSTradingTime 判断当前时间是否在美股交易时间内
func IsUSTradingTime(date time.Time) bool {
	est, err := time.LoadLocation("America/New_York")
	var estTime time.Time
	if err != nil {
		estTime = date.Add(time.Hour * -12)
	} else {
		estTime = date.In(est)
	}

	if !isUSTradingDay(estTime) {
		return false
	}

	hour, minute, _ := estTime.Clock()

	if (hour == 4) || (hour == 5) || (hour == 6) || (hour == 7) || (hour == 8) || (hour == 9 && minute < 30) {
		return true
	}

	if (hour == 9 && minute >= 30) || (hour >= 10 && hour < 16) || (hour == 16 && minute == 0) {
		return true
	}

	if (hour == 16 && minute > 0) || (hour >= 17 && hour < 20) || (hour == 20 && minute == 0) {
		return true
	}

	return false
}

func isUSTradingDay(estTime time.Time) bool {
	weekday := estTime.Weekday()
	dateStr := estTime.Format("2006-01-02")

	cacheKey := []byte("us:" + dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkUSHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkUSHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type usHolidayResp struct {
		IsHoliday    bool   `json:"is_holiday"`
		IsEarlyClose bool   `json:"is_early_close"`
		IsWeekend    bool   `json:"is_weekend"`
		Status       string `json:"status"`
	}
	var result usHolidayResp
	apiURL := fmt.Sprintf("https://fincalapi.com/v1/day_status?calendar=NYSE&date=%s", date)
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(apiURL)
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	return result.IsHoliday, true
}
func MonitorFundPrices(a *App) {
	// 检查 A 股是否开市（基金交易时间与 A 股一致）
	if !isTradingTime(time.Now()) {
		// logger.SugaredLogger.Debugf("当前 A 股未开市，跳过基金价格监控") // suppressed to reduce noise
		return
	}

	logger.SugaredLogger.Debugf("A 股市场已开市，开始基金价格监控")

	dest := &[]data.FollowedFund{}
	db.Dao.Model(&data.FollowedFund{}).Find(dest)
	for _, follow := range *dest {
		_, err := data.NewFundApi().CrawlFundBasic(follow.Code)
		if err != nil {
			logger.SugaredLogger.Errorf("获取基金基本信息失败，基金代码：%s，错误信息：%s", follow.Code, err.Error())
			continue
		}
		data.NewFundApi().CrawlFundNetEstimatedUnit(follow.Code)
		data.NewFundApi().CrawlFundNetUnitValue(follow.Code)
	}
}

// MonitorAiRecommendStockPrices 监控 AI 推荐股票的价格，当股价达到预警线时发送通知
func MonitorAiRecommendStockPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过 AI 推荐股票价格监控")
		return
	}

	var aiRecommendStocks []models.AiRecommendStocks
	db.Dao.Model(&models.AiRecommendStocks{}).Where("enable_alert = ?", true).Find(&aiRecommendStocks)

	if len(aiRecommendStocks) == 0 {
		return
	}

	stockCodes := make([]string, 0)
	stockCodeMap := make(map[string]*models.AiRecommendStocks)
	for i := range aiRecommendStocks {
		stock := &aiRecommendStocks[i]
		stopLossPrice, _ := convertor.ToFloat(stock.RecommendStopLossPrice)
		if stock.RecommendBuyPriceMin <= 0 && stock.RecommendStopProfitPriceMin <= 0 && stopLossPrice <= 0 {
			continue
		}
		stockCodes = append(stockCodes, tools.GetStockCode(stock.StockCode))
		stockCodeMap[tools.GetStockCode(stock.StockCode)] = stock
	}

	if len(stockCodes) == 0 {
		logger.SugaredLogger.Debugf("没有设置预警价格的 AI 推荐股票，跳过价格监控")
		return
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取 AI 推荐股票实时数据失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		aiStock, ok := stockCodeMap[tools.GetStockCode(stockInfo.Code)]
		if !ok {
			continue
		}

		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}

		baseAlertKey := fmt.Sprintf("%s:%s", aiStock.StockCode, aiStock.DataTime.Format("20060102"))

		buyAlertKey := baseAlertKey + ":BUY"
		if aiStock.RecommendBuyPriceMin > 0 && currentPrice <= aiStock.RecommendBuyPriceMin {
			priceSinceLastBuyAlert := a.getPriceAtAlertReset(buyAlertKey)
			if priceSinceLastBuyAlert == 0 || priceSinceLastBuyAlert > aiStock.RecommendBuyPriceMin {
				title := fmt.Sprintf("【买入预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议买入价**: %.2f - %.2f\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendBuyPriceMin, aiStock.RecommendBuyPriceMax,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议买入价: %.2f-%.2f",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendBuyPriceMin, aiStock.RecommendBuyPriceMax)
				if a.canSendAlert(buyAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("Blades Stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "blades-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(buyAlertKey)
					a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
			}
		} else {
			priceSinceLastBuyAlert := a.getPriceAtAlertReset(buyAlertKey)
			if currentPrice > aiStock.RecommendBuyPriceMin && (priceSinceLastBuyAlert == 0 || currentPrice > priceSinceLastBuyAlert) {
				a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
			}
		}

		profitAlertKey := baseAlertKey + ":PROFIT"
		if aiStock.RecommendStopProfitPriceMin > 0 && currentPrice >= aiStock.RecommendStopProfitPriceMin {
			priceSinceLastProfitAlert := a.getPriceAtAlertReset(profitAlertKey)
			if priceSinceLastProfitAlert == 0 || priceSinceLastProfitAlert < aiStock.RecommendStopProfitPriceMin {
				title := fmt.Sprintf("【止盈预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议止盈价**: %.2f - %.2f\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopProfitPriceMin, aiStock.RecommendStopProfitPriceMax,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议止盈价: %.2f-%.2f",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopProfitPriceMin, aiStock.RecommendStopProfitPriceMax)
				if a.canSendAlert(profitAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("Blades Stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "blades-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(profitAlertKey)
					a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
			}
		} else {
			priceSinceLastProfitAlert := a.getPriceAtAlertReset(profitAlertKey)
			if currentPrice < aiStock.RecommendStopProfitPriceMin && (priceSinceLastProfitAlert == 0 || currentPrice < priceSinceLastProfitAlert) {
				a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
			}
		}

		stopLossAlertKey := baseAlertKey + ":LOSS"
		stopLossPrice, _ := convertor.ToFloat(aiStock.RecommendStopLossPrice)
		if stopLossPrice > 0 && currentPrice <= stopLossPrice {
			priceSinceLastLossAlert := a.getPriceAtAlertReset(stopLossAlertKey)
			if priceSinceLastLossAlert == 0 || priceSinceLastLossAlert > stopLossPrice {
				title := fmt.Sprintf("【止损预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议止损价**: %s\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopLossPrice,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议止损价: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopLossPrice)
				if a.canSendAlert(stopLossAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("Blades Stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "blades-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(stopLossAlertKey)
					a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
			}
		} else {
			priceSinceLastLossAlert := a.getPriceAtAlertReset(stopLossAlertKey)
			if currentPrice > stopLossPrice && (priceSinceLastLossAlert == 0 || currentPrice > priceSinceLastLossAlert) {
				a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
			}
		}
	}
}

// MonitorFollowedStockCostPrices 监控自选股的持仓成本价，当股价低于成本价时发送预警
func MonitorFollowedStockCostPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过自选股成本价监控")
		return
	}

	var followedStocks []data.FollowedStock
	db.Dao.Model(&data.FollowedStock{}).Where("cost_price > 0").Find(&followedStocks)

	if len(followedStocks) == 0 {
		return
	}

	stockCodes := make([]string, 0)
	stockMap := make(map[string]*data.FollowedStock)
	for i := range followedStocks {
		stock := &followedStocks[i]
		stockCodes = append(stockCodes, tools.GetStockCode(stock.StockCode))
		stockMap[tools.GetStockCode(stock.StockCode)] = stock
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取自选股实时数据失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		followedStock, ok := stockMap[tools.GetStockCode(stockInfo.Code)]
		if !ok {
			continue
		}

		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}

		costPrice := followedStock.CostPrice
		if costPrice <= 0 {
			continue
		}

		alertKey := fmt.Sprintf("COST:%s:%s", followedStock.StockCode, followedStock.Time.Format("20060102"))

		if currentPrice < costPrice {
			priceSinceLastAlert := a.getPriceAtAlertReset(alertKey)
			if priceSinceLastAlert == 0 || priceSinceLastAlert >= costPrice {
				dropPercent := ((costPrice - currentPrice) / costPrice) * 100
				title := fmt.Sprintf("【成本价预警】%s", followedStock.Name)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **持仓成本价**: %.2f\n- **亏损比例**: %.2f%%\n- **关注时间**: %s",
					followedStock.Name, followedStock.StockCode, currentPrice, costPrice, dropPercent,
					followedStock.Time.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n成本价: %.2f\n亏损: %.2f%%",
					followedStock.Name, followedStock.StockCode, currentPrice, costPrice, dropPercent)
				if a.canSendAlert(alertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("Blades Stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "blades-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(alertKey)
					a.updatePriceAtAlertReset(alertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(alertKey, currentPrice)
			}
		} else {
			priceSinceLastAlert := a.getPriceAtAlertReset(alertKey)
			if currentPrice >= costPrice && (priceSinceLastAlert == 0 || currentPrice < priceSinceLastAlert) {
				a.updatePriceAtAlertReset(alertKey, currentPrice)
			}
		}
	}
}

// canSendAlert 检查是否可以发送预警，避免重复发送
// alertKey: 预警的唯一标识
// interval: 发送间隔
// 返回 true 表示可以发送，false 表示需要在间隔后才能发送
func (a *App) canSendAlert(alertKey string, interval time.Duration) bool {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()

	lastSent, exists := a.stockAlertLastSent[alertKey]
	if !exists {
		return true
	}

	return time.Since(lastSent) >= interval
}

// updateAlertSentTime 更新预警发送时间
func (a *App) updateAlertSentTime(alertKey string) {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	a.stockAlertLastSent[alertKey] = time.Now()
}

// getPriceAtAlertReset 获取预警重置后的价格（用于判断是否需要重新触发预警）
func (a *App) getPriceAtAlertReset(alertKey string) float64 {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	return a.priceAtAlertReset[alertKey]
}

// updatePriceAtAlertReset 更新预警重置后的价格
func (a *App) updatePriceAtAlertReset(alertKey string, price float64) {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	a.priceAtAlertReset[alertKey] = price
}

func GetStockInfos(follows ...data.FollowedStock) *[]data.StockInfo {
	stockInfos := make([]data.StockInfo, 0)
	stockCodes := make([]string, 0)
	for _, follow := range follows {
		if strutil.HasPrefixAny(follow.StockCode, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
			continue
		}
		stockCodes = append(stockCodes, follow.StockCode)
	}
	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil {
		return &stockInfos
	}
	for _, info := range *stockData {
		v, ok := slice.FindBy(follows, func(idx int, follow data.FollowedStock) bool {
			if strutil.HasPrefixAny(follow.StockCode, []string{"US", "us"}) {
				return strings.ToLower(strings.Replace(follow.StockCode, "us", "gb_", 1)) == info.Code
			}

			return follow.StockCode == info.Code
		})
		if ok {
			addStockFollowData(v, &info)
			stockInfos = append(stockInfos, info)
		}
	}
	return &stockInfos
}
func getStockInfo(follow data.FollowedStock) *data.StockInfo {
	stockCode := follow.StockCode
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
		return &data.StockInfo{}
	}
	stockData := (*stockDatas)[0]
	addStockFollowData(follow, &stockData)
	return &stockData
}

func addStockFollowData(follow data.FollowedStock, stockData *data.StockInfo) {
	stockData.PrePrice = follow.Price //上次当前价格
	stockData.Sort = follow.Sort
	stockData.CostPrice = follow.CostPrice //成本价
	stockData.CostVolume = follow.Volume   //成本量
	stockData.AlarmChangePercent = follow.AlarmChangePercent
	stockData.AlarmPrice = follow.AlarmPrice
	stockData.Groups = follow.Groups

	//当前价格
	price, _ := convertor.ToFloat(stockData.Price)
	//当前价格为0 时 使用卖一价格作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.A1P)
	}
	//当前价格依然为0 时 使用买一报价作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.B1P)
	}

	//昨日收盘价
	preClosePrice, _ := convertor.ToFloat(stockData.PreClose)

	//当前价格依然为0 时 使用昨日收盘价为当前价格
	if price == 0 {
		price = preClosePrice
	}

	//今日最高价
	highPrice, _ := convertor.ToFloat(stockData.High)
	if highPrice == 0 {
		highPrice, _ = convertor.ToFloat(stockData.Open)
	}

	//今日最低价
	lowPrice, _ := convertor.ToFloat(stockData.Low)
	if lowPrice == 0 {
		lowPrice, _ = convertor.ToFloat(stockData.Open)
	}
	//开盘价
	//openPrice, _ := convertor.ToFloat(stockData.Open)

	if price > 0 && preClosePrice > 0 {
		stockData.ChangePrice = mathutil.RoundToFloat(price-preClosePrice, 2)
		stockData.ChangePercent = mathutil.RoundToFloat(mathutil.Div(price-preClosePrice, preClosePrice)*100, 3)
	}
	if highPrice > 0 && preClosePrice > 0 {
		stockData.HighRate = mathutil.RoundToFloat(mathutil.Div(highPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if lowPrice > 0 && preClosePrice > 0 {
		stockData.LowRate = mathutil.RoundToFloat(mathutil.Div(lowPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if follow.CostPrice > 0 && follow.Volume > 0 {
		if price > 0 {
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(price-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((price-follow.CostPrice)*float64(follow.Volume), 2)
			stockData.ProfitAmountToday = mathutil.RoundToFloat((price-preClosePrice)*float64(follow.Volume), 2)
		} else {
			//未开盘时当前价格为昨日收盘价
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(preClosePrice-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((preClosePrice-follow.CostPrice)*float64(follow.Volume), 2)
			// 未开盘时，今日盈亏为 0
			stockData.ProfitAmountToday = 0
		}

	}

	//logger.SugaredLogger.Debugf("stockData:%+v", stockData)
	if follow.Price != price && price > 0 {
		go db.Dao.Model(follow).Where("stock_code = ?", follow.StockCode).Updates(map[string]interface{}{
			"price": price,
		})
	}
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	// 窗口大小已在 beforeClose 中保存（app_windows.go / app_linux.go）。
	// 这里不再调用 runtime.WindowGetSize —— 在 Wails 内部控件销毁阶段调用
	// 会在 winc.ScaleToDefaultDPI 触发 panic，导致关窗卡死 / 进程残留。
	if a.cron != nil {
		a.cron.Stop()
	}
	if db.Dao != nil {
		if err := db.Dao.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
			logger.SugaredLogger.Warnf("shutdown WAL checkpoint error: %v", err)
		} else {
			logger.SugaredLogger.Info("shutdown WAL checkpoint ok")
		}
	}
}

// BatchGreet returns stock info for multiple stock codes in a single call
func (a *App) BatchGreet(stockCodes []string) []*data.StockInfo {
	return a.marketService.BatchGreet(stockCodes)
}

// Greet returns a greeting for the given name
func (a *App) Greet(stockCode string) *data.StockInfo {
	return a.marketService.Greet(stockCode)
}

func (a *App) Follow(stockCode string) string {
	return a.marketService.Follow(stockCode)
}

func (a *App) UnFollow(stockCode string) string {
	return a.marketService.UnFollow(stockCode)
}

func (a *App) GetFollowList(groupId int) *[]data.FollowedStock {
	return a.marketService.GetFollowList(groupId)
}

func (a *App) GetStockList(key string) []data.StockBasic {
	return a.marketService.GetStockList(key)
}

func (a *App) SetCostPriceAndVolume(stockCode string, price float64, volume int64) string {
	return a.marketService.SetCostPriceAndVolume(stockCode, price, volume)
}

func (a *App) SetTradingPrice(stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string {
	return a.marketService.SetTradingPrice(stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice)
}

func (a *App) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	return a.marketService.SetAlarmChangePercent(val, alarmPrice, stockCode)
}
func (a *App) SetStockSort(sort int64, stockCode string) {
	a.marketService.SetStockSort(sort, stockCode)
}
func (a *App) SendDingDingMessage(message string, stockCode string) string {
	ttl, _ := a.cache.TTL([]byte(stockCode))
	//logger.SugaredLogger.Infof("stockCode %s ttl:%d", stockCode, ttl)
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), 60*5)
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	return data.NewDingDingAPI().SendDingDingMessage(message)
}

// SendDingDingMessageByType msgType 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警
func (a *App) SendDingDingMessageByType(message string, stockCode string, msgType int) string {

	if strutil.HasPrefixAny(stockCode, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
		return "非A股交易时间"
	}
	if strutil.HasPrefixAny(stockCode, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
		return "非港股交易时间"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
		return "非美股交易时间"
	}

	ttl, _ := a.cache.TTL([]byte(stockCode))
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), getMsgTypeTTL(msgType))
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	stockInfo := &data.StockInfo{}
	db.Dao.Model(stockInfo).Where("code = ?", stockCode).First(stockInfo)
	go data.NewAlertWindowsApi("Blades Stock消息通知", getMsgTypeName(msgType), GenNotificationMsg(stockInfo), "").SendNotification()

	go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
		"time":    "📈 " + getMsgTypeName(msgType),
		"isRed":   true,
		"source":  "blades-stock",
		"content": GenNotificationMsg(stockInfo),
	})

	return data.NewDingDingAPI().SendDingDingMessage(message)
}

// preFetchStockAnalysisData collects real-time market data for the given stock
// and returns a formatted reference string for the agent's system prompt.
// isETFRecommendQuestion 判断用户问题是否属于 ETF/基金推荐、轮动、板块资金类问询。
// 这类问题与当前页面个股无关，注入该个股预采集数据会误导模型偏离 ETF 分析框架。
func isETFRecommendQuestion(q string) bool {
	if strings.TrimSpace(q) == "" {
		return false
	}
	lower := strings.ToLower(q)
	etfHints := []string{"ETF", "etf", "基金", "板块", "轮动", "北向资金", "资金流向", "热点", "大盘", "指数"}
	for _, hint := range etfHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func preFetchStockAnalysisData(stockCode string) string {
	if stockCode == "" {
		return ""
	}
	api := data.NewStockDataApi()
	var parts []string

	// Real-time quote
	if info, err := api.GetStockCodeRealTimeData(stockCode); err == nil && info != nil && len(*info) > 0 {
		s := (*info)[0]
		changePercent := s.ChangePercent
		changePrice := s.ChangePrice
		parts = append(parts, fmt.Sprintf("【实时行情】%s(%s) 当前价:%s 涨跌幅:%.2f%% 涨跌额:%.2f 开盘:%s 最高:%s 最低:%s 昨收:%s 成交量:%s 成交额:%s",
			s.Name, s.Code, s.Price, changePercent, changePrice, s.Open, s.High, s.Low, s.PreClose, s.Volume, s.Amount))
	}

	// Latest financial data
	financeMd := api.GetStockLatestFinanceToMarkdown(stockCode)
	if financeMd != "" && !strings.HasPrefix(financeMd, "获取") {
		parts = append(parts, financeMd)
	}

	// Quarterly financial indicators (ROE, gross margin, etc.)
	qtrMd := api.GetStockQtrMainFinanceToMarkdown(stockCode)
	if qtrMd != "" && !strings.HasPrefix(qtrMd, "获取") {
		parts = append(parts, qtrMd)
	}

	// Valuation percentile (PE/PB percentile rank)
	valMd := api.GetStockValuationPercentileToMarkdown(stockCode)
	if valMd != "" && !strings.HasPrefix(valMd, "获取") {
		parts = append(parts, valMd)
	}

	// Institutional prediction summary
	predMd := api.GetStockPredictSummaryToMarkdown(stockCode)
	if predMd != "" && !strings.HasPrefix(predMd, "获取") {
		parts = append(parts, predMd)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

func (a *App) NewChatStream(stock, stockCode, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool) {
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Errorf("NewChatStream panic: %v", err)
			runtime.EventsEmit(a.ctx, "newChatStream", map[string]any{
				"code":    0,
				"content": fmt.Sprintf("AI分析异常: %v", err),
			})
			runtime.EventsEmit(a.ctx, "newChatStream", "DONE")
		}
	}()
	analysisQuestion := strings.TrimSpace(question)

	// 当用户问题涉及 ETF/基金推荐等非个股问询时，不注入当前页面的个股预采集数据，
	// 否则模型会被强制引导到当前股票（如 TCL科技）的个股研判，而不是执行 ETF 轮动 skill。
	isETFOriented := isETFRecommendQuestion(analysisQuestion)

	// Pre-fetch real-time stock data as reference context for the agent
	var preFetchCtx string
	if !isETFOriented {
		preFetchCtx = preFetchStockAnalysisData(stockCode)
	}

	if analysisQuestion == "" {
		analysisQuestion = fmt.Sprintf("请对%s(%s)进行全方位深度分析，从技术面、基本面、消息面、资金面四个维度综合研判，给出明确的多空判断和操作建议。", stock, stockCode)
	}
	if preFetchCtx != "" {
		analysisQuestion += "\n\n【预采集参考数据】以下数据由系统预先采集，是真实数据，可直接引用分析：\n" + preFetchCtx + "\n\n【重要】严禁编造或推算PE/PB/ROE/毛利率/营收/利润/资金流向/均线等任何数字指标。所有数据须来自预采集数据或工具返回结果。不确定的指标请明确说'数据不可得'，不要自行估算。"
	}

	agentMode := string(agent.AgentModePlanExecute)
	if !enableTools || (strings.TrimSpace(question) == "" && preFetchCtx != "") {
		agentMode = string(agent.AgentModeDirect)
	}

	sysPrompt := ""
	if agentMode == string(agent.AgentModeDirect) {
		sysPrompt = "你现在是顶级证券投资顾问与量化宏观专家“财智”。所有核心客观盘面数据、估值百分位与详尽财务指标已在用户输入中完整提供，无需额外调用外部工具。请直接基于输入中的真实数据，从技术面、基本面、消息面、资金面四个维度输出结构清晰、多空判断明确的深度研报，严禁在正文中生成任何伪造的工具调用标签。"
	}

	for msg := range agent.NewStockAiAgentApi().ChatWithContext(a.ctx, analysisQuestion, aiConfigId, sysPromptId, true, 5, think, agentMode, sysPrompt) {
		if msg == nil {
			continue
		}
		if msg.Content == "agent-DONE" {
			continue
		}
		runtime.EventsEmit(a.ctx, "newChatStream", map[string]any{
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		})
	}
	runtime.EventsEmit(a.ctx, "newChatStream", "DONE")
}

func (a *App) SaveAIResponseResult(stockCode, stockName, result, chatId, question string, aiConfigId int) {
	a.agentService.SaveAIResponseResult(a.ctx, stockCode, stockName, result, chatId, question, aiConfigId)
}

// GetSystemStateSnapshot 获取当前系统的全局态势感知快照
func (a *App) GetSystemStateSnapshot() *agent.SystemStateSnapshot {
	return a.agentService.GetSystemStateSnapshot()
}

// GetToolQuotaStatus 获取大模型工具调用成本预算与配额使用状态
func (a *App) GetToolQuotaStatus() agent.QuotaStatus {
	return a.agentService.GetToolQuotaStatus()
}

// ResetToolDailyQuota 手动重置今日工具调用配额与成本计数
func (a *App) ResetToolDailyQuota() string {
	return a.agentService.ResetToolDailyQuota()
}

// SyncRecommendStocksFromIntelligence 从最新研报、ETF动量池及历史深度分析中一键提炼推荐股票
func (a *App) SyncRecommendStocksFromIntelligence() (int, error) {
	return a.agentService.SyncRecommendStocksFromIntelligence(a.ctx)
}

func (a *App) DiagnoseStock(stockCode string) (*quant.StockDiagnosisResult, error) {
	return a.quantService.DiagnoseStock(stockCode)
}

// TriggerAIReportBacktest 手动或自动触发 AI 报告历史回测与胜率检验
func (a *App) TriggerAIReportBacktest(limit int) (*quant.AIReportBacktestResult, error) {
	return a.quantService.TriggerAIReportBacktest(limit)
}

// RunSkillABTest runs an A/B test between two skills.
func (a *App) RunSkillABTest(skillID1, skillID2 uint, limit int) (*quant.SkillABTestResult, error) {
	return a.quantService.RunSkillABTest(skillID1, skillID2, limit)
}

// GetAuditLogList 分页获取审计日志
func (a *App) GetAuditLogList(query models.AuditLogQuery) (*models.AuditLogPageResp, error) {
	return a.settingsService.GetAuditLogList(query)
}

// ExportAuditLogCSV 导出审计日志为 CSV 字符串
func (a *App) ExportAuditLogCSV(query models.AuditLogQuery) (string, error) {
	return a.settingsService.ExportAuditLogCSV(query)
}

func (a *App) GetVersionInfo() *models.VersionInfo {
	return &models.VersionInfo{
		Version:           Version,
		Icon:              GetImageBase(icon),
		Alipay:            GetImageBase(alipay),
		Wxpay:             GetImageBase(wxpay),
		Content:           VersionCommit,
		OfficialStatement: OFFICIAL_STATEMENT,
	}
}

func (a *App) GetUserManual() string {
	return string(userManual)
}

//// checkChromeOnWindows 在 Windows 系统上检查谷歌浏览器是否安装
//func checkChromeOnWindows() bool {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	_, _, err = key.GetValue("Path", nil)
//	return err == nil
//}
//
//// checkEdgeOnWindows 在 Windows 系统上检查Edge浏览器是否安装，并返回安装路径
//func checkEdgeOnWindows() (string, bool) {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return "", false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	path, _, err := key.GetStringValue("Path")
//	if err != nil {
//		return "", false
//	}
//	return path, true
//}

func GetImageBase(bytes []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(bytes)
}

func GenNotificationMsg(stockInfo *data.StockInfo) string {
	Price, err := convertor.ToFloat(stockInfo.Price)
	if err != nil {
		Price = 0
	}
	PreClose, err := convertor.ToFloat(stockInfo.PreClose)
	if err != nil {
		PreClose = 0
	}
	var RF float64
	if PreClose > 0 {
		RF = mathutil.RoundToFloat(((Price-PreClose)/PreClose)*100, 2)
	}

	return "[" + stockInfo.Name + "] " + stockInfo.Price + " " + convertor.ToString(RF) + "% " + stockInfo.Date + " " + stockInfo.Time
}

// msgType : 1 涨跌报警(5分钟);2 股价报警(30分钟) 3 成本价报警(30分钟) 4 止盈报警(5分钟) 5 止损报警(5分钟)
func getMsgTypeTTL(msgType int) int {
	switch msgType {
	case 1:
		return 60 * 5
	case 2:
		return 60 * 30
	case 3:
		return 60 * 30
	case 4:
		return 60 * 5
	case 5:
		return 60 * 5
	default:
		return 60 * 5
	}
}

func getMsgTypeName(msgType int) string {
	switch msgType {
	case 1:
		return "涨跌报警"
	case 2:
		return "股价报警"
	case 3:
		return "成本价报警"
	case 4:
		return "止盈报警"
	case 5:
		return "止损报警"
	default:
		return "未知类型"
	}
}

func (a *App) UpdateConfig(settingConfig *data.SettingConfig) string {
	//s1, _ := json.Marshal(settingConfig)
	//logger.SugaredLogger.Infof("UpdateConfig:%s", s1)
	if settingConfig.RefreshInterval > 0 {
		if entryID, exists := a.getCronEntry("MonitorStockPrices"); exists {
			a.cron.Remove(entryID)
		}
		id, _ := a.cron.AddFunc(fmt.Sprintf("@every %ds", settingConfig.RefreshInterval), func() {
			MonitorStockPrices(a)
		})
		a.setCronEntry("MonitorStockPrices", id)
	}

	return data.UpdateConfig(settingConfig)
}

func (a *App) GetConfig() *data.SettingConfig {
	return a.settingsService.GetConfig()
}

// GetAiConfigTemplates 返回预设的 AI 配置模板，供前端向导使用
func (a *App) GetAiConfigTemplates() []*data.AiConfigTemplate {
	return a.settingsService.GetAiConfigTemplates()
}

func (a *App) ExportConfig() string {
	config := data.NewSettingsApi().Export()
	file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "导出配置文件",
		CanCreateDirectories: true,
		DefaultFilename:      "config.json",
	})
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", err.Error())
		return err.Error()
	}
	err = os.WriteFile(file, []byte(config), os.ModePerm)
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", err.Error())
		return err.Error()
	}
	return "导出成功:" + file
}

// CleanupData 暴露给前端的清理入口
func (a *App) CleanupData(cleanupType string) string {
	return a.settingsService.CleanupData(cleanupType)
}

// BackupData 暴露给前端的数据备份入口 (兼容旧版 CSV 导出)
func (a *App) BackupData(backupType string) string {
	return a.settingsService.BackupData(backupType)
}

// RestoreData 暴露给前端的恢复入口 (兼容旧版 CSV 恢复)
func (a *App) RestoreData(backupType string) string {
	return a.settingsService.RestoreData(backupType)
}

// GetBackupOverview 获取系统数据资产概览
func (a *App) GetBackupOverview() (*data.BackupOverview, error) {
	return a.settingsService.GetBackupOverview()
}

// ListBackups 列出所有历史备份归档
func (a *App) ListBackups() ([]data.BackupFileItem, error) {
	return a.settingsService.ListBackups()
}

// CreateIntelligentBackup 一键创建全量智能资产备份包
func (a *App) CreateIntelligentBackup(note string) string {
	return a.settingsService.CreateIntelligentBackup(note)
}

// CreateDatabaseSnapshot 一键创建底层数据库物理快照
func (a *App) CreateDatabaseSnapshot() string {
	return a.settingsService.CreateDatabaseSnapshot()
}

// RestoreBackupFile 精确恢复指定文件名的备份
func (a *App) RestoreBackupFile(fileName string) string {
	return a.settingsService.RestoreBackupFile(fileName)
}

// DeleteBackupFile 删除指定的历史备份文件
func (a *App) DeleteBackupFile(fileName string) string {
	return a.settingsService.DeleteBackupFile(fileName)
}

// GetBackupContent 获取备份文件内容（用于前端直接下载保存）
func (a *App) GetBackupContent(fileName string) string {
	return a.settingsService.GetBackupContent(fileName)
}

// ImportIntelligentBackup 导入全量智能资产包 (JSON) 并安全合并写入
func (a *App) ImportIntelligentBackup(content string) string {
	return a.settingsService.ImportIntelligentBackup(content)
}

func (a *App) ShareAnalysis(stockCode, stockName string) string {
	return a.settingsService.ShareAnalysis(stockCode, stockName)
}

// ShareText 直接把文本分享到社区（已废弃：本项目无社区服务）
func (a *App) ShareText(text, title string) string {
	return a.settingsService.ShareText(text, title)
}

func (a *App) GetfundList(key string) []data.FundBasic {
	return a.marketService.GetfundList(key)
}
func (a *App) GetFollowedFund() []data.FollowedFund {
	return a.marketService.GetFollowedFund()
}
func (a *App) FollowFund(fundCode string) string {
	return a.marketService.FollowFund(fundCode)
}
func (a *App) UnFollowFund(fundCode string) string {
	return a.marketService.UnFollowFund(fundCode)
}
func (a *App) GetFundKLine(fundCode string, klt string, limit int) *data.KLineSourceResult {
	return a.marketService.GetFundKLine(fundCode, klt, limit)
}
func (a *App) GetFundHistoryNetValue(fundCode string, pageSize int, startDate string, endDate string) []data.FundHistoryNetValue {
	return a.marketService.GetFundHistoryNetValue(fundCode, pageSize, startDate, endDate)
}
func (a *App) GetFundTop10Holdings(fundCode string) []data.FundHoldingStock {
	return a.marketService.GetFundTop10Holdings(fundCode)
}
func (a *App) GetFundRanking(marketType, fundType, sortField, sortOrder string, pageIndex, pageSize int) *data.FundRankingResult {
	return a.marketService.GetFundRanking(marketType, fundType, sortField, sortOrder, pageIndex, pageSize)
}
func (a *App) SearchFundCodes(keyword string) []data.FundSearchItem {
	return a.marketService.SearchFundCodes(keyword)
}
func (a *App) GetFollowedFundPaged(pageIndex, pageSize int, keyword string) *data.FollowedFundPagedResult {
	return a.marketService.GetFollowedFundPaged(pageIndex, pageSize, keyword)
}
func (a *App) SaveAsMarkdown(stockCode, stockName string) string {
	res := data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stockCode)
	if res != nil && len(res.Content) > 100 {
		analysisTime := res.CreatedAt.Format("2006-01-02_15_04_05")
		file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "保存为Markdown",
			DefaultFilename: fmt.Sprintf("%s[%s]AI分析结果_%s.md", stockName, stockCode, analysisTime),
			Filters: []runtime.FileFilter{
				{
					DisplayName: "Markdown",
					Pattern:     "*.md;*.markdown",
				},
			},
		})
		if err != nil {
			return err.Error()
		}
		if file == "" {
			return "取消保存"
		}
		if err = os.WriteFile(file, []byte(res.Content), 0644); err != nil {
			return "保存失败：" + err.Error()
		}
		return "已保存至：" + file
	}
	return "分析结果异常,无法保存。"
}

func (a *App) SetStockAICron(cronText, stockCode string) {
	data.NewStockDataApi().SetStockAICron(cronText, stockCode)
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	if entryID, exists := a.getCronEntry(stockCode); exists {
		a.cron.Remove(entryID)
	}
	follow := data.NewStockDataApi().GetFollowedStockByStockCode(stockCode)
	id, _ := a.cron.AddFunc(cronText, a.AddCronTask(follow))
	a.setCronEntry(stockCode, id)

}
func (a *App) AddGroup(group data.Group) string {
	return a.marketService.AddGroup(group)
}
func (a *App) GetGroupList() []data.Group {
	return a.marketService.GetGroupList()
}

func (a *App) UpdateGroupSort(id int, newSort int) bool {
	return a.marketService.UpdateGroupSort(id, newSort)
}

// UpdateGroup 修改分组名称
func (a *App) UpdateGroup(id int, name string) string {
	return a.marketService.UpdateGroup(id, name)
}

func (a *App) InitializeGroupSort() bool {
	return a.marketService.InitializeGroupSort()
}

func (a *App) GetGroupStockList(groupId int) []data.GroupStock {
	return a.marketService.GetGroupStockList(groupId)
}

func (a *App) AddStockGroup(groupId int, stockCode string) string {
	return a.marketService.AddStockGroup(groupId, stockCode)
}

func (a *App) RemoveStockGroup(code, name string, groupId int) string {
	return a.marketService.RemoveStockGroup(code, name, groupId)
}

func (a *App) RemoveGroup(groupId int) string {
	return a.marketService.RemoveGroup(groupId)
}

func (a *App) GetStockKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	return a.marketService.GetStockKLine(stockCode, stockName, days)
}

func (a *App) GetStockMinutePriceLineData(stockCode, stockName string) map[string]any {
	return a.marketService.GetStockMinutePriceLineData(stockCode, stockName)
}

func (a *App) GetStockCommonKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	return a.marketService.GetStockCommonKLine(stockCode, stockName, days)
}

func (a *App) GetStockEastMoneyKLine(stockCode, stockName string, klt string, limit int) *[]data.KLineData {
	return a.marketService.GetStockEastMoneyKLine(stockCode, stockName, klt, limit)
}

func (a *App) GetStockEastMoneyKLinePage(stockCode, stockName string, klt string, limit int, end string) *[]data.KLineData {
	return a.marketService.GetStockEastMoneyKLinePage(stockCode, stockName, klt, limit, end)
}

func (a *App) GetStockKLineWithFallback(stockCode, stockName string, klt string, limit int) *data.KLineSourceResult {
	return a.marketService.GetStockKLineWithFallback(stockCode, stockName, klt, limit)
}

func (a *App) GetStockKLinePageWithFallback(stockCode, stockName string, klt string, limit int, end string) *data.KLineSourceResult {
	return a.marketService.GetStockKLinePageWithFallback(stockCode, stockName, klt, limit, end)
}

func (a *App) GetChipDistribution(stockCode string, days int, bins int, adjustFlag string) (*data.ChipDistributionResult, error) {
	return a.marketService.GetChipDistribution(stockCode, days, bins, adjustFlag)
}

func (a *App) GetTdxCallAuction(stockCode string, start uint32, count uint32) *[]data.TdxCallAuctionData {
	return a.marketService.GetTdxCallAuction(stockCode, start, count)
}

func (a *App) GetTdxCompanyInfo(stockCode string) *data.TdxCompanyInfoBundle {
	return a.marketService.GetTdxCompanyInfo(stockCode)
}

func (a *App) GetTdxFinanceInfo(stockCode string) *data.TdxFinanceInfo {
	return a.marketService.GetTdxFinanceInfo(stockCode)
}

func (a *App) GetTdxXDXRInfo(stockCode string) *[]data.TdxXDXRItem {
	return a.marketService.GetTdxXDXRInfo(stockCode)
}

func (a *App) GetTdxCompanyCategoryList(stockCode string) *[]data.TdxCompanyCategory {
	return a.marketService.GetTdxCompanyCategoryList(stockCode)
}

func (a *App) GetTdxCompanyCategoryContent(stockCode string, categoryName string) *data.TdxCompanyInfoSection {
	return a.marketService.GetTdxCompanyCategoryContent(stockCode, categoryName)
}

func (a *App) GetTdxSymbolBelongBoard(stockCode string) *[]data.MACBelongBoardItem {
	return a.marketService.GetTdxSymbolBelongBoard(stockCode)
}

func (a *App) GetTelegraphList(source string) *[]*models.Telegraph {
	return a.marketService.GetTelegraphList(source)
}

func (a *App) ReFleshTelegraphList(source string) *[]*models.Telegraph {
	return a.marketService.ReFleshTelegraphList(source)
}

func (a *App) GlobalStockIndexes() map[string]any {
	return a.marketService.GlobalStockIndexes()
}

func (a *App) GlobalStockIndexesReadable() string {
	return a.marketService.GlobalStockIndexesReadable()
}

func (a *App) SummaryStockNews(question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, eventName string, historyJSON string) {
	ctx, cancel := context.WithCancel(a.ctx)

	a.summaryMu.Lock()
	if a.summaryCancel != nil {
		a.summaryCancel()
	}
	a.summaryCancel = cancel
	a.summaryMu.Unlock()

	if strings.TrimSpace(eventName) == "" {
		eventName = "summaryStockNews"
	}

	// Pre-fetch market index data as reference context
	preFetchCtx := preFetchMarketSummaryData()
	if preFetchCtx != "" {
		preFetchCtx = "\n\n【预采集参考数据】以下为当前市场概况，可供直接引用：\n" + preFetchCtx
	}

	q := strings.TrimSpace(question)
	if q == "" {
		q = `请对当前A股市场进行全面的AI资讯总结和分析。

## 执行步骤
### 第1步：获取市场整体数据
- 用 GetMarketData 获取大盘指数行情、涨跌家数、涨跌分布
- 用 GlobalStockIndexesReadable 获取全球主要指数表现
- 用 GetStockChanges 获取今日异动数据（火箭发射、大笔买入等）

### 第2步：捕捉市场热点
- 用 GetUplimitHotPlates 获取今日涨停板块热度排名
- 用 HotStockTable 获取热门股票排名
- 用 GetUplimitHotStocks 获取涨停个股列表
- 用 HotspotDiscovery 发现当前市场热点题材

### 第3步：消息面分析
- 用 GetNewsListData 获取当天重要财经新闻
- 用 GetWallstreetcnLives 获取华尔街见闻7x24快讯
- 用 WebSearch 搜索最新的重要政策/行业消息

### 第4步：资金面分析
- 用 GetStockMoneyData 查看个股资金流入排名
- 用 GetIndustryMoneyRank 查看行业资金流向
- 用 GetMutualTop10Deal 查看北向资金动向

### 第5步：综合分析
- 当前市场整体趋势判断（多头/空头/震荡）
- 主线热点板块和持续性分析
- 资金偏好方向（大盘/小盘，价值/成长）
- 潜在风险和关注点
- 建议关注的方向/板块/个股

注意：每一步都必须调用对应的工具获取最新数据，严禁凭记忆编造。即使某些工具调用失败，也要基于已成功获取的数据进行分析，不要反复追问用户补充信息。` + preFetchCtx
	} else {
		q = q + preFetchCtx
	}

	for msg := range agent.NewStockAiAgentApi().ChatWithContext(ctx, q, aiConfigId, sysPromptId, false, 0, think, "plan_execute") {
		if msg == nil {
			continue
		}
		if msg.Content == "agent-DONE" {
			continue
		}
		runtime.EventsEmit(a.ctx, eventName, map[string]any{
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		})
	}

	a.summaryMu.Lock()
	a.summaryCancel = nil
	a.summaryMu.Unlock()

	runtime.EventsEmit(a.ctx, eventName, "DONE")
}

// preFetchMarketSummaryData collects macro-level market data for the AI market summary feature.
func preFetchMarketSummaryData() string {
	api := data.NewMarketNewsApi()
	var parts []string

	// Global indexes
	indexes := api.GlobalStockIndexesReadable(30)
	if indexes != "" {
		parts = append(parts, indexes)
	}

	return strings.Join(parts, "\n\n")
}
func (a *App) GetIndustryRank(sort string, cnt int) []any {
	return a.marketService.GetIndustryRank(sort, cnt)
}
func (a *App) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	return a.marketService.GetIndustryMoneyRankSina(fenlei, sort)
}
func (a *App) GetMoneyRankSina(sort string) []map[string]any {
	return a.marketService.GetMoneyRankSina(sort)
}

func (a *App) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	return a.marketService.GetStockMoneyTrendByDay(stockCode, days)
}

// OpenURL
//
//	@Description:  跨平台打开默认浏览器
//	@receiver a
//	@param url
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// SaveImage
//
//	@Description: 跨平台保存图片
//	@receiver a
//	@param name
//	@param base64Data
//	@return error
func (a *App) SaveImage(name, base64Data string) string {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存图片",
		DefaultFilename: name + "AI分析.png",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PNG 图片",
				Pattern:     "*.png",
			},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	base64Data = strings.ReplaceAll(base64Data, " ", "+")
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	} else if idx := strings.Index(base64Data, "base64,"); idx != -1 {
		base64Data = base64Data[idx+7:]
	} else if strings.HasPrefix(base64Data, "data:") {
		if commaIdx := strings.Index(base64Data, ","); commaIdx != -1 {
			base64Data = base64Data[commaIdx+1:]
		}
	}
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		decodeString, err = base64.RawStdEncoding.DecodeString(base64Data)
	}
	if err != nil {
		return "文件内容异常,无法保存。" + err.Error()
	}

	err = os.WriteFile(filepath.Clean(filePath), decodeString, os.ModePerm)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

// SaveWordFile
//
//	@Description: // 跨平台保存word
//	@receiver a
//	@param filename
//	@param base64Data
//	@return error
func (a *App) SaveWordFile(filename string, base64Data string) string {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 Word 文件",
		DefaultFilename: filename,
		Filters: []runtime.FileFilter{
			{DisplayName: "Word 文件", Pattern: "*.docx"},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	base64Data = strings.ReplaceAll(base64Data, " ", "+")
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	} else if idx := strings.Index(base64Data, "base64,"); idx != -1 {
		base64Data = base64Data[idx+7:]
	} else if strings.HasPrefix(base64Data, "data:") {
		if commaIdx := strings.Index(base64Data, ","); commaIdx != -1 {
			base64Data = base64Data[commaIdx+1:]
		}
	}
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		decodeString, err = base64.RawStdEncoding.DecodeString(base64Data)
	}
	if err != nil {
		return "文件内容异常,无法保存。" + err.Error()
	}
	err = os.WriteFile(filepath.Clean(filePath), decodeString, 0777)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

// GetAiConfigs
//
//	@Description: // 获取 AiConfig 列表
//	@receiver a
//	@return error
func (a *App) GetAiConfigs() []*data.AIConfig {
	return a.settingsService.GetAiConfigs()
}

// GetAiAssistantSession 获取 AI 助手会话消息列表，sessionId 为空时获取最新的
func (a *App) GetAiAssistantSession(sessionId string) (*models.AiAssistantSessionResp, error) {
	return a.agentService.GetAiAssistantSession(sessionId)
}

// SaveAiAssistantSession 保存 AI 助手会话消息到数据库
func (a *App) SaveAiAssistantSession(sessionId string, messages []models.AiAssistantMessage) error {
	return a.agentService.SaveAiAssistantSession(sessionId, messages)
}

// FetchAiModels 根据接口地址与 apiKey 自动获取支持的模型列表
func (a *App) FetchAiModels(baseUrl, apiKey string) []string {
	return a.settingsService.FetchAiModels(baseUrl, apiKey)
}

// TestAiConnection 真实调用 /chat/completions 验证 AI 配置连通性
func (a *App) TestAiConnection(baseUrl, apiKey, modelName string) string {
	return a.settingsService.TestAiConnection(baseUrl, apiKey, modelName)
}

type AiModelInfo = services.AiModelInfo

func (a *App) FetchAiModelInfo(baseUrl, apiKey, modelName string) *AiModelInfo {
	info := a.settingsService.FetchAiModelInfo(baseUrl, apiKey, modelName)
	if info == nil {
		return nil
	}
	if info.MaxTokens == 0 {
		if maxTokens := getBuiltinModelMaxTokens(modelName); maxTokens > 0 {
			info.MaxTokens = maxTokens
			info.Source = "builtin"
		}
	}
	return info
}

func getBuiltinModelMaxTokens(modelName string) int {
	modelTokenMap := map[string]int{
		"deepseek-chat":        65536,
		"deepseek-reasoner":    65536,
		"deepseek-coder":       16384,
		"deepseek-v3":          65536,
		"deepseek-r1":          65536,
		"gpt-4o":               16384,
		"gpt-4o-mini":          16384,
		"gpt-4o-2024-05-13":    4096,
		"gpt-4-turbo":          4096,
		"gpt-4-turbo-preview":  4096,
		"gpt-4":                8192,
		"gpt-4-32k":            32768,
		"gpt-3.5-turbo":        4096,
		"gpt-3.5-turbo-16k":    16384,
		"gpt-4.1":              32768,
		"gpt-4.1-mini":         32768,
		"gpt-4.1-nano":         32768,
		"o1":                   100000,
		"o1-mini":              65536,
		"o1-preview":           32768,
		"o3-mini":              100000,
		"o4-mini":              100000,
		"claude-3-5-sonnet":    8192,
		"claude-3-5-haiku":     8192,
		"claude-3-opus":        4096,
		"claude-3-sonnet":      4096,
		"claude-3-haiku":       4096,
		"glm-4":                8192,
		"glm-4-plus":           4096,
		"glm-4-air":            4096,
		"glm-4-flash":          4096,
		"glm-4-long":           4096,
		"chatglm-turbo":        4096,
		"moonshot-v1-8k":       8192,
		"moonshot-v1-32k":      32768,
		"moonshot-v1-128k":     131072,
		"qwen-turbo":           8192,
		"qwen-plus":            131072,
		"qwen-max":             8192,
		"qwen-long":            65536,
		"qwen2.5-72b-instruct": 32768,
		"hunyuan-lite":         4096,
		"hunyuan-standard":     4096,
		"hunyuan-pro":          4096,
		"hunyuan-turbo":        4096,
		"spark-lite":           4096,
		"spark-pro":            4096,
		"spark-max":            4096,
		"spark-4.0-ultra":      4096,
		"yi-light":             16384,
		"yi-large":             16384,
		"yi-medium":            16384,
		"yi-spark":             16384,
		"yi-vision":            16384,
		"abab6.5-chat":         8192,
		"abab6.5s-chat":        8192,
		"abab5.5-chat":         4096,
		"baichuan2-turbo":      4096,
		"baichuan2-53b":        4096,
		"ernie-4.0":            4096,
		"ernie-3.5":            4096,
		"ernie-speed":          4096,
		"ernie-lite":           4096,
	}

	if maxTokens, ok := modelTokenMap[modelName]; ok {
		return maxTokens
	}

	for prefix, maxTokens := range map[string]int{
		"deepseek":      65536,
		"gpt-4o":        16384,
		"gpt-4-turbo":   4096,
		"gpt-4-":        8192,
		"gpt-3.5":       4096,
		"gpt-4.1":       32768,
		"o1-":           65536,
		"o3-":           100000,
		"o4-":           100000,
		"claude-3":      8192,
		"glm-4":         8192,
		"chatglm":       4096,
		"moonshot-v1":   8192,
		"qwen-":         8192,
		"qwen2":         32768,
		"hunyuan-":      4096,
		"spark-":        4096,
		"yi-":           16384,
		"abab":          8192,
		"baichuan":      4096,
		"ernie-":        4096,
		"llama-3":       8192,
		"llama3":        8192,
		"mistral-":      8192,
		"mixtral-":      32768,
		"codestral-":    32768,
		"gemini-1.5":    8192,
		"gemini-2":      8192,
		"command-r":     4096,
		"Qwen/Qwen":     32768,
		"deepseek-ai/":  65536,
		"meta-llama/":   8192,
		"mistralai/":    32768,
		"Pro/deepseek-": 65536,
		"Pro/qwen-":     32768,
	} {
		if strings.HasPrefix(modelName, prefix) {
			return maxTokens
		}
	}

	return 0
}

// InitCronTasks 在应用启动时，自动为启用状态的定时任务创建调度
func (a *App) InitCronTasks() {
	cronApi := agent.NewCronTaskApi()
	if !cronApi.ExistsByTaskType("stock_change_save") {
		task := &models.CronTask{
			Name:        "异动数据保存",
			CronExpr:    "0 */10 * * * *",
			TaskType:    "stock_change_save",
			Enable:      true,
			Status:      "active",
			Description: "每10分钟自动保存A股异动数据（火箭发射、快速反弹、大笔买入、封涨停板等），交易时间外自动跳过",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建异动数据保存任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建异动数据保存定时任务")
		}
	}

	// 盘前智能前瞻晨报: 每个交易日 09:10 生成
	if !cronApi.ExistsByTaskType("pre_market_briefing") {
		task := &models.CronTask{
			Name:        "盘前智能前瞻晨报",
			CronExpr:    "0 10 9 * * 1-5",
			TaskType:    "pre_market_briefing",
			Enable:      true,
			Status:      "active",
			Description: "交易日 09:10 自动生成盘前智能前瞻晨报，含隔夜外盘、核心要闻与今日策略指引",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建盘前晨报任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建盘前智能前瞻晨报任务")
		}
	}

	// 盘后收盘复盘: 每个交易日 15:05 自动触发
	if !cronApi.ExistsByTaskType("post_market_review") {
		task := &models.CronTask{
			Name:        "盘后收盘深度复盘",
			CronExpr:    "0 5 15 * * 1-5",
			TaskType:    "post_market_review",
			Enable:      true,
			Status:      "active",
			Description: "交易日 15:05 自动生成盘后收盘复盘报告，含资金筹码博弈、AI预测回测验证与次日推演",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建收盘复盘任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建盘后收盘深度复盘任务")
		}
	}
	tasks := cronApi.GetAll()
	if len(tasks) == 0 {
		return
	}
	for _, t := range tasks {
		taskCopy := t
		entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
			err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
			if err != nil {
				logger.SugaredLogger.Errorf("启动任务失败：%v %s", err, taskCopy.Name)
				return
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建定时任务失败：%v %s", err, taskCopy.Name)
			continue
		}
		a.setCronEntry(convertor.ToString(taskCopy.ID)+"_"+taskCopy.Name, entryID)
	}
}

// AbortSummaryStockNews 取消当前进行中的 SummaryStockNews 流式回答
func (a *App) AbortSummaryStockNews() {
	a.summaryMu.Lock()
	defer a.summaryMu.Unlock()
	if a.summaryCancel != nil {
		a.summaryCancel()
		a.summaryCancel = nil
	}
}

// CreateCronTask
//
//	@Description: 创建定时任务
//	@receiver a
//	@param task 定时任务信息
//	@return string 操作结果
func (a *App) CreateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Create(task)
	if err != nil {
		return fmt.Sprintf("创建失败：%v", err)
	}
	taskCopy := *task
	entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, taskCopy.Name)
			return
		}
	})
	a.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return "任务创建成功,但定时失败"
	}
	return "创建成功"
}

func (a *App) UpdateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Update(task)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	if entryID, exists := a.getCronEntry(convertor.ToString(task.ID) + "_" + task.Name); exists {
		a.cron.Remove(entryID)
	}
	taskCopy := *task
	entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, taskCopy.Name)
			return
		}
	})
	a.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	return "更新成功"
}

// DeleteCronTask
//
//	@Description: 删除定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果
func (a *App) DeleteCronTask(id uint) string {
	_ = agent.NewCronTaskApi().Delete(id)
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err == nil {
		if entryID, exists := a.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			a.cron.Remove(entryID)
		}
	}
	if err != nil {
		return fmt.Sprintf("删除失败：%v", err)
	}
	return "删除成功"
}

// GetCronTaskByID
//
//	@Description: 根据 ID 获取定时任务
//	@receiver a
//	@param id 任务 ID
//	@return *models.CronTask 任务信息
func (a *App) GetCronTaskByID(id uint) *models.CronTask {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return nil
	}
	return task
}

// GetCronTaskList
//
//	@Description: 获取定时任务列表
//	@receiver a
//	@param query 查询条件
//	@return *models.CronTaskPageResp 分页结果
func (a *App) GetCronTaskList(query *models.CronTaskQuery) *models.CronTaskPageResp {
	return agent.NewCronTaskApi().List(query)
}

// EnableCronTask
//
//	@Description: 启用/禁用定时任务
//	@receiver a
func (a *App) EnableCronTask(id uint, enable bool) string {
	_ = agent.NewCronTaskApi().EnableTask(id, enable)
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err == nil {
		if entryID, exists := a.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			a.cron.Remove(entryID)
		}
		if enable {
			taskCopy := *task
			entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
				err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
				if err != nil {
					logger.SugaredLogger.Errorf("%s 执行任务失败：%v", taskCopy.Name, err)
					return
				}
			})
			a.setCronEntry(convertor.ToString(id)+"_"+task.Name, entryID)
			if err != nil {
				return "操作成功,但定时失败"
			}
		}

	}
	if err != nil {
		return fmt.Sprintf("操作失败：%v", err)
	}
	return "操作成功"
}

// ExecuteCronTaskNow
//
//	@Description: 立即执行定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果
func (a *App) ExecuteCronTaskNow(id uint) string {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return fmt.Sprintf("任务不存在：%v", err)
	}

	go func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, task)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, task.Name)
		}
	}()

	return "任务执行中"
}

// GetCronTaskTypes
//
//	@Description: 获取所有任务类型
//	@receiver a
//	@return []lo.Tuple2[string, string] 任务类型列表
func (a *App) GetCronTaskTypes() []lo.Tuple2[string, string] {
	return agent.NewCronTaskApi().GetTaskTypes()
}

// ValidateCronExpr
//
//	@Description: 验证 Cron 表达式
//	@receiver a
//	@param expr Cron 表达式
//	@return string 验证结果
func (a *App) ValidateCronExpr(expr string) string {
	err := agent.NewCronTaskApi().ValidateCronExpr(expr)
	if err != nil {
		return fmt.Sprintf("无效表达式：%v", err)
	}
	return "有效表达式"
}

// GetMarketLifecycleStatus
//
//	@Description: 获取市场生命周期状态（盘前/集合竞价/早盘/午休/下午盘/盘后/休市）
//	@receiver a
//	@return agent.MarketLifecycleStatus
func (a *App) GetMarketLifecycleStatus() agent.MarketLifecycleStatus {
	return a.agentService.GetMarketLifecycleStatus(a.ctx)
}

// GeneratePreMarketBriefing 生成盘前智能前瞻晨报
func (a *App) GeneratePreMarketBriefing(aiConfigId int) *agent.MarketIntelligenceReport {
	return a.agentService.GeneratePreMarketBriefing(a.ctx, aiConfigId)
}

// GeneratePostMarketReview 生成盘后收盘深度复盘
func (a *App) GeneratePostMarketReview(aiConfigId int) *agent.MarketIntelligenceReport {
	return a.agentService.GeneratePostMarketReview(a.ctx, aiConfigId)
}

// GetLatestMarketIntelligenceReport 获取最近一次盘前/盘后智能研报
func (a *App) GetLatestMarketIntelligenceReport() *agent.MarketIntelligenceReport {
	return a.agentService.GetLatestMarketIntelligenceReport(a.ctx)
}

// GetLatestMarketIntelligenceReportByType 获取指定类型的最新研报
func (a *App) GetLatestMarketIntelligenceReportByType(reportType string) *agent.MarketIntelligenceReport {
	return a.agentService.GetLatestMarketIntelligenceReportByType(a.ctx, reportType)
}

// SearchCronTasks
//
//	@Description: 搜索定时任务
//	@receiver a
//	@param keyword 搜索关键词
//	@return []models.CronTask 搜索结果
func (a *App) SearchCronTasks(keyword string) []models.CronTask {
	return agent.NewCronTaskApi().SearchTasks(keyword)
}

// CalculateNextRunTime 根据 Cron 表达式计算下一次运行时间
// 参数:
//   - cron: Cron 表达式，用于定义任务调度的时间规则
//
// 返回值:
//   - string: 格式化为 "2006-01-02 15:04:05" 的下一次运行时间字符串
func (a *App) CalculateNextRunTime(cron string) string {
	nextRunTime := agent.NewCronTaskApi().CalculateNextRunTime(cron)
	return nextRunTime.Format("2006-01-02 15:04:05")
}

// CalculateNextRunTimes 根据 Cron 表达式计算未来多次运行时间
// 参数:
//   - cron: Cron 表达式
//   - count: 需要计算的次数
//
// 返回值:
//   - []string: 按时间顺序排序的运行时间列表，格式为 "2006-01-02 15:04:05"
func (a *App) CalculateNextRunTimes(cron string, count int) []string {
	times := agent.NewCronTaskApi().CalculateNextRunTimes(cron, count)
	result := make([]string, 0, len(times))
	for _, t := range times {
		result = append(result, t.Format("2006-01-02 15:04:05"))
	}
	return result
}

// AddTradingRecord 添加交易记录
// 参数:
//   - record: 交易记录结构体
//
// 返回值:
//   - uint: 新添加的交易记录ID
//   - error: 错误信息
func (a *App) AddTradingRecord(record data.TradingRecord) (uint, error) {
	return a.settingsService.AddTradingRecord(record)
}

func (a *App) GetTradingRecordList(query data.TradingRecordListQuery) *data.TradingRecordPageData {
	return a.settingsService.GetTradingRecordList(query)
}

func (a *App) GetTradingRecordById(id uint) (*data.TradingRecord, error) {
	return a.settingsService.GetTradingRecordById(id)
}

func (a *App) GetTradingRecordStatistics() *data.TradingRecordStatistics {
	return a.settingsService.GetTradingRecordStatistics()
}

func (a *App) UpdateTradingRecord(record data.TradingRecord) error {
	return a.settingsService.UpdateTradingRecord(record)
}

func (a *App) DeleteTradingRecord(id uint) error {
	return a.settingsService.DeleteTradingRecord(id)
}

func (a *App) CheckFrequentTrading(stockCode string) map[string]any {
	return a.settingsService.CheckFrequentTrading(stockCode)
}

func (a *App) FetchAndSaveMarketStatistic() {
	a.marketService.FetchAndSaveMarketStatistic()
}

func (a *App) GetTodayMarketStatistic() []models.MarketStatistic {
	return a.marketService.GetTodayMarketStatistic()
}

func (a *App) GetMarketStatisticByDate(date string) []models.MarketStatistic {
	return a.marketService.GetMarketStatisticByDate(date)
}

func (a *App) GetRecentDaysMarketStatistic(days int) []models.MarketStatistic {
	return a.marketService.GetRecentDaysMarketStatistic(days)
}

func (a *App) CreateMCPServer(server *models.MCPServer) string {
	return a.agentService.CreateMCPServer(server)
}

func (a *App) UpdateMCPServer(server *models.MCPServer) string {
	return a.agentService.UpdateMCPServer(server)
}

func (a *App) DeleteMCPServer(id uint) string {
	return a.agentService.DeleteMCPServer(id)
}

func (a *App) GetMCPServerByID(id uint) *models.MCPServer {
	return a.agentService.GetMCPServerByID(id)
}

func (a *App) GetMCPServerList(query *models.MCPServerQuery) *models.MCPServerPageResp {
	return a.agentService.GetMCPServerList(query)
}

func (a *App) EnableMCPServer(id uint, enable bool) string {
	return a.agentService.EnableMCPServer(id, enable)
}

func (a *App) TestMCPServer(id uint) string {
	return a.agentService.TestMCPServer(id)
}

func (a *App) CreateSkill(skill *models.Skill) string {
	return a.agentService.CreateSkill(skill)
}

func (a *App) UpdateSkill(skill *models.Skill) string {
	return a.agentService.UpdateSkill(skill)
}

func (a *App) DeleteSkill(id uint) string {
	return a.agentService.DeleteSkill(id)
}

func (a *App) GetSkillByID(id uint) *models.Skill {
	return a.agentService.GetSkillByID(id)
}

func (a *App) GetSkillList(query *models.SkillQuery) *models.SkillPageResp {
	return a.agentService.GetSkillList(query)
}

func (a *App) EnableSkill(id uint, enable bool) string {
	return a.agentService.EnableSkill(id, enable)
}

func (a *App) GetAllSkills() []models.Skill {
	return a.agentService.GetAllSkills()
}

func (a *App) LearnSkillFromProject(projectName string, analysis string) string {
	return a.agentService.LearnSkillFromProject(projectName, analysis)
}

func (a *App) GetMCPToolsByServerID(serverID uint) []models.MCPServerTool {
	return a.agentService.GetMCPToolsByServerID(serverID)
}

func (a *App) EnableMCPTool(id uint, enable bool) string {
	return a.agentService.EnableMCPTool(id, enable)
}

func (a *App) GetAllMCPTools() []models.MCPServerTool {
	return a.agentService.GetAllMCPTools()
}

type PromptTemplateItem = services.PromptTemplateItem

func (a *App) GetPromptTemplates(name, promptType string) []PromptTemplateItem {
	return a.agentService.GetPromptTemplates(name, promptType)
}

// ============================================================================
// VectorBT 向量化回测 API - Wails 暴露接口
// ============================================================================

// VectorBTResult 向量化回测结果
type VectorBTResult struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// RunVectorBacktest 运行单次回测
func (a *App) RunVectorBacktest(stockCode, strategyType string, params map[string]any) map[string]any {
	return a.quantService.RunVectorBacktest(stockCode, strategyType, params)
}

// RunVectorGridSearch 运行参数网格扫描
func (a *App) RunVectorGridSearch(stockCode, strategyType string) map[string]any {
	return a.quantService.RunVectorGridSearch(stockCode, strategyType)
}

// GenerateETFRotationReport 手动或一键触发全市场ETF轮动多Agent决策分析
func (a *App) GenerateETFRotationReport(aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	return a.agentService.GenerateETFRotationReport(a.ctx, aiConfigId)
}

// GenerateSerenityResearchReport 手动或一键触发Serenity产业深度拆解
func (a *App) GenerateSerenityResearchReport(targetName string, code string, aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	return a.agentService.GenerateSerenityResearchReport(a.ctx, targetName, code, aiConfigId)
}

// ReloadSkills 热重载技能定义，无需重启应用
func (a *App) ReloadSkills() error {
	return a.agentService.ReloadSkills()
}

// GetAIResponseResult 获取指定标的最新AI分析结果
func (a *App) GetAIResponseResult(stock string) *models.AIResponseResult {
	return data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stock)
}

// FetchAuditLogs 前端兼容别名：分页获取审计日志
func (a *App) FetchAuditLogs(query models.AuditLogQuery) (*models.AuditLogPageResp, error) {
	return a.settingsService.FetchAuditLogs(query)
}

// FetchSkillChanges 查询技能审核与变更流水
func (a *App) FetchSkillChanges(query any) (any, error) {
	return a.agentService.FetchSkillChanges(query)
}

// ApproveSkillChange 批准技能发布/更新
func (a *App) ApproveSkillChange(id any) (any, error) {
	return a.agentService.ApproveSkillChange(id)
}

// RejectSkillChange 驳回技能发布/更新
func (a *App) RejectSkillChange(id any) (any, error) {
	return a.agentService.RejectSkillChange(id)
}

// BatchApproveSkillChanges 批量批准技能发布
func (a *App) BatchApproveSkillChanges(ids any) (any, error) {
	return a.agentService.BatchApproveSkillChanges(ids)
}

// BatchRejectSkillChanges 批量驳回技能发布
func (a *App) BatchRejectSkillChanges(ids any) (any, error) {
	return a.agentService.BatchRejectSkillChanges(ids)
}

// ============================================================================
// AI 语义向量精炼与知识结晶 (Semantic Distillation & Knowledge Crystallization)
// ============================================================================

// TriggerSemanticDistillation 手动或一键触发全局语义精炼、知识结晶与存储自愈
func (a *App) TriggerSemanticDistillation() (*agent.DistillationStats, error) {
	return a.agentService.TriggerSemanticDistillation(a.ctx)
}

// GetDistillationStats 获取当前语义精炼、结晶总数与瘦身指标
func (a *App) GetDistillationStats() (*agent.DistillationStats, error) {
	return a.agentService.GetDistillationStats(a.ctx)
}

// GetKnowledgeCrystals 获取平台长期沉淀凝练的高胜率经验结晶库
func (a *App) GetKnowledgeCrystals(category string, limit int) ([]agent.KnowledgeCrystal, error) {
	return a.agentService.GetKnowledgeCrystals(a.ctx, category, limit)
}


