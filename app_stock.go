package main

import (
	"blades-stock/backend/agent/tools"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"context"
	"fmt"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	runtime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

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

// syncStockBasicFromTdx 用通达信即时数据对 A 股基础信息做增量校准。
// 在 CheckStockBaseInfo 全量覆盖之后调用：通达信本地证券列表新股上市当天即可见，
// 以 upsert 方式补充全量 JSON 未及时覆盖的新上市/改名/退市股票。
func (a *App) syncStockBasicFromTdx() {
	defer PanicHandler()
	added, updated, err := data.SyncStockBasicToDB(data.NewTdxKLineApi())
	if err != nil {
		logger.SugaredLogger.Warnf("通达信同步股票基础信息失败:%s", err.Error())
		return
	}
	logger.SugaredLogger.Infof("通达信同步股票基础信息完成：新增 %d 条，更新 %d 条", added, updated)
}

// syncHKUSStockBasicFromTdx 用通达信扩展行情即时数据对港股/美股基础信息做增量校准。

// syncHKUSStockBasicFromTdx 用通达信扩展行情即时数据对港股/美股基础信息做增量校准。
func (a *App) syncHKUSStockBasicFromTdx() {
	defer PanicHandler()
	hkAdded, hkUpdated, usAdded, usUpdated, err := data.SyncHKUSStockBasicToDB(data.NewTdxKLineApi())
	if err != nil {
		logger.SugaredLogger.Warnf("通达信同步港美股基础信息失败:%s", err.Error())
		return
	}
	logger.SugaredLogger.Infof("通达信同步港美股基础信息完成：港股新增 %d 更新 %d，美股新增 %d 更新 %d",
		hkAdded, hkUpdated, usAdded, usUpdated)
}

// MonitorFollowedStockCostPrices 监控自选股的持仓成本价，当股价低于成本价时发送预警
func MonitorFollowedStockCostPrices(a *App) {
	defer PanicHandler()

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

func (a *App) Follow(stockCode string) string {
	return a.marketService.Follow(stockCode)
}

func (a *App) UnFollow(stockCode string) string {
	return a.marketService.UnFollow(stockCode)
}

func (a *App) GetFollowList(groupId int) *[]data.FollowedStock {
	return a.marketService.GetFollowList(groupId)
}

func (a *App) GetFollowListPaged(query any) *[]data.FollowedStock {
	return a.marketService.GetFollowListPaged(query)
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

func (a *App) GetStockFinancialInfo(stockCode string) map[string]any {
	return a.marketService.GetStockFinancialInfo(stockCode)
}

func (a *App) GetStockLatestFinance(stockCode string) map[string]any {
	return a.marketService.GetStockLatestFinance(stockCode)
}

func (a *App) GetStockHolderTrend(stockCode string) map[string]any {
	return a.marketService.GetStockHolderTrend(stockCode)
}

func (a *App) SearchStockByIndicators(indicators map[string]any) map[string]any {
	return a.marketService.SearchStockByIndicators(indicators)
}

func (a *App) FilterStocks(filter map[string]any) map[string]any {
	return a.marketService.FilterStocks(filter)
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
