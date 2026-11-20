package data

import (
	"blades-stock/backend/datasource"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/util"

	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/robertkrimen/otto"
	"github.com/tidwall/gjson"
)

func (m MarketNewsApi) GetIndustryRank(sort string, cnt int) map[string]any {

	url := fmt.Sprintf("https://proxy.finance.qq.com/ifzqgtimg/appstock/app/mktHs/rank?l=%d&p=1&t=01/averatio&ordertype=&o=%s", cnt, sort)
	response, err := SharedHTTPClient.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Referer", "https://stockapp.finance.qq.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Warnf("GetIndustryRank 请求失败: %v", err)
		return map[string]any{}
	}
	res := make(map[string]any)
	if err := json.Unmarshal(response.Body(), &res); err != nil {
		logger.SugaredLogger.Warnf("GetIndustryRank 解析失败: %v", err)
		return map[string]any{}
	}
	return res
}

func (m MarketNewsApi) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	url := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_bkzj_bk?page=1&num=20&sort=%s&asc=0&fenlei=%s", sort, fenlei)

	response, _ := SharedHTTPClient.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res
}

func (m MarketNewsApi) GetMoneyRankSina(sort string) []map[string]any {
	if sort == "" {
		sort = "netamount"
	}
	url := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_bkzj_ssggzj?page=1&num=20&sort=%s&asc=0&bankuai=&shichang=", sort)
	response, _ := SharedHTTPClient.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res
}

func (m MarketNewsApi) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	url := fmt.Sprintf("http://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_qsfx_zjlrqs?page=1&num=%d&sort=opendate&asc=0&daima=%s", days, stockCode)

	response, _ := SharedHTTPClient.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res

}

func (m MarketNewsApi) LongTiger(date string) *[]models.LongTigerRankData {
	ranks := &[]models.LongTigerRankData{}
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get"
	//logger.SugaredLogger.Infof("url:%s", url)
	params := make(map[string]string)
	params["callback"] = "callback"
	params["sortColumns"] = "TURNOVERRATE,TRADE_DATE,SECURITY_CODE"
	params["sortTypes"] = "-1,-1,1"
	params["pageSize"] = "500"
	params["pageNumber"] = "1"
	params["reportName"] = "RPT_DAILYBILLBOARD_DETAILSNEW"
	params["columns"] = "SECURITY_CODE,SECUCODE,SECURITY_NAME_ABBR,TRADE_DATE,EXPLAIN,CLOSE_PRICE,CHANGE_RATE,BILLBOARD_NET_AMT,BILLBOARD_BUY_AMT,BILLBOARD_SELL_AMT,BILLBOARD_DEAL_AMT,ACCUM_AMOUNT,DEAL_NET_RATIO,DEAL_AMOUNT_RATIO,TURNOVERRATE,FREE_MARKET_CAP,EXPLANATION,D1_CLOSE_ADJCHRATE,D2_CLOSE_ADJCHRATE,D5_CLOSE_ADJCHRATE,D10_CLOSE_ADJCHRATE,SECURITY_TYPE_CODE"
	params["source"] = "WEB"
	params["client"] = "WEB"
	params["filter"] = fmt.Sprintf("(TRADE_DATE<='%s')(TRADE_DATE>='%s')", date, date)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/stock/tradedetail.html").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		SetQueryParams(params).
		Get(url)
	if err != nil {
		return ranks
	}
	js := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", js)

	js = strutil.ReplaceWithMap(js, map[string]string{
		"callback(": "var data=",
		");":        ";",
	})
	//logger.SugaredLogger.Info(js)
	vm := otto.New()
	vm.Run(js)
	vm.Run("var data = JSON.stringify(data);")
	value, _ := vm.Get("data")
	//logger.SugaredLogger.Infof("resp-json:%s", value.String())
	data := gjson.Get(value.String(), "result.data")
	//logger.SugaredLogger.Infof("resp:%v", data)
	err = json.Unmarshal([]byte(data.String()), ranks)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return ranks
	}
	for _, rankData := range *ranks {
		temp := &models.LongTigerRankData{}
		db.Dao.Model(temp).Where(&models.LongTigerRankData{
			TRADEDATE: rankData.TRADEDATE,
			SECUCODE:  rankData.SECUCODE,
		}).First(temp)
		if temp.SECURITYTYPECODE == "" {
			db.Dao.Model(temp).Create(&rankData)
		}
	}
	return ranks
}

func (m MarketNewsApi) IndustryResearchReport(industryCode string, days int) []any {
	beginDate := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	if strutil.Trim(industryCode) != "" {
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	}

	//logger.SugaredLogger.Infof("IndustryResearchReport-name:%s", industryCode)
	params := map[string]string{
		"industry":     "*",
		"industryCode": industryCode,
		"beginTime":    beginDate,
		"endTime":      endDate,
		"pageNo":       "1",
		"pageSize":     "50",
		"p":            "1",
		"pageNum":      "1",
		"pageNumber":   "1",
		"qType":        "1",
	}

	url := "https://reportapi.eastmoney.com/report/list"

	//logger.SugaredLogger.Infof("beginDate:%s endDate:%s", beginDate, endDate)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/stock.jshtml").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		SetHeader("Content-Type", "application/json").
		SetQueryParams(params).Get(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	if err := json.Unmarshal(resp.Body(), &respMap); err != nil {
		logger.SugaredLogger.Errorf("IndustryResearchReport json unmarshal err: %v", err)
		return []any{}
	}
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	if dataList, ok := respMap["data"].([]any); ok && dataList != nil {
		return dataList
	}
	return []any{}
}

func (m MarketNewsApi) StockResearchReport(stockCode string, days int) []any {
	endDate := time.Now().Format("2006-01-02")
	var beginDate string
	if strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = strings.Split(stockCode, ".")[0]
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	} else {
		stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
			"sh":  "",
			"sz":  "",
			"gb_": "",
			"us":  "",
			"us_": "",
		})
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	}

	//logger.SugaredLogger.Infof("StockResearchReport-stockCode:%s", stockCode)

	type Req struct {
		BeginTime    string      `json:"beginTime"`
		EndTime      string      `json:"endTime"`
		IndustryCode string      `json:"industryCode"`
		RatingChange string      `json:"ratingChange"`
		Rating       string      `json:"rating"`
		OrgCode      interface{} `json:"orgCode"`
		Code         string      `json:"code"`
		Rcode        string      `json:"rcode"`
		PageSize     int         `json:"pageSize"`
		PageNo       int         `json:"pageNo"`
		P            int         `json:"p"`
		PageNum      int         `json:"pageNum"`
		PageNumber   int         `json:"pageNumber"`
	}

	url := "https://reportapi.eastmoney.com/report/list2"

	//logger.SugaredLogger.Infof("beginDate:%s endDate:%s", beginDate, endDate)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/stock.jshtml").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		SetHeader("Content-Type", "application/json").
		SetBody(&Req{
			Code:         stockCode,
			IndustryCode: "*",
			BeginTime:    beginDate,
			EndTime:      endDate,
			PageNo:       1,
			PageSize:     50,
			P:            1,
			PageNum:      1,
			PageNumber:   1,
		}).Post(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	if err := json.Unmarshal(resp.Body(), &respMap); err != nil {
		logger.SugaredLogger.Errorf("StockResearchReport json unmarshal err: %v", err)
		return []any{}
	}
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	if dataList, ok := respMap["data"].([]any); ok && dataList != nil {
		return dataList
	}
	return []any{}
}

func (m MarketNewsApi) StockNotice(stock_list string) []any {
	var stockCodes []string
	for _, stockCode := range strings.Split(stock_list, ",") {
		if strutil.ContainsAny(stockCode, []string{"."}) {
			stockCode = strings.Split(stockCode, ".")[0]
			stockCodes = append(stockCodes, stockCode)
		} else {
			stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
				"sh":  "",
				"sz":  "",
				"gb_": "",
				"us":  "",
				"us_": "",
			})
			stockCodes = append(stockCodes, stockCode)
		}
	}

	url := "https://np-anotice-stock.eastmoney.com/api/security/ann?page_size=50&page_index=1&ann_type=SHA%2CCYB%2CSZA%2CBJA%2CINV&client_source=web&f_node=0&stock_list=" + strings.Join(stockCodes, ",")
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "np-anotice-stock.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/notices/hsa/5.html").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	if err := json.Unmarshal(resp.Body(), &respMap); err != nil {
		logger.SugaredLogger.Errorf("StockNotice json unmarshal err: %v", err)
		return []any{}
	}
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	if dataMap, ok := respMap["data"].(map[string]any); ok {
		if list, ok := dataMap["list"].([]any); ok {
			return list
		}
	}
	return []any{}
}

func (m MarketNewsApi) EMDictCode(code string, cache *freecache.Cache) []any {
	respMap := map[string]any{}

	d, _ := cache.Get([]byte(code))
	if d != nil {
		json.Unmarshal(d, &respMap)
		if dataList, ok := respMap["data"].([]any); ok && dataList != nil {
			return dataList
		}
	}

	url := "https://reportapi.eastmoney.com/report/bk"

	params := map[string]string{
		"bkCode": code,
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/industry.jshtml").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		SetHeader("Content-Type", "application/json").
		SetQueryParams(params).Get(url)

	if err != nil {
		return []any{}
	}
	if err := json.Unmarshal(resp.Body(), &respMap); err != nil {
		logger.SugaredLogger.Errorf("EMDictCode json unmarshal err: %v", err)
		return []any{}
	}
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	cache.Set([]byte(code), resp.Body(), 60*60*24)
	if dataList, ok := respMap["data"].([]any); ok && dataList != nil {
		return dataList
	}
	return []any{}
}

func (m MarketNewsApi) XUEQIUHotStock(size int, marketType string) *[]models.HotItem {
	url := fmt.Sprintf("https://stock.xueqiu.com/v5/stock/hot_stock/list.json?page=1&size=%d&_type=%s&type=%s", size, marketType, marketType)
	empty := &[]models.HotItem{}

	res := &models.XUEQIUHot{}
	request := SharedHTTPClient.SetTimeout(time.Duration(30) * time.Second).R()
	request.SetHeader("Host", "stock.xueqiu.com").
		SetHeader("Origin", "https://xueqiu.com").
		SetHeader("Referer", "https://xueqiu.com/").
		SetHeader("User-Agent", datasource.FirefoxUserAgent)
	_, err := request.SetResult(res).Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("XUEQIUHotStock err:%s", err.Error())
		return empty
	}
	if res.ErrorCode != 0 {
		logger.SugaredLogger.Errorf("XUEQIUHotStock API error: code=%d, desc=%s", res.ErrorCode, res.ErrorDescription)
		return empty
	}
	return &res.Data.Items
}

func (m MarketNewsApi) HotEvent(size int) *[]models.HotEvent {
	events := &[]models.HotEvent{}
	sprintf := fmt.Sprintf("https://xueqiu.com/hot_event/list.json?count=%d", size)
	request := SharedHTTPClient.SetTimeout(time.Duration(30) * time.Second).R()
	request.SetHeader("Host", "xueqiu.com").
		SetHeader("Referer", "https://xueqiu.com/").
		SetHeader("User-Agent", datasource.FirefoxUserAgent)
	resp, err := request.Get(sprintf)
	if err != nil {
		logger.SugaredLogger.Errorf("HotEvent err:%s", err.Error())
		return events
	}
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	if err != nil {
		logger.SugaredLogger.Errorf("HotEvent json unmarshal err:%s", err.Error())
		return events
	}
	items, err := json.Marshal(respMap["list"])
	if err != nil {
		return events
	}
	if err := json.Unmarshal(items, events); err != nil {
		logger.SugaredLogger.Errorf("HotEvent json unmarshal err: %v", err)
		return events
	}
	return events
}

func (m MarketNewsApi) HotTopic(size int) []any {
	url := "https://gubatopic.eastmoney.com/interface/GetData.aspx?path=newtopic/api/Topic/HomePageListRead"
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "gubatopic.eastmoney.com").
		SetHeader("Origin", "https://gubatopic.eastmoney.com").
		SetHeader("Referer", "https://gubatopic.eastmoney.com/").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		SetFormData(map[string]string{
			"param": fmt.Sprintf("ps=%d&p=1&type=0", size),
			"path":  "newtopic/api/Topic/HomePageListRead",
			"env":   "2",
		}).
		Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("HotTopic err:%s", err.Error())
		return []any{}
	}
	//logger.SugaredLogger.Infof("HotTopic:%s", resp.Body())
	respMap := map[string]any{}
	if err := json.Unmarshal(resp.Body(), &respMap); err != nil {
		logger.SugaredLogger.Errorf("HotTopic json unmarshal err: %v", err)
		return []any{}
	}
	if reList, ok := respMap["re"].([]any); ok && reList != nil {
		return reList
	}
	return []any{}

}

func (receiver MarketNewsApi) GetSecuritiesCompanyOpinion(startDate string, endDate string) *models.SecuritiesCompanyOpinionResp {
	res := models.SecuritiesCompanyOpinionResp{}

	url := fmt.Sprintf("https://reportapi.eastmoney.com/report/jg?cb=data&pageSize=50&beginTime=%s&endTime=%s&pageNo=1&fields=&qType=4&orgCode=&author=&p=1&pageNum=1&pageNumber=1&_=%d", startDate, endDate, time.Now().Unix())
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetSecuritiesCompanyOpinion err:%s", err.Error())
		return &res
	}
	body := resp.Body()
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, _ := vm.Run(body)

	data, _ := val.Object().Value().Export()
	marshal, _ := json.Marshal(data)

	if err := json.Unmarshal(marshal, &res); err != nil {
		logger.SugaredLogger.Errorf("GetIndustryReportInfo json unmarshal err: %v", err)
	}

	for _, d := range (&res).Data {
		//logger.SugaredLogger.Debugf("PublishDate: %s,OrgSName: %s,Title: %s,EncodeUrl: %s", d.PublishDate, d.OrgSName, d.Title, d.EncodeUrl)
		markdown := receiver.GetSecuritiesCompanyOpinionContent(d.OrgSName, d.EncodeUrl)
		d.OpinionData = markdown
	}
	return &res
}

func (m MarketNewsApi) GetSecuritiesCompanyOpinionContent(OrgSName, encodeUrl string) string {
	url := "https://data.eastmoney.com/report/zw_brokerreport.jshtml?encodeUrl=" + encodeUrl
	resp, _ := SharedHTTPClient.R().
		SetHeader("Host", "data.eastmoney.com").
		SetHeader("User-Agent", datasource.FirefoxUserAgent).
		Get(url)
	body := resp.Body()
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	title, _ := doc.Find("div.c-title").Html()
	content, _ := doc.Find("div.ctx-content").Html()
	markdown, _ := util.HTMLToMarkdown("<h1>" + OrgSName + "</h1>" + title + content)
	return markdown
}

func (m MarketNewsApi) InteractiveAnswer(page int, pageSize int, keyWord string) *models.InteractiveAnswer {
	client := SharedHTTPClient
	config := GetSettingConfig()
	if config.HttpProxyEnabled && config.HttpProxy != "" {
		client.SetProxy(config.HttpProxy)
	}
	url := fmt.Sprintf("https://irm.cninfo.com.cn/newircs/index/search?_t=%d", time.Now().Unix())
	answers := &models.InteractiveAnswer{}
	//logger.SugaredLogger.Infof("请求url:%s", url)
	_, err := client.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "irm.cninfo.com.cn").
		SetHeader("Origin", "https://irm.cninfo.com.cn").
		SetHeader("Referer", "https://irm.cninfo.com.cn/views/interactiveAnswer").
		SetHeader("handleError", "true").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:142.0) Gecko/20100101 Firefox/142.0").
		SetFormData(map[string]string{
			"pageNo":      convertor.ToString(page),
			"pageSize":    convertor.ToString(pageSize),
			"searchTypes": "11",
			"highLight":   "true",
			"keyWord":     keyWord,
		}).
		SetResult(answers).
		Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("InteractiveAnswer-err:%+v", err)
	}
	//logger.SugaredLogger.Debugf("InteractiveAnswer-resp:%s", resp.Body())
	return answers

}

func (m MarketNewsApi) GetUplimitHot(date string, limit int) map[string]any {
	if limit <= 0 {
		limit = 20
	}
	if date == "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		date = time.Now().In(loc).Format("2006-01-02")
	}
	apiUrl := fmt.Sprintf("https://api.zizizaizai.com/v3/open/review/uplimit/hot?date1=%s&limit=%d", date, limit)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json").
		Get(apiUrl)
	if err != nil {
		logger.SugaredLogger.Errorf("GetUplimitHot error: %v", err)
		return map[string]any{"code": 50000, "message": "请求失败"}
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Errorf("GetUplimitHot unmarshal error: %v", err)
		return map[string]any{"code": 50000, "message": "数据解析失败"}
	}
	return result
}
