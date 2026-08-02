package models

// StockFinancialInfoResp
type StockFinancialInfoResp struct {
	Version string `json:"version"`
	Result  struct {
		Pages int                            `json:"pages"`
		Data  []StockFinancialInfoRespResult `json:"data"`
		Count int                            `json:"count"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// StockFinancialInfoRespResult
type StockFinancialInfoRespResult struct {
	//SECUCODE         string `json:"SECUCODE" md:"股票代码"`
	//SECURITYCODE     string `json:"SECURITY_CODE" md:"-"`
	//SECURITYNAMEABBR string `json:"SECURITY_NAME_ABBR" md:"股票名称"`
	//ORGCODE                string  `json:"ORG_CODE" md:"-"`
	//ORGTYPE                string  `json:"ORG_TYPE" md:"-"`
	REPORTDATE string `json:"REPORT_DATE" md:"报告日期"`
	//REPORTTYPE             string  `json:"REPORT_TYPE" md:"报告类型"`
	//REPORTDATENAME         string  `json:"REPORT_DATE_NAME" md:"报告类型"`
	//SECURITYTYPECODE       string  `json:"SECURITY_TYPE_CODE" md:"-"`
	//NOTICEDATE             string  `json:"NOTICE_DATE" md:"提醒日期"`
	//UPDATEDATE             string  `json:"UPDATE_DATE" md:"更新日期"`
	//CURRENCY               string  `json:"CURRENCY" md:"货币单位"`
	NETPROFIT             float64 `json:"NETPROFIT" md:"净利润(元)"`
	TOTALOPERATEINCOME    float64 `json:"TOTAL_OPERATE_INCOME" md:"营业总收入(元)"`
	TOTALASSETS           float64 `json:"TOTAL_ASSETS" md:"总资产(元)"`
	TOTALLIABILITIES      float64 `json:"TOTAL_LIABILITIES" md:"总负债(元)"`
	TOTALCURRENTASSETS    float64 `json:"TOTAL_CURRENT_ASSETS" md:"总流动资产(元)"`
	TOTALNONCURRENTASSETS float64 `json:"TOTAL_NONCURRENT_ASSETS" md:"总非流动资产(元)"`
	PARENTNETPROFIT       float64 `json:"PARENT_NETPROFIT" md:"归属于母公司股东的净利润(元)"`
	SALENPR               float64 `json:"SALE_NPR" md:"销售净利率(%)"`
	TOTALASSETSTR         float64 `json:"TOTAL_ASSETS_TR" md:"总资产周转率(%)"`
	JROA                  float64 `json:"JROA" md:"总资产收益率(加权)(%)"`
	PARENTNETPROFITRATIO  float64 `json:"PARENT_NETPROFIT_RATIO" md:"母公司净利润占比(%)"`
	//EQUITYMULTIPLIER      float64 `json:"EQUITY_MULTIPLIER" md:"权益乘数(%)"`
	ROE            float64 `json:"ROE" md:"ROE(%)"`
	DEBTASSETRATIO float64 `json:"DEBT_ASSET_RATIO" md:"负债资产比率(%)"`
	TOTALINCOME    float64 `json:"TOTAL_INCOME" md:"总收入(元)"`
	TOTALCOST      float64 `json:"TOTAL_COST" md:"总成本(元)"`
	TOTALEXPENSE   float64 `json:"TOTAL_EXPENSE" md:"总费用(元)"`
	MONETARYFUNDS  float64 `json:"MONETARYFUNDS" md:"货币资金(元)"`
	TRADEFINASSET  float64 `json:"TRADE_FINASSET" md:"交易性金融资产(元)"`
	NOTERECE       float64 `json:"NOTE_RECE" md:"应收票据(元)"`
	ACCOUNTSRECE   float64 `json:"ACCOUNTS_RECE" md:"应收账款(元)"`
	FINANCERECE    float64 `json:"FINANCE_RECE" md:"应收款项融资(元)"`
	OTHERRECE      float64 `json:"OTHER_RECE" md:"其他应收款项(元)"`
	INVENTORY      float64 `json:"INVENTORY" md:"存货(元)"`
	//CREDITORINVEST        float64 `json:"CREDITOR_INVEST" md:"CREDITORINVEST"`
	LONGEQUITYINVEST   float64 `json:"LONG_EQUITY_INVEST" md:"长期股权投资(元)"`
	INVESTREALESTATE   float64 `json:"INVEST_REALESTATE" md:"投资性房地产(元)"`
	FIXEDASSET         float64 `json:"FIXED_ASSET" md:"固定资产(元)"`
	CIP                float64 `json:"CIP" md:"在建工程(元)"`
	USERIGHTASSET      float64 `json:"USERIGHT_ASSET" md:"使用权资产(元)"`
	INTANGIBLEASSET    float64 `json:"INTANGIBLE_ASSET" md:"无形资产(元)"`
	DEVELOPEXPENSE     float64 `json:"DEVELOP_EXPENSE" md:"开发支出(元)"`
	GOODWILL           float64 `json:"GOODWILL" md:"商誉(元)"`
	LONGPREPAIDEXPENSE float64 `json:"LONG_PREPAID_EXPENSE" md:"长期待摊费用(元)"`
	DEFERTAXASSET      float64 `json:"DEFER_TAX_ASSET" md:"递延所得税资产(元)"`
	INVESTINCOME       float64 `json:"INVEST_INCOME" md:"投资收益(元)"`
	//EXCHANGEINCOME         float64 `json:"EXCHANGE_INCOME" md:"EXCHANGEINCOME"`
	FAIRVALUECHANGEINCOME float64 `json:"FAIRVALUE_CHANGE_INCOME" md:"公允价值变动收益(元)"`
	ASSETDISPOSALINCOME   float64 `json:"ASSET_DISPOSAL_INCOME" md:"资产处置收益(元)"`
	OPERATECOST           float64 `json:"OPERATE_COST" md:"经营成本(元)"`
	//SURRENDERVALUE         float64 `json:"SURRENDER_VALUE" md:"SURRENDERVALUE"`
	//NETCOMPENSATEEXPENSE   float64 `json:"NET_COMPENSATE_EXPENSE" md:"NETCOMPENSATEEXPENSE"`
	//NETCONTRACTRESERVE     float64 `json:"NET_CONTRACT_RESERVE" md:"NETCONTRACTRESERVE"`
	//POLICYBONUSEXPENSE     float64 `json:"POLICY_BONUS_EXPENSE" md:"POLICYBONUSEXPENSE"`
	OPERATETAXADD          float64 `json:"OPERATE_TAX_ADD" md:"营业税金及附加(元)"`
	INCOMETAX              float64 `json:"INCOME_TAX" md:"所得税(元)"`
	ASSETIMPAIRMENTINCOME  float64 `json:"ASSET_IMPAIRMENT_INCOME" md:"资产减值损失(新)"`
	CREDITIMPAIRMENTINCOME float64 `json:"CREDIT_IMPAIRMENT_INCOME" md:"信用减值损失(新)"`
	NONBUSINESSEXPENSE     float64 `json:"NONBUSINESS_EXPENSE" md:"营业外支出(元)"`
	FINANCEEXPENSE         float64 `json:"FINANCE_EXPENSE" md:"财务费用(元)"`
	SALEEXPENSE            float64 `json:"SALE_EXPENSE" md:"销售费用(元)"`
	MANAGEEXPENSE          float64 `json:"MANAGE_EXPENSE" md:"管理费用(元)"`
	RESEARCHEXPENSE        float64 `json:"RESEARCH_EXPENSE" md:"研发费用(元)"`
	//INTERESTNI             float64 `json:"INTEREST_NI" md:"INTERESTNI"`
	//FEECOMMISSIONNI        float64 `json:"FEE_COMMISSION_NI" md:"FEECOMMISSIONNI"`
	//EARNEDPREMIUM          float64 `json:"EARNED_PREMIUM" md:"EARNEDPREMIUM"`
	//BUSINESSMANAGEEXPENSE  float64 `json:"BUSINESS_MANAGE_EXPENSE" md:"BUSINESSMANAGEEXPENSE"`
	//OTHERCREDITORINVEST    float64 `json:"OTHER_CREDITOR_INVEST" md:"OTHERCREDITORINVEST"`
	//OTHEREQUITYINVEST     float64 `json:"OTHER_EQUITY_INVEST" md:"其他权益工具投资(元)"`
	//LONGRECE              float64 `json:"LONG_RECE" md:"长期应收款(元)"`
	//AVAILABLESALEFINASSET float64 `json:"AVAILABLE_SALE_FINASSET" md:"可售金融资产(元)"`
	//HOLDMATURITYINVEST     float64 `json:"HOLD_MATURITY_INVEST" md:"HOLDMATURITYINVEST"`
	//FEECOMMISSIONEXPENSE   float64 `json:"FEE_COMMISSION_EXPENSE" md:"FEECOMMISSIONEXPENSE"`
}
