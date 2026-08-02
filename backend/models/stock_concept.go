package models

type StockConceptInfoResp struct {
	Version string                 `json:"version"`
	Result  StockConceptInfoResult `json:"result"`
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Code    int                    `json:"code"`
}

type StockConceptInfoResult struct {
	Pages int                `json:"pages"`
	Data  []StockConceptInfo `json:"data"`
	Count int                `json:"count"`
}

type StockConceptInfo struct {
	//SECUCODE string `json:"SECUCODE" md:"完整股票代码"`
	//SECURITYCODE        string  `json:"SECURITY_CODE" md:"股票代码"`
	//SECURITYNAMEABBR string `json:"SECURITY_NAME_ABBR" md:"股票名称"`
	//NEWBOARDCODE        string  `json:"NEW_BOARD_CODE" md:"板块/概念代码"`
	BOARDNAME           string `json:"BOARD_NAME" md:"板块/概念名称"`
	SELECTEDBOARDREASON string `json:"SELECTED_BOARD_REASON" md:"板块/概念描述"`
	//ISPRECISE           string  `json:"IS_PRECISE" md:"-"`
	//BOARDRANK           int     `json:"BOARD_RANK" md:"-"`
	BOARDYIELD float64 `json:"BOARD_YIELD" md:"板块/概念涨跌幅(%)"`
	//DERIVEBOARDCODE     string  `json:"DERIVE_BOARD_CODE" md:"-"`
}
