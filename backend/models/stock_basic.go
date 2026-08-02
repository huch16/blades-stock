package models

import (
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type StockChangeHistory struct {
	ID         uint      `json:"id" gorm:"primarykey" md:"-"`
	ChangeTime string    `json:"changeTime" gorm:"uniqueIndex:idx_unique_change;size:10" md:"异动时间"`       // 异动时间 HH:MM:SS
	ChangeDate string    `json:"changeDate" gorm:"uniqueIndex:idx_unique_change;index;size:10" md:"异动日期"` // 异动日期 YYYY-MM-DD
	StockCode  string    `json:"stockCode" gorm:"uniqueIndex:idx_unique_change;index;size:20" md:"股票代码"`  // 股票代码
	StockName  string    `json:"stockName" gorm:"size:50" md:"股票名称"`                                      // 股票名称
	Market     int       `json:"market" md:"-"`                                                           // 市场
	ChangeType int       `json:"changeType" gorm:"uniqueIndex:idx_unique_change;index" md:"-"`            // 异动类型代码
	TypeName   string    `json:"typeName" gorm:"size:20" md:"异动类型名称"`                                     // 异动类型名称
	Volume     int64     `json:"volume" gorm:"uniqueIndex:idx_unique_change" md:"成交量(股)"`                 // 成交量(股)
	Price      float64   `json:"price" gorm:"uniqueIndex:idx_unique_change" md:"价格"`                      // 价格
	ChangeRate float64   `json:"changeRate" gorm:"uniqueIndex:idx_unique_change" md:" 涨跌幅(%)"`            // 涨跌幅(%)
	Amount     float64   `json:"amount" gorm:"uniqueIndex:idx_unique_change" md:"金额"`                     // 金额
	Industry   string    `json:"industry" gorm:"size:100" md:"所属行业"`                                      // 所属行业
	Concept    string    `json:"concept" gorm:"size:500" md:"所属概念"`                                       // 所属概念
	CreatedAt  time.Time `json:"createdAt" gorm:"autoCreateTime" md:"-" wails:"-"`
}

func (StockChangeHistory) TableName() string {
	return "stock_change_history"
}

type StockChangeHistoryQuery struct {
	StockCode     string  `json:"stockCode"`
	StockName     string  `json:"stockName"`
	ChangeType    int     `json:"changeType"`
	ChangeTypes   []int   `json:"changeTypes"`
	TypeName      string  `json:"typeName"`
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	StartTime     string  `json:"startTime"`
	EndTime       string  `json:"endTime"`
	MinVolume     int64   `json:"minVolume"`
	MinAmount     float64 `json:"minAmount"`
	MinChangeRate float64 `json:"minChangeRate"`
	MaxChangeRate float64 `json:"maxChangeRate"`
	Industry      string  `json:"industry"`
	Concept       string  `json:"concept"`
	Page          int     `json:"page"`
	PageSize      int     `json:"pageSize"`
}

type StockChangeHistoryPageData struct {
	List       []StockChangeHistory `json:"list"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"pageSize"`
	TotalPages int                  `json:"totalPages"`
}

type StockInfoHK struct {
	gorm.Model
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	FullName string                `json:"fullName"`
	EName    string                `json:"eName"`
	IsDel    soft_delete.DeletedAt `gorm:"softDelete:flag"`
	BKName   string                `json:"bk_name"`
	BKCode   string                `json:"bk_code"`
}

func (receiver StockInfoHK) TableName() string {
	return "stock_base_info_hk"
}

type StockInfoUS struct {
	gorm.Model
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	FullName string                `json:"fullName"`
	EName    string                `json:"eName"`
	Exchange string                `json:"exchange"`
	Type     string                `json:"type"`
	IsDel    soft_delete.DeletedAt `gorm:"softDelete:flag"`
	BKName   string                `json:"bk_name"`
	BKCode   string                `json:"bk_code"`
}

func (receiver StockInfoUS) TableName() string {
	return "stock_base_info_us"
}

type BKDict struct {
	gorm.Model  `md:"-"`
	BkCode      string `json:"bkCode" md:"行业/板块代码"`
	BkName      string `json:"bkName" md:"行业/板块名称"`
	FirstLetter string `json:"firstLetter" md:"first_letter"`
	FubkCode    string `json:"fubkCode" md:"fubk_code"`
	PublishCode string `json:"publishCode" md:"publish_code"`
}

func (b BKDict) TableName() string {
	return "bk_dict"
}

type SinaStockInfo struct {
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	Engname       string `json:"engname"`
	Tradetype     string `json:"tradetype"`
	Lasttrade     string `json:"lasttrade"`
	Prevclose     string `json:"prevclose"`
	Open          string `json:"open"`
	High          string `json:"high"`
	Low           string `json:"low"`
	Volume        string `json:"volume"`
	Currentvolume string `json:"currentvolume"`
	Amount        string `json:"amount"`
	Ticktime      string `json:"ticktime"`
	Buy           string `json:"buy"`
	Sell          string `json:"sell"`
	High52Week    string `json:"high_52week"`
	Low52Week     string `json:"low_52week"`
	Eps           string `json:"eps"`
	Dividend      string `json:"dividend"`
	StocksSum     string `json:"stocks_sum"`
	Pricechange   string `json:"pricechange"`
	Changepercent string `json:"changepercent"`
	MarketValue   string `json:"market_value"`
	PeRatio       string `json:"pe_ratio"`
}

type AllStocksResp struct {
	Version interface{} `json:"version"`
	Result  struct {
		Nextpage    bool          `json:"nextpage"`
		Currentpage int           `json:"currentpage"`
		Data        []StockInfo   `json:"data"`
		Config      []interface{} `json:"config"`
		Count       int           `json:"count"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
	Url     string `json:"url"`
}

type StockInfo struct {
	SECUCODE         string `json:"SECUCODE" md:"股票代码" gorm:"index;secucode"`
	SECURITYCODE     string `json:"SECURITY_CODE" md:"股票代码(精简)" gorm:"index;securitycode"`
	SECURITYNAMEABBR string `json:"SECURITY_NAME_ABBR" md:"股票名称" gorm:"index;securitynameabbr"`
	NEWPRICE         any    `json:"NEW_PRICE" md:"最新价" gorm:"newprice"`
	CHANGERATE       any    `json:"CHANGE_RATE" md:"涨跌幅(%)" gorm:"changerate"`
	VOLUMERATIO      any    `json:"VOLUME_RATIO" md:"量比" gorm:"volumeratio"`
	HIGHPRICE        any    `json:"HIGH_PRICE" md:"最高价" gorm:"highprice"`
	LOWPRICE         any    `json:"LOW_PRICE" md:"最低价" gorm:"lowprice"`
	PRECLOSEPRICE    any    `json:"PRE_CLOSE_PRICE" md:"前一交易日收盘价" gorm:"precloseprice"`
	VOLUME           any    `json:"VOLUME" md:"成交量" gorm:"volume"`
	DEALAMOUNT       any    `json:"DEAL_AMOUNT" md:"成交额（元）" gorm:"dealamount"`
	TURNOVERRATE     any    `json:"TURNOVERRATE" md:"换手率(%)" gorm:"turnoverrate"`
	MARKET           string `json:"MARKET" md:"交易所" gorm:"index;market"`
	CONCEPT          any    `json:"CONCEPT" md:"所属概念" gorm:"index;concept"`
	INDUSTRY         string `json:"INDUSTRY" md:"所属行业" gorm:"index;industry"`
	MAXTRADEDATE     string `json:"MAX_TRADE_DATE" md:"数据日期" gorm:"index;maxtradedate"`
}

func (receiver StockInfo) ToAllStockInfo() AllStockInfo {
	return AllStockInfo{
		SECUCODE:         receiver.SECUCODE,
		SECURITYCODE:     receiver.SECURITYCODE,
		SECURITYNAMEABBR: receiver.SECURITYNAMEABBR,
		NEWPRICE:         convertor.ToString(receiver.NEWPRICE),
		CHANGERATE:       convertor.ToString(receiver.CHANGERATE),
		VOLUMERATIO:      convertor.ToString(receiver.VOLUMERATIO),
		HIGHPRICE:        convertor.ToString(receiver.HIGHPRICE),
		LOWPRICE:         convertor.ToString(receiver.LOWPRICE),
		PRECLOSEPRICE:    convertor.ToString(receiver.PRECLOSEPRICE),
		VOLUME:           convertor.ToString(receiver.VOLUME),
		DEALAMOUNT:       convertor.ToString(receiver.DEALAMOUNT),
		TURNOVERRATE:     convertor.ToString(receiver.TURNOVERRATE),
		MARKET:           receiver.MARKET,
		CONCEPT:          convertor.ToString(receiver.CONCEPT),
		INDUSTRY:         receiver.INDUSTRY,
		MAXTRADEDATE:     receiver.MAXTRADEDATE,
	}

}

type AllStockInfo struct {
	gorm.Model
	SECUCODE         string `json:"SECUCODE" md:"股票代码" gorm:"index;secucode"`
	SECURITYCODE     string `json:"SECURITY_CODE" md:"股票代码(精简)" gorm:"index;securitycode"`
	SECURITYNAMEABBR string `json:"SECURITY_NAME_ABBR" md:"股票名称" gorm:"index;securitynameabbr"`
	NEWPRICE         string `json:"NEW_PRICE" md:"最新价" gorm:"newprice"`
	CHANGERATE       string `json:"CHANGE_RATE" md:"涨跌幅(%)" gorm:"changerate"`
	VOLUMERATIO      string `json:"VOLUME_RATIO" md:"量比" gorm:"volumeratio"`
	HIGHPRICE        string `json:"HIGH_PRICE" md:"最高价" gorm:"highprice"`
	LOWPRICE         string `json:"LOW_PRICE" md:"最低价" gorm:"lowprice"`
	PRECLOSEPRICE    string `json:"PRE_CLOSE_PRICE" md:"前一交易日收盘价" gorm:"precloseprice"`
	VOLUME           string `json:"VOLUME" md:"成交量" gorm:"volume"`
	DEALAMOUNT       string `json:"DEAL_AMOUNT" md:"成交额（元）" gorm:"dealamount"`
	TURNOVERRATE     string `json:"TURNOVERRATE" md:"换手率(%)" gorm:"turnoverrate"`
	MARKET           string `json:"MARKET" md:"交易所" gorm:"index;market"`
	CONCEPT          string `json:"CONCEPT" md:"所属概念" gorm:"index;concept"`
	INDUSTRY         string `json:"INDUSTRY" md:"所属行业" gorm:"index;industry"`
	MAXTRADEDATE     string `json:"MAX_TRADE_DATE" md:"数据日期" gorm:"index;maxtradedate"`
}

func (s AllStockInfo) TableName() string {
	return "all_stock_info"
}

type TechnicalIndicators struct {
	MACDGOLDENFORK     bool `json:"MACD_GOLDEN_FORK" operator:"="`
	KDJGOLDENFORK      bool `json:"KDJ_GOLDEN_FORK" operator:"="`
	BREAKTHROUGH       bool `json:"BREAK_THROUGH" operator:"="`
	LOWFUNDSINFLOW     bool `json:"LOW_FUNDS_INFLOW" operator:"="`
	HIGHFUNDSOUTFLOW   bool `json:"HIGH_FUNDS_OUTFLOW" operator:"="`
	BREAKUPMA5DAYS     bool `json:"BREAKUP_MA_5DAYS" operator:"="`
	LONGAVGARRAY       bool `json:"LONG_AVG_ARRAY" operator:"="`
	SHORTAVGARRAY      bool `json:"SHORT_AVG_ARRAY" operator:"="`
	UPPERLARGEVOLUME   bool `json:"UPPER_LARGE_VOLUME" operator:"="`
	DOWNNARROWVOLUME   bool `json:"DOWN_NARROW_VOLUME" operator:"="`
	ONEDAYANGLINE      bool `json:"ONE_DAYANG_LINE" operator:"="`
	TWODAYANGLINES     bool `json:"TWO_DAYANG_LINES" operator:"="`
	RISESUN            bool `json:"RISE_SUN" operator:"="`
	POWERFULGUN        bool `json:"POWER_FULGUN" operator:"="`
	RESTOREJUSTICE     bool `json:"RESTORE_JUSTICE" operator:"="`
	DOWN7DAYS          bool `json:"DOWN_7DAYS" operator:"="`
	UPPER8DAYS         bool `json:"UPPER_8DAYS" operator:"="`
	UPPER9DAYS         bool `json:"UPPER_9DAYS" operator:"="`
	UPPER4DAYS         bool `json:"UPPER_4DAYS" operator:"="`
	HEAVENRULE         bool `json:"HEAVEN_RULE" operator:"="`
	UPSIDEVOLUME       bool `json:"UPSIDE_VOLUME" operator:"="`
	BEARISHENGULFING   bool `json:"BEARISH_ENGULFING" operator:"="`
	REVERSINGHAMMER    bool `json:"REVERSING_HAMMER" operator:"="`
	SHOOTINGSTAR       bool `json:"SHOOTING_STAR" operator:"="`
	EVENINGSTAR        bool `json:"EVENING_STAR" operator:"="`
	FIRSTDAWN          bool `json:"FIRST_DAWN" operator:"="`
	PREGNANT           bool `json:"PREGNANT" operator:"="`
	BLACKCLOUDTOPS     bool `json:"BLACK_CLOUD_TOPS" operator:"="`
	MORNINGSTAR        bool `json:"MORNING_STAR" operator:"="`
	NARROWFINISH       bool `json:"NARROW_FINISH" operator:"="`
	UPP_DAYS           int  `json:"UPP_DAYS" operator:">="`
	CONCERN_RANK_7DAYS int  `json:"CONCERN_RANK_7DAYS" operator:"<="`
	UPNDAY             int  `json:"UPNDAY" operator:">="`
	DOWNNDAY           int  `json:"DOWNNDAY" operator:">="`
}
