package data

import (
	"blades-stock/backend/logger"

	"fmt"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

func (t *TdxKLineApi) GetCallAuction(stockCode string, start uint32, count uint32) *[]TdxCallAuctionData {
	result := &[]TdxCallAuctionData{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	if count <= 0 {
		count = 500
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	list, err := t.client.StockAuction(market, code, start, count)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockAuction error: %v, reconnecting...", err)
		for attempt := 1; attempt <= 3; attempt++ {
			if err := t.reconnect(); err != nil {
				logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", err)
				if attempt < 3 {
					tdxRetryDelay(attempt)
					continue
				}
				return result
			}
			t.mu.Lock()
			list, err = t.client.StockAuction(market, code, start, count)
			t.mu.Unlock()
			if err == nil {
				break
			}
			logger.SugaredLogger.Warnf("TdxKLine StockAuction retry #%d error: %v", attempt+1, err)
			tdxRetryDelay(attempt)
		}
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockAuction exhaust retry: %v", err)
			return result
		}
	}

	converted := convertAuctionData(list)
	return &converted
}

func convertAuctionData(list []proto.AuctionData) []TdxCallAuctionData {
	result := make([]TdxCallAuctionData, 0, len(list))
	for _, item := range list {
		flagStr := "买盘"
		if item.Flag < 0 {
			flagStr = "卖盘"
		}
		result = append(result, TdxCallAuctionData{
			Time:      item.Time,
			Price:     fmt.Sprintf("%.2f", item.Price),
			Matched:   fmt.Sprintf("%d", item.Matched),
			Unmatched: fmt.Sprintf("%d", item.Unmatched),
			Flag:      flagStr,
		})
	}
	return result
}

func (t *TdxKLineApi) GetKLineData(stockCode string, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	if limit <= 0 {
		limit = 500
	}
	market, code := tdxMarketFromStockCode(stockCode)

	aggSrc, aggN := tdxAggregationParams(klt)
	actualKlt := klt
	if aggSrc != "" {
		actualKlt = aggSrc
	}

	klineType := tdxKLineTypeFromKlt(actualKlt)
	if klineType < 0 {
		logger.SugaredLogger.Warnf("TdxKLine: unsupported klt %s", klt)
		return result
	}

	fetchCount := limit
	if aggN > 1 {
		fetchCount = limit * aggN
		if fetchCount > 8000 {
			fetchCount = 8000
		}
	}

	t.mu.Lock()
	bars, err := t.client.StockKLine(uint16(klineType), market, code, 0, uint16(fetchCount), 0, types.AdjustQFQ)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockKLine error: %v, reconnecting...", err)
		for attempt := 1; attempt <= 3; attempt++ {
			if err := t.reconnect(); err != nil {
				logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", err)
				if attempt < 3 {
					tdxRetryDelay(attempt)
					continue
				}
				return result
			}
			t.mu.Lock()
			bars, err = t.client.StockKLine(uint16(klineType), market, code, 0, uint16(fetchCount), 0, types.AdjustQFQ)
			t.mu.Unlock()
			if err == nil {
				break
			}
			logger.SugaredLogger.Warnf("TdxKLine StockKLine retry #%d error: %v", attempt+1, err)
			tdxRetryDelay(attempt)
		}
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockKLine exhaust retry: %v", err)
			return result
		}
	}

	if len(bars) == 0 {
		return result
	}

	converted := convertTdxKLine(bars, actualKlt)

	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}

	return &converted
}

func tdxKLineTypeFromKlt(klt string) int {
	switch klt {
	case "1":
		return 8
	case "5":
		return 0
	case "15":
		return 1
	case "30":
		return 2
	case "60":
		return 3
	case "101":
		return 4
	case "102":
		return 5
	case "103":
		return 6
	case "104":
		return 10
	case "106":
		return 11
	default:
		return -1
	}
}

func tdxAggregationParams(klt string) (srcKlt string, n int) {
	switch klt {
	case "10":
		return "1", 10
	case "120":
		return "60", 2
	case "105":
		return "102", 26
	default:
		return "", 1
	}
}

// GetMACKLineData 通过 MAC 行情接口获取 K 线数据
// A股使用 MAC 客户端，港美股使用 MAC Ex 客户端
// 港股同时在 MAC 和 MAC Ex 上尝试

// GetMACKLineData 通过 MAC 行情接口获取 K 线数据
// A股使用 MAC 客户端，港美股使用 MAC Ex 客户端
// 港股同时在 MAC 和 MAC Ex 上尝试
func (t *TdxKLineApi) GetMACKLineData(stockCode string, klt string, limit int) *[]KLineData {
	if limit <= 0 {
		limit = 500
	}

	// 判断是否港美股
	if exMarket, exCode, ok := macExMarketFromStockCode(stockCode); ok {
		// 港股：先尝试 MAC 主服务器（MarketHK=3），再尝试扩展行情 ExKLine2（主板=31/创业板=48）
		if IsHKStockCode(stockCode) {
			data := t.getMACMainKLineData(uint8(types.MarketHK), exCode, klt, limit)
			if data != nil && len(*data) > 0 {
				return data
			}
		}
		// MAC Ex 扩展行情
		return t.getMACExKLineData(exMarket, exCode, klt, limit)
	}

	// A股走 MAC 客户端
	return t.getMACMainKLineDataEx(stockCode, klt, limit)
}

// getMACMainKLineDataEx A股走 MAC 主客户端

// getMACMainKLineDataEx A股走 MAC 主客户端
func (t *TdxKLineApi) getMACMainKLineDataEx(stockCode string, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	aggSrc, aggN := tdxAggregationParams(klt)
	actualKlt := klt
	if aggSrc != "" {
		actualKlt = aggSrc
	}

	klineType := tdxKLineTypeFromKlt(actualKlt)
	if klineType < 0 {
		logger.SugaredLogger.Warnf("TdxKLine MAC: unsupported klt %s", klt)
		return result
	}

	fetchCount := uint32(limit)
	if aggN > 1 {
		fetchCount = uint32(limit * aggN)
		if fetchCount > 8000 {
			fetchCount = 8000
		}
	}

	t.macMu.Lock()
	bars, err := t.macClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, types.AdjustQFQ)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACSymbolBars error: %v, reconnecting...", err)
		for attempt := 1; attempt <= 3; attempt++ {
			if err := t.reconnectMAC(); err != nil {
				logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", err)
				if attempt < 3 {
					tdxRetryDelay(attempt)
					continue
				}
				return result
			}
			t.macMu.Lock()
			bars, err = t.macClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, types.AdjustQFQ)
			t.macMu.Unlock()
			if err == nil {
				break
			}
			logger.SugaredLogger.Warnf("TdxKLine MACSymbolBars retry #%d error: %v", attempt+1, err)
			tdxRetryDelay(attempt)
		}
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACSymbolBars exhaust retry: %v", err)
			return result
		}
	}

	if len(bars) == 0 {
		return result
	}

	converted := convertMACSymbolBar(bars, actualKlt)

	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}

	return &converted
}

// getMACMainKLineData 通过 MAC 主客户端获取K线（指定 market 和 code）

// getMACMainKLineData 通过 MAC 主客户端获取K线（指定 market 和 code）
func (t *TdxKLineApi) getMACMainKLineData(market uint8, code string, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}

	aggSrc, aggN := tdxAggregationParams(klt)
	actualKlt := klt
	if aggSrc != "" {
		actualKlt = aggSrc
	}

	klineType := tdxKLineTypeFromKlt(actualKlt)
	if klineType < 0 {
		return result
	}

	fetchCount := uint32(limit)
	if aggN > 1 {
		fetchCount = uint32(limit * aggN)
		if fetchCount > 8000 {
			fetchCount = 8000
		}
	}

	t.macMu.Lock()
	bars, err := t.macClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, types.AdjustNone)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Debugf("TdxKLine MAC main MACSymbolBars for HK error: %v", err)
		return result
	}

	if len(bars) == 0 {
		return result
	}

	converted := convertMACSymbolBar(bars, actualKlt)
	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}
	return &converted
}

// getMACExKLineData 通过 MAC 扩展行情接口获取港美股 K 线数据

// getMACExKLineData 通过 MAC 扩展行情接口获取港美股 K 线数据
func (t *TdxKLineApi) getMACExKLineData(market uint8, code string, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if err := t.ensureMACExClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACExClient error: %v", err)
		return result
	}

	aggSrc, aggN := tdxAggregationParams(klt)
	actualKlt := klt
	if aggSrc != "" {
		actualKlt = aggSrc
	}

	klineType := tdxKLineTypeFromKlt(actualKlt)
	if klineType < 0 {
		logger.SugaredLogger.Warnf("TdxKLine MAC Ex: unsupported klt %s", klt)
		return result
	}

	fetchCount := uint32(limit)
	if aggN > 1 {
		fetchCount = uint32(limit * aggN)
		if fetchCount > 8000 {
			fetchCount = 8000
		}
	}

	// 港美股通过扩展行情接口 ExKLine2 获取，不复权
	t.macExMu.Lock()
	items, err := t.macExClient.ExKLine2(market, code, uint16(klineType), 0, fetchCount, 1)
	t.macExMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACEx ExKLine2 error: %v, reconnecting...", err)
		for attempt := 1; attempt <= 3; attempt++ {
			if err := t.reconnectMACEx(); err != nil {
				logger.SugaredLogger.Errorf("TdxKLine reconnectMACEx error: %v", err)
				if attempt < 3 {
					tdxRetryDelay(attempt)
					continue
				}
				return result
			}
			t.macExMu.Lock()
			items, err = t.macExClient.ExKLine2(market, code, uint16(klineType), 0, fetchCount, 1)
			t.macExMu.Unlock()
			if err == nil {
				break
			}
			logger.SugaredLogger.Warnf("TdxKLine MACEx ExKLine2 retry #%d error: %v", attempt+1, err)
			tdxRetryDelay(attempt)
		}
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACEx ExKLine2 exhaust retry: %v", err)
			return result
		}
	}

	if len(items) == 0 {
		return result
	}

	converted := convertExKLineItem(items, actualKlt)

	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}

	return &converted
}

func convertMACSymbolBar(list []proto.MACSymbolBar, klt string) []KLineData {
	result := make([]KLineData, 0, len(list))
	isDayOrHigher := klt == "101" || klt == "102" || klt == "103" || klt == "104" || klt == "105"
	for i, bar := range list {
		day := formatMACDateTime(bar.DateTime, isDayOrHigher)
		kd := KLineData{
			Day:    day,
			Open:   fmt.Sprintf("%.2f", bar.Open),
			Close:  fmt.Sprintf("%.2f", bar.Close),
			High:   fmt.Sprintf("%.2f", bar.High),
			Low:    fmt.Sprintf("%.2f", bar.Low),
			Volume: fmt.Sprintf("%.0f", bar.Vol),
			Amount: fmt.Sprintf("%.2f", bar.Amount),
		}
		if i > 0 {
			prevClose := list[i-1].Close
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (bar.Close-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", bar.Close-prevClose)
				kd.Amplitude = fmt.Sprintf("%.2f", (bar.High-bar.Low)/prevClose*100)
			}
		}
		if bar.Turnover > 0 {
			kd.TurnoverRate = fmt.Sprintf("%.2f", bar.Turnover)
		}
		result = append(result, kd)
	}
	return result
}

// convertExKLineItem 将扩展行情 ExKLine2 返回的 ExKLineItem 列表转为 KLineData
// 用于港股（cat=31/48）和美股（cat=74）K 线

// convertExKLineItem 将扩展行情 ExKLine2 返回的 ExKLineItem 列表转为 KLineData
// 用于港股（cat=31/48）和美股（cat=74）K 线
func convertExKLineItem(list []proto.ExKLineItem, klt string) []KLineData {
	result := make([]KLineData, 0, len(list))
	isDayOrHigher := klt == "101" || klt == "102" || klt == "103" || klt == "104" || klt == "105"
	for i, item := range list {
		day := formatMACDateTime(item.DateTime, isDayOrHigher)
		kd := KLineData{
			Day:    day,
			Open:   fmt.Sprintf("%.2f", item.Open),
			Close:  fmt.Sprintf("%.2f", item.Close),
			High:   fmt.Sprintf("%.2f", item.High),
			Low:    fmt.Sprintf("%.2f", item.Low),
			Volume: fmt.Sprintf("%d", item.Vol),
			Amount: fmt.Sprintf("%.2f", item.Amount),
		}
		if i > 0 {
			prevClose := list[i-1].Close
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (item.Close-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", item.Close-prevClose)
				kd.Amplitude = fmt.Sprintf("%.2f", (item.High-item.Low)/prevClose*100)
			}
		}
		result = append(result, kd)
	}
	return result
}

// formatMACDateTime 将 MAC 返回的 DateTime 字符串转为统一格式
// MAC DateTime: "2006-01-02 15:04:05" 或 "2006-01-02 00:00:00"
// 分钟线需要时间: "2006-01-02 15:04"
// 日线及以上只需日期: "2006-01-02"

// formatMACDateTime 将 MAC 返回的 DateTime 字符串转为统一格式
// MAC DateTime: "2006-01-02 15:04:05" 或 "2006-01-02 00:00:00"
// 分钟线需要时间: "2006-01-02 15:04"
// 日线及以上只需日期: "2006-01-02"
func formatMACDateTime(dt string, isDayOrHigher bool) string {
	if len(dt) <= 10 {
		return dt
	}
	if isDayOrHigher {
		return dt[:10]
	}
	// 有时间部分，判断是否为 00:00:00（日线及以上）
	timePart := dt[11:]
	if timePart == "00:00:00" {
		return dt[:10]
	}
	// 分钟线：去掉秒，保留 "YYYY-MM-DD HH:MM"
	if len(dt) >= 16 {
		return dt[:16]
	}
	return dt[:10]
}

func convertTdxKLine(list []proto.SecurityBar, klt string) []KLineData {
	result := make([]KLineData, 0, len(list))
	isDayOrHigher := klt == "101" || klt == "102" || klt == "103" || klt == "104" || klt == "105"
	for i, bar := range list {
		var day string
		if isDayOrHigher {
			day = bar.DateTime.Format("2006-01-02")
		} else {
			day = bar.DateTime.Format("2006-01-02 15:04")
		}
		kd := KLineData{
			Day:    day,
			Open:   fmt.Sprintf("%.2f", bar.Open),
			Close:  fmt.Sprintf("%.2f", bar.Close),
			High:   fmt.Sprintf("%.2f", bar.High),
			Low:    fmt.Sprintf("%.2f", bar.Low),
			Volume: fmt.Sprintf("%.0f", bar.Vol),
			Amount: fmt.Sprintf("%.2f", bar.Amount),
		}
		if bar.RiseRate != 0 {
			kd.ChangePercent = fmt.Sprintf("%.2f", bar.RiseRate)
			kd.ChangeValue = fmt.Sprintf("%.2f", bar.RisePrice)
		} else if i > 0 {
			prevClose := list[i-1].Close
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (bar.Close-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", bar.Close-prevClose)
			}
		}
		if i > 0 {
			prevClose := list[i-1].Close
			if prevClose > 0 {
				kd.Amplitude = fmt.Sprintf("%.2f", (bar.High-bar.Low)/prevClose*100)
			}
		}
		if bar.Turnover > 0 {
			kd.TurnoverRate = fmt.Sprintf("%.2f", bar.Turnover)
		}
		result = append(result, kd)
	}
	return result
}
