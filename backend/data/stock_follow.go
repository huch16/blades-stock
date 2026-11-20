package data

import (
	"strings"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func (receiver StockDataApi) Follow(stockCode string) string {
	stockCode = FormatMarketStockCode(strings.TrimSpace(stockCode))
	stockInfos, err := receiver.GetStockCodeRealTimeData(stockCode)
	if err != nil || len(*stockInfos) == 0 {
		logger.SugaredLogger.Error(err)
		return "关注失败"
	}
	if strings.HasPrefix(stockCode, "us") {
		stockCode = strings.Replace(stockCode, "us", "gb_", 1)
	}
	if strings.HasPrefix(stockCode, "US") {
		stockCode = strings.Replace(stockCode, "US", "gb_", 1)
	}
	count := int64(0)
	db.Dao.Model(&FollowedStock{}).Where("is_del = ?", 0).Count(&count)
	stockCode = strings.ToLower(stockCode)

	// 检查是否已经关注过该股票（同时兼容纯数字代码与标准化前缀代码查重）
	pureCode := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(stockCode, "sh"), "sz"), "bj")
	var existingStock FollowedStock
	result := db.Dao.Model(&FollowedStock{}).Where("(stock_code = ? OR stock_code = ?) AND is_del = ?", stockCode, pureCode, 0).First(&existingStock)
	if result.Error == nil {
		// 股票已经关注过
		return "已经关注了"
	}

	maxSort := int64(0)
	db.Dao.Model(&FollowedStock{}).Raw("select max(sort) as sort from followed_stock").Scan(&maxSort)

	//logger.SugaredLogger.Infof("Follow-maxSort %v", maxSort)

	stockInfo := (*stockInfos)[0]
	price, _ := convertor.ToFloat(stockInfo.Price)
	db.Dao.Model(&FollowedStock{}).FirstOrCreate(&FollowedStock{
		StockCode:          stockCode,
		Name:               stockInfo.Name,
		Price:              price,
		Time:               time.Now(),
		ChangePercent:      0,
		PriceChange:        0,
		Sort:               maxSort + 1,
		AlarmChangePercent: 3,
		AlarmPrice:         price + 1,
	}, &FollowedStock{StockCode: stockCode})
	return "关注成功"
}

func (receiver StockDataApi) UnFollow(stockCode string) string {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).Delete(&FollowedStock{})
	return "取消关注成功"
}

func (receiver StockDataApi) SetCostPriceAndVolume(price float64, volume int64, stockCode string) string {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).Update("cost_price", price).Update("volume", volume).Error
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return "设置失败"
	}
	return "设置成功"
}

func (receiver StockDataApi) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).Updates(&map[string]any{
		"alarm_change_percent": val,
		"alarm_price":          alarmPrice,
	}).Error
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return "设置失败"
	}
	return "设置成功"
}

func (receiver StockDataApi) SetStockSort(newSort int64, stockCode string) {
	//if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
	//	stockCode = strings.ToLower(stockCode)
	//	stockCode = strings.Replace(stockCode, "gb_", "us", 1)
	//}

	// 获取当前排序值
	var currentStock FollowedStock
	if err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).First(&currentStock).Error; err != nil {
		logger.SugaredLogger.Error("找不到当前股票: ", err.Error())
		return
	}

	oldSort := currentStock.Sort

	// 如果排序值没有变化，直接返回
	if oldSort == newSort {
		return
	}
	// 检查新排序位置是否被占用
	var count int64
	if err := db.Dao.Model(&FollowedStock{}).Where("sort = ?", newSort).Count(&count).Error; err != nil {
		logger.SugaredLogger.Error("检查新排序位置被占用失败: ", err.Error())
		return
	}
	if count == 0 {
		// 新位置未被占用，直接更新当前记录
		if err := db.Dao.Model(&FollowedStock{}).
			Where("stock_code = ?", strings.ToLower(stockCode)).
			Update("sort", newSort).Error; err != nil {
			logger.SugaredLogger.Error("更新排序位置失败: ", err.Error())
		}
	} else {
		// 新位置已被占用，需要移动其他记录
		if newSort < oldSort {
			// 向前移动：将中间记录向后移动
			if err := db.Dao.Model(&FollowedStock{}).
				Where("sort >= ? AND sort < ?", newSort, oldSort).
				Update("sort", gorm.Expr("sort + 1")).Error; err != nil {
				logger.SugaredLogger.Error("向前排序更新失败: ", err.Error())
			}
		} else {
			// 向后移动：将中间记录向前移动
			if err := db.Dao.Model(&FollowedStock{}).
				Where("sort > ? AND sort <= ?", oldSort, newSort).
				Update("sort", gorm.Expr("sort - 1")).Error; err != nil {
				logger.SugaredLogger.Error("向后排序更新失败: ", err.Error())
			}
		}

		// 更新目标记录的排序
		if err := db.Dao.Model(&FollowedStock{}).
			Where("stock_code = ?", strings.ToLower(stockCode)).
			Update("sort", newSort).Error; err != nil {
			logger.SugaredLogger.Error("更新股票排序失败: ", err.Error())
		}
	}

}

func (receiver StockDataApi) SetStockAICron(cron string, stockCode string) {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).Update("cron", cron)

}

func (receiver StockDataApi) SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice float64, stockCode string) string {
	stockCode = strings.ToUpper(stockCode)
	if strings.HasSuffix(stockCode, ".SZ") {
		stockCode = "sz" + strings.TrimSuffix(stockCode, ".SZ")
	} else if strings.HasSuffix(stockCode, ".SH") {
		stockCode = "sh" + strings.TrimSuffix(stockCode, ".SH")
	} else if strings.HasSuffix(stockCode, ".HK") {
		stockCode = "hk" + strings.TrimSuffix(stockCode, ".HK")
	} else if strings.HasSuffix(stockCode, ".BJ") {
		stockCode = "bj" + strings.TrimSuffix(stockCode, ".BJ")
	} else if strings.HasPrefix(stockCode, "GB_") {
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	lowerStockCode := strings.ToLower(stockCode)

	var stock FollowedStock
	if err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", lowerStockCode).First(&stock).Error; err != nil {
		return "股票未关注"
	}

	updates := &map[string]any{
		"entry_price":       entryPrice,
		"take_profit_price": takeProfitPrice,
		"stop_loss_price":   stopLossPrice,
		"cost_price":        costPrice,
	}
	result := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", lowerStockCode).Updates(updates)
	if result.Error != nil {
		return "设置失败"
	}
	if result.RowsAffected == 0 {
		return "设置失败"
	}
	return "设置成功"
}

func (receiver StockDataApi) GetFollowList(groupId int) *[]FollowedStock {
	//logger.SugaredLogger.Infof("GetFollowList %d", groupId)

	var result *[]FollowedStock
	if groupId == 0 {
		db.Dao.Model(&FollowedStock{}).Order("sort asc,time desc").Find(&result)
	} else {
		infos := NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
		codes := lo.FlatMap(infos, func(info GroupStock, idx int) []string {
			return []string{info.StockCode}
		})
		db.Dao.Model(&FollowedStock{}).Where("stock_code in ?", codes).Order("sort asc,time desc").Find(&result)
		//logger.SugaredLogger.Infof("GetFollowList %+v", result)
	}
	return result
}

func (receiver StockDataApi) GetFollowedStockByStockCode(code string) FollowedStock {
	var result FollowedStock
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(code)).First(&result)
	return result
}

// GB18030ToUTF8 GB18030 转换为 UTF8
