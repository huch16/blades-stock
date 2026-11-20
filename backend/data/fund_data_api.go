package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/strutil"
	resty "github.com/go-resty/resty/v2"
	"gorm.io/gorm"
)

type FundApi struct {
	client *resty.Client
	config *SettingConfig
}

func NewFundApi() *FundApi {
	return &FundApi{
		client: SharedHTTPClient,
		config: GetSettingConfig(),
	}
}

type FollowedFund struct {
	gorm.Model
	Code string `json:"code" gorm:"index"`
	Name string `json:"name"`

	NetUnitValue     *float64 `json:"netUnitValue"`
	NetUnitValueDate string   `json:"netUnitValueDate"`
	NetEstimatedUnit *float64 `json:"netEstimatedUnit"`
	NetEstimatedTime string   `json:"netEstimatedUnitTime"`
	NetAccumulated   *float64 `json:"netAccumulated"`

	NetEstimatedRate *float64 `json:"netEstimatedRate"`

	NetUnitValuePrev *float64 `json:"netUnitValuePrev"`
	NetActualRate    *float64 `json:"netActualRate"`

	FundBasic FundBasic `json:"fundBasic" gorm:"foreignKey:Code;references:Code"`
}

func (FollowedFund) TableName() string {
	return "followed_fund"
}

type FundBasic struct {
	gorm.Model
	Code           string `json:"code" gorm:"index"`
	Name           string `json:"name"`
	FullName       string `json:"fullName"`
	Type           string `json:"type"`
	Establishment  string `json:"establishment"`
	Scale          string `json:"scale"`
	Company        string `json:"company"`
	Manager        string `json:"manager"`
	Rating         string `json:"rating"`
	TrackingTarget string `json:"trackingTarget"`

	NetUnitValue     *float64 `json:"netUnitValue"`
	NetUnitValueDate string   `json:"netUnitValueDate"`
	NetEstimatedUnit *float64 `json:"netEstimatedUnit"`
	NetEstimatedTime string   `json:"netEstimatedUnitTime"`
	NetAccumulated   *float64 `json:"netAccumulated"`

	NetGrowth1   *float64 `json:"netGrowth1"`
	NetGrowth3   *float64 `json:"netGrowth3"`
	NetGrowth6   *float64 `json:"netGrowth6"`
	NetGrowth12  *float64 `json:"netGrowth12"`
	NetGrowth36  *float64 `json:"netGrowth36"`
	NetGrowth60  *float64 `json:"netGrowth60"`
	NetGrowthYTD *float64 `json:"netGrowthYTD"`
	NetGrowthAll *float64 `json:"netGrowthAll"`
}

func (FundBasic) TableName() string {
	return "fund_basic"
}

var (
	reFSName      = regexp.MustCompile(`var\s+fS_name\s*=\s*"([^"]*)"`)
	rePerformance = regexp.MustCompile(`var\s+Data_performance\s*=\s*(\{.+?\});`)
	reFundManager = regexp.MustCompile(`var\s+Data_currentFundManager\s*=\s*(\[.+?\]);`)
	reFluctuation = regexp.MustCompile(`var\s+Data_fluctuationScale\s*=\s*(\{.+?\});`)
	reSyl1y       = regexp.MustCompile(`var\s+syl_1y\s*=\s*"([^"]*)"`)
	reSyl3y       = regexp.MustCompile(`var\s+syl_3y\s*=\s*"([^"]*)"`)
	reSyl6y       = regexp.MustCompile(`var\s+syl_6y\s*=\s*"([^"]*)"`)
	reSyl1n       = regexp.MustCompile(`var\s+syl_1n\s*=\s*"([^"]*)"`)
)

type performanceData struct {
	SamePeriod map[string][]interface{} `json:"samePeriod"`
	Hb         map[string][]interface{} `json:"hb"`
}

func (f *FundApi) GetFundList(key string) []FundBasic {
	var funds []FundBasic
	db.Dao.Where("code like ? or name like ? or full_name like ?", "%"+key+"%", "%"+key+"%", "%"+key+"%").Limit(10).Find(&funds)
	if len(funds) == 0 {
		f.searchFundOnline(key)
		db.Dao.Where("code like ? or name like ? or full_name like ?", "%"+key+"%", "%"+key+"%", "%"+key+"%").Limit(10).Find(&funds)
	}
	return funds
}

func (f *FundApi) searchFundOnline(key string) {
	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" {
		return
	}

	// 先尝试 m=1（开放式基金/ETF），若未找到再尝试 m=0（全品类包含场内ETF）
	urls := []string{
		fmt.Sprintf("https://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx?callback=&m=1&key=%s", cleanKey),
		fmt.Sprintf("https://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx?callback=&m=0&key=%s", cleanKey),
	}

	type onlineFundItem struct {
		Code         string `json:"CODE"`
		CodeLower    string `json:"Code"`
		Name         string `json:"NAME"`
		NameLower    string `json:"Name"`
		FundBaseInfo struct {
			FType string `json:"FTYPE"`
			JJGS  string `json:"JJGS"`
		} `json:"FundBaseInfo"`
	}

	foundCount := 0
	for _, u := range urls {
		resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
			SetHeader("User-Agent", getRandomUA()).
			SetHeader("Referer", "https://fund.eastmoney.com/").
			Get(u)
		if err != nil || resp.StatusCode() != 200 {
			continue
		}
		var result struct {
			Datas []onlineFundItem `json:"Datas"`
		}
		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			continue
		}
		for _, item := range result.Datas {
			c := strings.TrimSpace(item.Code)
			if c == "" {
				c = strings.TrimSpace(item.CodeLower)
			}
			n := strings.TrimSpace(item.Name)
			if n == "" {
				n = strings.TrimSpace(item.NameLower)
			}
			if c == "" {
				continue
			}
			var count int64
			db.Dao.Model(&FundBasic{}).Where("code=?", c).Count(&count)
			if count == 0 {
				fund := &FundBasic{
					Code:    c,
					Name:    n,
					Type:    item.FundBaseInfo.FType,
					Company: item.FundBaseInfo.JJGS,
				}
				db.Dao.Create(fund)
				foundCount++
			} else {
				foundCount++
			}
		}
		if foundCount > 0 {
			break
		}
	}

	// 若未搜出且输入为 6 位数字，直接尝试爬取基本信息（场内 ETF / 场外基金自愈抓取）
	if foundCount == 0 && regexp.MustCompile(`^\d{6}$`).MatchString(cleanKey) {
		_, _ = f.CrawlFundBasic(cleanKey)
	}
}

func (f *FundApi) GetFollowedFund() []FollowedFund {
	var funds []FollowedFund
	db.Dao.Preload("FundBasic").Find(&funds)
	for i, fund := range funds {
		if fund.FundBasic.Code != "" && (fund.FundBasic.Company == "" || fund.FundBasic.NetGrowthYTD == nil) {
			if crawled, err := f.CrawlFundBasic(fund.Code); err == nil && crawled != nil {
				funds[i].FundBasic = *crawled
			}
		}
		f.CrawlFundNetUnitValue(fund.Code)
		f.CrawlFundNetEstimatedUnit(fund.Code)
		for i2 := range funds {
			if funds[i2].Code == fund.Code {
				var updated FollowedFund
				db.Dao.Where("code=?", fund.Code).First(&updated)
				if updated.NetUnitValue != nil {
					funds[i2].NetUnitValue = updated.NetUnitValue
				}
				if updated.NetUnitValueDate != "" {
					funds[i2].NetUnitValueDate = updated.NetUnitValueDate
				}
				if updated.NetEstimatedUnit != nil {
					funds[i2].NetEstimatedUnit = updated.NetEstimatedUnit
				}
				if updated.NetEstimatedTime != "" {
					funds[i2].NetEstimatedTime = updated.NetEstimatedTime
				}
				if updated.NetUnitValuePrev != nil {
					funds[i2].NetUnitValuePrev = updated.NetUnitValuePrev
				}
				if updated.NetActualRate != nil {
					funds[i2].NetActualRate = updated.NetActualRate
				}
				break
			}
		}
		if fund.NetUnitValue == nil && fund.FundBasic.NetUnitValue != nil {
			funds[i].NetUnitValue = fund.FundBasic.NetUnitValue
		}
		if fund.NetUnitValueDate == "" && fund.FundBasic.NetUnitValueDate != "" {
			funds[i].NetUnitValueDate = fund.FundBasic.NetUnitValueDate
		}
		if fund.NetEstimatedUnit == nil && fund.FundBasic.NetEstimatedUnit != nil {
			funds[i].NetEstimatedUnit = fund.FundBasic.NetEstimatedUnit
		}
		if fund.NetEstimatedTime == "" && fund.FundBasic.NetEstimatedTime != "" {
			funds[i].NetEstimatedTime = fund.FundBasic.NetEstimatedTime
		}
		if fund.NetAccumulated == nil && fund.FundBasic.NetAccumulated != nil {
			funds[i].NetAccumulated = fund.FundBasic.NetAccumulated
		}
		if funds[i].NetEstimatedUnit != nil && funds[i].NetUnitValuePrev != nil && *funds[i].NetUnitValuePrev > 0 {
			netEstimatedRate := (*(funds[i].NetEstimatedUnit) - *(funds[i].NetUnitValuePrev)) / *(funds[i].NetUnitValuePrev) * 100
			netEstimatedRate = mathutil.RoundToFloat(netEstimatedRate, 2)
			funds[i].NetEstimatedRate = &netEstimatedRate
		} else if funds[i].NetUnitValue != nil && funds[i].NetEstimatedUnit != nil && *funds[i].NetUnitValue > 0 {
			netEstimatedRate := (*(funds[i].NetEstimatedUnit) - *(funds[i].NetUnitValue)) / *(funds[i].NetUnitValue) * 100
			netEstimatedRate = mathutil.RoundToFloat(netEstimatedRate, 2)
			funds[i].NetEstimatedRate = &netEstimatedRate
		}
		if funds[i].NetActualRate == nil && funds[i].NetUnitValue != nil && funds[i].NetUnitValuePrev != nil && *funds[i].NetUnitValuePrev > 0 {
			estimatedDate := ""
			if funds[i].NetEstimatedTime != "" {
				parts := strings.SplitN(funds[i].NetEstimatedTime, " ", 2)
				estimatedDate = parts[0]
			}
			isNetValueUpdated := false
			if estimatedDate != "" {
				isNetValueUpdated = funds[i].NetUnitValueDate >= estimatedDate
			} else {
				isNetValueUpdated = funds[i].NetUnitValueDate == time.Now().Format("2006-01-02")
			}
			if isNetValueUpdated {
				netActualRate := (*(funds[i].NetUnitValue) - *(funds[i].NetUnitValuePrev)) / *(funds[i].NetUnitValuePrev) * 100
				netActualRate = mathutil.RoundToFloat(netActualRate, 2)
				funds[i].NetActualRate = &netActualRate
			}
		}
	}
	return funds
}

type FollowedFundPagedResult struct {
	Items      []FollowedFund `json:"items"`
	TotalCount int64          `json:"totalCount"`
	PageIndex  int            `json:"pageIndex"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

var sinajsVarRe = regexp.MustCompile(`var\s+hq_str_([^\s=]+)\s*=\s*"([^"]*)"`)

type sinajsFData struct {
	Name             string
	NetUnitValue     *float64
	NetUnitValueDate string
}

type sinajsFuData struct {
	NetEstimatedUnit *float64
	NetEstimatedRate *float64
	NetUnitValuePrev *float64
	EstimatedTime    string
}

type sinajsStockData struct {
	NetEstimatedUnit *float64
	NetEstimatedRate *float64
	NetEstimatedTime string
	PrevClose        *float64
}

func (f *FundApi) parseSinajsBatchResponse(body string, parsedF map[string]*sinajsFData, parsedFu map[string]*sinajsFuData, parsedStock map[string]*sinajsStockData) {
	matches := sinajsVarRe.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		key := m[1]
		value := m[2]
		if value == "" {
			continue
		}

		if strings.HasPrefix(key, "f_") && !strings.HasPrefix(key, "fu_") {
			code := key[2:]
			parts := strings.Split(value, ",")
			if len(parts) < 5 {
				continue
			}
			data := &sinajsFData{Name: strings.TrimSpace(parts[0])}
			if val, err := convertor.ToFloat(parts[1]); err == nil && val > 0 {
				data.NetUnitValue = &val
			}
			if len(parts) > 4 {
				data.NetUnitValueDate = strings.TrimSpace(parts[4])
			}
			if data.NetUnitValue != nil {
				parsedF[code] = data
			}
		} else if strings.HasPrefix(key, "fu_") {
			code := key[3:]
			parts := strings.Split(value, ",")
			if len(parts) < 8 {
				continue
			}
			data := &sinajsFuData{}
			if val, err := convertor.ToFloat(parts[2]); err == nil && val > 0 {
				data.NetEstimatedUnit = &val
			}
			if val, err := convertor.ToFloat(parts[3]); err == nil && val > 0 {
				data.NetUnitValuePrev = &val
			}
			if val, err := convertor.ToFloat(parts[6]); err == nil {
				data.NetEstimatedRate = &val
			}
			tm := strings.TrimSpace(parts[1])
			date := strings.TrimSpace(parts[7])
			if tm != "" && date != "" {
				data.EstimatedTime = date + " " + tm
			}
			if data.NetEstimatedUnit != nil {
				parsedFu[code] = data
			}
		} else if strings.HasPrefix(key, "sh") || strings.HasPrefix(key, "sz") {
			code := key[2:]
			parts := strings.Split(value, ",")
			if len(parts) < 32 {
				continue
			}
			data := &sinajsStockData{}
			currentPrice, err1 := convertor.ToFloat(parts[3])
			prevClose, err2 := convertor.ToFloat(parts[2])
			if err1 != nil || currentPrice == 0 || err2 != nil || prevClose == 0 {
				continue
			}
			data.NetEstimatedUnit = &currentPrice
			data.PrevClose = &prevClose
			changeRate := (currentPrice - prevClose) / prevClose * 100
			changeRate = mathutil.RoundToFloat(changeRate, 2)
			data.NetEstimatedRate = &changeRate
			if len(parts) > 31 {
				data.NetEstimatedTime = strings.TrimSpace(parts[31])
			}
			parsedStock[code] = data
		}
	}
}

func (f *FundApi) batchCrawlFundData(funds []FollowedFund) {
	var sinajsList []string
	var offExchangeCodes []string
	var onExchangeCodes []string

	for _, fund := range funds {
		if IsOnExchangeFund(fund.Code) {
			onExchangeCodes = append(onExchangeCodes, fund.Code)
			if len(fund.Code) >= 1 {
				switch fund.Code[0:1] {
				case "5", "6":
					sinajsList = append(sinajsList, "sh"+fund.Code)
				case "1", "2":
					sinajsList = append(sinajsList, "sz"+fund.Code)
				}
			}
		} else {
			offExchangeCodes = append(offExchangeCodes, fund.Code)
			sinajsList = append(sinajsList, "f_"+fund.Code, "fu_"+fund.Code)
		}
	}

	parsedF := make(map[string]*sinajsFData)
	parsedFu := make(map[string]*sinajsFuData)
	parsedStock := make(map[string]*sinajsStockData)

	if len(sinajsList) > 0 {
		listStr := strings.Join(sinajsList, ",")
		reqURL := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), listStr)
		resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
			SetHeader("Host", "hq.sinajs.cn").
			SetHeader("User-Agent", getRandomUA()).
			SetHeader("Referer", "https://finance.sina.com.cn").
			Get(reqURL)
		if err == nil && resp.StatusCode() == 200 {
			body := string(GB18030ToUTF8(resp.Body()))
			f.parseSinajsBatchResponse(body, parsedF, parsedFu, parsedStock)
		}
	}

	for _, code := range offExchangeCodes {
		if data, ok := parsedF[code]; ok {
			fund := &FollowedFund{
				Code:             code,
				NetUnitValue:     data.NetUnitValue,
				NetUnitValueDate: data.NetUnitValueDate,
			}
			if data.Name != "" {
				fund.Name = data.Name
			}
			db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
		}
		if data, ok := parsedFu[code]; ok {
			fund := &FollowedFund{
				Code:             code,
				NetEstimatedUnit: data.NetEstimatedUnit,
				NetEstimatedRate: data.NetEstimatedRate,
				NetEstimatedTime: data.EstimatedTime,
			}
			if data.NetUnitValuePrev != nil {
				fund.NetUnitValuePrev = data.NetUnitValuePrev
			}
			db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
		}
	}

	for _, code := range onExchangeCodes {
		if data, ok := parsedStock[code]; ok {
			fund := &FollowedFund{
				Code:             code,
				NetEstimatedUnit: data.NetEstimatedUnit,
				NetEstimatedRate: data.NetEstimatedRate,
				NetEstimatedTime: data.NetEstimatedTime,
			}
			if data.PrevClose != nil {
				fund.NetUnitValuePrev = data.PrevClose
			}
			db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)
		}
	}

	var wg sync.WaitGroup
	for _, fund := range funds {
		code := fund.Code
		if fund.FundBasic.Code != "" && (fund.FundBasic.Company == "" || fund.FundBasic.NetGrowthYTD == nil) {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Warnf("CrawlFundBasic panic for %s: %v", c, r)
					}
				}()
				f.CrawlFundBasic(c)
			}(code)
		}
	}

	for _, code := range offExchangeCodes {
		if _, ok := parsedF[code]; !ok {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Warnf("crawlFundNetUnitValueViaEastMoney panic for %s: %v", c, r)
					}
				}()
				f.crawlFundNetUnitValueViaEastMoney(c)
			}(code)
		}
		if _, ok := parsedFu[code]; !ok {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Warnf("CrawlFundNetEstimatedUnit panic for %s: %v", c, r)
					}
				}()
				f.CrawlFundNetEstimatedUnit(c)
			}(code)
		}
	}

	for _, code := range onExchangeCodes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Warnf("crawlOnExchangeFundNetUnitValue panic for %s: %v", c, r)
				}
			}()
			f.crawlOnExchangeFundNetUnitValue(c)
		}(code)
		if _, ok := parsedStock[code]; !ok {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Warnf("crawlOnExchangeFundQuote panic for %s: %v", c, r)
					}
				}()
				f.crawlOnExchangeFundQuote(c)
			}(code)
		}
	}

	wg.Wait()
}

func computeFundRates(fund *FollowedFund) {
	if fund.NetEstimatedUnit != nil && fund.NetUnitValuePrev != nil && *fund.NetUnitValuePrev > 0 {
		rate := (*(fund.NetEstimatedUnit) - *(fund.NetUnitValuePrev)) / *(fund.NetUnitValuePrev) * 100
		rate = mathutil.RoundToFloat(rate, 2)
		fund.NetEstimatedRate = &rate
	} else if fund.NetUnitValue != nil && fund.NetEstimatedUnit != nil && *fund.NetUnitValue > 0 {
		rate := (*(fund.NetEstimatedUnit) - *(fund.NetUnitValue)) / *(fund.NetUnitValue) * 100
		rate = mathutil.RoundToFloat(rate, 2)
		fund.NetEstimatedRate = &rate
	}
	if fund.NetActualRate == nil && fund.NetUnitValue != nil && fund.NetUnitValuePrev != nil && *fund.NetUnitValuePrev > 0 {
		estimatedDate := ""
		if fund.NetEstimatedTime != "" {
			parts := strings.SplitN(fund.NetEstimatedTime, " ", 2)
			estimatedDate = parts[0]
		}
		isNetValueUpdated := false
		if estimatedDate != "" {
			isNetValueUpdated = fund.NetUnitValueDate >= estimatedDate
		} else {
			isNetValueUpdated = fund.NetUnitValueDate == time.Now().Format("2006-01-02")
		}
		if isNetValueUpdated {
			rate := (*(fund.NetUnitValue) - *(fund.NetUnitValuePrev)) / *(fund.NetUnitValuePrev) * 100
			rate = mathutil.RoundToFloat(rate, 2)
			fund.NetActualRate = &rate
		}
	}
}

func mergeFundBasicDefaults(fund *FollowedFund) {
	if fund.NetUnitValue == nil && fund.FundBasic.NetUnitValue != nil {
		fund.NetUnitValue = fund.FundBasic.NetUnitValue
	}
	if fund.NetUnitValueDate == "" && fund.FundBasic.NetUnitValueDate != "" {
		fund.NetUnitValueDate = fund.FundBasic.NetUnitValueDate
	}
	if fund.NetEstimatedUnit == nil && fund.FundBasic.NetEstimatedUnit != nil {
		fund.NetEstimatedUnit = fund.FundBasic.NetEstimatedUnit
	}
	if fund.NetEstimatedTime == "" && fund.FundBasic.NetEstimatedTime != "" {
		fund.NetEstimatedTime = fund.FundBasic.NetEstimatedTime
	}
	if fund.NetAccumulated == nil && fund.FundBasic.NetAccumulated != nil {
		fund.NetAccumulated = fund.FundBasic.NetAccumulated
	}
}

func (f *FundApi) GetFollowedFundPaged(pageIndex, pageSize int, keyword string) *FollowedFundPagedResult {
	if pageIndex <= 0 {
		pageIndex = 1
	}
	if pageSize <= 0 {
		pageSize = 4
	}

	query := db.Dao.Model(&FollowedFund{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		query = query.Joins("LEFT JOIN fund_basic ON fund_basic.code = followed_fund.code").
			Where("followed_fund.code LIKE ? OR followed_fund.name LIKE ? OR fund_basic.full_name LIKE ?", kw, kw, kw)
	}

	var totalCount int64
	query.Count(&totalCount)

	totalPages := int(math.Ceil(float64(totalCount) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	var funds []FollowedFund
	offset := (pageIndex - 1) * pageSize
	if keyword != "" {
		kw := "%" + keyword + "%"
		db.Dao.Preload("FundBasic").
			Joins("LEFT JOIN fund_basic ON fund_basic.code = followed_fund.code").
			Where("followed_fund.code LIKE ? OR followed_fund.name LIKE ? OR fund_basic.full_name LIKE ?", kw, kw, kw).
			Offset(offset).Limit(pageSize).Find(&funds)
	} else {
		db.Dao.Preload("FundBasic").Offset(offset).Limit(pageSize).Find(&funds)
	}

	if len(funds) > 0 {
		f.batchCrawlFundData(funds)

		var codes []string
		for _, fund := range funds {
			codes = append(codes, fund.Code)
		}
		var updatedFunds []FollowedFund
		db.Dao.Preload("FundBasic").Where("code IN ?", codes).Find(&updatedFunds)

		for i := range updatedFunds {
			mergeFundBasicDefaults(&updatedFunds[i])
			computeFundRates(&updatedFunds[i])
		}
		funds = updatedFunds
	}

	if funds == nil {
		funds = []FollowedFund{}
	}

	return &FollowedFundPagedResult{
		Items:      funds,
		TotalCount: totalCount,
		PageIndex:  pageIndex,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

func (f *FundApi) FollowFund(fundCode string) string {
	fundCode = strings.TrimSpace(fundCode)
	if fundCode == "" {
		return "基金代码不能为空"
	}
	var fund FundBasic
	db.Dao.Where("code=?", fundCode).First(&fund)
	if fund.Code == "" || fund.Name == "" {
		crawled, err := f.CrawlFundBasic(fundCode)
		if err != nil || crawled == nil || crawled.Name == "" {
			// 若依然为空且为场内 ETF，尝试通过 Sina 行情快速拿到名称保底
			if IsOnExchangeFund(fundCode) {
				var listStr string
				if fundCode[0] == '5' || fundCode[0] == '6' {
					listStr = "sh" + fundCode
				} else {
					listStr = "sz" + fundCode
				}
				reqURL := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), listStr)
				resp, errSina := f.client.SetTimeout(5*time.Second).R().
					SetHeader("Host", "hq.sinajs.cn").
					SetHeader("Referer", "https://finance.sina.com.cn").
					Get(reqURL)
				if errSina == nil && resp.StatusCode() == 200 {
					body := string(GB18030ToUTF8(resp.Body()))
					reName := regexp.MustCompile(`var\s+hq_str_\w+="([^,]+),`)
					if m := reName.FindStringSubmatch(body); len(m) > 1 && m[1] != "" {
						fund = FundBasic{
							Code: fundCode,
							Name: strings.TrimSpace(m[1]),
							Type: "场内ETF",
						}
						db.Dao.Create(&fund)
					}
				}
			}
			if fund.Code == "" || fund.Name == "" {
				return "基金信息不存在或获取失败"
			}
		} else {
			fund = *crawled
		}
	}
	follow := &FollowedFund{
		Code: fundCode,
		Name: fund.Name,
	}
	err := db.Dao.Model(follow).Where("code = ?", fundCode).FirstOrCreate(follow, "code", fund.Code).Error
	if err != nil {
		return "关注失败"
	}
	return "关注成功"
}

func (f *FundApi) UnFollowFund(fundCode string) string {
	var fund FollowedFund
	db.Dao.Where("code=?", fundCode).First(&fund)
	if fund.Code != "" {
		err := db.Dao.Model(&fund).Delete(&fund).Error
		if err != nil {
			return "取消关注失败"
		}
		return "取消关注成功"
	} else {
		return "基金信息不存在"
	}
}

func (f *FundApi) AllFund() {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("AllFund panic: %v", r)
		}
	}()

	response, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", "https://fund.eastmoney.com/").
		Get("https://fund.eastmoney.com/allfund.html")
	if err != nil {
		return
	}
	htmlContent := GB18030ToUTF8(response.Body())

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Errorf("AllFund parse HTML error: %v", err)
		return
	}
	cnt := 0
	doc.Find("ul.num_right li").Each(func(i int, s *goquery.Selection) {
		text := strutil.SplitEx(s.Text(), "|", true)
		if len(text) > 0 {
			cnt++
			name := text[0]
			str := strutil.SplitAndTrim(name, "）", "（", "）")
			fund := &FundBasic{
				Code: str[0],
				Name: str[1],
			}
			count := int64(0)
			db.Dao.Model(fund).Where("code=?", fund.Code).Count(&count)
			if count == 0 {
				db.Dao.Create(fund)
			}
		}
	})
}

type FundNetUnitValue struct {
	Fundcode string `json:"fundcode"`
	Name     string `json:"name"`
	Jzrq     string `json:"jzrq"`
	Dwjz     string `json:"dwjz"`
	Gsz      string `json:"gsz"`
	Gszzl    string `json:"gszzl"`
	Gztime   string `json:"gztime"`
}

type FundHistoryNetValue struct {
	Date        string  `json:"date"`
	NetValue    float64 `json:"netValue"`
	AccumValue  float64 `json:"accumValue"`
	DailyGrowth float64 `json:"dailyGrowth"`
	BuyStatus   string  `json:"buyStatus"`
	SellStatus  string  `json:"sellStatus"`
}

type FundHoldingStock struct {
	Rank       int      `json:"rank"`
	StockCode  string   `json:"stockCode"`
	StockName  string   `json:"stockName"`
	Ratio      float64  `json:"ratio"`
	Shares     string   `json:"shares"`
	MarketCap  string   `json:"marketCap"`
	Quarter    string   `json:"quarter"`
	Price      *float64 `json:"price"`
	ChangeRate *float64 `json:"changeRate"`
	Market     string   `json:"market"`
}

type quoteData struct {
	price      float64
	changeRate float64
}

type FundRankingItem struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Pinyin           string   `json:"pinyin"`
	NetValueDate     string   `json:"netValueDate"`
	NetUnitValue     *float64 `json:"netUnitValue"`
	NetAccumulated   *float64 `json:"netAccumulated"`
	DailyGrowth      *float64 `json:"dailyGrowth"`
	WeekGrowth       *float64 `json:"weekGrowth"`
	MonthGrowth      *float64 `json:"monthGrowth"`
	ThreeMonthGrowth *float64 `json:"threeMonthGrowth"`
	SixMonthGrowth   *float64 `json:"sixMonthGrowth"`
	YearGrowth       *float64 `json:"yearGrowth"`
	TwoYearGrowth    *float64 `json:"twoYearGrowth"`
	ThreeYearGrowth  *float64 `json:"threeYearGrowth"`
	YTDGrowth        *float64 `json:"ytdGrowth"`
	SinceInception   *float64 `json:"sinceInception"`
	EstablishDate    string   `json:"establishDate"`
	Purchasable      bool     `json:"purchasable"`
	Scale            *float64 `json:"scale"`
	PurchaseRate     *float64 `json:"purchaseRate"`
	DiscountRate     *float64 `json:"discountRate"`
	FundTypeDetail   string   `json:"fundTypeDetail"`
}

type FundRankingResult struct {
	Items      []FundRankingItem `json:"items"`
	TotalCount int               `json:"totalCount"`
	PageIndex  int               `json:"pageIndex"`
	PageSize   int               `json:"pageSize"`
	TotalPages int               `json:"totalPages"`
}

type FundSearchItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}
