package models

type GDP struct {
	REPORTDATE           string  `json:"REPORT_DATE" md:"报告时间"`
	TIME                 string  `json:"TIME" md:"报告期"`
	DOMESTICLPRODUCTBASE float64 `json:"DOMESTICL_PRODUCT_BASE" md:"国内生产总值(亿元)"`
	SUMSAME              float64 `json:"SUM_SAME" md:"国内生产总值同比增长(%)"`
	FIRSTPRODUCTBASE     float64 `json:"FIRST_PRODUCT_BASE" md:"第一产业(亿元)"`
	FIRSTSAME            int     `json:"FIRST_SAME" md:"第一产业同比增长(%)"`
	SECONDPRODUCTBASE    float64 `json:"SECOND_PRODUCT_BASE" md:"第二产业(亿元)"`
	SECONDSAME           float64 `json:"SECOND_SAME" md:"第二产业同比增长(%)"`
	THIRDPRODUCTBASE     float64 `json:"THIRD_PRODUCT_BASE" md:"第三产业(亿元)"`
	THIRDSAME            float64 `json:"THIRD_SAME" md:"第三产业同比增长(%)"`
}
type CPI struct {
	REPORTDATE         string  `json:"REPORT_DATE" md:"报告时间"`
	TIME               string  `json:"TIME" md:"报告期"`
	NATIONALBASE       float64 `json:"NATIONAL_BASE" md:"全国当月"`
	NATIONALSAME       float64 `json:"NATIONAL_SAME" md:"全国当月同比增长(%)"`
	NATIONALSEQUENTIAL float64 `json:"NATIONAL_SEQUENTIAL" md:"全国当月环比增长(%)"`
	NATIONALACCUMULATE float64 `json:"NATIONAL_ACCUMULATE" md:"全国当月累计"`
	CITYBASE           float64 `json:"CITY_BASE" md:"城市当月"`
	CITYSAME           float64 `json:"CITY_SAME" md:"城市当月同比增长(%)"`
	CITYSEQUENTIAL     float64 `json:"CITY_SEQUENTIAL" md:"城市当月环比增长(%)"`
	CITYACCUMULATE     int     `json:"CITY_ACCUMULATE" md:"城市当月累计"`
	RURALBASE          float64 `json:"RURAL_BASE" md:"农村当月"`
	RURALSAME          float64 `json:"RURAL_SAME" md:"农村当月同比增长(%)"`
	RURALSEQUENTIAL    int     `json:"RURAL_SEQUENTIAL" md:"农村当月环比增长(%)"`
	RURALACCUMULATE    float64 `json:"RURAL_ACCUMULATE" md:"农村当月累计"`
}
type PPI struct {
	REPORTDATE     string  `json:"REPORT_DATE" md:"报告时间"`
	TIME           string  `json:"TIME" md:"报告期"`
	BASE           float64 `json:"BASE" md:"当月"`
	BASESAME       float64 `json:"BASE_SAME" md:"当月同比增长(%)"`
	BASEACCUMULATE float64 `json:"BASE_ACCUMULATE" md:"累计"`
}
type PMI struct {
	REPORTDATE string  `md:"报告时间" json:"REPORT_DATE"`
	TIME       string  `md:"报告期" json:"TIME"`
	MAKEINDEX  float64 `md:"制造业指数" json:"MAKE_INDEX"`
	MAKESAME   float64 `md:"制造业指数同比增长(%)" json:"MAKE_SAME"`
	NMAKEINDEX float64 `md:"非制造业" json:"NMAKE_INDEX"`
	NMAKESAME  float64 `md:"非制造业同比增长(%)" json:"NMAKE_SAME"`
}

type DCResp struct {
	Version string `json:"version"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type GDPResult struct {
	Pages int   `json:"pages"`
	Data  []GDP `json:"data"`
	Count int   `json:"count"`
}
type CPIResult struct {
	Pages int   `json:"pages"`
	Data  []CPI `json:"data"`
	Count int   `json:"count"`
}

type PPIResult struct {
	Pages int   `json:"pages"`
	Data  []PPI `json:"data"`
	Count int   `json:"count"`
}
type PMIResult struct {
	Pages int   `json:"pages"`
	Data  []PMI `json:"data"`
	Count int   `json:"count"`
}
type GDPResp struct {
	DCResp
	GDPResult GDPResult `json:"result"`
}

type CPIResp struct {
	DCResp
	CPIResult CPIResult `json:"result"`
}

type PPIResp struct {
	DCResp
	PPIResult PPIResult `json:"result"`
}
type PMIResp struct {
	DCResp
	PMIResult PMIResult `json:"result"`
}
