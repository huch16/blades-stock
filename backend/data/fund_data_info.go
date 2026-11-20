package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
)

func (f *FundApi) CrawlFundBasic(fundCode string) (*FundBasic, error) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("CrawlFundBasic panic: %v", r)
		}
	}()

	fund, err := f.crawlFundBasicViaHTML(fundCode)
	if err != nil {
		logger.SugaredLogger.Warnf("crawlFundBasicViaHTML failed for %s: %v, trying pingzhongdata fallback", fundCode, err)
		fund, err = f.crawlFundBasicViaPingZhongData(fundCode)
		if err != nil {
			return nil, fmt.Errorf("所有数据源获取基金基本信息失败: %w", err)
		}
	}

	if fund.NetUnitValue == nil {
		f.crawlFundNetValueViaAPI(fund, fundCode)
	}

	count := int64(0)
	db.Dao.Model(fund).Where("code=?", fund.Code).Count(&count)
	if count == 0 {
		db.Dao.Create(fund)
	} else {
		db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
	}

	return fund, nil
}

func (f *FundApi) crawlFundBasicViaHTML(fundCode string) (*FundBasic, error) {
	url := fmt.Sprintf("http://fund.eastmoney.com/%s.html", fundCode)
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8").
		SetHeader("Referer", "http://fund.eastmoney.com/").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode())
	}

	htmlContent := string(resp.Body())
	if strings.Contains(htmlContent, "抱歉，您查找的基金不存在") || len(htmlContent) < 500 {
		return nil, fmt.Errorf("基金不存在或页面内容过短")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("HTML解析失败: %w", err)
	}

	fund := &FundBasic{Code: fundCode}

	name := doc.Find(".merchandiseDetail .fundDetail-tit").First().Text()
	if name == "" {
		name = doc.Find(".detailTit-tit").First().Text()
	}
	if name == "" {
		name = doc.Find(".fix_fname").First().Text()
	}
	if name == "" {
		// 从 title 标签提取，如 沪深300ETF华泰柏瑞(510300)基金净值...
		titleText := doc.Find("title").First().Text()
		if m := regexp.MustCompile(`^([^(（]+)[(（]`).FindStringSubmatch(titleText); len(m) > 1 {
			name = strings.TrimSpace(m[1])
		}
	}
	fund.Name = strings.TrimSpace(strutil.ReplaceWithMap(name, map[string]string{
		"查看相关ETF联接>": "",
		"查看相关ETF>":   "",
	}))

	doc.Find(".infoOfFund table td").Each(func(i int, s *goquery.Selection) {
		text := strutil.RemoveWhiteSpace(s.Text(), true)
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("recovered panic: %v", r)
			}
		}()
		splitEx := strutil.SplitEx(text, "：", true)
		if len(splitEx) < 2 {
			return
		}
		if strutil.ContainsAny(text, []string{"基金类型", "类型"}) {
			fund.Type = splitEx[1]
		}
		if strutil.ContainsAny(text, []string{"成立日期", "成立日"}) {
			fund.Establishment = splitEx[1]
		}
		if strutil.ContainsAny(text, []string{"基金规模", "规模"}) {
			fund.Scale = splitEx[1]
		}
		if strutil.ContainsAny(text, []string{"管理人", "基金公司"}) {
			fund.Company = splitEx[1]
		}
		if strutil.ContainsAny(text, []string{"基金经理", "经理人"}) {
			fund.Manager = splitEx[1]
		}
		if strutil.ContainsAny(text, []string{"基金评级", "评级"}) {
			fund.Rating = splitEx[1]
		}
		if strutil.ContainsAny(text, []string{"跟踪标的", "标的"}) {
			fund.TrackingTarget = splitEx[1]
		}
	})

	doc.Find(".dataOfFund dl > dd").Each(func(i int, s *goquery.Selection) {
		text := strutil.RemoveWhiteSpace(s.Text(), true)
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("recovered panic: %v", r)
			}
		}()
		splitEx := strutil.SplitAndTrim(text, "：", "%")
		if len(splitEx) < 2 {
			return
		}
		toFloat, err1 := convertor.ToFloat(splitEx[1])
		if err1 != nil {
			return
		}
		if strutil.ContainsAny(text, []string{"近1月"}) {
			fund.NetGrowth1 = &toFloat
		}
		if strutil.ContainsAny(text, []string{"近3月"}) {
			fund.NetGrowth3 = &toFloat
		}
		if strutil.ContainsAny(text, []string{"近6月"}) {
			fund.NetGrowth6 = &toFloat
		}
		if strutil.ContainsAny(text, []string{"近1年"}) {
			fund.NetGrowth12 = &toFloat
		}
		if strutil.ContainsAny(text, []string{"近3年"}) {
			fund.NetGrowth36 = &toFloat
		}
		if strutil.ContainsAny(text, []string{"近5年"}) {
			fund.NetGrowth60 = &toFloat
		}
		if strutil.ContainsAny(text, []string{"今年来"}) {
			fund.NetGrowthYTD = &toFloat
		}
		if strutil.ContainsAny(text, []string{"成立来"}) {
			fund.NetGrowthAll = &toFloat
		}
	})

	f.setGrowthFromTable(doc, "#increaseAmount_stage table", fund)
	f.setGrowthFromTable(doc, ".dataOfFund table", fund)

	return fund, nil
}

func (f *FundApi) setGrowthFromTable(doc *goquery.Document, selector string, fund *FundBasic) {
	table := doc.Find(selector)
	if table.Length() == 0 {
		return
	}
	rows := table.Find("tr")
	if rows.Length() < 2 {
		return
	}

	var headers []string
	rows.Eq(0).Find("th").Each(func(_ int, th *goquery.Selection) {
		headers = append(headers, strutil.RemoveWhiteSpace(th.Text(), true))
	})

	rows.Each(func(rowIndex int, row *goquery.Selection) {
		if rowIndex == 0 {
			return
		}
		tds := row.Find("td")
		if tds.Length() == 0 {
			return
		}
		firstTd := strutil.RemoveWhiteSpace(tds.Eq(0).Text(), true)
		if !strutil.ContainsAny(firstTd, []string{"阶段涨幅", "涨幅"}) {
			return
		}
		for j := 1; j < len(headers) && j < tds.Length(); j++ {
			header := headers[j]
			valText := strutil.RemoveWhiteSpace(tds.Eq(j).Text(), true)
			valText = strings.TrimSuffix(valText, "%")
			toFloat, err := convertor.ToFloat(valText)
			if err != nil {
				continue
			}
			if strings.Contains(header, "近1月") && fund.NetGrowth1 == nil {
				fund.NetGrowth1 = &toFloat
			} else if strings.Contains(header, "近3月") && fund.NetGrowth3 == nil {
				fund.NetGrowth3 = &toFloat
			} else if strings.Contains(header, "近6月") && fund.NetGrowth6 == nil {
				fund.NetGrowth6 = &toFloat
			} else if strings.Contains(header, "近1年") && fund.NetGrowth12 == nil {
				fund.NetGrowth12 = &toFloat
			} else if strings.Contains(header, "近3年") && fund.NetGrowth36 == nil {
				fund.NetGrowth36 = &toFloat
			} else if strings.Contains(header, "近5年") && fund.NetGrowth60 == nil {
				fund.NetGrowth60 = &toFloat
			} else if strings.Contains(header, "今年来") && fund.NetGrowthYTD == nil {
				fund.NetGrowthYTD = &toFloat
			} else if strings.Contains(header, "成立来") && fund.NetGrowthAll == nil {
				fund.NetGrowthAll = &toFloat
			}
		}
	})
}

func (f *FundApi) crawlFundBasicViaPingZhongData(fundCode string) (*FundBasic, error) {
	url := fmt.Sprintf("http://fund.eastmoney.com/pingzhongdata/%s.js", fundCode)
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", fmt.Sprintf("http://fund.eastmoney.com/%s.html", fundCode)).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("pingzhongdata请求失败: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("pingzhongdata状态码: %d", resp.StatusCode())
	}

	body := string(resp.Body())
	fund := &FundBasic{Code: fundCode}

	if m := reFSName.FindStringSubmatch(body); len(m) > 1 {
		fund.Name = strings.TrimSpace(m[1])
	}

	if m := rePerformance.FindStringSubmatch(body); len(m) > 1 {
		f.parsePerformanceJSON(fund, m[1])
	}

	// 针对场内 ETF pingzhongdata 采用独立 syl_ 变量收益率的适配
	if fund.NetGrowth1 == nil {
		if m := reSyl1y.FindStringSubmatch(body); len(m) > 1 && m[1] != "" && m[1] != "--" {
			if v, err := convertor.ToFloat(m[1]); err == nil {
				fund.NetGrowth1 = &v
			}
		}
	}
	if fund.NetGrowth3 == nil {
		if m := reSyl3y.FindStringSubmatch(body); len(m) > 1 && m[1] != "" && m[1] != "--" {
			if v, err := convertor.ToFloat(m[1]); err == nil {
				fund.NetGrowth3 = &v
			}
		}
	}
	if fund.NetGrowth6 == nil {
		if m := reSyl6y.FindStringSubmatch(body); len(m) > 1 && m[1] != "" && m[1] != "--" {
			if v, err := convertor.ToFloat(m[1]); err == nil {
				fund.NetGrowth6 = &v
			}
		}
	}
	if fund.NetGrowth12 == nil {
		if m := reSyl1n.FindStringSubmatch(body); len(m) > 1 && m[1] != "" && m[1] != "--" {
			if v, err := convertor.ToFloat(m[1]); err == nil {
				fund.NetGrowth12 = &v
			}
		}
	}

	if m := reFundManager.FindStringSubmatch(body); len(m) > 1 {
		f.parseFundManagerJSON(fund, m[1])
	}

	if m := reFluctuation.FindStringSubmatch(body); len(m) > 1 {
		f.parseFluctuationScaleJSON(fund, m[1])
	}

	if fund.Name == "" {
		return nil, fmt.Errorf("pingzhongdata解析失败: 名称为空")
	}

	return fund, nil
}

func (f *FundApi) parsePerformanceJSON(fund *FundBasic, jsonStr string) {
	var perf performanceData
	if err := json.Unmarshal([]byte(jsonStr), &perf); err != nil {
		logger.SugaredLogger.Warnf("parsePerformanceJSON error: %v", err)
		return
	}

	for key, values := range perf.SamePeriod {
		if len(values) < 2 {
			continue
		}
		val, ok := values[1].(float64)
		if !ok {
			continue
		}
		switch key {
		case "1":
			fund.NetGrowth1 = &val
		case "3":
			fund.NetGrowth3 = &val
		case "6":
			fund.NetGrowth6 = &val
		case "12":
			fund.NetGrowth12 = &val
		case "36":
			fund.NetGrowth36 = &val
		case "60":
			fund.NetGrowth60 = &val
		case "ytd":
			fund.NetGrowthYTD = &val
		case "all":
			fund.NetGrowthAll = &val
		}
	}
}

func (f *FundApi) parseFundManagerJSON(fund *FundBasic, jsonStr string) {
	var managers []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &managers); err != nil {
		return
	}
	if len(managers) > 0 {
		if name, ok := managers[0]["name"].(string); ok {
			fund.Manager = name
		}
	}
}

func (f *FundApi) parseFluctuationScaleJSON(fund *FundBasic, jsonStr string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return
	}
	if categories, ok := data["categories"].([]interface{}); ok && len(categories) > 0 {
		lastCat, ok := categories[len(categories)-1].(map[string]interface{})
		if ok {
			if scale, ok := lastCat["y"].(float64); ok {
				fund.Scale = fmt.Sprintf("%.2f亿元", scale)
			}
		}
	}
}

func (f *FundApi) crawlFundNetValueViaAPI(fund *FundBasic, fundCode string) {
	url := fmt.Sprintf("http://api.fund.eastmoney.com/f10/lsjz?fundCode=%s&pageIndex=1&pageSize=1&startDate=&endDate=&_%d",
		fundCode, time.Now().UnixMilli())
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", fmt.Sprintf("http://fundf10.eastmoney.com/jjjz_%s.html", fundCode)).
		Get(url)
	if err != nil {
		return
	}

	var result struct {
		Data struct {
			LSJZList []struct {
				FSRQ  string `json:"FSRQ"`
				DWJZ  string `json:"DWJZ"`
				LJJZ  string `json:"LJJZ"`
				JZZZL string `json:"JZZZL"`
			} `json:"LSJZList"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return
	}
	if len(result.Data.LSJZList) == 0 {
		return
	}
	item := result.Data.LSJZList[0]
	if val, err := convertor.ToFloat(item.DWJZ); err == nil {
		fund.NetUnitValue = &val
	}
	if val, err := convertor.ToFloat(item.LJJZ); err == nil {
		fund.NetAccumulated = &val
	}
	fund.NetUnitValueDate = item.FSRQ
}
