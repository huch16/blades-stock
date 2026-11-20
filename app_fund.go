package main

import (
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"time"
)

func MonitorFundPrices(a *App) {
	defer PanicHandler()

	// 检查 A 股是否开市（基金交易时间与 A 股一致）
	if !isTradingTime(time.Now()) {
		// logger.SugaredLogger.Debugf("当前 A 股未开市，跳过基金价格监控") // suppressed to reduce noise
		return
	}

	logger.SugaredLogger.Debugf("A 股市场已开市，开始基金价格监控")

	dest := &[]data.FollowedFund{}
	db.Dao.Model(&data.FollowedFund{}).Find(dest)
	for _, follow := range *dest {
		_, err := a.marketService.CrawlFundBasic(follow.Code)
		if err != nil {
			logger.SugaredLogger.Errorf("获取基金基本信息失败，基金代码：%s，错误信息：%s", follow.Code, err.Error())
			continue
		}
		a.marketService.CrawlFundNetEstimatedUnit(follow.Code)
		a.marketService.CrawlFundNetUnitValue(follow.Code)
	}
}

// MonitorAiRecommendStockPrices 监控 AI 推荐股票的价格，当股价达到预警线时发送通知

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

func (a *App) GetFundTop(fundCode string) []data.FundHoldingStock {
	return a.marketService.GetFundTop(fundCode)
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
