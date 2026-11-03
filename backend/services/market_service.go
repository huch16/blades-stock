package services

import (
	"fmt"
	"strconv"
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

// GetFollowList 获取关注股票列表（自动扫描并回填 0 元空价格标的，杜绝首屏零价惨案）
func (s *MarketService) GetFollowList(groupId int) *[]data.FollowedStock {
	list := data.NewStockDataApi().GetFollowList(groupId)
	if list == nil || len(*list) == 0 {
		return list
	}

	// 探测是否存在 Price <= 0 的标的，若有则立即批量拉取最新实时行情并回写数据库
	var zeroCodes []string
	for _, item := range *list {
		if item.Price <= 0 && item.StockCode != "" {
			zeroCodes = append(zeroCodes, data.FormatMarketStockCode(item.StockCode))
		}
	}

	if len(zeroCodes) > 0 {
		quotes, err := data.NewStockDataApi().GetStockCodeRealTimeData(zeroCodes...)
		if err == nil && quotes != nil && len(*quotes) > 0 {
			quoteMap := make(map[string]data.StockInfo)
			for _, q := range *quotes {
				quoteMap[data.FormatMarketStockCode(q.Code)] = q
				pure := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(q.Code), "sh"), "sz")
				quoteMap[pure] = q
			}

			for i := range *list {
				item := &(*list)[i]
				canonical := data.FormatMarketStockCode(item.StockCode)
				pure := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(item.StockCode), "sh"), "sz")
				if q, ok := quoteMap[canonical]; ok {
					s.updateFollowedPrice(item, q)
				} else if q, ok := quoteMap[pure]; ok {
					s.updateFollowedPrice(item, q)
				}
			}
		}
	}

	return list
}

func (s *MarketService) updateFollowedPrice(item *data.FollowedStock, q data.StockInfo) {
	p, err := strconv.ParseFloat(q.Price, 64)
	if err != nil || p <= 0 {
		return
	}
	preClose, _ := strconv.ParseFloat(q.PreClose, 64)
	chgPct := 0.0
	chgPrice := 0.0
	if p > 0 && preClose > 0 {
		chgPrice = mathutil.RoundToFloat(p-preClose, 2)
		chgPct = mathutil.RoundToFloat(mathutil.Div(p-preClose, preClose)*100, 3)
	}
	item.Price = p
	item.ChangePercent = chgPct
	item.PriceChange = chgPrice
	_ = db.Dao.Model(&data.FollowedStock{}).
		Where("stock_code = ? AND is_del = 0", item.StockCode).
		Updates(map[string]any{
			"price":          p,
			"change_percent": chgPct,
			"price_change":   chgPrice,
			"time":           time.Now(),
		}).Error
}

// GetFollowListPaged 分页/别名兼容接口，兼容前端调用
func (s *MarketService) GetFollowListPaged(query any) *[]data.FollowedStock {
	return s.GetFollowList(0)
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
	res := data.FetchKLineWithFallback(stockCode, stockName, klt, limit, "")
	// 严格按前端请求参数约束截断切片，杜绝每次强制返回 500 根导致的带宽轰炸
	if res != nil && res.Data != nil && len(*res.Data) > limit {
		sliced := (*res.Data)[len(*res.Data)-limit:]
		res.Data = &sliced
	}
	return res
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
	switch source {
	case "财联社电报":
		data.NewMarketNewsApi().TelegraphList(15)
	case "新浪财经":
		data.NewMarketNewsApi().GetSinaNews(15)
	case "外媒":
		data.NewMarketNewsApi().TradingViewNews()
	default:
		data.NewMarketNewsApi().TelegraphList(15)
		data.NewMarketNewsApi().GetSinaNews(15)
	}
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

// GetFundTop 兼容前端 GetFundTop 调用，映射到前十大重仓股
func (s *MarketService) GetFundTop(fundCode string) []data.FundHoldingStock {
	return s.GetFundTop10Holdings(fundCode)
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

// ----------------- 交易时钟与日历 -----------------

var shanghaiZone = time.FixedZone("CST", 8*60*60)

// GetTimezone 获取系统时区配置
func (s *MarketService) GetTimezone() map[string]any {
	return map[string]any{
		"offset":   8 * 60 * 60,
		"location": "Asia/Shanghai",
	}
}

// IsTradingTime 判断当前是否处于 A 股交易时间
func (s *MarketService) IsTradingTime() bool {
	now := time.Now().In(shanghaiZone)
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	hour, minute, _ := now.Clock()
	if (hour == 9 && minute >= 15) || (hour == 10) || (hour == 11 && minute <= 30) {
		return true
	}
	if (hour == 13) || (hour == 14) || (hour == 15 && minute <= 0) {
		return true
	}
	return false
}

// IsHKTradingTime 判断当前是否处于港股交易时间
func (s *MarketService) IsHKTradingTime() bool {
	now := time.Now().In(shanghaiZone)
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	hour, minute, _ := now.Clock()
	if (hour == 9 && minute >= 0) || (hour >= 10 && hour < 12) || (hour == 12 && minute == 0) {
		return true
	}
	if (hour >= 13 && hour < 16) || (hour == 16 && minute <= 10) {
		return true
	}
	return false
}

// IsUSTradingTime 判断当前是否处于美股交易时间
func (s *MarketService) IsUSTradingTime() bool {
	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		return false
	}
	now := time.Now().In(est)
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	hour, minute, _ := now.Clock()
	// 盘前 4:00 - 9:30, 盘中 9:30 - 16:00, 盘后 16:00 - 20:00
	return (hour >= 4 && hour < 20) || (hour == 20 && minute == 0)
}

// IsTradingDay 判断指定 yyyy-MM-dd 是否为交易日
func (s *MarketService) IsTradingDay(date string) bool {
	date = strings.TrimSpace(date)
	if date == "" {
		return false
	}
	t, err := time.ParseInLocation("2006-01-02", date, shanghaiZone)
	if err != nil {
		return false
	}
	return t.Weekday() != time.Saturday && t.Weekday() != time.Sunday
}

// GetLatestTradingDay 获取最近一个已完成或进行中的交易日
func (s *MarketService) GetLatestTradingDay() string {
	now := time.Now().In(shanghaiZone)
	if now.Weekday() != time.Saturday && now.Weekday() != time.Sunday {
		hour, minute, _ := now.Clock()
		if hour < 15 || (hour == 15 && minute == 0) {
			return now.AddDate(0, 0, -1).Format("2006-01-02")
		}
		return now.Format("2006-01-02")
	}
	for i := 1; i <= 7; i++ {
		d := now.AddDate(0, 0, -i)
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			return d.Format("2006-01-02")
		}
	}
	return now.Format("2006-01-02")
}

// ----------------- 热榜、日历与资讯 -----------------

// HotStock 获取雪球热门股票排行
func (s *MarketService) HotStock(marketType string) *[]models.HotItem {
	return data.NewMarketNewsApi().XUEQIUHotStock(100, marketType)
}

// HotEvent 获取当前市场热点事件
func (s *MarketService) HotEvent(size int) *[]models.HotEvent {
	if size <= 0 {
		size = 10
	}
	return data.NewMarketNewsApi().HotEvent(size)
}

// HotTopic 获取热门话题
func (s *MarketService) HotTopic(size int) []any {
	if size <= 0 {
		size = 10
	}
	return data.NewMarketNewsApi().HotTopic(size)
}

// ClsCalendar 获取财联社投资日历
func (s *MarketService) ClsCalendar() []any {
	return data.NewMarketNewsApi().ClsCalendar()
}

// InvestCalendarTimeLine 获取指定年月的投资日历时间线
func (s *MarketService) InvestCalendarTimeLine(yearMonth string) []any {
	return data.NewMarketNewsApi().InvestCalendar(yearMonth)
}

// GetUplimitHot 获取涨停复盘与题材热度
func (s *MarketService) GetUplimitHot(date string, limit int) map[string]any {
	return data.NewMarketNewsApi().GetUplimitHot(date, limit)
}

// LongTigerRank 获取龙虎榜数据
func (s *MarketService) LongTigerRank(date string) *[]models.LongTigerRankData {
	return data.NewMarketNewsApi().LongTiger(date)
}

// StockResearchReport 获取个股近期券商研报
func (s *MarketService) StockResearchReport(stockCode string) []any {
	return data.NewMarketNewsApi().StockResearchReport(stockCode, 7)
}

// StockNotice 获取个股最新重要公告
func (s *MarketService) StockNotice(stockCode string) []any {
	return data.NewMarketNewsApi().StockNotice(stockCode)
}

// IndustryResearchReport 获取行业近期研报
func (s *MarketService) IndustryResearchReport(industryCode string) []any {
	return data.NewMarketNewsApi().IndustryResearchReport(industryCode, 7)
}

// EMDictCode 获取东财行业/概念字典码
func (s *MarketService) EMDictCode(code string) []any {
	return data.NewMarketNewsApi().EMDictCode(code, nil)
}

// ----------------- 板块资金流向 -----------------

// GetBKFundFlowList 获取板块资金流向历史数据（折线图用）
func (s *MarketService) GetBKFundFlowList(code string, limit int) []models.BKFundFlowPoint {
	return data.NewBKFundFlowApi().GetBKFundFlowList(code, limit)
}

// GetBKFundFlowListByDate 获取板块指定日期的资金流向历史数据
func (s *MarketService) GetBKFundFlowListByDate(code string, date string) []models.BKFundFlowPoint {
	return data.NewBKFundFlowApi().GetBKFundFlowListByDate(code, date)
}

// GetBKFundFlowTopList 获取最新板块资金排名
func (s *MarketService) GetBKFundFlowTopList(topN int) []models.BKFundFlow {
	return data.NewBKFundFlowApi().GetBKFundFlowTopList(topN)
}

// GetBKFundFlowTopListByDate 获取指定日期的板块资金排名
func (s *MarketService) GetBKFundFlowTopListByDate(date string, topN int) []models.BKFundFlow {
	return data.NewBKFundFlowApi().GetBKFundFlowTopListByDate(date, topN)
}

// GetAllBKCodes 获取所有已记录的板块代码
func (s *MarketService) GetAllBKCodes() []map[string]string {
	return data.NewBKFundFlowApi().GetAllBKCodes()
}

// GetConceptFundFlowList 获取概念资金流向历史数据（折线图用）
func (s *MarketService) GetConceptFundFlowList(code string, limit int) []models.ConceptFundFlowPoint {
	return data.NewConceptFundFlowApi().GetConceptFundFlowList(code, limit)
}

// GetConceptFundFlowListByDate 获取概念指定日期的资金流向历史数据
func (s *MarketService) GetConceptFundFlowListByDate(code string, date string) []models.ConceptFundFlowPoint {
	return data.NewConceptFundFlowApi().GetConceptFundFlowListByDate(code, date)
}

// GetConceptFundFlowTopList 获取最新概念资金排名
func (s *MarketService) GetConceptFundFlowTopList(topN int) []models.ConceptFundFlow {
	return data.NewConceptFundFlowApi().GetConceptFundFlowTopList(topN)
}

// GetConceptFundFlowTopListByDate 获取指定日期的概念资金排名
func (s *MarketService) GetConceptFundFlowTopListByDate(date string, topN int) []models.ConceptFundFlow {
	return data.NewConceptFundFlowApi().GetConceptFundFlowTopListByDate(date, topN)
}

// GetAllConceptCodes 获取所有概念代码
func (s *MarketService) GetAllConceptCodes() []map[string]string {
	return data.NewConceptFundFlowApi().GetAllConceptCodes()
}

// ----------------- 全市场选股、策略与情绪 -----------------

// GetAllStocks 分页获取全市场股票列表及技术指标
func (s *MarketService) GetAllStocks(page int, pageSize int, name string, technicalIndicators models.TechnicalIndicators) *models.AllStocksResp {
	return data.NewStockDataApi().GetAllStocks(page, pageSize, name, technicalIndicators)
}

// AnalyzeSentiment 分析单条财经文本情绪
func (s *MarketService) AnalyzeSentiment(text string) models.SentimentResult {
	return data.AnalyzeSentiment(text)
}

// AnalyzeSentimentWithFreqWeight 结合词频权重的深度情绪分析
func (s *MarketService) AnalyzeSentimentWithFreqWeight(text string) map[string]any {
	result, cleanFrequencies := data.NewsAnalyze(text, false)
	return map[string]any{
		"result":      result,
		"frequencies": cleanFrequencies,
	}
}

// SearchStock 全网与本地股票搜索联想
func (s *MarketService) SearchStock(words string) map[string]any {
	return data.NewSearchStockApi(words).SearchStock(5000)
}

// GetHotStrategy 获取当前热门选股策略
func (s *MarketService) GetHotStrategy() map[string]any {
	return data.NewSearchStockApi("").HotStrategy()
}

// GetCustomStrategyList 获取自定义选股策略列表
func (s *MarketService) GetCustomStrategyList(query models.CustomStrategyQuery) *models.CustomStrategyPageData {
	page, err := data.NewCustomStrategyApi().GetCustomStrategyList(&query)
	if err != nil {
		return &models.CustomStrategyPageData{}
	}
	return page
}

// GetAllCustomStrategies 获取所有自定义策略
func (s *MarketService) GetAllCustomStrategies() *[]models.CustomStrategy {
	return data.NewCustomStrategyApi().GetAllCustomStrategies()
}

// SaveCustomStrategy 保存自定义选股策略
func (s *MarketService) SaveCustomStrategy(strategy models.CustomStrategy) string {
	return data.NewCustomStrategyApi().SaveCustomStrategy(strategy)
}

// DeleteCustomStrategy 删除自定义选股策略
func (s *MarketService) DeleteCustomStrategy(id uint) string {
	return data.NewCustomStrategyApi().DeleteCustomStrategy(id)
}

// ----------------- 异动大盘与全景透视 -----------------

// GetStockChanges 获取盘口异动列表
func (s *MarketService) GetStockChanges(changeTypes []int, pageIndex, pageSize int) *data.StockChangesResponse {
	return data.NewStockChangesApi().GetStockChanges(changeTypes, pageIndex, pageSize)
}

// GetAllStockChangesWithPaging 分页获取所有异动并自动去重归档
func (s *MarketService) GetAllStockChangesWithPaging(pageSize int) *data.StockChangesResponse {
	all := data.NewStockChangesApi().GetAllStockChangesWithPaging(pageSize)
	historyService := data.NewStockChangeHistoryService()
	if all != nil && len(all.Data) > 0 {
		_, _ = historyService.SaveStockChangesWithDedup(all.Data)
	}
	return all
}

// GetStockChangeHistory 查询异动历史归档
func (s *MarketService) GetStockChangeHistory(query models.StockChangeHistoryQuery) *models.StockChangeHistoryPageData {
	result, err := data.NewStockChangeHistoryService().GetHistoryList(query)
	if err != nil {
		return &models.StockChangeHistoryPageData{}
	}
	return result
}

// SaveStockChangesToHistory 将异动记录手动保存至历史库
func (s *MarketService) SaveStockChangesToHistory(changeTypes []int) string {
	api := data.NewStockChangesApi()
	result := api.GetStockChanges(changeTypes, 0, 500)
	if result == nil || len(result.Data) == 0 {
		return "没有获取到异动数据"
	}

	err := data.NewStockChangeHistoryService().SaveStockChanges(result.Data)
	if err != nil {
		return "保存失败: " + err.Error()
	}
	return fmt.Sprintf("成功保存 %d 条异动数据", len(result.Data))
}

// DeleteStockChangeHistory 清理指定天数前的异动历史
func (s *MarketService) DeleteStockChangeHistory(days int) string {
	err := data.NewStockChangeHistoryService().DeleteOldData(days)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return fmt.Sprintf("已删除 %d 天前的历史数据", days)
}

// GetDailyChangeStats 统计每日异动数量趋势
func (s *MarketService) GetDailyChangeStats(days int) []data.DailyChangeStats {
	result, err := data.NewStockChangeHistoryService().GetDailyChangeStats(days)
	if err != nil {
		return []data.DailyChangeStats{}
	}
	return result
}

// GetChangeTypeDailyStats 按异动类型统计每日分布
func (s *MarketService) GetChangeTypeDailyStats(days int) []data.ChangeTypeDailyStats {
	result, err := data.NewStockChangeHistoryService().GetChangeTypeDailyStats(days)
	if err != nil {
		return []data.ChangeTypeDailyStats{}
	}
	return result
}

// GetChangeRank 获取异动频次个股排行榜
func (s *MarketService) GetChangeRank(days int, topN int) *data.ChangeRankResult {
	result, err := data.NewStockChangeHistoryService().GetChangeRank(days, topN)
	if err != nil {
		return &data.ChangeRankResult{}
	}
	return result
}

// GetDailyDimensionStats 获取指定维度的异动聚合
func (s *MarketService) GetDailyDimensionStats(dimension string, name string, days int) []data.DailyDimensionStats {
	result, err := data.NewStockChangeHistoryService().GetDailyDimensionStats(dimension, name, days)
	if err != nil {
		return []data.DailyDimensionStats{}
	}
	return result
}

// GetTypeStatsByDate 获取指定日期各类型异动总量
func (s *MarketService) GetTypeStatsByDate(date string) []data.TypeCountStats {
	result, err := data.NewStockChangeHistoryService().GetTypeStatsByDate(date)
	if err != nil {
		return []data.TypeCountStats{}
	}
	return result
}

// ----------------- 推荐股票与股票基础全库 -----------------

// GetAiRecommendStocksList 获取 AI 推荐自选股票列表
func (s *MarketService) GetAiRecommendStocksList(query models.AiRecommendStocksQuery) *models.AiRecommendStocksPageData {
	page, err := data.NewAiRecommendStocksService().GetAiRecommendStocksList(&query)
	if err != nil {
		return &models.AiRecommendStocksPageData{}
	}
	return page
}

// DeleteAiRecommendStocks 删除 AI 推荐自选股
func (s *MarketService) DeleteAiRecommendStocks(id uint) string {
	err := data.NewAiRecommendStocksService().DeleteAiRecommendStocks(id)
	if err != nil {
		return "删除失败"
	}
	return "删除成功"
}

// UpdateAiRecommendStocksAlert 更新 AI 推荐股票预警状态
func (s *MarketService) UpdateAiRecommendStocksAlert(id uint, enableAlert bool) string {
	err := data.NewAiRecommendStocksService().UpdateAiRecommendStocksAlert(id, enableAlert)
	if err != nil {
		return "更新预警状态失败"
	}
	return "更新预警状态成功"
}

// GetAllStockInfoList 分页获取全量基础股票库
func (s *MarketService) GetAllStockInfoList(query data.AllStockInfoQuery) *data.AllStockInfoPageData {
	page, err := data.NewStockDataApi().GetAllStockInfoList(&query)
	if err != nil {
		return &data.AllStockInfoPageData{}
	}
	return page
}

// GetAllStockInfoById 获取单只基础股票详情
func (s *MarketService) GetAllStockInfoById(id uint) *models.AllStockInfo {
	stock, err := data.NewStockDataApi().GetAllStockInfoById(id)
	if err != nil {
		return &models.AllStockInfo{}
	}
	return stock
}

// AddAllStockInfo 添加基础股票条目
func (s *MarketService) AddAllStockInfo(stock models.AllStockInfo) string {
	err := data.NewStockDataApi().AddAllStockInfo(stock)
	if err != nil {
		return "操作失败: " + err.Error()
	}
	return "操作成功"
}

// DeleteAllStockInfo 删除基础股票条目
func (s *MarketService) DeleteAllStockInfo(id uint) string {
	err := data.NewStockDataApi().DeleteAllStockInfo(id)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

// BatchDeleteAllStockInfo 批量删除基础股票条目
func (s *MarketService) BatchDeleteAllStockInfo(ids []uint) string {
	err := data.NewStockDataApi().BatchDeleteAllStockInfo(ids)
	if err != nil {
		return "批量删除失败: " + err.Error()
	}
	return "批量删除成功"
}

// GetAllMarkets 获取全量市场标签
func (s *MarketService) GetAllMarkets() []string {
	markets, err := data.NewStockDataApi().GetAllMarkets()
	if err != nil {
		return []string{}
	}
	return markets
}

// GetAllIndustries 获取全量行业分类
func (s *MarketService) GetAllIndustries() []string {
	industries, err := data.NewStockDataApi().GetAllIndustries()
	if err != nil {
		return []string{}
	}
	return industries
}

// GetAllConcepts 获取全量概念板块标签
func (s *MarketService) GetAllConcepts() []string {
	concepts, err := data.NewStockDataApi().GetAllConcepts()
	if err != nil {
		return []string{}
	}
	return concepts
}

// GetStockRealTimePrice 获取股票当前最新快照价格
func (s *MarketService) GetStockRealTimePrice(stockCode string) map[string]any {
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
		return map[string]any{
			"code":    -1,
			"message": "获取股票价格失败",
			"price":   0,
		}
	}
	stock := (*stockDatas)[0]
	price, _ := convertor.ToFloat(stock.Price)
	if price == 0 {
		price, _ = convertor.ToFloat(stock.A1P)
	}
	if price == 0 {
		price, _ = convertor.ToFloat(stock.B1P)
	}
	if price == 0 {
		price, _ = convertor.ToFloat(stock.PreClose)
	}
	return map[string]any{
		"code":    0,
		"message": "success",
		"price":   price,
		"name":    stock.Name,
	}
}

// ----------------- 研报历史记录管理 -----------------

// GetAIResponseResultList 获取 AI 研报历史生成记录
func (s *MarketService) GetAIResponseResultList(query models.AIResponseResultQuery) *models.AIResponseResultPageData {
	page, err := data.NewAIResponseResultService().GetAIResponseResultList(query)
	if err != nil {
		return &models.AIResponseResultPageData{}
	}
	return page
}

// DeleteAIResponseResult 删除指定研报历史
func (s *MarketService) DeleteAIResponseResult(id uint) string {
	err := data.NewAIResponseResultService().DeleteAIResponseResult(id)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

// BatchDeleteAIResponseResult 批量删除研报历史
func (s *MarketService) BatchDeleteAIResponseResult(ids []uint) string {
	err := data.NewAIResponseResultService().BatchDeleteAIResponseResult(ids)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

// GetEastMoneyKLineWithMA 东方财富 K 线 + 均线数据
func (s *MarketService) GetEastMoneyKLineWithMA(stockCode, kLineType string, days int) map[string]any {
	if days <= 0 {
		days = 60
	}
	var content string
	if data.IsAStockCode(stockCode) {
		content = data.FetchKLineWithMASection(stockCode, data.NormalizeKLineType(kLineType), days, "")
	} else {
		api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
		content = data.EastMoneyKLineWithMASection(api, stockCode, kLineType, days, "")
	}
	return map[string]any{
		"code": 0,
		"data": content,
	}
}

// GetStockFinancialInfo 获取个股主要财务指标
func (s *MarketService) GetStockFinancialInfo(stockCode string) map[string]any {
	info := data.NewStockDataApi().GetStockFinancialInfo(stockCode)
	return map[string]any{
		"code": 0,
		"data": info,
	}
}

// GetStockLatestFinance 获取个股最新财务季报 F10 数据 (Markdown)
func (s *MarketService) GetStockLatestFinance(stockCode string) map[string]any {
	content := data.NewStockDataApi().GetStockLatestFinanceToMarkdown(stockCode)
	return map[string]any{
		"code": 0,
		"data": content,
	}
}

// GetStockHolderTrend 获取个股股东户数增减态势 (Markdown)
func (s *MarketService) GetStockHolderTrend(stockCode string) map[string]any {
	content := data.NewStockDataApi().GetStockHolderTrendToMarkdown(stockCode)
	return map[string]any{
		"code": 0,
		"data": content,
	}
}

// SearchStockByIndicators 多维度技术与财务指标选股
func (s *MarketService) SearchStockByIndicators(indicators map[string]any) map[string]any {
	words := ""
	if w, ok := indicators["words"].(string); ok {
		words = w
	} else if k, ok := indicators["keyword"].(string); ok {
		words = k
	}
	res := data.NewSearchStockApi(words).SearchStock(100)
	return map[string]any{
		"code": 0,
		"data": res,
	}
}

// FilterStocks 条件选股筛选
func (s *MarketService) FilterStocks(filter map[string]any) map[string]any {
	page := 1
	pageSize := 20
	keyword := ""
	if p, ok := filter["page"].(float64); ok && p > 0 {
		page = int(p)
	} else if p, ok := filter["page"].(int); ok && p > 0 {
		page = p
	}
	if ps, ok := filter["pageSize"].(float64); ok && ps > 0 {
		pageSize = int(ps)
	} else if ps, ok := filter["pageSize"].(int); ok && ps > 0 {
		pageSize = ps
	}
	if k, ok := filter["keyword"].(string); ok {
		keyword = k
	}

	tech := models.TechnicalIndicators{}
	if b, ok := filter["macdGoldenFork"].(bool); ok {
		tech.MACDGOLDENFORK = b
	}
	if b, ok := filter["kdjGoldenFork"].(bool); ok {
		tech.KDJGOLDENFORK = b
	}
	if b, ok := filter["breakThrough"].(bool); ok {
		tech.BREAKTHROUGH = b
	}
	if b, ok := filter["lowFundsInflow"].(bool); ok {
		tech.LOWFUNDSINFLOW = b
	}
	if b, ok := filter["highFundsOutflow"].(bool); ok {
		tech.HIGHFUNDSOUTFLOW = b
	}
	if b, ok := filter["breakUpMa5Days"].(bool); ok {
		tech.BREAKUPMA5DAYS = b
	}
	if b, ok := filter["longAvgArray"].(bool); ok {
		tech.LONGAVGARRAY = b
	}
	if b, ok := filter["shortAvgArray"].(bool); ok {
		tech.SHORTAVGARRAY = b
	}
	if b, ok := filter["upperLargeVolume"].(bool); ok {
		tech.UPPERLARGEVOLUME = b
	}
	if b, ok := filter["downNarrowVolume"].(bool); ok {
		tech.DOWNNARROWVOLUME = b
	}
	if b, ok := filter["morningStar"].(bool); ok {
		tech.MORNINGSTAR = b
	}
	if b, ok := filter["eveningStar"].(bool); ok {
		tech.EVENINGSTAR = b
	}

	res := data.NewStockDataApi().GetAllStocks(page, pageSize, keyword, tech)
	return map[string]any{
		"code": 0,
		"data": res,
	}
}

// WebSearch 联网投研检索
func (s *MarketService) WebSearch(query string, maxResults int) map[string]any {
	if maxResults <= 0 {
		maxResults = 10
	}
	api := data.NewWebSearchApi(30)
	results := api.Search(query, maxResults)
	return map[string]any{
		"code": 0,
		"data": results,
	}
}
