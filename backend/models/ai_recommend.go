package models

import (
	"time"

	"gorm.io/gorm"
)

type AiRecommendStocks struct {
	gorm.Model                  `md:"-"`
	DataTime                    *time.Time `json:"dataTime" gorm:"index;autoCreateTime" md:"推荐时间" wails:"-"`
	ModelName                   string     `json:"modelName" md:"模型名称"`
	Rating                      string     `json:"rating" md:"评级"`
	StockCode                   string     `json:"stockCode" md:"股票代码"`
	StockName                   string     `json:"stockName" md:"股票名称"`
	BkCode                      string     `json:"bkCode" md:"行业/板块代码"`
	BkName                      string     `json:"bkName" md:"行业/板块名称"`
	StockPrice                  string     `json:"stockPrice" md:"推荐时股票价格"`
	StockCurrentPrice           string     `json:"stockCurrentPrice" md:"当前价格"`
	StockCurrentPriceTime       string     `json:"stockCurrentPriceTime" md:"当前价格时间"`
	StockClosePrice             string     `json:"stockClosePrice" md:"推荐时股票收盘价格"`
	StockPrePrice               string     `json:"stockPrePrice" md:"前一交易日股票价格"`
	RecommendReason             string     `json:"recommendReason" md:"推荐理由/驱动因素/逻辑"`
	RecommendBuyPrice           string     `json:"recommendBuyPrice" md:"ai建议买入价范围"`
	RecommendBuyPriceMin        float64    `json:"recommendBuyPriceMin" md:"ai建议最低买入价"`
	RecommendBuyPriceMax        float64    `json:"recommendBuyPriceMax" md:"ai建议最高买入价"`
	RecommendStopProfitPrice    string     `json:"recommendStopProfitPrice" md:"ai建议止盈价范围"`
	RecommendStopProfitPriceMin float64    `json:"recommendStopProfitPriceMin" md:"ai建议最低止盈价"`
	RecommendStopProfitPriceMax float64    `json:"recommendStopProfitPriceMax" md:"ai建议最高止盈价"`
	RecommendStopLossPrice      string     `json:"recommendStopLossPrice" md:"ai建议止损价"`
	RiskRemarks                 string     `json:"riskRemarks" md:"风险提示"`
	Remarks                     string     `json:"remarks" md:"备注"`
	EnableAlert                 bool       `json:"enableAlert" gorm:"default:false" md:"开启预警"`
}

type AiRecommendStocksMdExport struct {
	DataTime                    string  `json:"dataTime"  md:"推荐时间"`
	ModelName                   string  `json:"modelName" md:"模型名称"`
	Rating                      string  `json:"rating" md:"评级"`
	StockCode                   string  `json:"stockCode" md:"股票代码"`
	StockName                   string  `json:"stockName" md:"股票名称"`
	BkCode                      string  `json:"bkCode" md:"行业/板块代码"`
	BkName                      string  `json:"bkName" md:"行业/板块名称"`
	StockPrice                  string  `json:"stockPrice" md:"推荐时股票价格"`
	StockCurrentPrice           string  `json:"stockCurrentPrice" md:"当前价格"`
	StockCurrentPriceTime       string  `json:"stockCurrentPriceTime" md:"当前价格时间"`
	StockClosePrice             string  `json:"stockClosePrice" md:"推荐时股票收盘价格"`
	StockPrePrice               string  `json:"stockPrePrice" md:"前一交易日股票价格"`
	RecommendReason             string  `json:"recommendReason" md:"推荐理由/驱动因素/逻辑"`
	RecommendBuyPrice           string  `json:"recommendBuyPrice" md:"ai建议买入价范围"`
	RecommendBuyPriceMin        float64 `json:"recommendBuyPriceMin" md:"ai建议最低买入价"`
	RecommendBuyPriceMax        float64 `json:"recommendBuyPriceMax" md:"ai建议最高买入价"`
	RecommendStopProfitPrice    string  `json:"recommendStopProfitPrice" md:"ai建议止盈价范围"`
	RecommendStopProfitPriceMin float64 `json:"recommendStopProfitPriceMin" md:"ai建议最低止盈价"`
	RecommendStopProfitPriceMax float64 `json:"recommendStopProfitPriceMax" md:"ai建议最高止盈价"`
	RecommendStopLossPrice      string  `json:"recommendStopLossPrice" md:"ai建议止损价"`
	RiskRemarks                 string  `json:"riskRemarks" md:"风险提示"`
	Remarks                     string  `json:"remarks" md:"备注"`
}

func (receiver AiRecommendStocks) TableName() string { return "ai_recommend_stocks" }

func (receiver AiRecommendStocks) ToMdExportStruct() AiRecommendStocksMdExport {
	return AiRecommendStocksMdExport{
		DataTime:                    receiver.DataTime.Format("2006-01-02 15:04:05"),
		ModelName:                   receiver.ModelName,
		Rating:                      receiver.Rating,
		StockCode:                   receiver.StockCode,
		StockName:                   receiver.StockName,
		BkCode:                      receiver.BkCode,
		BkName:                      receiver.BkName,
		StockPrice:                  receiver.StockPrice,
		StockCurrentPrice:           receiver.StockCurrentPrice,
		StockCurrentPriceTime:       receiver.StockCurrentPriceTime,
		StockClosePrice:             receiver.StockClosePrice,
		StockPrePrice:               receiver.StockPrePrice,
		RecommendReason:             receiver.RecommendReason,
		RecommendBuyPrice:           receiver.RecommendBuyPrice,
		RecommendBuyPriceMin:        receiver.RecommendBuyPriceMin,
		RecommendBuyPriceMax:        receiver.RecommendBuyPriceMax,
		RecommendStopProfitPrice:    receiver.RecommendStopProfitPrice,
		RecommendStopProfitPriceMin: receiver.RecommendStopProfitPriceMin,
		RecommendStopProfitPriceMax: receiver.RecommendStopProfitPriceMax,
		RecommendStopLossPrice:      receiver.RecommendStopLossPrice,
		RiskRemarks:                 receiver.RiskRemarks,
		Remarks:                     receiver.Remarks,
	}

}

type AiRecommendStocksQuery struct {
	Page        int    `form:"page" json:"page"`               // 页码
	PageSize    int    `form:"pageSize" json:"pageSize"`       // 每页大小
	ModelName   string `form:"modelName" json:"modelName"`     // 模型名称筛选
	StockCode   string `form:"stockCode" json:"stockCode"`     // 股票代码筛选
	StockName   string `form:"stockName" json:"stockName"`     // 股票名称筛选
	BkCode      string `form:"bkCode" json:"bkCode"`           // 板块代码筛选
	BkName      string `form:"bkName" json:"bkName"`           // 板块名称筛选
	StartDate   string `form:"startDate" json:"startDate"`     // 开始日期
	EndDate     string `form:"endDate" json:"endDate"`         // 结束日期
	EnableAlert *bool  `form:"enableAlert" json:"enableAlert"` // 预警状态筛选
}

type AiRecommendStocksPageResp struct {
	Code    int                       `json:"code"`
	Message string                    `json:"message"`
	Data    AiRecommendStocksPageData `json:"data"`
}

type AiRecommendStocksPageData struct {
	List       []AiRecommendStocks `json:"list"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	TotalPages int                 `json:"totalPages"`
}
