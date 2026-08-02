package models

import (
	"time"

	"gorm.io/gorm"
)

// GlobalStockIndex 全球股市指数缓存表
type GlobalStockIndex struct {
	gorm.Model
	Code       string `json:"code" gorm:"index"`   // 指数代码，如 DJI, IXIC, 000001
	Name       string `json:"name" md:"指数名称"`      // 指数名称，如 道琼斯, 上证指数
	Location   string `json:"location" md:"地区"`    // 地区，如 纽约, 上海
	Qtcode     string `json:"qtcode" gorm:"index"` // 行情代码，如 s_usDJI, sh000001
	State      string `json:"state" md:"状态"`       // 状态：open(开盘), close(收盘), break(休市)
	Zdf        string `json:"zdf" md:"涨跌幅(%)"`     // 涨跌幅百分比
	Zxj        string `json:"zxj" md:"最新点位"`       // 最新点位/价格
	Img        string `json:"img" md:"图标URL"`      // 图标URL
	Region     string `json:"region" gorm:"index"` // 区域：america(美洲), asia(亚洲), europe(欧洲), common(重点关注), other(其他)
	RegionName string `json:"regionName" md:"区域名称"`
}

func (GlobalStockIndex) TableName() string {
	return "global_stock_index"
}

type MarketStatistic struct {
	ID            uint      `json:"id" gorm:"primarykey"`
	DataDate      string    `json:"dataDate" gorm:"index;size:10"` // 日期 YYYY-MM-DD
	DataTime      string    `json:"dataTime" gorm:"index;size:8"`  // 时间 HH:MM
	UpCount       int       `json:"upCount"`                       // 上涨家数
	DownCount     int       `json:"downCount"`                     // 下跌家数
	UpRatio       float64   `json:"upRatio"`                       // 涨跌比(上涨家数占比)
	UpDownRatio   float64   `json:"upDownRatio"`                   // 市场情绪指标(上涨家数/下跌家数)
	SentimentDesc string    `json:"sentimentDesc" gorm:"size:20"`  // 市场情绪描述
	LimitUp       int       `json:"limitUp"`                       // 涨停家数
	LimitDown     int       `json:"limitDown"`                     // 跌停家数
	LimitRatio    float64   `json:"limitRatio"`                    // 涨跌停比(涨停/跌停)
	ShUpCount     int       `json:"shUpCount"`                     // 上海上涨家数
	ShDownCount   int       `json:"shDownCount"`                   // 上海下跌家数
	SzUpCount     int       `json:"szUpCount"`                     // 深圳上涨家数
	SzDownCount   int       `json:"szDownCount"`                   // 深圳下跌家数
	CreatedAt     time.Time `json:"createdAt" gorm:"autoCreateTime" wails:"-"`
}

func (MarketStatistic) TableName() string {
	return "market_statistic"
}
