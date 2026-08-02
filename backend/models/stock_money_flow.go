package models

import "time"

type StockMoneyDataResp struct {
	Rc     int            `json:"rc"`
	Rt     int            `json:"rt"`
	Svr    int            `json:"svr"`
	Lt     int            `json:"lt"`
	Full   int            `json:"full"`
	Dlmkts string         `json:"dlmkts"`
	Data   StockMoneyData `json:"data"`
}

type StockMoneyData struct {
	Total int                  `json:"total"`
	Diff  []StockMoneyDataDiff `json:"diff"`
}

type StockMoneyDataDiff struct {
	//F1   int     `json:"f1" md:"-"`
	F12 string `json:"f12" md:"股票代码"`
	//F13  int     `json:"f13" md:"-"`
	F14  string  `json:"f14" md:"股票名称"`
	F2   float64 `json:"f2" md:"最新价"`
	F3   float64 `json:"f3" md:"今日涨跌幅(%)"`
	F62  float64 `json:"f62" md:"今日主力净额(元)"`
	F184 float64 `json:"f184" md:"今日主力净占比(%)"`
	F66  float64 `json:"f66" md:"今日超大单净额(元)"`
	F69  float64 `json:"f69" md:"今日超大单净占比(%)"`
	F72  float64 `json:"f72" md:"今日大单净额(元)"`
	F75  float64 `json:"f75" md:"今日大单净占比(%)"`
	F78  float64 `json:"f78" md:"今日中单净额(元)"`
	F81  float64 `json:"f81" md:"今日中单净占比(%)"`
	F84  float64 `json:"f84" md:"今日小单净额(元)"`
	F87  float64 `json:"f87" md:"今日小单净占比(%)"`
	//F124 int     `json:"f124" md:"f124"`
	F100 string `json:"f100" md:"所属板块"`
	F265 string `json:"f265" md:"板块代码"`
}

// MutualTop10DealResp 沪深港通十大成交股响应
type MutualTop10DealResp struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Result  MutualTop10DealData `json:"result"`
	Success bool                `json:"success"`
	Version string              `json:"version"`
}

// MutualTop10DealData 结果集
type MutualTop10DealData struct {
	Pages int                 `json:"pages"`
	Data  []MutualTop10Record `json:"data"`
	Count int                 `json:"count"`
}

// MutualTop10Record 单条互联互通十大成交股记录
// MUTUAL_TYPE 含义：001=沪股通十大成交股, 002=港股通(沪)十大成交股, 003=深股通十大成交股, 004=港股通(深)十大成交股
// 字段名参考东方财富 datacenter RPT_MUTUAL_TOP10DEAL 报表
type MutualTop10Record struct {
	MUTUALTYPE         string      `json:"MUTUAL_TYPE" md:"通道类型"`
	SECURITYCODE       string      `json:"SECURITY_CODE" md:"股票代码"`
	DERIVESECURITYCODE string      `json:"DERIVE_SECURITY_CODE" md:"完整股票代码"`
	SECURITYNAME       string      `json:"SECURITY_NAME" md:"股票名称"`
	TRADEDATE          string      `json:"TRADE_DATE" md:"交易日期"`
	CLOSEPRICE         float64     `json:"CLOSE_PRICE" md:"收盘价"`
	CHANGERATE         float64     `json:"CHANGE_RATE" md:"涨跌幅"`
	NETBUYAMT          interface{} `json:"NET_BUY_AMT" md:"净买入额"`
	RANK               int         `json:"RANK" md:"成交金额排名"`
	BUYAMT             interface{} `json:"BUY_AMT" md:"买入额"`
	SELLAMT            interface{} `json:"SELL_AMT" md:"卖出额"`
	DEALAMT            int64       `json:"DEAL_AMT" md:"成交金额"`
	DEALAMOUNT         float64     `json:"DEAL_AMOUNT" md:"总成交金额"`
	MUTUALRATIO        float64     `json:"MUTUAL_RATIO" md:"占比成交"`
	TURNOVERRATE       float64     `json:"TURNOVERRATE" md:"换手率"`
	CHANGE             float64     `json:"CHANGE" md:"涨跌额"`
}

type StockHolderNumResp struct {
	Version string `json:"version"`
	Result  struct {
		Pages int                        `json:"pages"`
		Data  []StockHolderNumRespResult `json:"data"`
		Count int                        `json:"count"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
type StockHolderNumRespResult struct {
	//SECUCODE           string  `json:"SECUCODE" md:"股票代码"`
	SECURITYCODE       string  `json:"SECURITY_CODE" md:"-"`
	ENDDATE            string  `json:"END_DATE" md:"报告结束日期"`
	HOLDERTOTALNUM     int     `json:"HOLDER_TOTAL_NUM" md:"股东人数(户)"`
	TOTALNUMRATIO      float64 `json:"TOTAL_NUM_RATIO" md:"较上期变化(%)"`
	AVGFREESHARES      int     `json:"AVG_FREE_SHARES" md:"人均流通股(股)"`
	AVGFREESHARESRATIO float64 `json:"AVG_FREESHARES_RATIO" md:"较上期变化(%)"`
	HOLDFOCUS          string  `json:"HOLD_FOCUS" md:"筹码集中度"`
	PRICE              float64 `json:"PRICE" md:"股价(元)"`
	AVGHOLDAMT         float64 `json:"AVG_HOLD_AMT" md:"人均持股金额(元)"`
	HOLDRATIOTOTAL     float64 `json:"HOLD_RATIO_TOTAL" md:"十大股东持股合计(%)"`
	FREEHOLDRATIOTOTAL float64 `json:"FREEHOLD_RATIO_TOTAL" md:"十大流通股东持股合计(%) "`
}

type StockHistoryMoneyDataResp struct {
	Rc     int    `json:"rc"`
	Rt     int    `json:"rt"`
	Svr    int    `json:"svr"`
	Lt     int    `json:"lt"`
	Full   int    `json:"full"`
	Dlmkts string `json:"dlmkts"`
	Data   struct {
		Code   string   `json:"code"`
		Market int      `json:"market"`
		Name   string   `json:"name"`
		Klines []string `json:"klines"`
	} `json:"data"`
}

type StockMoneyDataHis struct {
	Date string `json:"date" md:"日期"`
	F2   string `json:"f2" md:"最新价"`
	F3   string `json:"f3" md:"涨跌幅(%)"`
	F62  string `json:"f62" md:"主力净额(元)"`
	F184 string `json:"f184" md:"主力净占比(%)"`
	F66  string `json:"f66" md:"超大单净额(元)"`
	F69  string `json:"f69" md:"超大单净占比(%)"`
	F72  string `json:"f72" md:"大单净额(元)"`
	F75  string `json:"f75" md:"大单净占比(%)"`
	F78  string `json:"f78" md:"中单净额(元)"`
	F81  string `json:"f81" md:"中单净占比(%)"`
	F84  string `json:"f84" md:"小单净额(元)"`
	F87  string `json:"f87" md:"小单净占比(%)"`
}

type IndustryValuationResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		Count int                     `json:"count"`
		Data  []IndustryValuationData `json:"data"`
		Pages int                     `json:"pages"`
	} `json:"result"`
	Success bool   `json:"success"`
	Version string `json:"version"`
}

type IndustryValuationData struct {
	BOARDCODE            string  `json:"BOARD_CODE" md:"行业/板块代码"`
	BOARDNAME            string  `json:"BOARD_NAME" md:"行业/板块名称"`
	INDUSTRYTYPE         string  `json:"INDUSTRY_TYPE" md:"估值类型"`
	MARKETCAPVAG         float64 `json:"MARKET_CAP_VAG" md:"总市值(元)"`
	NOMARKETCAPAVAG      float64 `json:"NOMARKETCAP_A_VAG" md:"流通市值(元)"`
	NOTLIMITEDMARKETCAPA float64 `json:"NOTLIMITED_MARKETCAP_A" md:"-"`
	PBMRQ                float64 `json:"PB_MRQ" md:"市净率"`
	PCFOCFTTM            float64 `json:"PCF_OCF_TTM" md:"市现率"`
	PEGCAR               float64 `json:"PEG_CAR" md:"PEG值"`
	PELAR                float64 `json:"PE_LAR" md:"PE(静)"`
	PETTM                float64 `json:"PE_TTM" md:"PE(TTM)"`
	TOTALMARKETCAP       float64 `json:"TOTAL_MARKET_CAP" md:"-"`
	TOTALSHARES          float64 `json:"TOTAL_SHARES" md:"-"`
	TOTALSHARESVAG       float64 `json:"TOTAL_SHARES_VAG" md:"总股本(股)"`
	TRADEDATE            string  `json:"TRADE_DATE" md:"日期"`
}

type StockRZRQInfoResp struct {
	Version string `json:"version"`
	Result  struct {
		Pages int             `json:"pages"`
		Data  []StockRZRQInfo `json:"data"`
		Count int             `json:"count"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type StockRZRQInfo struct {
	//MARKETNAME string `json:"MARKET_NAME" md:"交易所"`
	//MARKETCODE string `json:"MARKET_CODE" md:"交易所代码"`
	TRADEDATE string `json:"TRADE_DATE" md:"交易日期"`
	//SECURITYCODE     string  `json:"SECURITY_CODE" md:"股票代码"`
	//SECUCODE         string  `json:"SECUCODE" md:"股票代码"`
	//SECURITYNAMEABBR string  `json:"SECURITY_NAME_ABBR" md:"股票名称"`
	FINBALANCE     int64   `json:"FIN_BALANCE" md:"融资余额(元)"`
	FINBUYAMT      int     `json:"FIN_BUY_AMT" md:"融资买入额(元)"`
	FINREPAYAMT    int     `json:"FIN_REPAY_AMT" md:"融资还款额(元)"`
	LOANBALANCE    float64 `json:"LOAN_BALANCE" md:"融券余额(元)"`
	LOANSELLVOL    int     `json:"LOAN_SELL_VOL" md:"融券卖出量(股)"`
	LOANREPAYVOL   int     `json:"LOAN_REPAY_VOL" md:"融券还款量(股)"`
	MARGINBALANCE  float64 `json:"MARGIN_BALANCE" md:"两融余额(股)"`
	LOANBALANCEVOL int     `json:"LOAN_BALANCE_VOL" md:"融券余量(股)"`
	FINNETBUYAMT   int     `json:"FIN_NETBUY_AMT" md:"融资净买入额(元)"`
}

// BKFundFlow 板块资金流向数据
type BKFundFlow struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Code      string    `json:"code" gorm:"size:20;index:idx_bk_code_time"`     // 板块代码 BK0475
	Name      string    `json:"name" gorm:"size:50"`                            // 板块名称
	NetInflow int64     `json:"netInflow"`                                      // 主力净流入金额（元）
	SnapTime  string    `json:"snapTime" gorm:"size:19;index:idx_bk_code_time"` // 快照时间 YYYY-MM-DD HH:MM:SS
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime" wails:"-"`
}

func (BKFundFlow) TableName() string {
	return "bk_fund_flow"
}

// BKFundFlowPoint 板块资金流向数据点（前端折线图用）
type BKFundFlowPoint struct {
	SnapTime  string `json:"snapTime"`
	NetInflow int64  `json:"netInflow"`
}

// ConceptFundFlow 概念资金流向数据
type ConceptFundFlow struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Code      string    `json:"code" gorm:"size:20;index:idx_concept_code_time"`     // 概念代码
	Name      string    `json:"name" gorm:"size:50"`                                 // 概念名称
	NetInflow int64     `json:"netInflow"`                                           // 主力净流入金额（元）
	SnapTime  string    `json:"snapTime" gorm:"size:19;index:idx_concept_code_time"` // 快照时间 YYYY-MM-DD HH:MM:SS
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime" wails:"-"`
}

func (ConceptFundFlow) TableName() string {
	return "concept_fund_flow"
}

// ConceptFundFlowPoint 概念资金流向数据点（前端折线图用）
type ConceptFundFlowPoint struct {
	SnapTime  string `json:"snapTime"`
	NetInflow int64  `json:"netInflow"`
}
