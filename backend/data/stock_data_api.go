package data

// @Author spark
// @Date 2024/12/10 9:21
// @Desc
//-----------------------------------------------------------------------------------

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"blades-stock/backend/datasource"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"github.com/duke-git/lancet/v2/slice"
	resty "github.com/go-resty/resty/v2"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

const sinaStockUrl = "http://hq.sinajs.cn/rn=%d&list=%s"

const txStockUrl = "http://qt.gtimg.cn/?_=%d&q=%s"
const tushareApiUrl = "http://api.tushare.pro"
const eastMoneyUTParam = "b2884a393a59ad64002292a3e90d46a5"

type StockDataApi struct {
	client *resty.Client
	config *SettingConfig
}
type StockInfo struct {
	gorm.Model
	Date     string  `json:"日期" gorm:"index"`
	Time     string  `json:"时间" gorm:"index" `
	Code     string  `json:"股票代码" gorm:"index"`
	Name     string  `json:"股票名称" gorm:"index"`
	PrePrice float64 `json:"上次当前价格"`
	Price    string  `json:"当前价格"`
	Volume   string  `json:"成交的股票数"`
	Amount   string  `json:"成交金额"`
	Open     string  `json:"今日开盘价"`
	PreClose string  `json:"昨日收盘价"`
	High     string  `json:"今日最高价"`
	Low      string  `json:"今日最低价"`
	Bid      string  `json:"竞买价"`
	Ask      string  `json:"竞卖价"`
	B1P      string  `json:"买一报价"`
	B1V      string  `json:"买一申报"`
	B2P      string  `json:"买二报价"`
	B2V      string  `json:"买二申报"`
	B3P      string  `json:"买三报价"`
	B3V      string  `json:"买三申报"`
	B4P      string  `json:"买四报价"`
	B4V      string  `json:"买四申报"`
	B5P      string  `json:"买五报价"`
	B5V      string  `json:"买五申报"`
	A1P      string  `json:"卖一报价"`
	A1V      string  `json:"卖一申报"`
	A2P      string  `json:"卖二报价"`
	A2V      string  `json:"卖二申报"`
	A3P      string  `json:"卖三报价"`
	A3V      string  `json:"卖三申报"`
	A4P      string  `json:"卖四报价"`
	A4V      string  `json:"卖四申报"`
	A5P      string  `json:"卖五报价"`
	A5V      string  `json:"卖五申报"`
	Market   string  `json:"市场"`
	BA       string  `json:"盘前盘后"`
	BAChange string  `json:"盘前盘后涨跌幅"`

	//以下是字段值需二次计算
	ChangePercent     float64 `json:"changePercent"`     //涨跌幅
	ChangePrice       float64 `json:"changePrice"`       //涨跌额
	HighRate          float64 `json:"highRate"`          //最高涨跌
	LowRate           float64 `json:"lowRate"`           //最低涨跌
	CostPrice         float64 `json:"costPrice"`         //成本价
	CostVolume        int64   `json:"costVolume"`        //持仓数量
	Profit            float64 `json:"profit"`            //总盈亏率
	ProfitAmount      float64 `json:"profitAmount"`      //总盈亏金额
	ProfitAmountToday float64 `json:"profitAmountToday"` //今日盈亏金额

	Sort               int64   `json:"sort"` //排序
	AlarmChangePercent float64 `json:"alarmChangePercent"`
	AlarmPrice         float64 `json:"alarmPrice"`

	Groups []GroupStock `gorm:"-:all"`
}

func (receiver StockInfo) TableName() string {
	return "stock_info"
}

type TushareRequest struct {
	ApiName string `json:"api_name"`
	Token   string `json:"token"`
	Params  any    `json:"params"`
	Fields  string `json:"fields"`
}
type TushareResponse struct {
	RequestId string `json:"request_id"`
	Code      int    `json:"code"`
	Data      any    `json:"data"`
	Msg       string `json:"msg"`
}

/*
	字段	类型	说明
	ts_code	str	TS代码
	symbol	str	股票代码
	name	str	股票名称
	area	str	地域
	industry	str	所属行业
	fullname	str	股票全称
	enname	str	英文全称
	cnspell	str	拼音缩写
	market	str	市场类型
	exchange	str	交易所代码
	curr_type	str	交易货币
	list_status	str	上市状态 L上市 D退市 P暂停上市
	list_date	str	上市日期
	delist_date	str	退市日期
	is_hs	str	是否沪深港通标的，N否 H沪股通 S深股通
	act_name	str	实控人名称
	act_ent_type	str	实控人企业性质*/

type StockBasic struct {
	gorm.Model
	TsCode     string `json:"ts_code" gorm:"index"`
	Symbol     string `json:"symbol" gorm:"index"`
	Name       string `json:"name" gorm:"index"`
	Area       string `json:"area"`
	Industry   string `json:"industry" gorm:"index"`
	Fullname   string `json:"fullname"`
	Ename      string `json:"enname"`
	Cnspell    string `json:"cnspell"`
	Market     string `json:"market"`
	Exchange   string `json:"exchange"`
	CurrType   string `json:"curr_type"`
	ListStatus string `json:"list_status"`
	ListDate   string `json:"list_date"`
	DelistDate string `json:"delist_date"`
	IsHs       string `json:"is_hs"`
	ActName    string `json:"act_name"`
	ActEntType string `json:"act_ent_type"`
	BKName     string `json:"bk_name"`
	BKCode     string `json:"bk_code"`
}

type FollowedStock struct {
	StockCode          string
	Name               string
	Volume             int64
	CostPrice          float64
	Price              float64
	PriceChange        float64
	ChangePercent      float64
	AlarmChangePercent float64
	AlarmPrice         float64
	Time               time.Time
	Sort               int64
	Cron               *string
	IsDel              soft_delete.DeletedAt `gorm:"softDelete:flag"`
	Groups             []GroupStock          `gorm:"foreignKey:StockCode;references:StockCode"`
	AiConfigId         int
	EntryPrice         float64
	TakeProfitPrice    float64
	StopLossPrice      float64
}

func (receiver FollowedStock) TableName() string {
	return "followed_stock"
}

// TradingRecord 交易日志结构体
type TradingRecord struct {
	ID              uint   `gorm:"primaryKey"`
	StockCode       string `gorm:"index"`
	StockName       string
	Direction       string `gorm:"index"` // 买入/卖出
	Price           float64
	Volume          int64
	Amount          float64   `gorm:"-"` // 计算字段: Price * Volume
	TradingTime     time.Time `gorm:"index"`
	Reason          string    `gorm:"type:text"`
	StopLossPrice   float64
	TakeProfitPrice float64
	Fee             float64
	MarketValue     float64
	Mindset         string `gorm:"type:text"`
	// RecordedClosePrice 保存时写入的当日收盘价或现价快照，列表盈亏计算优先使用，减少重复请求行情
	RecordedClosePrice float64 `json:"recordedClosePrice" gorm:"column:recorded_close_price"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (receiver TradingRecord) TableName() string {
	return "trading_records"
}

// TradingRecordListQuery 交易日志列表查询（与前端分页、筛选参数一致）
type TradingRecordListQuery struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Keyword   string `json:"keyword"`   // 股票代码或名称模糊匹配
	Direction string `json:"direction"` // 买入 / 卖出，空表示全部
	StartDate string `json:"startDate"` // yyyy-MM-dd，交易时间起始（含当日 0 点）
	EndDate   string `json:"endDate"`   // yyyy-MM-dd，交易时间结束（含当日）
}

// TradingRecordPageData 交易日志分页结果
type TradingRecordPageData struct {
	List       []TradingRecordItem `json:"list"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	TotalPages int                 `json:"totalPages"`
}

// TradingRecordItem 交易日志项（包含盈亏信息）
type TradingRecordItem struct {
	TradingRecord
	ClosePrice    float64 `json:"closePrice"`    // 收盘价或最新价
	ProfitAmount  float64 `json:"profitAmount"`  // 盈亏金额
	ProfitPercent float64 `json:"profitPercent"` // 盈亏收益率
}

type TradingRecordStatistics struct {
	TotalBuyAmount  float64 `json:"totalBuyAmount"`
	TotalSellAmount float64 `json:"totalSellAmount"`
	TotalProfit     float64 `json:"totalProfit"`
	ProfitRate      float64 `json:"profitRate"`
	HoldingsAmount  float64 `json:"holdingsAmount"`
	CurrentValue    float64 `json:"currentValue"`
	StockCount      int64   `json:"stockCount"`
}

type TushareStockBasicResponse struct {
	TushareResponse
	Data StockBasicResponse `json:"data"`
}

type StockBasicResponse struct {
	Fields  []string `json:"fields"`
	Items   [][]any  `json:"items"`
	HasMore bool     `json:"has_more"`
	Count   int      `json:"count"`
}

func (receiver StockBasic) TableName() string {
	return "tushare_stock_basic"
}

func NewStockDataApi() *StockDataApi {
	return &StockDataApi{
		client: SharedHTTPClient,
		config: GetSettingConfig(),
	}
}

// GetIndexBasic 获取指数信息
func (receiver StockDataApi) GetIndexBasic() {
	res := &TushareStockBasicResponse{}
	fields := "ts_code,name,market,publisher,category,base_date,base_point,list_date,fullname,index_type,weight_rule,desc"
	_, err := receiver.client.R().
		SetHeader("content-type", "application/json").
		SetBody(&TushareRequest{
			ApiName: "index_basic",
			Token:   receiver.config.TushareToken,
			Params:  nil,
			Fields:  fields}).
		SetResult(res).
		Post(tushareApiUrl)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return
	}
	if res.Code != 0 {
		logger.SugaredLogger.Error(res.Msg)
		return
	}
	//ioutil.WriteFile("index_basic.json", resp.Body(), 0666)

	for _, item := range res.Data.Items {
		data := map[string]any{}
		for _, field := range strings.Split(fields, ",") {
			idx := slice.IndexOf(res.Data.Fields, field)
			if idx == -1 {
				continue
			}
			data[field] = item[idx]
		}
		index := &IndexBasic{}
		jsonData, _ := json.Marshal(data)
		err := json.Unmarshal(jsonData, index)
		if err != nil {
			continue
		}
		index.ID = 0
		db.Dao.Model(&IndexBasic{}).FirstOrCreate(index, &IndexBasic{TsCode: index.TsCode}).Where("ts_code = ?", index.TsCode).Updates(index)
	}

}

// map转换为结构体

func (receiver StockDataApi) GetStockBaseInfo() {
	res := &TushareStockBasicResponse{}
	fields := "ts_code,symbol,name,area,industry,cnspell,market,list_date,act_name,act_ent_type,fullname,exchange,list_status,curr_type,enname,delist_date,is_hs"
	resp, err := receiver.client.R().
		SetHeader("content-type", "application/json").
		SetBody(&TushareRequest{
			ApiName: "stock_basic",
			Token:   receiver.config.TushareToken,
			Params:  nil,
			Fields:  fields,
		}).
		SetResult(res).
		Post(tushareApiUrl)
	//logger.SugaredLogger.Infof("GetStockBaseInfo %s", string(resp.Body()))
	os.WriteFile("stock_basic.json", resp.Body(), 0666)
	//logger.SugaredLogger.Infof("GetStockBaseInfo %+v", res)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return
	}
	if res.Code != 0 {
		logger.SugaredLogger.Error(res.Msg)
		return
	}
	for _, item := range res.Data.Items {
		stock := &StockBasic{}
		data := map[string]any{}
		for _, field := range strings.Split(fields, ",") {
			//logger.SugaredLogger.Infof("field: %s", field)
			idx := slice.IndexOf(res.Data.Fields, field)
			if idx == -1 {
				continue
			}
			data[field] = item[idx]
		}
		jsonData, _ := json.Marshal(data)
		err := json.Unmarshal(jsonData, stock)
		if err != nil {
			continue
		}
		stock.ID = 0
		db.Dao.Model(&StockBasic{}).FirstOrCreate(stock, &StockBasic{TsCode: stock.TsCode}).Where("ts_code = ?", stock.TsCode).Updates(stock)
	}

}

func (receiver StockDataApi) GetStockList(key string) []StockBasic {
	var result []StockBasic
	db.Dao.Model(&StockBasic{}).Where("name like ? or ts_code like ?", "%"+key+"%", "%"+key+"%").Find(&result)
	var result2 []IndexBasic
	db.Dao.Model(&IndexBasic{}).Where("market in ?", []string{"SSE", "SZSE"}).Where("name like ? or ts_code like ?", "%"+key+"%", "%"+key+"%").Find(&result2)

	var result3 []models.StockInfoHK
	db.Dao.Model(&models.StockInfoHK{}).Where("name like ? or code like ?", "%"+key+"%", "%"+key+"%").Find(&result3)

	var result4 []models.StockInfoUS
	db.Dao.Model(&models.StockInfoUS{}).Where("name like ? or code like ? or e_name like ?", "%"+key+"%", "%"+key+"%", "%"+key+"%").Find(&result4)

	var result5 []models.AllStockInfo
	db.Dao.Model(&models.AllStockInfo{}).Where("secucode like ? or sec_uri_tynameabbr like ?", "%"+key+"%", "%"+key+"%").Find(&result5)

	// 创建一个 map 来存储已存在的股票，用于去重
	// 使用 TsCode 作为唯一标识符
	existingStocks := make(map[string]bool)
	for _, item := range result {
		existingStocks[item.TsCode] = true
	}
	for _, item := range result2 {
		if existingStocks[item.TsCode] {
			continue
		}
		result = append(result, StockBasic{
			TsCode:   item.TsCode,
			Name:     item.Name,
			Fullname: item.FullName,
			Symbol:   item.Symbol,
			Market:   item.Market,
			ListDate: item.ListDate,
		})
		existingStocks[item.TsCode] = true

	}
	for _, item := range result3 {
		if existingStocks[item.Code] {
			continue
		}
		result = append(result, StockBasic{
			TsCode:   item.Code,
			Name:     item.Name,
			Fullname: item.Name,
			Market:   "HK",
		})
		existingStocks[item.Code] = true
	}
	for _, item := range result4 {
		code := strings.ToLower(strings.Replace(item.Code, "us", "gb_", 1))
		if existingStocks[code] {
			continue
		}
		result = append(result, StockBasic{
			TsCode:   code,
			Name:     item.Name,
			Fullname: item.Name,
			Market:   "US",
		})
		existingStocks[code] = true
	}
	for _, item := range result5 {
		if existingStocks[item.SECUCODE] {
			continue
		}
		result = append(result, StockBasic{
			TsCode:   item.SECUCODE,
			Name:     item.SECURITYNAMEABBR,
			Fullname: item.SECURITYNAMEABBR,
			Market:   item.MARKET,
		})
		existingStocks[item.SECUCODE] = true
	}

	return result
}

type IndexBasic struct {
	gorm.Model
	TsCode        string  `json:"ts_code" gorm:"index"`
	Symbol        string  `json:"symbol" gorm:"index"`
	Name          string  `json:"name" gorm:"index"`
	FullName      string  `json:"fullname"`
	IndexType     string  `json:"index_type"`
	IndexCategory string  `json:"category"`
	Market        string  `json:"market"`
	ListDate      string  `json:"list_date"`
	BaseDate      string  `json:"base_date"`
	BasePoint     float64 `json:"base_point"`
	Publisher     string  `json:"publisher"`
	WeightRule    string  `json:"weight_rule"`
	DESC          string  `json:"desc"`
}

func (IndexBasic) TableName() string {
	return "tushare_index_basic"
}

type RealTimeStockPriceInfo struct {
	StockCode string
	Price     string `json:"当前价格"`
	Time      time.Time
}

// KLineData 是 K 线行 DTO，定义已搬入 backend/datasource（R-25a），
// 此处留 type alias，全仓引用零改动。
type KLineData = datasource.KLineData

type MinuteData struct {
	Time   string  `json:"time"`
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

// AllStockInfoQuery 分页查询参数
type AllStockInfoQuery struct {
	Page          int    `form:"page" json:"page"`                 // 页码
	PageSize      int    `form:"pageSize" json:"pageSize"`         // 每页大小
	SecurityCode  string `form:"securityCode" json:"securityCode"` // 股票代码筛选
	SecurityName  string `form:"securityName" json:"securityName"` // 股票名称筛选
	Market        string `form:"market" json:"market"`             // 交易所筛选
	Industry      string `form:"industry" json:"industry"`         // 行业筛选
	Concept       string `form:"concept" json:"concept"`           // 概念筛选
	MinPrice      string `form:"minPrice" json:"minPrice"`         // 最低价筛选
	MaxPrice      string `form:"maxPrice" json:"maxPrice"`         // 最高价筛选
	MinChange     string `form:"minChange" json:"minChange"`       // 最小涨跌幅筛选
	MaxChange     string `form:"maxChange" json:"maxChange"`       // 最大涨跌幅筛选
	SearchKeyWord string `form:"searchKeyWord" json:"searchKeyWord"`
}

// AllStockInfoPageData 分页查询结果
type AllStockInfoPageData struct {
	List       []models.AllStockInfo `json:"list"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"pageSize"`
	TotalPages int                   `json:"totalPages"`
}

// GetAllStockInfoList 分页查询AllStockInfo记录
func (receiver StockDataApi) GetAllStockInfoList(query *AllStockInfoQuery) (*AllStockInfoPageData, error) {
	var list []models.AllStockInfo
	var total int64

	q := db.Dao.Model(&models.AllStockInfo{})

	// 构建查询条件
	if query.SecurityCode != "" {
		q = q.Where("secucode LIKE ?", "%"+query.SecurityCode+"%")
	}
	if query.SecurityName != "" {
		q = q.Where("sec_uri_tynameabbr LIKE ?", "%"+query.SecurityName+"%")
	}
	if query.Market != "" {
		q = q.Where("MARKET = ?", query.Market)
	}
	if query.Industry != "" {
		q = q.Where("INDUSTRY LIKE ?", "%"+query.Industry+"%")
	}
	if query.Concept != "" {
		q = q.Where("CONCEPT LIKE ?", "%"+query.Concept+"%")
	}
	if query.SearchKeyWord != "" {
		q = q.Where("secucode LIKE ? OR sec_uri_tynameabbr LIKE ?", "%"+query.SearchKeyWord+"%", "%"+query.SearchKeyWord+"%")
		q.Or("CONCEPT LIKE ? OR INDUSTRY LIKE ?", "%"+query.SearchKeyWord+"%", "%"+query.SearchKeyWord+"%")
	}

	// 计算总数
	err := q.Count(&total).Error
	if err != nil {
		return nil, err
	}

	// 设置默认分页参数
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// 执行分页查询
	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("maxtradedate DESC, secucode ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &AllStockInfoPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetAllStockInfoById 根据ID获取单个AllStockInfo记录
func (receiver StockDataApi) GetAllStockInfoById(id uint) (*models.AllStockInfo, error) {
	var stock models.AllStockInfo
	err := db.Dao.Model(&models.AllStockInfo{}).Where("id = ?", id).First(&stock).Error
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

// AddAllStockInfo 添加或更新AllStockInfo记录
func (receiver StockDataApi) AddAllStockInfo(stock models.AllStockInfo) error {
	if stock.ID > 0 {
		// 更新操作
		return db.Dao.Model(&models.AllStockInfo{}).Where("id = ?", stock.ID).Updates(stock).Error
	} else {
		// 新增操作
		return db.Dao.Model(&models.AllStockInfo{}).Create(&stock).Error
	}
}

// DeleteAllStockInfo 删除AllStockInfo记录
func (receiver StockDataApi) DeleteAllStockInfo(id uint) error {
	return db.Dao.Model(&models.AllStockInfo{}).Where("id = ?", id).Delete(&models.AllStockInfo{}).Error
}

// BatchDeleteAllStockInfo 批量删除AllStockInfo记录
func (receiver StockDataApi) BatchDeleteAllStockInfo(ids []uint) error {
	return db.Dao.Model(&models.AllStockInfo{}).Where("id IN ?", ids).Delete(&models.AllStockInfo{}).Error
}

// GetAllMarkets 获取所有交易所列表
func (receiver StockDataApi) GetAllMarkets() ([]string, error) {
	var markets []string
	err := db.Dao.Model(&models.AllStockInfo{}).Distinct("MARKET").Where("MARKET IS NOT NULL AND MARKET != ''").Order("MARKET").Pluck("MARKET", &markets).Error
	return markets, err
}

// GetAllIndustries 获取所有行业列表
func (receiver StockDataApi) GetAllIndustries() ([]string, error) {
	var industries []string
	err := db.Dao.Model(&models.AllStockInfo{}).Distinct("INDUSTRY").Where("INDUSTRY IS NOT NULL AND INDUSTRY != ''").Order("INDUSTRY").Pluck("INDUSTRY", &industries).Error
	return industries, err
}

// GetAllConcepts 获取所有概念列表
func (receiver StockDataApi) GetAllConcepts() ([]string, error) {
	var concepts []string
	err := db.Dao.Model(&models.AllStockInfo{}).Distinct("CONCEPT").Where("CONCEPT IS NOT NULL AND CONCEPT != ''").Order("CONCEPT").Pluck("CONCEPT", &concepts).Error
	return concepts, err
}

func (receiver StockDataApi) GetStockInfoByCode(secucode string) models.AllStockInfo {
	var stock models.AllStockInfo
	db.Dao.Model(&models.AllStockInfo{}).Where("secucode = ?", secucode).First(&stock)
	return stock
}

type tradingRecordFIFOLot struct {
	Volume int64
	Price  float64
}
