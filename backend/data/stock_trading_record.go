package data

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"github.com/duke-git/lancet/v2/convertor"
	"gorm.io/gorm"
)

// AddTradingRecord 添加交易日志
func (receiver StockDataApi) AddTradingRecord(record TradingRecord) (uint, error) {
	record.TradingTime = record.TradingTime.In(time.Local)

	// 检查频繁交易
	if record.Direction == "买入" {
		canTrade, msg := receiver.CheckFrequentTrading(record.StockCode)
		if !canTrade {
			return 0, fmt.Errorf("%s", msg)
		}
	}

	// 自动计算金额（价格 * 数量）
	record.Amount = record.Price * float64(record.Volume)

	// 设置交易时间为当前时间（如果未提供）
	if record.TradingTime.IsZero() {
		record.TradingTime = time.Now()
	}

	receiver.fillTradingRecordCloseSnapshot(&record)

	// 保存到数据库
	err := db.Dao.Model(&TradingRecord{}).Create(&record).Error
	if err != nil {
		logger.SugaredLogger.Errorf("添加交易日志失败: %s", err.Error())
		return 0, err
	}

	return record.ID, nil
}

// tradingRecordFIFOLot 交易日志先入先出批次

// fifoAvgUnitCost 按先入先出计算卖出数量对应的单位持仓成本（不修改批次）
func fifoAvgUnitCost(lots []tradingRecordFIFOLot, sellVol int64) (avg float64, ok bool) {
	if sellVol <= 0 {
		return 0, false
	}
	var cost float64
	var got int64
	for i := range lots {
		if got >= sellVol {
			break
		}
		if lots[i].Volume <= 0 {
			continue
		}
		need := sellVol - got
		take := need
		if take > lots[i].Volume {
			take = lots[i].Volume
		}
		cost += float64(take) * lots[i].Price
		got += take
	}
	if got < sellVol {
		return 0, false
	}
	return cost / float64(got), true
}

// normalizeTradingRecordAPI 将交易日志中的代码转为实时/K 线接口使用的代码

// normalizeTradingRecordAPI 将交易日志中的代码转为实时/K 线接口使用的代码
func normalizeTradingRecordAPI(stockCode string) string {
	apiCode := stockCode
	if strings.Contains(apiCode, " - ") {
		apiCode = strings.Split(apiCode, " - ")[0]
	}
	apiCode = strings.ToLower(apiCode)
	if strings.HasSuffix(apiCode, ".sh") {
		apiCode = "sh" + strings.TrimSuffix(apiCode, ".sh")
	} else if strings.HasSuffix(apiCode, ".sz") {
		apiCode = "sz" + strings.TrimSuffix(apiCode, ".sz")
	} else if strings.HasSuffix(apiCode, ".bj") {
		apiCode = "bj" + strings.TrimSuffix(apiCode, ".bj")
	} else if strings.HasPrefix(apiCode, "6") || len(apiCode) == 6 {
		apiCode = "sh" + apiCode
	} else if strings.HasPrefix(apiCode, "0") || strings.HasPrefix(apiCode, "3") {
		apiCode = "sz" + apiCode
	} else if strings.HasPrefix(apiCode, "4") || strings.HasPrefix(apiCode, "8") {
		apiCode = "bj" + apiCode
	}
	return apiCode
}

// resolveTradingRecordClosePrice 按交易日期解析收盘价或现价（无缓存，供写入快照与列表补拉共用）

// resolveTradingRecordClosePrice 按交易日期解析收盘价或现价（无缓存，供写入快照与列表补拉共用）
func (receiver StockDataApi) resolveTradingRecordClosePrice(apiCode string, tradingTime time.Time, fallback float64) float64 {
	if strings.TrimSpace(apiCode) == "" {
		return fallback
	}
	tradingTime = tradingTime.In(time.Local)
	now := time.Now()
	tradingDateStr := tradingTime.Format("2006-01-02")
	todayStr := now.Format("2006-01-02")
	closePrice := fallback
	isToday := tradingDateStr == todayStr
	isFuture := tradingTime.After(now)
	if isToday || isFuture {
		stockDatas, err := receiver.GetStockCodeRealTimeData(apiCode)
		if err == nil && stockDatas != nil && len(*stockDatas) > 0 {
			price, _ := convertor.ToFloat((*stockDatas)[0].Price)
			if price > 0 {
				closePrice = price
			}
		}
	} else {
		klines := receiver.GetCommonKLineData(apiCode, "day", 30)
		if klines != nil {
			for _, k := range *klines {
				if k.Day == tradingDateStr {
					cp, _ := convertor.ToFloat(k.Close)
					if cp > 0 {
						closePrice = cp
					}
					break
				}
			}
		}
	}
	return closePrice
}

// fillTradingRecordCloseSnapshot 写入/刷新记录的收盘价快照（添加、修改时调用）

// fillTradingRecordCloseSnapshot 写入/刷新记录的收盘价快照（添加、修改时调用）
func (receiver StockDataApi) fillTradingRecordCloseSnapshot(record *TradingRecord) {
	if record == nil {
		return
	}
	apiCode := normalizeTradingRecordAPI(record.StockCode)
	record.RecordedClosePrice = receiver.resolveTradingRecordClosePrice(apiCode, record.TradingTime, record.Price)
}

// GetTradingRecordList 获取交易日志列表（分页、关键词、方向、交易日期范围）

// GetTradingRecordList 获取交易日志列表（分页、关键词、方向、交易日期范围）
func (receiver StockDataApi) GetTradingRecordList(query TradingRecordListQuery) (*TradingRecordPageData, error) {
	var records []TradingRecord
	q := db.Dao.Model(&TradingRecord{})

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("stock_code LIKE ? OR stock_name LIKE ?", like, like)
	}
	if dir := strings.TrimSpace(query.Direction); dir != "" {
		q = q.Where("direction = ?", dir)
	}
	if sd := strings.TrimSpace(query.StartDate); sd != "" {
		if start, err := time.ParseInLocation("2006-01-02", sd, time.Local); err == nil {
			q = q.Where("trading_time >= ?", start)
		}
	}
	if ed := strings.TrimSpace(query.EndDate); ed != "" {
		if end, err := time.ParseInLocation("2006-01-02", ed, time.Local); err == nil {
			q = q.Where("trading_time < ?", end.Add(24*time.Hour))
		}
	}

	var total int64
	err := q.Count(&total).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取交易日志总数失败: %s", err.Error())
		return nil, err
	}

	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("trading_time DESC, id DESC").Find(&records).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取交易日志列表失败: %s", err.Error())
		return nil, err
	}

	needProfitByID := make(map[uint]struct{}, len(records))
	for _, r := range records {
		needProfitByID[r.ID] = struct{}{}
	}

	var allGlobal []TradingRecord
	if err := db.Dao.Model(&TradingRecord{}).Order("trading_time ASC, id ASC").Find(&allGlobal).Error; err != nil {
		logger.SugaredLogger.Errorf("获取交易日志全局序失败: %s", err.Error())
		return nil, err
	}

	type rowProfit struct {
		closePrice    float64
		profitAmount  float64
		profitPercent float64
	}
	profitByID := make(map[uint]rowProfit, len(records))

	closeCache := make(map[string]float64)

	resolveClose := func(apiCode string, tradingTime time.Time, fallback float64, recorded float64) float64 {
		// 当天或未来日期的记录始终获取实时行情，不使用缓存快照
		tradingDateStr := tradingTime.Format("2006-01-02")
		key := apiCode + "|" + tradingDateStr
		if tradingDateStr == time.Now().Format("2006-01-02") || tradingTime.After(time.Now()) {
			closePrice := receiver.resolveTradingRecordClosePrice(apiCode, tradingTime, fallback)
			closeCache[key] = closePrice
			return closePrice
		}
		// 历史记录优先使用已保存的快照
		if recorded > 0 {
			return recorded
		}
		if v, ok := closeCache[key]; ok {
			return v
		}
		closePrice := receiver.resolveTradingRecordClosePrice(apiCode, tradingTime, fallback)
		closeCache[key] = closePrice
		return closePrice
	}

	stockHoldings := make(map[string][]tradingRecordFIFOLot)
	// 需要回写收盘价快照的历史记录：RecordedClosePrice == 0 且成功获取到 closePrice
	type closeBackfill struct {
		id         uint
		closePrice float64
	}
	var backfills []closeBackfill
	todayStr := time.Now().Format("2006-01-02")
	now := time.Now()

	for _, r := range allGlobal {
		_, need := needProfitByID[r.ID]
		apiCode := normalizeTradingRecordAPI(r.StockCode)
		tradingDateStr := r.TradingTime.In(time.Local).Format("2006-01-02")

		if r.Direction == "买入" {
			if need {
				closePrice := resolveClose(apiCode, r.TradingTime, r.Price, r.RecordedClosePrice)
				// 历史交易日：把解析到的收盘价落库，避免每次列表重复拉 K 线
				if r.RecordedClosePrice == 0 && closePrice > 0 && tradingDateStr != todayStr && !r.TradingTime.After(now) {
					backfills = append(backfills, closeBackfill{id: r.ID, closePrice: closePrice})
				}
				if r.Price > 0 {
					profitByID[r.ID] = rowProfit{
						closePrice:    closePrice,
						profitAmount:  (closePrice-r.Price)*float64(r.Volume) - r.Fee,
						profitPercent: (closePrice - r.Price) / r.Price * 100,
					}
				} else {
					profitByID[r.ID] = rowProfit{closePrice: closePrice}
				}
			}
			stockHoldings[r.StockCode] = append(stockHoldings[r.StockCode], tradingRecordFIFOLot{Volume: r.Volume, Price: r.Price})
		} else if r.Direction == "卖出" {
			if need {
				avgCost, ok := fifoAvgUnitCost(stockHoldings[r.StockCode], r.Volume)
				closePrice := resolveClose(apiCode, r.TradingTime, r.Price, r.RecordedClosePrice)
				if r.RecordedClosePrice == 0 && closePrice > 0 && tradingDateStr != todayStr && !r.TradingTime.After(now) {
					backfills = append(backfills, closeBackfill{id: r.ID, closePrice: closePrice})
				}
				if ok && avgCost > 0 {
					profitByID[r.ID] = rowProfit{
						closePrice:    closePrice,
						profitAmount:  (r.Price-avgCost)*float64(r.Volume) - r.Fee,
						profitPercent: (r.Price - avgCost) / avgCost * 100,
					}
				} else {
					profitByID[r.ID] = rowProfit{closePrice: closePrice}
				}
			}
			remaining := r.Volume
			for i := range stockHoldings[r.StockCode] {
				if remaining == 0 {
					break
				}
				rec := &stockHoldings[r.StockCode][i]
				if rec.Volume <= remaining {
					remaining -= rec.Volume
					rec.Volume = 0
				} else {
					rec.Volume -= remaining
					remaining = 0
				}
			}
		}
	}

	seenBackfill := make(map[uint]struct{}, len(backfills))
	for _, bf := range backfills {
		if _, dup := seenBackfill[bf.id]; dup {
			continue
		}
		seenBackfill[bf.id] = struct{}{}
		res := db.Dao.Model(&TradingRecord{}).Where("id = ? AND (recorded_close_price IS NULL OR recorded_close_price = 0)", bf.id).
			Update("recorded_close_price", bf.closePrice)
		if res.Error != nil {
			logger.SugaredLogger.Warnf("回写交易记录收盘价快照失败 id=%d: %s", bf.id, res.Error.Error())
		}
	}

	items := make([]TradingRecordItem, 0, len(records))
	for _, r := range records {
		r.Amount = r.Price * float64(r.Volume)
		item := TradingRecordItem{TradingRecord: r}
		if p, ok := profitByID[r.ID]; ok {
			item.ClosePrice = p.closePrice
			item.ProfitAmount = p.profitAmount
			item.ProfitPercent = p.profitPercent
		}
		items = append(items, item)
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	return &TradingRecordPageData{
		List:       items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetTradingRecordStatistics 获取交易日志统计数据

// GetTradingRecordStatistics 获取交易日志统计数据
func (receiver StockDataApi) GetTradingRecordStatistics() (*TradingRecordStatistics, error) {
	type BuyRecord struct {
		Volume int64
		Price  float64
	}

	var records []TradingRecord
	err := db.Dao.Model(&TradingRecord{}).Order("trading_time ASC, id ASC").Find(&records).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取交易日志统计失败: %s", err.Error())
		return nil, err
	}

	stockMap := make(map[string][]BuyRecord)
	totalBuyAmount := 0.0
	totalSellAmount := 0.0
	holdingsCost := 0.0
	holdingsValue := 0.0
	costOfSoldShares := 0.0

	for _, r := range records {
		amount := r.Price * float64(r.Volume)
		if r.Direction == "买入" {
			totalBuyAmount += amount
			stockMap[r.StockCode] = append(stockMap[r.StockCode], BuyRecord{Volume: r.Volume, Price: r.Price})
		} else if r.Direction == "卖出" {
			totalSellAmount += amount
			remainingVolume := r.Volume
			for i := range stockMap[r.StockCode] {
				if remainingVolume == 0 {
					break
				}
				record := &stockMap[r.StockCode][i]
				if record.Volume <= remainingVolume {
					costOfSoldShares += float64(record.Volume) * record.Price
					remainingVolume -= record.Volume
					record.Volume = 0
				} else {
					costOfSoldShares += float64(remainingVolume) * record.Price
					record.Volume -= remainingVolume
					remainingVolume = 0
				}
			}
		}
	}

	var stockCount int64
	for code, buyRecords := range stockMap {
		currentVolume := int64(0)
		currentCost := 0.0
		for _, br := range buyRecords {
			if br.Volume > 0 {
				currentVolume += br.Volume
				currentCost += float64(br.Volume) * br.Price
			}
		}
		if currentVolume > 0 {
			stockCount++
			holdingsCost += currentCost

			apiCode := normalizeTradingRecordAPI(code)
			stockDatas, err := receiver.GetStockCodeRealTimeData(apiCode)
			if err == nil && stockDatas != nil && len(*stockDatas) > 0 {
				stock := (*stockDatas)[0]
				price, _ := convertor.ToFloat(stock.Price)
				if price == 0 {
					price, _ = convertor.ToFloat(stock.A1P)
				}
				if price > 0 {
					holdingsValue += price * float64(currentVolume)
				}
			}
		}
	}

	totalProfit := totalSellAmount - costOfSoldShares + (holdingsValue - holdingsCost)
	profitRate := 0.0
	denom := holdingsCost
	if denom <= 0 && costOfSoldShares > 0 {
		denom = costOfSoldShares
	}
	if denom <= 0 && totalBuyAmount > 0 {
		denom = totalBuyAmount
	}
	if denom > 0 {
		profitRate = (totalProfit / denom) * 100
	}

	return &TradingRecordStatistics{
		TotalBuyAmount:  totalBuyAmount,
		TotalSellAmount: totalSellAmount,
		TotalProfit:     totalProfit,
		ProfitRate:      profitRate,
		HoldingsAmount:  holdingsCost,
		CurrentValue:    holdingsValue,
		StockCount:      stockCount,
	}, nil
}

// GetTradingRecordById 根据ID获取单个交易日志

// GetTradingRecordById 根据ID获取单个交易日志
func (receiver StockDataApi) GetTradingRecordById(id uint) (*TradingRecord, error) {
	var record TradingRecord
	err := db.Dao.Model(&TradingRecord{}).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.SugaredLogger.Errorf("获取交易日志失败: %s", err.Error())
		return nil, err
	}
	return &record, nil
}

// UpdateTradingRecord 更新交易日志

// UpdateTradingRecord 更新交易日志
func (receiver StockDataApi) UpdateTradingRecord(record TradingRecord) error {
	logger.SugaredLogger.Infof("UpdateTradingRecord: %v", record)
	// 自动计算金额（价格 * 数量）
	record.Amount = record.Price * float64(record.Volume)

	record.TradingTime = record.TradingTime.In(time.Local)

	receiver.fillTradingRecordCloseSnapshot(&record)

	err := db.Dao.Model(&TradingRecord{}).Where("id = ?", record.ID).Updates(&record).Error
	if err != nil {
		logger.SugaredLogger.Errorf("更新交易日志失败: %s", err.Error())
		return err
	}
	// Updates(struct) 会忽略零值字段，收盘价快照单独写入保证落库
	if err := db.Dao.Model(&TradingRecord{}).Where("id = ?", record.ID).
		Update("recorded_close_price", record.RecordedClosePrice).Error; err != nil {
		logger.SugaredLogger.Errorf("更新交易日志收盘价快照失败: %s", err.Error())
		return err
	}
	return nil
}

// DeleteTradingRecord 删除交易日志

// DeleteTradingRecord 删除交易日志
func (receiver StockDataApi) DeleteTradingRecord(id uint) error {
	err := db.Dao.Model(&TradingRecord{}).Where("id = ?", id).Delete(&TradingRecord{}).Error
	if err != nil {
		logger.SugaredLogger.Errorf("删除交易日志失败: %s", err.Error())
		return err
	}
	return nil
}

// CheckFrequentTrading 检查是否频繁交易
// 返回值：(是否可以交易, 提示消息)

// CheckFrequentTrading 检查是否频繁交易
// 返回值：(是否可以交易, 提示消息)
func (receiver StockDataApi) CheckFrequentTrading(stockCode string) (bool, string) {
	// 检查最近24小时内是否有同一只股票的交易日志
	var count int64
	cutoffTime := time.Now().Add(-24 * time.Hour)

	err := db.Dao.Model(&TradingRecord{}).Where("stock_code = ? AND direction = ? AND trading_time > ?", stockCode, "买入", cutoffTime).Count(&count).Error
	if err != nil {
		logger.SugaredLogger.Errorf("检查频繁交易失败: %s", err.Error())
		return true, "检查频繁交易失败，默认允许交易"
	}

	if count > 0 {
		return false, "最近24小时内已对该股票进行过买入操作，为避免频繁交易，建议稍后再操作"
	}

	// 检查最近7天内的交易次数是否超过限制（例如：5次）
	cutoffTime7Days := time.Now().Add(-7 * 24 * time.Hour)
	err = db.Dao.Model(&TradingRecord{}).Where("direction = ? AND trading_time > ?", "买入", cutoffTime7Days).Count(&count).Error
	if err != nil {
		logger.SugaredLogger.Errorf("检查频繁交易失败: %s", err.Error())
		return true, "检查频繁交易失败，默认允许交易"
	}

	if count >= 5 {
		return false, "最近7天内交易次数已达上限（5次），为避免频繁交易，建议稍后再操作"
	}

	return true, "可以交易"
}
