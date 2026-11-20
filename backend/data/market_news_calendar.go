package data

import (
	"blades-stock/backend/datasource"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/util"

	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/robertkrimen/otto"
)

func (m MarketNewsApi) InvestCalendar(yearMonth string) []any {
	if yearMonth == "" {
		yearMonth = time.Now().Format("2006-01")
	}

	url := "https://app.jiuyangongshe.com/jystock-app/api/v1/timeline/list"
	token := os.Getenv("JYGS_TOKEN")
	if token == "" {
		logger.SugaredLogger.Errorf("InvestCalendar: 未配置 JYGS_TOKEN 环境变量，无法获取投资日历")
		return []any{}
	}
	session := os.Getenv("JYGS_SESSION")
	if session == "" {
		logger.SugaredLogger.Errorf("InvestCalendar: 未配置 JYGS_SESSION 环境变量，无法获取投资日历")
		return []any{}
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "app.jiuyangongshe.com").
		SetHeader("Origin", "https://www.jiuyangongshe.com").
		SetHeader("Referer", "https://www.jiuyangongshe.com/").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		SetHeader("Content-Type", "application/json").
		SetHeader("token", token).
		SetHeader("platform", "3").
		SetHeader("Cookie", session).
		SetHeader("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10)).
		SetBody(map[string]string{
			"date":  yearMonth,
			"grade": "0",
		}).
		Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("InvestCalendar err:%s", err.Error())
		return []any{}
	}
	//logger.SugaredLogger.Infof("InvestCalendar:%s", resp.Body())
	respMap := map[string]any{}
	if err := json.Unmarshal(resp.Body(), &respMap); err != nil {
		logger.SugaredLogger.Errorf("InvestCalendar json unmarshal err: %v", err)
		return []any{}
	}
	if dataList, ok := respMap["data"].([]any); ok && dataList != nil {
		return dataList
	}
	return []any{}
}

func (m MarketNewsApi) ClsCalendar() []any {
	url := "https://www.cls.cn/api/calendar/web/list?app=CailianpressWeb&flag=0&os=web&sv=8.4.6&type=0&sign=4b839750dc2f6b803d1c8ca00d2b40be"
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "www.cls.cn").
		SetHeader("Origin", "https://www.cls.cn").
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ClsCalendar err:%s", err.Error())
		return []any{}
	}
	respMap := map[string]any{}
	if err := json.Unmarshal(resp.Body(), &respMap); err != nil {
		logger.SugaredLogger.Errorf("ClsCalendar json unmarshal err: %v", err)
		return []any{}
	}
	if dataList, ok := respMap["data"].([]any); ok && dataList != nil {
		return dataList
	}
	return []any{}
}

func (m MarketNewsApi) GetGDP() *models.GDPResp {
	res := &models.GDPResp{}

	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&columns=REPORT_DATE%2CTIME%2CDOMESTICL_PRODUCT_BASE%2CFIRST_PRODUCT_BASE%2CSECOND_PRODUCT_BASE%2CTHIRD_PRODUCT_BASE%2CSUM_SAME%2CFIRST_SAME%2CSECOND_SAME%2CTHIRD_SAME&pageNumber=1&pageSize=20&sortColumns=REPORT_DATE&sortTypes=-1&source=WEB&client=WEB&reportName=RPT_ECONOMY_GDP&p=1&pageNo=1&pageNum=1&_=" + strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Origin", "https://datacenter.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/cjsj/gdp.html").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GDP err:%s", err.Error())
		return res
	}
	body := resp.Body()
	////logger.SugaredLogger.Debugf("GDP:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, err := vm.Run(body)
	if err != nil {
		logger.SugaredLogger.Errorf("GDP err:%s", err.Error())
		return res
	}
	data, _ := val.Object().Value().Export()
	//logger.SugaredLogger.Infof("GDP:%v", data)
	marshal, err := json.Marshal(data)
	if err != nil {
		return res
	}
	if err := json.Unmarshal(marshal, &res); err != nil {
		logger.SugaredLogger.Errorf("GetGDP json unmarshal err: %v", err)
	}
	//logger.SugaredLogger.Infof("GDP:%+v", res)
	return res
}

func (m MarketNewsApi) GetCPI() *models.CPIResp {
	res := &models.CPIResp{}

	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&columns=REPORT_DATE%2CTIME%2CNATIONAL_SAME%2CNATIONAL_BASE%2CNATIONAL_SEQUENTIAL%2CNATIONAL_ACCUMULATE%2CCITY_SAME%2CCITY_BASE%2CCITY_SEQUENTIAL%2CCITY_ACCUMULATE%2CRURAL_SAME%2CRURAL_BASE%2CRURAL_SEQUENTIAL%2CRURAL_ACCUMULATE&pageNumber=1&pageSize=20&sortColumns=REPORT_DATE&sortTypes=-1&source=WEB&client=WEB&reportName=RPT_ECONOMY_CPI&p=1&pageNo=1&pageNum=1&_=" + strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Origin", "https://datacenter.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/cjsj/gdp.html").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetCPI err:%s", err.Error())
		return res
	}
	body := resp.Body()
	////logger.SugaredLogger.Debugf("GetCPI:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, err := vm.Run(body)
	if err != nil {
		logger.SugaredLogger.Errorf("GetCPI err:%s", err.Error())
		return res
	}
	data, _ := val.Object().Value().Export()
	//logger.SugaredLogger.Infof("GetCPI:%v", data)
	marshal, err := json.Marshal(data)
	if err != nil {
		return res
	}
	if err := json.Unmarshal(marshal, &res); err != nil {
		logger.SugaredLogger.Errorf("GetCPI json unmarshal err: %v", err)
	}
	//logger.SugaredLogger.Infof("GetCPI:%+v", res)
	return res
}

// GetPPI PPI

// GetPPI PPI
func (m MarketNewsApi) GetPPI() *models.PPIResp {
	res := &models.PPIResp{}
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&columns=REPORT_DATE,TIME,BASE,BASE_SAME,BASE_ACCUMULATE&pageNumber=1&pageSize=20&sortColumns=REPORT_DATE&sortTypes=-1&source=WEB&client=WEB&reportName=RPT_ECONOMY_PPI&p=1&pageNo=1&pageNum=1&_=" + strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Origin", "https://datacenter.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/cjsj/gdp.html").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetPPI err:%s", err.Error())
		return res
	}
	body := resp.Body()
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, err := vm.Run(body)
	if err != nil {
		return res
	}
	data, _ := val.Object().Value().Export()
	marshal, err := json.Marshal(data)
	if err != nil {
		return res
	}
	if err := json.Unmarshal(marshal, &res); err != nil {
		logger.SugaredLogger.Errorf("GetPPI json unmarshal err: %v", err)
	}
	return res
}

func (m MarketNewsApi) GetPMI() *models.PMIResp {
	res := &models.PMIResp{}
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&columns=REPORT_DATE%2CTIME%2CMAKE_INDEX%2CMAKE_SAME%2CNMAKE_INDEX%2CNMAKE_SAME&pageNumber=1&pageSize=20&sortColumns=REPORT_DATE&sortTypes=-1&source=WEB&client=WEB&reportName=RPT_ECONOMY_PMI&p=1&pageNo=1&pageNum=1&_=" + strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Origin", "https://datacenter.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/cjsj/gdp.html").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	if err != nil {
		return res
	}
	body := resp.Body()
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, err := vm.Run(body)
	if err != nil {
		return res
	}
	data, _ := val.Object().Value().Export()
	marshal, err := json.Marshal(data)
	if err != nil {
		return res
	}
	if err := json.Unmarshal(marshal, &res); err != nil {
		logger.SugaredLogger.Errorf("GetPMI json unmarshal err: %v", err)
	}
	return res

}

func (m MarketNewsApi) GetIndustryReportInfo(infoCode string) string {
	url := "https://data.eastmoney.com/report/zw_industry.jshtml?infocode=" + infoCode
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "data.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/industry.jshtml").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetIndustryReportInfo err:%s", err.Error())
		return ""
	}
	body := resp.Body()
	////logger.SugaredLogger.Debugf("GetIndustryReportInfo:%s", body)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	title, _ := doc.Find("div.c-title").Html()
	content, _ := doc.Find("div.ctx-content").Html()
	markdown, err := util.HTMLToMarkdown(title + content)
	if err != nil {
		return ""
	}
	//logger.SugaredLogger.Infof("GetIndustryReportInfo markdown:\n%s", markdown)
	return markdown
}
