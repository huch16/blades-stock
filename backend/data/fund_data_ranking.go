package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func fundParseFloatPtr(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil
	}
	val, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return &val
	}
	return nil
}

func (f *FundApi) SearchFundCodes(keyword string) []FundSearchItem {
	if keyword == "" {
		return []FundSearchItem{}
	}
	url := fmt.Sprintf("https://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx?callback=&m=1&key=%s", keyword)
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", "https://fund.eastmoney.com/").
		Get(url)
	if err != nil || resp.StatusCode() != 200 {
		return []FundSearchItem{}
	}
	var result struct {
		Datas []struct {
			Code         string `json:"CODE"`
			Name         string `json:"NAME"`
			FundBaseInfo *struct {
				FCODE     string  `json:"FCODE"`
				SHORTNAME string  `json:"SHORTNAME"`
				FTYPE     string  `json:"FTYPE"`
				DWJZ      float64 `json:"DWJZ"`
			} `json:"FundBaseInfo"`
		} `json:"Datas"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return []FundSearchItem{}
	}
	var items []FundSearchItem
	for _, item := range result.Datas {
		if item.FundBaseInfo == nil {
			continue
		}
		ftype := item.FundBaseInfo.FTYPE
		items = append(items, FundSearchItem{
			Code: item.Code,
			Name: item.Name,
			Type: ftype,
		})
		var count int64
		db.Dao.Model(&FundBasic{}).Where("code=?", item.Code).Count(&count)
		if count == 0 {
			db.Dao.Create(&FundBasic{Code: item.Code, Name: item.Name, Type: ftype})
		} else {
			db.Dao.Model(&FundBasic{}).Where("code=?", item.Code).Updates(map[string]interface{}{"name": item.Name, "type": ftype})
		}
	}
	if items == nil {
		items = []FundSearchItem{}
	}
	return items
}

func (f *FundApi) GetFundRanking(marketType, fundType, sortField, sortOrder string, pageIndex, pageSize int) (*FundRankingResult, error) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("GetFundRanking panic: %v", r)
		}
	}()

	if marketType == "" {
		marketType = "kf"
	}
	if fundType == "" {
		fundType = "all"
	}
	if sortField == "" {
		sortField = "jnzf"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}
	if pageIndex <= 0 {
		pageIndex = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	referer := "https://fund.eastmoney.com/data/fundranking.html"
	if marketType == "fb" {
		referer = "https://fund.eastmoney.com/data/fbsfundranking.html"
		if fundType == "all" || fundType == "gp" || fundType == "hh" || fundType == "zq" || fundType == "zs" || fundType == "qdii" || fundType == "fof" {
			fundType = "ct"
		}
	}

	apiUrl := "https://fund.eastmoney.com/data/rankhandler.aspx"
	queryParams := map[string]string{
		"op": "ph",
		"dt": marketType,
		"ft": fundType,
		"rs": "",
		"gs": "0",
		"sc": sortField,
		"st": sortOrder,
		"sd": "",
		"ed": "",
		"pi": strconv.Itoa(pageIndex),
		"pn": strconv.Itoa(pageSize),
		"v":  strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	if marketType == "kf" {
		queryParams["qdii"] = ""
		queryParams["tabSubtype"] = ",,,,"
		queryParams["dx"] = "1"
	}

	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", referer).
		SetQueryParams(queryParams).
		Get(apiUrl)
	if err != nil {
		return nil, fmt.Errorf("请求基金排行API失败: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode())
	}

	body := string(resp.Body())

	startIdx := strings.Index(body, "datas:[")
	if startIdx == -1 {
		return nil, fmt.Errorf("未找到基金排行数据(datas)")
	}
	startIdx += len("datas:[")
	endIdx := strings.Index(body[startIdx:], "]")
	if endIdx == -1 {
		return nil, fmt.Errorf("基金排行数据格式错误")
	}
	datasContent := body[startIdx : startIdx+endIdx]

	allRecordsRe := regexp.MustCompile(`allRecords:(\d+)`)
	allRecordsMatch := allRecordsRe.FindStringSubmatch(body)
	totalCount := 0
	if len(allRecordsMatch) > 1 {
		totalCount, _ = strconv.Atoi(allRecordsMatch[1])
	}

	allPagesRe := regexp.MustCompile(`allPages:(\d+)`)
	allPagesMatch := allPagesRe.FindStringSubmatch(body)
	totalPages := 0
	if len(allPagesMatch) > 1 {
		totalPages, _ = strconv.Atoi(allPagesMatch[1])
	}

	recordRe := regexp.MustCompile(`"([^"]*)"`)
	records := recordRe.FindAllStringSubmatch(datasContent, -1)

	var items []FundRankingItem
	for _, record := range records {
		if len(record) < 2 {
			continue
		}
		fields := strings.Split(record[1], ",")
		if len(fields) < 17 {
			continue
		}

		item := FundRankingItem{
			Code:             fields[0],
			Name:             fields[1],
			Pinyin:           fields[2],
			NetValueDate:     fields[3],
			NetUnitValue:     fundParseFloatPtr(fields[4]),
			NetAccumulated:   fundParseFloatPtr(fields[5]),
			DailyGrowth:      fundParseFloatPtr(fields[6]),
			WeekGrowth:       fundParseFloatPtr(fields[7]),
			MonthGrowth:      fundParseFloatPtr(fields[8]),
			ThreeMonthGrowth: fundParseFloatPtr(fields[9]),
			SixMonthGrowth:   fundParseFloatPtr(fields[10]),
			YearGrowth:       fundParseFloatPtr(fields[11]),
			TwoYearGrowth:    fundParseFloatPtr(fields[12]),
			ThreeYearGrowth:  fundParseFloatPtr(fields[13]),
			YTDGrowth:        fundParseFloatPtr(fields[14]),
			SinceInception:   fundParseFloatPtr(fields[15]),
			EstablishDate:    fields[16],
		}

		if marketType == "kf" && len(fields) >= 21 {
			item.Purchasable = fields[17] == "1"
			item.Scale = fundParseFloatPtr(fields[18])
			item.PurchaseRate = fundParseFloatPtr(fields[19])
			item.DiscountRate = fundParseFloatPtr(fields[20])
		} else if marketType == "fb" && len(fields) >= 23 {
			item.FundTypeDetail = fields[21]
			item.Scale = fundParseFloatPtr(fields[22])
		}

		items = append(items, item)
	}

	if items == nil {
		items = []FundRankingItem{}
	}

	return &FundRankingResult{
		Items:      items,
		TotalCount: totalCount,
		PageIndex:  pageIndex,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}
