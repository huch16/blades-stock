package models

import (
	"time"

	"gorm.io/gorm"
)

type LongTigerRankData struct {
	ACCUMAMOUNT      float64 `json:"ACCUM_AMOUNT"`
	BILLBOARDBUYAMT  float64 `json:"BILLBOARD_BUY_AMT"`
	BILLBOARDDEALAMT float64 `json:"BILLBOARD_DEAL_AMT"`
	BILLBOARDNETAMT  float64 `json:"BILLBOARD_NET_AMT"`
	BILLBOARDSELLAMT float64 `json:"BILLBOARD_SELL_AMT"`
	CHANGERATE       float64 `json:"CHANGE_RATE"`
	CLOSEPRICE       float64 `json:"CLOSE_PRICE"`
	DEALAMOUNTRATIO  float64 `json:"DEAL_AMOUNT_RATIO"`
	DEALNETRATIO     float64 `json:"DEAL_NET_RATIO"`
	EXPLAIN          string  `json:"EXPLAIN"`
	EXPLANATION      string  `json:"EXPLANATION"`
	FREEMARKETCAP    float64 `json:"FREE_MARKET_CAP"`
	SECUCODE         string  `json:"SECUCODE" gorm:"index"`
	SECURITYCODE     string  `json:"SECURITY_CODE"`
	SECURITYNAMEABBR string  `json:"SECURITY_NAME_ABBR"`
	SECURITYTYPECODE string  `json:"SECURITY_TYPE_CODE"`
	TRADEDATE        string  `json:"TRADE_DATE" gorm:"index"`
	TURNOVERRATE     float64 `json:"TURNOVERRATE"`
}

func (l LongTigerRankData) TableName() string {
	return "long_tiger_rank"
}

type XUEQIUHot struct {
	Data struct {
		Items     []HotItem `json:"items"`
		ItemsSize int       `json:"items_size"`
	} `json:"data"`
	ErrorCode        int    `json:"error_code"`
	ErrorDescription string `json:"error_description"`
}

type HotItem struct {
	//Type         int         `json:"type" md:"-"`
	Code       string  `json:"code" md:"股票代码"`
	Name       string  `json:"name" md:"股票名称"`
	Value      float64 `json:"value" md:"热度"`
	Increment  int     `json:"increment" md:"热度变化"`
	RankChange int     `json:"rank_change" md:"排名变化"`
	//HasExist     interface{} `json:"has_exist" md:"-"`
	//Symbol       string      `json:"symbol" md:"-"`
	Percent  float64 `json:"percent" md:"涨跌幅(%)"`
	Current  float64 `json:"current" md:"股价"`
	Chg      float64 `json:"chg" md:"股价变化"`
	Exchange string  `json:"exchange" md:"交易所代码"`
	//StockType    int         `json:"stock_type" md:"-"`
	//SubType      string      `json:"sub_type" md:"-"`
	//Ad           int         `json:"ad" md:"-"`
	//AdId         interface{} `json:"ad_id" md:"-"`
	//ContentId    interface{} `json:"content_id" md:"-"`
	//Page         interface{} `json:"page" md:"-"`
	//Model        interface{} `json:"model" md:"-"`
	//Location     interface{} `json:"location" md:"-"`
	//TradeSession interface{} `json:"trade_session" md:"-"`
	//CurrentExt   interface{} `json:"current_ext" md:"-"`
	//PercentExt   interface{} `json:"percent_ext" md:"-"`
}

type HotEvent struct {
	PicSize     interface{} `json:"pic_size"`
	Tag         string      `json:"tag"`
	Id          int         `json:"id"`
	Pic         string      `json:"pic"`
	Hot         int         `json:"hot"`
	StatusCount int         `json:"status_count"`
	Content     string      `json:"content"`
}

type HotStrategy struct {
	ChgEffect bool               `json:"chgEffect"`
	Code      int                `json:"code"`
	Data      []*HotStrategyData `json:"data"`
	Message   string             `json:"message"`
}

type HotStrategyData struct {
	Chg       float64 `json:"chg" md:"平均涨幅(%)"`
	Code      string  `json:"code" md:"-"`
	HeatValue int     `json:"heatValue" md:"热度值"`
	Market    string  `json:"market" md:"-"`
	Question  string  `json:"question" md:"选股策略"`
	Rank      int     `json:"rank" md:"-"`
}

type THSHotStrategy struct {
	Result struct {
		Num  int `json:"num"`
		List []struct {
			Author struct {
				Avatar   string `json:"avatar"`
				UserName string `json:"userName"`
				UserId   int    `json:"userId"`
			} `json:"author"`
			Property struct {
				Id          int         `json:"id"`
				Name        string      `json:"name"`
				Query       string      `json:"query"`
				Logic       string      `json:"logic"`
				BuyPosition interface{} `json:"buyPosition"`
				Ctime       string      `json:"ctime"`
				Tags        []string    `json:"tags"`
				WinRate     string      `json:"winRate"`
				AnnualYield string      `json:"annualYield"`
				Type        int         `json:"type"`
			} `json:"property"`
			Interaction struct {
				CommentNum  int  `json:"commentNum"`
				CollectNum  int  `json:"collectNum"`
				IsCollected bool `json:"isCollected"`
				IsSubscribe int  `json:"isSubscribe"`
				IsPublish   int  `json:"isPublish"`
				Pid         int  `json:"pid"`
			} `json:"interaction"`
		} `json:"list"`
	} `json:"result"`
}

type SecuritiesCompanyOpinionResp struct {
	Hits        int                             `json:"hits"`
	Size        int                             `json:"size"`
	Data        []*SecuritiesCompanyOpinionData `json:"data"`
	TotalPage   int                             `json:"TotalPage"`
	AuthorName  string                          `json:"authorName"`
	PageNo      int                             `json:"pageNo"`
	OrgSName    string                          `json:"orgSName"`
	CurrentYear int                             `json:"currentYear"`
}

type SecuritiesCompanyOpinionData struct {
	Id           int64       `json:"id"`
	Title        string      `json:"title"`
	Author       interface{} `json:"author"`
	OrgName      string      `json:"orgName"`
	OrgCode      string      `json:"orgCode"`
	OrgSName     string      `json:"orgSName"`
	PublishDate  string      `json:"publishDate"`
	EncodeUrl    string      `json:"encodeUrl"`
	Researcher   string      `json:"researcher"`
	Market       string      `json:"market"`
	IndustryCode string      `json:"industryCode"`
	IndustryName string      `json:"industryName"`
	AuthorID     interface{} `json:"authorID"`
	Count        int         `json:"count"`
	OrgType      string      `json:"orgType"`
	OpinionData  string      `json:"opinionData"`
}

type WordAnalyze struct {
	gorm.Model
	DataTime *time.Time `json:"dataTime" gorm:"index;autoCreateTime" wails:"-"`
	WordFreqWithWeight
}

// WordFreqWithWeight 词频统计结果，包含权重信息
type WordFreqWithWeight struct {
	Word      string
	Frequency int
	Weight    float64
	Score     float64
}

// SentimentResult 情感分析结果类型
type SentimentResult struct {
	Score         float64       // 情感得分
	Category      SentimentType // 情感类别
	PositiveCount int           // 正面词数量
	NegativeCount int           // 负面词数量
	Description   string        // 情感描述
}

type SentimentResultAnalyze struct {
	gorm.Model
	DataTime *time.Time `json:"dataTime" gorm:"index;autoCreateTime" wails:"-"`
	SentimentResult
}

// SentimentType 情感类型枚举
type SentimentType int
