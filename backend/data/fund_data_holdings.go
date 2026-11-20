package data

import (
	"blades-stock/backend/logger"

	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
)

func (f *FundApi) GetFundTop10Holdings(fundCode string) ([]FundHoldingStock, error) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("GetFundTop10Holdings panic: %v", r)
		}
	}()

	holdings, err := f.getTop10HoldingsViaHTML(fundCode)
	if err != nil {
		logger.SugaredLogger.Warnf("getTop10HoldingsViaHTML failed for %s: %v", fundCode, err)
		return nil, fmt.Errorf("获取基金十大持仓股失败: %w", err)
	}

	f.fillHoldingStockQuotes(holdings)

	return holdings, nil
}

func stockSinaPrefix(code string) string {
	for len(code) < 6 {
		code = "0" + code
	}
	switch code[0:1] {
	case "5", "6", "9":
		return "sh" + code
	case "0", "1", "2", "3":
		return "sz" + code
	case "4", "8":
		return "bj" + code
	default:
		return ""
	}
}

func detectStockMarket(href, code string) string {
	if href != "" {
		href = strings.ToLower(href)
		if strings.Contains(href, "/hk/") || strings.Contains(href, "hk0") {
			return "HK"
		}
		if strings.Contains(href, "/us/") || strings.Contains(href, "us0") {
			return "US"
		}
		if strings.Contains(href, "/concept/") || strings.Contains(href, "/sh") || strings.Contains(href, "/sz") || strings.Contains(href, "/bj") {
			return "A"
		}
	}
	if code == "" {
		return ""
	}
	hasAlpha := false
	for _, c := range code {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasAlpha = true
			break
		}
	}
	if hasAlpha {
		return "US"
	}
	if stockSinaPrefix(code) != "" {
		return "A"
	}
	if len(code) >= 4 && len(code) <= 6 {
		allDigit := true
		for _, c := range code {
			if c < '0' || c > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			return "HK"
		}
	}
	return ""
}

func stockQuoteCode(code, market string) string {
	padded := code
	switch market {
	case "A":
		return stockSinaPrefix(padded)
	case "HK":
		for len(padded) < 5 {
			padded = "0" + padded
		}
		return "hk" + padded
	case "US":
		return "gb_" + strings.ToLower(padded)
	default:
		return ""
	}
}

func (f *FundApi) fillHoldingStockQuotes(holdings []FundHoldingStock) {
	if len(holdings) == 0 {
		return
	}

	var aCodes, hkCodes, usCodes []string
	for _, h := range holdings {
		switch h.Market {
		case "A":
			if p := stockSinaPrefix(h.StockCode); p != "" {
				aCodes = append(aCodes, p)
			}
		case "HK":
			if p := stockQuoteCode(h.StockCode, "HK"); p != "" {
				hkCodes = append(hkCodes, p)
			}
		case "US":
			if p := stockQuoteCode(h.StockCode, "US"); p != "" {
				usCodes = append(usCodes, p)
			}
		}
	}

	quoteMap := make(map[string]quoteData)

	if len(aCodes) > 0 {
		url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), strings.Join(aCodes, ","))
		resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
			SetHeader("Host", "hq.sinajs.cn").
			SetHeader("User-Agent", getRandomUA()).
			SetHeader("Referer", "https://finance.sina.com.cn").
			Get(url)
		if err == nil && resp.StatusCode() == 200 {
			body := string(resp.Body())
			f.parseSinaAShareQuotes(body, quoteMap)
		} else if err != nil {
			logger.SugaredLogger.Warnf("fillHoldingStockQuotes A-share request failed: %v", err)
		}
	}

	if len(hkCodes) > 0 {
		url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), strings.Join(hkCodes, ","))
		resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
			SetHeader("Host", "hq.sinajs.cn").
			SetHeader("User-Agent", getRandomUA()).
			SetHeader("Referer", "https://finance.sina.com.cn").
			Get(url)
		if err == nil && resp.StatusCode() == 200 {
			body := string(resp.Body())
			f.parseSinaHKQuotes(body, quoteMap)
		} else if err != nil {
			logger.SugaredLogger.Warnf("fillHoldingStockQuotes HK request failed: %v", err)
		}
	}

	if len(usCodes) > 0 {
		url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), strings.Join(usCodes, ","))
		resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
			SetHeader("Host", "hq.sinajs.cn").
			SetHeader("User-Agent", getRandomUA()).
			SetHeader("Referer", "https://finance.sina.com.cn").
			Get(url)
		if err == nil && resp.StatusCode() == 200 {
			body := string(resp.Body())
			f.parseSinaUSQuotes(body, quoteMap)
		} else if err != nil {
			logger.SugaredLogger.Warnf("fillHoldingStockQuotes US request failed: %v", err)
		}
	}

	for i := range holdings {
		key := holdings[i].StockCode
		switch holdings[i].Market {
		case "A":
			for len(key) < 6 {
				key = "0" + key
			}
		case "HK":
			for len(key) < 5 {
				key = "0" + key
			}
		case "US":
			key = strings.ToLower(key)
		}
		if q, ok := quoteMap[key]; ok {
			holdings[i].Price = &q.price
			holdings[i].ChangeRate = &q.changeRate
		}
	}
}

func (f *FundApi) parseSinaAShareQuotes(body string, quoteMap map[string]quoteData) {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eqParts := strings.SplitN(line, "=", 2)
		if len(eqParts) < 2 {
			continue
		}
		sinaCode := strings.TrimSpace(eqParts[0])
		content := strings.Trim(eqParts[1], " \"\n\r;")
		if content == "" {
			continue
		}
		fields := strings.Split(content, ",")
		if len(fields) < 4 {
			continue
		}
		currentPrice, err1 := convertor.ToFloat(fields[3])
		if err1 != nil || currentPrice == 0 {
			continue
		}
		yesterdayPrice, err2 := convertor.ToFloat(fields[2])
		if err2 != nil || yesterdayPrice == 0 {
			continue
		}
		changeRate := mathutil.RoundToFloat((currentPrice-yesterdayPrice)/yesterdayPrice*100, 2)
		pureCode := sinaCode
		if idx := strings.LastIndex(sinaCode, "_"); idx >= 0 {
			pureCode = sinaCode[idx+1:]
		}
		pureCode = strings.TrimPrefix(strings.TrimPrefix(pureCode, "sh"), "sz")
		pureCode = strings.TrimPrefix(pureCode, "bj")
		quoteMap[pureCode] = quoteData{price: currentPrice, changeRate: changeRate}
	}
}

func (f *FundApi) parseSinaHKQuotes(body string, quoteMap map[string]quoteData) {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eqParts := strings.SplitN(line, "=", 2)
		if len(eqParts) < 2 {
			continue
		}
		sinaCode := strings.TrimSpace(eqParts[0])
		content := strings.Trim(eqParts[1], " \"\n\r;")
		if content == "" {
			continue
		}
		fields := strings.Split(content, ",")
		if len(fields) < 9 {
			continue
		}
		currentPrice, err1 := convertor.ToFloat(fields[6])
		if err1 != nil || currentPrice == 0 {
			continue
		}
		yesterdayPrice, err2 := convertor.ToFloat(fields[3])
		if err2 != nil || yesterdayPrice == 0 {
			continue
		}
		changeRate := mathutil.RoundToFloat((currentPrice-yesterdayPrice)/yesterdayPrice*100, 2)
		pureCode := sinaCode
		if idx := strings.LastIndex(sinaCode, "_"); idx >= 0 {
			pureCode = sinaCode[idx+1:]
		}
		pureCode = strings.TrimPrefix(pureCode, "hk")
		quoteMap[pureCode] = quoteData{price: currentPrice, changeRate: changeRate}
	}
}

func (f *FundApi) parseSinaUSQuotes(body string, quoteMap map[string]quoteData) {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eqParts := strings.SplitN(line, "=", 2)
		if len(eqParts) < 2 {
			continue
		}
		sinaCode := strings.TrimSpace(eqParts[0])
		content := strings.Trim(eqParts[1], " \"\n\r;")
		if content == "" {
			continue
		}
		fields := strings.Split(content, ",")
		if len(fields) < 2 {
			continue
		}
		currentPrice, err1 := convertor.ToFloat(fields[1])
		if err1 != nil || currentPrice == 0 {
			continue
		}
		var yesterdayPrice float64
		var err2 error
		if len(fields) >= 36 {
			yesterdayPrice, err2 = convertor.ToFloat(fields[26])
		} else if len(fields) >= 2 {
			yesterdayPrice, err2 = convertor.ToFloat(fields[len(fields)-1])
		}
		if err2 != nil || yesterdayPrice == 0 {
			continue
		}
		changeRate := mathutil.RoundToFloat((currentPrice-yesterdayPrice)/yesterdayPrice*100, 2)
		pureCode := sinaCode
		if idx := strings.LastIndex(sinaCode, "_"); idx >= 0 {
			pureCode = sinaCode[idx+1:]
		}
		pureCode = strings.TrimPrefix(pureCode, "gb_")
		quoteMap[strings.ToLower(pureCode)] = quoteData{price: currentPrice, changeRate: changeRate}
	}
}

func (f *FundApi) getTop10HoldingsViaHTML(fundCode string) ([]FundHoldingStock, error) {
	url := fmt.Sprintf("https://fundf10.eastmoney.com/FundArchivesDatas.aspx?type=jjcc&code=%s&topline=10&year=&month=&rt=%f",
		fundCode, float64(time.Now().UnixMilli())/1000.0)

	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Accept", "*/*").
		SetHeader("Referer", fmt.Sprintf("https://fundf10.eastmoney.com/ccmx_%s.html", fundCode)).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode())
	}

	body := string(resp.Body())

	var htmlContent string
	contentMatch := regexp.MustCompile(`(?s)content:"(.*?)",\w`).FindStringSubmatch(body)
	if len(contentMatch) > 1 {
		htmlContent = contentMatch[1]
	}

	if htmlContent == "" {
		return nil, fmt.Errorf("未找到持仓数据内容")
	}

	quarter := ""
	quarterMatch := regexp.MustCompile(`(\d{4})[年-](\d{1,2})[月-](\d{1,2})日?`).FindStringSubmatch(htmlContent)
	if len(quarterMatch) > 0 {
		quarter = quarterMatch[0]
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %w", err)
	}

	var holdings []FundHoldingStock
	dataRowCount := 0
	doc.Find("table.tzpgtab tbody tr, table.tzxq tbody tr, table.tzpgtab tr, table.tzxq tr").Each(func(i int, s *goquery.Selection) {
		tds := s.Find("td")
		if tds.Length() < 7 {
			return
		}
		if dataRowCount >= 10 {
			return
		}
		dataRowCount++

		rankStr := strings.TrimSpace(tds.Eq(0).Text())
		rank, _ := strconv.Atoi(rankStr)
		if rank == 0 {
			rank = i + 1
		}

		stockCode := strings.TrimSpace(tds.Eq(1).Text())
		stockName := strings.TrimSpace(tds.Eq(2).Text())
		href, _ := tds.Eq(1).Find("a").Attr("href")
		market := detectStockMarket(href, stockCode)
		ratioStr := strings.TrimSpace(tds.Eq(6).Text())
		ratioStr = strings.TrimSuffix(ratioStr, "%")
		ratio, _ := strconv.ParseFloat(ratioStr, 64)

		var price *float64
		var changeRate *float64
		if tds.Length() >= 5 {
			priceStr := strings.TrimSpace(tds.Eq(3).Text())
			changeStr := strings.TrimSpace(tds.Eq(4).Text())
			changeStr = strings.TrimSuffix(changeStr, "%")
			if p, err := strconv.ParseFloat(priceStr, 64); err == nil && p > 0 {
				price = &p
			}
			if c, err := strconv.ParseFloat(changeStr, 64); err == nil {
				changeRate = &c
			}
		}

		shares := ""
		marketCap := ""
		if tds.Length() >= 9 {
			shares = strings.TrimSpace(tds.Eq(7).Text())
			marketCap = strings.TrimSpace(tds.Eq(8).Text())
		}

		if stockCode == "" && stockName == "" {
			return
		}

		holdings = append(holdings, FundHoldingStock{
			Rank:       rank,
			StockCode:  stockCode,
			StockName:  stockName,
			Ratio:      ratio,
			Shares:     shares,
			MarketCap:  marketCap,
			Quarter:    quarter,
			Price:      price,
			ChangeRate: changeRate,
			Market:     market,
		})
	})

	if len(holdings) == 0 {
		fallbackCount := 0
		doc.Find("table tr").Each(func(i int, s *goquery.Selection) {
			tds := s.Find("td")
			if tds.Length() < 4 {
				return
			}
			if fallbackCount >= 10 {
				return
			}
			fallbackCount++

			rank := i
			stockCode := strings.TrimSpace(tds.Eq(1).Text())
			stockName := strings.TrimSpace(tds.Eq(2).Text())
			href, _ := tds.Eq(1).Find("a").Attr("href")
			market := detectStockMarket(href, stockCode)

			var price *float64
			var changeRate *float64
			if tds.Length() >= 5 {
				priceStr := strings.TrimSpace(tds.Eq(3).Text())
				changeStr := strings.TrimSpace(tds.Eq(4).Text())
				changeStr = strings.TrimSuffix(changeStr, "%")
				if p, err := strconv.ParseFloat(priceStr, 64); err == nil && p > 0 {
					price = &p
				}
				if c, err := strconv.ParseFloat(changeStr, 64); err == nil {
					changeRate = &c
				}
			}

			ratioIdx := 6
			if tds.Length() < 7 {
				ratioIdx = tds.Length() - 1
			}
			ratioStr := strings.TrimSpace(tds.Eq(ratioIdx).Text())
			ratioStr = strings.TrimSuffix(ratioStr, "%")
			ratio, _ := strconv.ParseFloat(ratioStr, 64)

			if stockCode == "" && stockName == "" {
				return
			}

			shares := ""
			marketCap := ""
			if tds.Length() >= 9 {
				shares = strings.TrimSpace(tds.Eq(7).Text())
				marketCap = strings.TrimSpace(tds.Eq(8).Text())
			}

			holdings = append(holdings, FundHoldingStock{
				Rank:       rank,
				StockCode:  stockCode,
				StockName:  stockName,
				Ratio:      ratio,
				Shares:     shares,
				MarketCap:  marketCap,
				Quarter:    quarter,
				Price:      price,
				ChangeRate: changeRate,
				Market:     market,
			})
		})
	}

	return holdings, nil
}
