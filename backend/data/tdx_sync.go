package data

import (
	"blades-stock/backend/datasource"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"fmt"
	"strings"

	"gorm.io/gorm"
)

// SyncStockBasicToDB / SyncHKUSStockBasicToDB 是 DB 写入操作，留在 data 包
//（R-25b：TdxKLineApi 已搬入 datasource，data 无法再为其定义方法，
// 故改为包级函数，调用方改为 data.SyncXxxToDB(data.NewTdxKLineApi())）。

// SyncStockBasicToDB 将通达信拉取的沪深京全市场股票列表同步（upsert）到 StockBasic 表。
// 以 Symbol（纯代码）为去重键：已存在则更新名称/市场，不存在则新增。
// 经内存预检与批量事务优化，杜绝单条高频读写对 SQLite 锁的长期占用。

// SyncStockBasicToDB 将通达信拉取的沪深京全市场股票列表同步（upsert）到 StockBasic 表。
// 以 Symbol（纯代码）为去重键：已存在则更新名称/市场，不存在则新增。
// 经内存预检与批量事务优化，杜绝单条高频读写对 SQLite 锁的长期占用。
func SyncStockBasicToDB(api *datasource.TdxKLineApi) (added, updated int, err error) {
	list := api.GetAllStockList()
	if list == nil || len(*list) == 0 {
		return 0, 0, fmt.Errorf("通达信未返回股票列表数据")
	}

	// 1. 预先一次性将已有 Symbol 载入内存哈希映射，消除逐行 First() 导致的数千次读磁盘开销
	var existing []struct {
		Symbol string
		Name   string
		Market string
	}
	db.Dao.Model(&StockBasic{}).Select("symbol, name, market").Find(&existing)
	existMap := make(map[string]struct{ Name, Market string }, len(existing))
	for _, e := range existing {
		existMap[e.Symbol] = struct{ Name, Market string }{Name: e.Name, Market: e.Market}
	}

	var toCreate []*StockBasic
	type updateItem struct {
		symbol string
		tsCode string
		name   string
		market string
	}
	var toUpdate []updateItem

	for _, item := range *list {
		tsCode := fmt.Sprintf("%s.%s", item.Code, item.Market) // 如 600519.SH
		cur, exists := existMap[item.Code]
		if !exists {
			toCreate = append(toCreate, &StockBasic{
				Symbol: item.Code,
				TsCode: tsCode,
				Name:   item.Name,
				Market: item.Market,
			})
			existMap[item.Code] = struct{ Name, Market string }{Name: item.Name, Market: item.Market}
		} else if cur.Name != item.Name || cur.Market != item.Market {
			toUpdate = append(toUpdate, updateItem{
				symbol: item.Code,
				tsCode: tsCode,
				name:   item.Name,
				market: item.Market,
			})
		}
	}

	// 2. 批量创建新股票
	if len(toCreate) > 0 {
		if err := db.Dao.CreateInBatches(toCreate, 300).Error; err != nil {
			logger.SugaredLogger.Warnf("SyncStockBasicToDB 批量新增失败: %v", err)
		} else {
			added = len(toCreate)
		}
	}

	// 3. 在单一事务中批量更新发生变动的标的，避免逐行 autocommit 争用 SQLite 锁
	if len(toUpdate) > 0 {
		err := db.Dao.Transaction(func(tx *gorm.DB) error {
			for _, u := range toUpdate {
				if e := tx.Model(&StockBasic{}).Where("symbol = ?", u.symbol).Updates(map[string]any{
					"ts_code": u.tsCode,
					"name":    u.name,
					"market":  u.market,
				}).Error; e != nil {
					return e
				}
			}
			return nil
		})
		if err != nil {
			logger.SugaredLogger.Warnf("SyncStockBasicToDB 批量更新失败: %v", err)
		} else {
			updated = len(toUpdate)
		}
	}

	logger.SugaredLogger.Infof("SyncStockBasicToDB 完成：新增 %d 条，更新 %d 条", added, updated)
	return added, updated, nil
}

// SyncHKUSStockBasicToDB 将通达信扩展行情拉取的港股/美股列表同步（upsert）到对应表。
// 港股写入 stock_base_info_hk（Code 格式 00001.HK），美股写入 stock_base_info_us。
// 以 Code 为去重键：已存在则更新名称，不存在则新增。

// SyncHKUSStockBasicToDB 将通达信扩展行情拉取的港股/美股列表同步（upsert）到对应表。
// 港股写入 stock_base_info_hk（Code 格式 00001.HK），美股写入 stock_base_info_us。
// 以 Code 为去重键：已存在则更新名称，不存在则新增。
func SyncHKUSStockBasicToDB(api *datasource.TdxKLineApi) (hkAdded, hkUpdated, usAdded, usUpdated int, err error) {
	hkList, usList, err := api.GetHKUSStockList()
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// 1. 港股差异对比
	var existingHK []struct {
		Code string
		Name string
	}
	db.Dao.Model(&models.StockInfoHK{}).Select("code, name").Find(&existingHK)
	hkMap := make(map[string]string, len(existingHK))
	for _, e := range existingHK {
		hkMap[e.Code] = e.Name
	}

	var hkCreate []*models.StockInfoHK
	type hkUpdateItem struct {
		code string
		name string
	}
	var hkUpdate []hkUpdateItem

	for _, item := range *hkList {
		code := fmt.Sprintf("%05s.HK", item.Code)
		oldName, exists := hkMap[code]
		if !exists {
			hkCreate = append(hkCreate, &models.StockInfoHK{Code: code, Name: item.Name})
			hkMap[code] = item.Name
		} else if oldName != item.Name {
			hkUpdate = append(hkUpdate, hkUpdateItem{code: code, name: item.Name})
		}
	}

	if len(hkCreate) > 0 {
		if err := db.Dao.CreateInBatches(hkCreate, 300).Error; err != nil {
			logger.SugaredLogger.Warnf("SyncHKUSStockBasicToDB 批量新增港股失败: %v", err)
		} else {
			hkAdded = len(hkCreate)
		}
	}
	if len(hkUpdate) > 0 {
		_ = db.Dao.Transaction(func(tx *gorm.DB) error {
			for _, u := range hkUpdate {
				tx.Model(&models.StockInfoHK{}).Where("code = ?", u.code).Update("name", u.name)
			}
			return nil
		})
		hkUpdated = len(hkUpdate)
	}

	// 2. 美股差异对比
	var existingUS []struct {
		Code string
		Name string
	}
	db.Dao.Model(&models.StockInfoUS{}).Select("code, name").Find(&existingUS)
	usMap := make(map[string]string, len(existingUS))
	for _, e := range existingUS {
		usMap[e.Code] = e.Name
	}

	var usCreate []*models.StockInfoUS
	type usUpdateItem struct {
		code string
		name string
	}
	var usUpdate []usUpdateItem

	for _, item := range *usList {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		oldName, exists := usMap[code]
		if !exists {
			usCreate = append(usCreate, &models.StockInfoUS{Code: code, Name: item.Name})
			usMap[code] = item.Name
		} else if oldName != item.Name {
			usUpdate = append(usUpdate, usUpdateItem{code: code, name: item.Name})
		}
	}

	if len(usCreate) > 0 {
		if err := db.Dao.CreateInBatches(usCreate, 300).Error; err != nil {
			logger.SugaredLogger.Warnf("SyncHKUSStockBasicToDB 批量新增美股失败: %v", err)
		} else {
			usAdded = len(usCreate)
		}
	}
	if len(usUpdate) > 0 {
		_ = db.Dao.Transaction(func(tx *gorm.DB) error {
			for _, u := range usUpdate {
				tx.Model(&models.StockInfoUS{}).Where("code = ?", u.code).Update("name", u.name)
			}
			return nil
		})
		usUpdated = len(usUpdate)
	}

	logger.SugaredLogger.Infof("SyncHKUSStockBasicToDB 完成：港股新增 %d 更新 %d，美股新增 %d 更新 %d",
		hkAdded, hkUpdated, usAdded, usUpdated)
	return hkAdded, hkUpdated, usAdded, usUpdated, nil
}
