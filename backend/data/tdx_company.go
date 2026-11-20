package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"fmt"
	"strings"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
	"gorm.io/gorm"
)

func (t *TdxKLineApi) GetF10Data(stockCode string) *TdxCompanyInfoBundle {
	result := &TdxCompanyInfoBundle{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	bundle, err := t.client.StockF10(market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockF10 error: %v, reconnecting...", err)
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
			bundle, err = t.client.StockF10(market, code)
			t.mu.Unlock()
			if err == nil {
				break
			}
			logger.SugaredLogger.Warnf("TdxKLine StockF10 retry #%d error: %v", attempt+1, err)
			tdxRetryDelay(attempt)
		}
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockF10 exhaust retry: %v", err)
			return result
		}
	}

	if bundle == nil {
		return result
	}

	result.Sections = make([]TdxCompanyInfoSection, 0, len(bundle.Sections))
	for _, s := range bundle.Sections {
		result.Sections = append(result.Sections, TdxCompanyInfoSection{
			Name:    s.Name,
			Content: s.Content,
		})
	}

	result.XDXR = make([]TdxXDXRItem, 0, len(bundle.XDXR))
	for _, x := range bundle.XDXR {
		item := TdxXDXRItem{
			Date:     x.Date.Format("2006-01-02"),
			Category: x.Category,
			Name:     x.Name,
		}
		if x.Fenhong != nil {
			v := float64(*x.Fenhong)
			item.Fenhong = &v
		}
		if x.Peigujia != nil {
			v := float64(*x.Peigujia)
			item.Peigujia = &v
		}
		if x.Songzhuangu != nil {
			v := float64(*x.Songzhuangu)
			item.Songzhuangu = &v
		}
		if x.Peigu != nil {
			v := float64(*x.Peigu)
			item.Peigu = &v
		}
		if x.Suogu != nil {
			v := float64(*x.Suogu)
			item.Suogu = &v
		}
		if x.PreFloatShares != nil {
			v := float64(*x.PreFloatShares)
			item.PreFloatShares = &v
		}
		if x.PreTotalShares != nil {
			v := float64(*x.PreTotalShares)
			item.PreTotalShares = &v
		}
		if x.PostFloatShares != nil {
			v := float64(*x.PostFloatShares)
			item.PostFloatShares = &v
		}
		if x.PostTotalShares != nil {
			v := float64(*x.PostTotalShares)
			item.PostTotalShares = &v
		}
		result.XDXR = append(result.XDXR, item)
	}

	if bundle.Finance != nil {
		f := bundle.Finance
		result.Finance = &TdxFinanceInfo{
			Market:              f.Market,
			Code:                f.Code,
			FloatShares:         float64(f.FloatShares),
			TotalShares:         float64(f.TotalShares),
			EPS:                 float64(f.EPS),
			TotalAssets:         float64(f.TotalAssets),
			CurrentAssets:       float64(f.CurrentAssets),
			FixedAssets:         float64(f.FixedAssets),
			IntangibleAssets:    float64(f.IntangibleAssets),
			ShareholderCount:    float64(f.ShareholderCount),
			CurrentLiabilities:  float64(f.CurrentLiabilities),
			LongTermLiabilities: float64(f.LongTermLiabilities),
			CapitalReserve:      float64(f.CapitalReserve),
			TotalEquity:         float64(f.TotalEquity),
			OperatingRevenue:    float64(f.OperatingRevenue),
			OperatingCost:       float64(f.OperatingCost),
			AccountsReceivable:  float64(f.AccountsReceivable),
			OperatingProfit:     float64(f.OperatingProfit),
			InvestmentIncome:    float64(f.InvestmentIncome),
			NetCashFlow:         float64(f.NetCashFlow),
			Inventory:           float64(f.Inventory),
			TotalProfit:         float64(f.TotalProfit),
			AfterTaxProfit:      float64(f.AfterTaxProfit),
			NetProfit:           float64(f.NetProfit),
			UndistributedProfit: float64(f.UndistributedProfit),
			NetAssetsPerShare:   float64(f.NetAssetsPerShare),
		}
		if f.IPODate > 0 {
			result.Finance.IPODate = tdxDateToString(f.IPODate)
		}
		if f.UpdatedDate > 0 {
			result.Finance.UpdatedDate = tdxDateToString(f.UpdatedDate)
		}
	}

	return result
}

func (t *TdxKLineApi) GetF10CategoryList(stockCode string) *[]TdxCompanyCategory {
	result := &[]TdxCompanyCategory{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	if _, err := t.client.Connect(); err != nil {
		t.mu.Unlock()
		logger.SugaredLogger.Warnf("TdxKLine Connect error: %v", err)
		return result
	}
	categories, err := t.client.GetCompanyCategories(market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine GetCompanyCategories error: %v", err)
		return result
	}

	if categories == nil || len(categories.Categories) == 0 {
		return result
	}

	items := make([]TdxCompanyCategory, 0, len(categories.Categories))
	for _, c := range categories.Categories {
		items = append(items, TdxCompanyCategory{
			Name:     c.Name,
			Filename: c.Filename,
		})
	}
	return &items
}

func (t *TdxKLineApi) GetF10CategoryContent(stockCode string, categoryName string) *TdxCompanyInfoSection {
	result := &TdxCompanyInfoSection{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	if _, err := t.client.Connect(); err != nil {
		t.mu.Unlock()
		logger.SugaredLogger.Warnf("TdxKLine Connect error: %v", err)
		return result
	}
	categories, err := t.client.GetCompanyCategories(market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine GetCompanyCategories error: %v", err)
		return result
	}

	if categories == nil {
		return result
	}

	var target *proto.CompanyCategory
	for i := range categories.Categories {
		if categories.Categories[i].Name == categoryName {
			target = &categories.Categories[i]
			break
		}
	}
	if target == nil {
		logger.SugaredLogger.Warnf("TdxKLine category '%s' not found for %s", categoryName, stockCode)
		return result
	}

	t.mu.Lock()
	content, err := t.client.GetCompanyContent(market, code, target.Filename, target.Start, target.Length)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine GetCompanyContent error: %v", err)
		return result
	}

	result.Name = target.Name
	result.Content = content.Content
	return result
}

// GetMACSymbolBelongBoard 通过 MAC 行情接口获取股票所属板块信息
func (t *TdxKLineApi) GetMACSymbolBelongBoard(stockCode string) *[]MACBelongBoardItem {
	result := &[]MACBelongBoardItem{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}

	market, code := tdxMarketFromStockCode(stockCode)

	t.macMu.Lock()
	items, err := t.macClient.MACSymbolBelongBoard(code, market)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACSymbolBelongBoard error: %v, reconnecting...", err)
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
			items, err = t.macClient.MACSymbolBelongBoard(code, market)
			t.macMu.Unlock()
			if err == nil {
				break
			}
			logger.SugaredLogger.Warnf("TdxKLine MACSymbolBelongBoard retry #%d error: %v", attempt+1, err)
			tdxRetryDelay(attempt)
		}
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACSymbolBelongBoard exhaust retry: %v", err)
			return result
		}
	}

	converted := make([]MACBelongBoardItem, 0, len(items))
	for _, item := range items {
		converted = append(converted, MACBelongBoardItem{
			BoardType:      item.BoardType,
			BoardCode:      item.BoardCode,
			BoardName:      item.BoardName,
			Price:          item.Price,
			PreClose:       item.PreClose,
			LimitUpCount:   item.LimitUpCount,
			LimitDownCount: item.LimitDownCount,
		})
	}
	return &converted
}

// TdxStockBasic 通达信返回的股票基础信息（代码+名称+昨收+小数位+量单位）

// GetAllStockList 通过通达信标准行情接口拉取沪深京全市场股票代码+名称列表。
// 即时性高（新股上市当天即可见），不会被封 IP。仅覆盖 A 股，不含港美股。
// 返回结果按市场顺序：深圳 -> 上海 -> 北京，已用 types.IsStock 过滤掉指数/基金/债券等非股票标的。
func (t *TdxKLineApi) GetAllStockList() *[]TdxStockBasic {
	result := &[]TdxStockBasic{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}

	// 沪深京三个市场，顺序与 gotdx examples/stock_all 一致
	markets := []types.Market{types.MarketSZ, types.MarketSH, types.MarketBJ}
	for _, market := range markets {
		if err := t.fetchStockListByMarket(market, result); err != nil {
			logger.SugaredLogger.Debugf("TdxKLine GetAllStockList market %s error: %v, retry...", market.String(), err)
			for attempt := 1; attempt <= 3; attempt++ {
				if err := t.reconnect(); err != nil {
					logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", err)
					if attempt < 3 {
						tdxRetryDelay(attempt)
						continue
					}
					break
				}
				if retryErr := t.fetchStockListByMarket(market, result); retryErr != nil {
					logger.SugaredLogger.Debugf("TdxKLine GetAllStockList market %s retry #%d error: %v", market.String(), attempt, retryErr)
					tdxRetryDelay(attempt)
					continue
				}
				break
			}
		}
	}
	return result
}

// fetchStockListByMarket 拉取单个市场的全部证券列表，过滤出股票后追加到 result

// fetchStockListByMarket 拉取单个市场的全部证券列表，过滤出股票后追加到 result
func (t *TdxKLineApi) fetchStockListByMarket(market types.Market, result *[]TdxStockBasic) error {
	t.mu.Lock()
	items, err := t.client.StockAll(market.Uint8())
	t.mu.Unlock()
	if err != nil {
		return err
	}

	marketStr := market.String()
	for _, item := range items {
		// 用 types.IsStock 过滤指数/基金/债券等非股票标的（要求 代码.SH/SZ/BJ 格式）
		symbol := fmt.Sprintf("%s.%s", item.Code, marketStr)
		if !types.IsStock(symbol) {
			continue
		}
		*result = append(*result, TdxStockBasic{
			StockCode:    strings.ToLower(marketStr) + item.Code,
			Code:         item.Code,
			Name:         item.Name,
			Market:       marketStr,
			PreClose:     item.PreClose,
			DecimalPoint: item.DecimalPoint,
			VolUnit:      item.VolUnit,
		})
	}
	return nil
}

// SyncStockBasicToDB 将通达信拉取的沪深京全市场股票列表同步（upsert）到 StockBasic 表。
// 以 Symbol（纯代码）为去重键：已存在则更新名称/市场，不存在则新增。
// 经内存预检与批量事务优化，杜绝单条高频读写对 SQLite 锁的长期占用。

// SyncStockBasicToDB 将通达信拉取的沪深京全市场股票列表同步（upsert）到 StockBasic 表。
// 以 Symbol（纯代码）为去重键：已存在则更新名称/市场，不存在则新增。
// 经内存预检与批量事务优化，杜绝单条高频读写对 SQLite 锁的长期占用。
func (t *TdxKLineApi) SyncStockBasicToDB() (added, updated int, err error) {
	list := t.GetAllStockList()
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

// 扩展行情分类常量（由探测测试实测得到）

// GetHKUSStockList 通过通达信扩展行情接口拉取港股+美股代码+名称列表。
// 即时性高（新股上市当天即可见），不会被封 IP。覆盖港股主板/创业板、美股。
func (t *TdxKLineApi) GetHKUSStockList() (*[]TdxExStockBasic, *[]TdxExStockBasic, error) {
	hkList := &[]TdxExStockBasic{}
	usList := &[]TdxExStockBasic{}
	if err := t.ensureMACExClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACExClient error: %v", err)
		return hkList, usList, err
	}

	total, err := t.fetchExCountWithRetry()
	if err != nil {
		return hkList, usList, err
	}

	pageSize := uint16(1000)
	for start := uint32(0); start < total; start += uint32(pageSize) {
		remain := total - start
		size := pageSize
		if remain < uint32(pageSize) {
			size = uint16(remain)
		}
		items, err := t.fetchExListWithRetry(start, size)
		if err != nil {
			logger.SugaredLogger.Warnf("GetHKUSStockList ExList start=%d error: %v", start, err)
			continue
		}
		for _, item := range items {
			switch item.Category {
			case exCategoryHKMain, exCategoryHKGem:
				*hkList = append(*hkList, TdxExStockBasic{
					Code:     item.Code,
					Name:     item.Name,
					Category: item.Category,
					Market:   "HK",
				})
			case exCategoryUS:
				*usList = append(*usList, TdxExStockBasic{
					Code:     item.Code,
					Name:     item.Name,
					Category: item.Category,
					Market:   "US",
				})
			}
		}
		if len(items) == 0 {
			break
		}
	}
	return hkList, usList, nil
}

func (t *TdxKLineApi) fetchExCountWithRetry() (uint32, error) {
	t.macExMu.Lock()
	total, err := t.macExClient.ExCount()
	t.macExMu.Unlock()
	if err != nil {
		logger.SugaredLogger.Warnf("ExCount error: %v, retry...", err)
		for attempt := 1; attempt <= 3; attempt++ {
			if e := t.reconnectMACEx(); e != nil {
				logger.SugaredLogger.Errorf("ExCount reconnect error: %v", e)
				if attempt < 3 {
					tdxRetryDelay(attempt)
					continue
				}
				return 0, e
			}
			t.macExMu.Lock()
			total, err = t.macExClient.ExCount()
			t.macExMu.Unlock()
			if err == nil {
				return total, nil
			}
			logger.SugaredLogger.Warnf("ExCount retry #%d error: %v", attempt+1, err)
			tdxRetryDelay(attempt)
		}
		return 0, err
	}
	return total, nil
}

func (t *TdxKLineApi) fetchExListWithRetry(start uint32, count uint16) ([]proto.ExListItem, error) {
	t.macExMu.Lock()
	items, err := t.macExClient.ExList(start, count)
	t.macExMu.Unlock()
	if err != nil {
		logger.SugaredLogger.Debugf("ExList start=%d error: %v, retry...", start, err)
		for attempt := 1; attempt <= 3; attempt++ {
			if e := t.reconnectMACEx(); e != nil {
				logger.SugaredLogger.Errorf("ExList reconnect error: %v", e)
				if attempt < 3 {
					tdxRetryDelay(attempt)
					continue
				}
				return nil, e
			}
			t.macExMu.Lock()
			items, err = t.macExClient.ExList(start, count)
			t.macExMu.Unlock()
			if err == nil {
				return items, nil
			}
			logger.SugaredLogger.Debugf("ExList retry #%d error: %v", attempt+1, err)
			tdxRetryDelay(attempt)
		}
		return nil, err
	}
	return items, nil
}

// SyncHKUSStockBasicToDB 将通达信扩展行情拉取的港股/美股列表同步（upsert）到对应表。
// 港股写入 stock_base_info_hk（Code 格式 00001.HK），美股写入 stock_base_info_us。
// 以 Code 为去重键：已存在则更新名称，不存在则新增。

// SyncHKUSStockBasicToDB 将通达信扩展行情拉取的港股/美股列表同步（upsert）到对应表。
// 港股写入 stock_base_info_hk（Code 格式 00001.HK），美股写入 stock_base_info_us。
// 以 Code 为去重键：已存在则更新名称，不存在则新增。
func (t *TdxKLineApi) SyncHKUSStockBasicToDB() (hkAdded, hkUpdated, usAdded, usUpdated int, err error) {
	hkList, usList, err := t.GetHKUSStockList()
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
