package services

import (
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/slice"
)

// MarketService 股票市场行情、自选、分组、K线与盘口数据服务
type MarketService struct{}

// NewMarketService 创建市场行情服务实例
func NewMarketService() *MarketService {
	return &MarketService{}
}

// BatchGreet 批量获取自选股票实时数据
func (s *MarketService) BatchGreet(stockCodes []string) []*data.StockInfo {
	if len(stockCodes) == 0 {
		return nil
	}

	var follows []data.FollowedStock
	db.Dao.Where("stock_code IN ?", stockCodes).Preload("Groups").Preload("Groups.GroupInfo").Find(&follows)
	followMap := make(map[string]data.FollowedStock, len(follows))
	for _, f := range follows {
		followMap[f.StockCode] = f
	}

	stockInfos, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockInfos == nil {
		return nil
	}

	results := make([]*data.StockInfo, 0, len(*stockInfos))
	for i := range *stockInfos {
		info := &(*stockInfos)[i]
		if follow, ok := followMap[info.Code]; ok {
			s.AddStockFollowData(follow, info)
		}
		results = append(results, info)
	}
	return results
}

// Greet 获取单只自选股票详情
func (s *MarketService) Greet(stockCode string) *data.StockInfo {
	follow := &data.FollowedStock{
		StockCode: stockCode,
	}
	db.Dao.Model(follow).Where("stock_code = ?", stockCode).Preload("Groups").Preload("Groups.GroupInfo").First(follow)
	return s.GetStockInfo(*follow)
}

// GetStockInfo 内部辅助：结合关注信息组装行情
func (s *MarketService) GetStockInfo(follow data.FollowedStock) *data.StockInfo {
	stockCode := follow.StockCode
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
		return &data.StockInfo{}
	}
	stockData := (*stockDatas)[0]
	s.AddStockFollowData(follow, &stockData)
	return &stockData
}

// AddStockFollowData 注入持仓、报警及成本字段
func (s *MarketService) AddStockFollowData(follow data.FollowedStock, stockData *data.StockInfo) {
	stockData.PrePrice = follow.Price
	stockData.Sort = follow.Sort
	stockData.CostPrice = follow.CostPrice
	stockData.CostVolume = follow.Volume
	stockData.AlarmChangePercent = follow.AlarmChangePercent
	stockData.AlarmPrice = follow.AlarmPrice
	stockData.Groups = follow.Groups

	price, _ := convertor.ToFloat(stockData.Price)
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.A1P)
	}
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.B1P)
	}

	preClosePrice, _ := convertor.ToFloat(stockData.PreClose)
	if price == 0 {
		price = preClosePrice
	}

	highPrice, _ := convertor.ToFloat(stockData.High)
	if highPrice == 0 {
		highPrice, _ = convertor.ToFloat(stockData.Open)
	}

	lowPrice, _ := convertor.ToFloat(stockData.Low)
	if lowPrice == 0 {
		lowPrice, _ = convertor.ToFloat(stockData.Open)
	}

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
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(preClosePrice-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((preClosePrice-follow.CostPrice)*float64(follow.Volume), 2)
			stockData.ProfitAmountToday = 0
		}
	}
}

// Follow 关注股票
func (s *MarketService) Follow(stockCode string) string {
	return data.NewStockDataApi().Follow(stockCode)
}

// UnFollow 取消关注股票
func (s *MarketService) UnFollow(stockCode string) string {
	return data.NewStockDataApi().UnFollow(stockCode)
}

// GetFollowList 获取关注股票列表
func (s *MarketService) GetFollowList(groupId int) *[]data.FollowedStock {
	return data.NewStockDataApi().GetFollowList(groupId)
}

// GetStockList 搜索股票列表
func (s *MarketService) GetStockList(key string) []data.StockBasic {
	return data.NewStockDataApi().GetStockList(key)
}

// SetCostPriceAndVolume 设置持仓成本与持股数量
func (s *MarketService) SetCostPriceAndVolume(stockCode string, price float64, volume int64) string {
	return data.NewStockDataApi().SetCostPriceAndVolume(price, volume, stockCode)
}

// SetTradingPrice 设置建仓价、止盈价、止损价
func (s *MarketService) SetTradingPrice(stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string {
	return data.NewStockDataApi().SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode)
}

// SetAlarmChangePercent 设置涨跌预警阈值
func (s *MarketService) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	return data.NewStockDataApi().SetAlarmChangePercent(val, alarmPrice, stockCode)
}

// SetStockSort 设置自选股排序
func (s *MarketService) SetStockSort(sort int64, stockCode string) {
	data.NewStockDataApi().SetStockSort(sort, stockCode)
}

// AddGroup 添加分组
func (s *MarketService) AddGroup(group data.Group) string {
	ok := data.NewStockGroupApi(db.Dao).AddGroup(group)
	if ok {
		return "添加成功"
	}
	return "添加失败"
}

// GetGroupList 获取分组列表
func (s *MarketService) GetGroupList() []data.Group {
	return data.NewStockGroupApi(db.Dao).GetGroupList()
}

// UpdateGroupSort 更新分组排序
func (s *MarketService) UpdateGroupSort(id int, newSort int) bool {
	return data.NewStockGroupApi(db.Dao).UpdateGroupSort(id, newSort)
}

// UpdateGroup 修改分组名称
func (s *MarketService) UpdateGroup(id int, name string) string {
	ok := data.NewStockGroupApi(db.Dao).UpdateGroup(id, name)
	if ok {
		return "修改成功"
	}
	return "修改失败"
}

// InitializeGroupSort 初始化分组排序
func (s *MarketService) InitializeGroupSort() bool {
	return data.NewStockGroupApi(db.Dao).InitializeGroupSort()
}

// GetGroupStockList 获取指定分组下的股票关联
func (s *MarketService) GetGroupStockList(groupId int) []data.GroupStock {
	return data.NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
}

// AddStockGroup 将股票加入分组
func (s *MarketService) AddStockGroup(groupId int, stockCode string) string {
	ok := data.NewStockGroupApi(db.Dao).AddStockGroup(groupId, stockCode)
	if ok {
		return "添加成功"
	}
	return "添加失败"
}

// RemoveStockGroup 从分组移除股票
func (s *MarketService) RemoveStockGroup(code, name string, groupId int) string {
	ok := data.NewStockGroupApi(db.Dao).RemoveStockGroup(code, name, groupId)
	if ok {
		return "移除成功"
	}
	return "移除失败"
}

// RemoveGroup 删除分组
func (s *MarketService) RemoveGroup(groupId int) string {
	ok := data.NewStockGroupApi(db.Dao).RemoveGroup(groupId)
	if ok {
		return "移除成功"
	}
	return "移除失败"
}

// GetStockKLine 获取港股日K线或普通K线
func (s *MarketService) GetStockKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	if data.IsHKStockCode(stockCode) {
		tdxData := data.NewTdxKLineApi().GetMACKLineData(stockCode, "101", int(days))
		if tdxData != nil && len(*tdxData) > 0 {
			return tdxData
		}
	}
	return data.NewStockDataApi().GetHK_KLineData(stockCode, "day", days)
}

// GetStockMinutePriceLineData 获取分时走势图数据
func (s *MarketService) GetStockMinutePriceLineData(stockCode, stockName string) map[string]any {
	res := make(map[string]any, 4)
	priceData, date := data.NewStockDataApi().GetStockMinutePriceData(stockCode)
	res["priceData"] = priceData
	res["date"] = date
	res["stockName"] = stockName
	res["stockCode"] = stockCode
	return res
}

// GetStockCommonKLine 获取普通A股日K线
func (s *MarketService) GetStockCommonKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	return data.NewStockDataApi().GetCommonKLineData(stockCode, "day", days)
}

// GetStockEastMoneyKLine 获取东方财富多周期K线
func (s *MarketService) GetStockEastMoneyKLine(stockCode, stockName string, klt string, limit int) *[]data.KLineData {
	return s.GetStockEastMoneyKLinePage(stockCode, stockName, klt, limit, "")
}

// GetStockEastMoneyKLinePage 分页获取东方财富K线
func (s *MarketService) GetStockEastMoneyKLinePage(stockCode, stockName string, klt string, limit int, end string) *[]data.KLineData {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "1"
	}
	api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
	end = strings.TrimSpace(end)
	return api.GetKLineDataBefore(stockCode, klt, "", limit, end)
}

// GetStockKLineWithFallback 多数据源自动降级K线（东财优先，新浪后备）
func (s *MarketService) GetStockKLineWithFallback(stockCode, stockName string, klt string, limit int) *data.KLineSourceResult {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "101"
	}
	return data.FetchKLineWithFallback(stockCode, stockName, klt, limit, "")
}

// GetStockKLinePageWithFallback 分页多数据源自动降级K线
func (s *MarketService) GetStockKLinePageWithFallback(stockCode, stockName string, klt string, limit int, end string) *data.KLineSourceResult {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "101"
	}
	end = strings.TrimSpace(end)
	return data.FetchKLineWithFallback(stockCode, stockName, klt, limit, end)
}

// GetChipDistribution 计算个股筹码分布
func (s *MarketService) GetChipDistribution(stockCode string, days int, bins int, adjustFlag string) (*data.ChipDistributionResult, error) {
	stockCode = strings.TrimSpace(stockCode)
	if stockCode == "" {
		return nil, fmt.Errorf("stockCode 不能为空")
	}
	if days <= 0 {
		days = 120
	}
	if bins <= 0 {
		bins = 80
	}
	adjustFlag = strings.TrimSpace(strings.ToLower(adjustFlag))
	if adjustFlag != "" && adjustFlag != "qfq" && adjustFlag != "hfq" {
		adjustFlag = "qfq"
	}

	api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
	if !api.ValidateStockCode(stockCode) {
		return nil, fmt.Errorf("股票代码无效：%s", stockCode)
	}

	var kLines *[]data.KLineData
	if adjustFlag != "" {
		kLines = api.GetKLineData(stockCode, "101", adjustFlag, days)
	} else {
		result := data.FetchKLineWithFallback(stockCode, "", "101", days, "")
		if result != nil && result.Data != nil {
			kLines = result.Data
		}
	}

	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("未获取到K线数据")
	}
	calculator := data.NewChipDistributionCalculator()
	return calculator.Calculate(stockCode, *kLines, bins)
}

// GetTdxCallAuction 获取通达信集合竞价
func (s *MarketService) GetTdxCallAuction(stockCode string, start uint32, count uint32) *[]data.TdxCallAuctionData {
	if count <= 0 {
		count = 500
	}
	return data.NewTdxKLineApi().GetCallAuction(stockCode, start, count)
}

// GetTdxCompanyInfo 获取通达信 F10 公司详情
func (s *MarketService) GetTdxCompanyInfo(stockCode string) *data.TdxCompanyInfoBundle {
	return data.NewTdxKLineApi().GetF10Data(stockCode)
}

// GetTdxFinanceInfo 获取通达信财务分析
func (s *MarketService) GetTdxFinanceInfo(stockCode string) *data.TdxFinanceInfo {
	return data.NewTdxKLineApi().GetFinanceInfo(stockCode)
}

// GetTdxXDXRInfo 获取通达信除权除息
func (s *MarketService) GetTdxXDXRInfo(stockCode string) *[]data.TdxXDXRItem {
	return data.NewTdxKLineApi().GetXDXRInfo(stockCode)
}

// GetTdxCompanyCategoryList 获取通达信 F10 分类列表
func (s *MarketService) GetTdxCompanyCategoryList(stockCode string) *[]data.TdxCompanyCategory {
	return data.NewTdxKLineApi().GetF10CategoryList(stockCode)
}

// GetTdxCompanyCategoryContent 获取通达信 F10 分类详情
func (s *MarketService) GetTdxCompanyCategoryContent(stockCode string, categoryName string) *data.TdxCompanyInfoSection {
	return data.NewTdxKLineApi().GetF10CategoryContent(stockCode, categoryName)
}

// GetTdxSymbolBelongBoard 获取股票所属板块
func (s *MarketService) GetTdxSymbolBelongBoard(stockCode string) *[]data.MACBelongBoardItem {
	return data.NewTdxKLineApi().GetMACSymbolBelongBoard(stockCode)
}

// GetTelegraphList 获取电报新闻列表
func (s *MarketService) GetTelegraphList(source string) *[]*models.Telegraph {
	return data.NewMarketNewsApi().GetTelegraphList(source)
}

// ReFleshTelegraphList 刷新电报新闻列表
func (s *MarketService) ReFleshTelegraphList(source string) *[]*models.Telegraph {
	go data.NewMarketNewsApi().TelegraphList(30)
	go data.NewMarketNewsApi().GetSinaNews(30)
	go data.NewMarketNewsApi().TradingViewNews()
	return data.NewMarketNewsApi().GetTelegraphList(source)
}

// GlobalStockIndexes 获取全球重要股指
func (s *MarketService) GlobalStockIndexes() map[string]any {
	res := data.NewMarketNewsApi().GlobalStockIndexes(30)
	if res == nil {
		return map[string]any{}
	}
	return res
}

// GlobalStockIndexesReadable 全球股指转易读 Markdown
func (s *MarketService) GlobalStockIndexesReadable() string {
	return data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
}

// GetIndustryRank 行业排行
func (s *MarketService) GetIndustryRank(sort string, cnt int) []any {
	res := data.NewMarketNewsApi().GetIndustryRank(sort, cnt)
	if res == nil {
		return []any{}
	}
	dataVal, ok := res["data"]
	if !ok || dataVal == nil {
		return []any{}
	}
	dataArr, ok := dataVal.([]any)
	if !ok {
		return []any{}
	}
	return dataArr
}

// GetIndustryMoneyRankSina 新浪行业资金流向
func (s *MarketService) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	return data.NewMarketNewsApi().GetIndustryMoneyRankSina(fenlei, sort)
}

// GetMoneyRankSina 新浪个股资金流向排行
func (s *MarketService) GetMoneyRankSina(sort string) []map[string]any {
	return data.NewMarketNewsApi().GetMoneyRankSina(sort)
}

// GetStockMoneyTrendByDay 个股近期资金流向趋势
func (s *MarketService) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	res := data.NewMarketNewsApi().GetStockMoneyTrendByDay(stockCode, days)
	slice.Reverse(res)
	return res
}

// FetchAndSaveMarketStatistic 获取并保存今日盘面多空统计
func (s *MarketService) FetchAndSaveMarketStatistic() {
	now := time.Now()
	// 周末不处理
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return
	}
	err := data.NewMarketStatisticApi().FetchAndSave()
	if err != nil {
		logger.SugaredLogger.Errorf("获取市场统计数据失败: %v", err)
	}
}

// GetTodayMarketStatistic 获取今日盘面统计
func (s *MarketService) GetTodayMarketStatistic() []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetTodayData()
}

// GetMarketStatisticByDate 获取指定日期盘面统计
func (s *MarketService) GetMarketStatisticByDate(date string) []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetByDate(date)
}

// GetRecentDaysMarketStatistic 获取最近 N 日盘面统计
func (s *MarketService) GetRecentDaysMarketStatistic(days int) []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetRecentDaysData(days)
}

// ----------------- 基金全功能 -----------------

// GetfundList 获取基金基础列表
func (s *MarketService) GetfundList(key string) []data.FundBasic {
	return data.NewFundApi().GetFundList(key)
}

// GetFollowedFund 获取关注基金列表
func (s *MarketService) GetFollowedFund() []data.FollowedFund {
	return data.NewFundApi().GetFollowedFund()
}

// FollowFund 关注基金
func (s *MarketService) FollowFund(fundCode string) string {
	return data.NewFundApi().FollowFund(fundCode)
}

// UnFollowFund 取消关注基金
func (s *MarketService) UnFollowFund(fundCode string) string {
	return data.NewFundApi().UnFollowFund(fundCode)
}

// GetFundKLine 获取基金K线
func (s *MarketService) GetFundKLine(fundCode string, klt string, limit int) *data.KLineSourceResult {
	return data.NewFundKLineApi().GetFundKLineWithFallback(fundCode, klt, limit)
}

// GetFundHistoryNetValue 获取基金历史净值
func (s *MarketService) GetFundHistoryNetValue(fundCode string, pageSize int, startDate string, endDate string) []data.FundHistoryNetValue {
	res, _ := data.NewFundApi().GetFundHistoryNetValue(fundCode, 1, pageSize, startDate, endDate)
	if res == nil {
		return []data.FundHistoryNetValue{}
	}
	return res
}

// GetFundTop10Holdings 获取基金前十大重仓股
func (s *MarketService) GetFundTop10Holdings(fundCode string) []data.FundHoldingStock {
	res, err := data.NewFundApi().GetFundTop10Holdings(fundCode)
	if err != nil || res == nil {
		return []data.FundHoldingStock{}
	}
	return res
}

// GetFundRanking 获取基金排行
func (s *MarketService) GetFundRanking(marketType, fundType, sortField, sortOrder string, pageIndex, pageSize int) *data.FundRankingResult {
	res, err := data.NewFundApi().GetFundRanking(marketType, fundType, sortField, sortOrder, pageIndex, pageSize)
	if err != nil || res == nil {
		return &data.FundRankingResult{}
	}
	return res
}

// SearchFundCodes 联想搜索基金代码
func (s *MarketService) SearchFundCodes(keyword string) []data.FundSearchItem {
	return data.NewFundApi().SearchFundCodes(keyword)
}

// GetFollowedFundPaged 分页获取关注的基金
func (s *MarketService) GetFollowedFundPaged(pageIndex, pageSize int, keyword string) *data.FollowedFundPagedResult {
	return data.NewFundApi().GetFollowedFundPaged(pageIndex, pageSize, keyword)
}
