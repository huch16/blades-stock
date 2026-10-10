package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"
)

// StockRecommendationDraft 提取出的推荐草稿
type StockRecommendationDraft struct {
	Code      string
	Name      string
	BkName    string
	ModelName string
	Reason    string
	Rating    string
}

func getPayloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// ExtractAndSaveRecommendationsFromReport 从生成的研报中提取标的并自动沉淀入库
func ExtractAndSaveRecommendationsFromReport(report *MarketIntelligenceReport) ([]*models.AiRecommendStocks, error) {
	if report == nil {
		return nil, nil
	}

	draftsMap := make(map[string]*StockRecommendationDraft)

	// 1. 优先从报告携带的伴随动作中提取结构化标的
	for _, act := range report.Actions {
		if act.Type == types.ActionOpenDiagnosis || act.Type == types.ActionShowKline {
			code := strings.TrimSpace(getPayloadString(act.Payload, "code"))
			cleanCode := models.CleanStockCode(code)
			if cleanCode != "" && len(cleanCode) == 6 {
				name := strings.TrimSpace(getPayloadString(act.Payload, "name"))
				reason := strings.TrimSpace(getPayloadString(act.Payload, "reason"))
				if reason == "" {
					reason = fmt.Sprintf("来自【%s】推荐关注", report.Title)
				}
				draftsMap[cleanCode] = &StockRecommendationDraft{
					Code:      cleanCode,
					Name:      name,
					ModelName: report.Source,
					Reason:    reason,
					Rating:    "买入/关注",
				}
			}
		}
	}

	// 2. 如果 Actions 中标的不足，正则扫描研报正文中可能提及的明确股票/ETF代码
	if len(draftsMap) == 0 && report.Content != "" {
		codeRegex := regexp.MustCompile(`(?i)(?:股票|代码|ETF|标的|关注)?[:：\s]*([036]\d{5}|51\d{4}|58\d{4}|15\d{4})(?:\.?[a-z]{2})?(?:[（\(]([\p{Han}A-Za-z0-9]+)[）\)])?`)
		matches := codeRegex.FindAllStringSubmatch(report.Content, 10)
		for _, m := range matches {
			if len(m) > 1 {
				cleanCode := models.CleanStockCode(m[1])
				if cleanCode != "" && len(cleanCode) == 6 {
					if _, exists := draftsMap[cleanCode]; !exists {
						name := ""
						if len(m) > 2 {
							name = strings.TrimSpace(m[2])
						}
						draftsMap[cleanCode] = &StockRecommendationDraft{
							Code:      cleanCode,
							Name:      name,
							ModelName: report.Source,
							Reason:    fmt.Sprintf("从研报《%s》提炼重点关注标的", report.Title),
							Rating:    "关注",
						}
					}
				}
			}
		}
	}

	if len(draftsMap) == 0 {
		return nil, nil
	}

	var drafts []*StockRecommendationDraft
	for _, d := range draftsMap {
		drafts = append(drafts, d)
	}

	return saveDraftsToDatabase(drafts)
}

// SyncRecommendStocksFromAllIntelligence 一键从所有已有研报、ETF动量池、市场异动中聚合并补全推荐股票
func SyncRecommendStocksFromAllIntelligence(ctx context.Context) (int, error) {
	draftsMap := make(map[string]*StockRecommendationDraft)

	// 1. 扫描当前内存缓存中的最新研报
	lifecycleReportCacheMu.RLock()
	cachedReport := lifecycleLatestReport
	lifecycleReportCacheMu.RUnlock()
	if cachedReport != nil {
		for _, act := range cachedReport.Actions {
			code := models.CleanStockCode(getPayloadString(act.Payload, "code"))
			if code != "" && len(code) == 6 {
				draftsMap[code] = &StockRecommendationDraft{
					Code:      code,
					Name:      getPayloadString(act.Payload, "name"),
					ModelName: cachedReport.Source,
					Reason:    fmt.Sprintf("【%s】精选标的", cachedReport.Title),
					Rating:    "增持",
				}
			}
		}
	}

	// 2. 从量化 ETF 21日加权动量池中直接抓取动量领先的前 3 只核心 ETF
	coreETFs := []string{"512660", "512480", "512880", "515050", "159995", "510300", "588000"}
	topETFs, err := quant.BatchCalculateETFMomentum(coreETFs)
	if err == nil && len(topETFs) > 0 {
		limit := 3
		if len(topETFs) < limit {
			limit = len(topETFs)
		}
		for i := 0; i < limit; i++ {
			etf := topETFs[i]
			cleanCode := models.CleanStockCode(etf.Code)
			if cleanCode != "" {
				draftsMap[cleanCode] = &StockRecommendationDraft{
					Code:      cleanCode,
					Name:      etf.Name,
					BkName:    "核心行业ETF",
					ModelName: "ETF21日加权动量引擎",
					Reason:    fmt.Sprintf("21日半衰期动量分 %.2f，行业排名第 %d，多周期趋势共振", etf.Score, i+1),
					Rating:    "强力推荐",
				}
			}
		}
	}

	// 3. 从数据库已保存的历史 AI 报告 (ai_response_result) 倒序扫描最近 10 条
	var recentResponses []models.AIResponseResult
	if db.Dao != nil {
		db.Dao.Order("id desc").Limit(10).Find(&recentResponses)
		for _, item := range recentResponses {
			cleanCode := models.CleanStockCode(item.StockCode)
			if cleanCode != "" && len(cleanCode) == 6 && !strings.Contains(item.StockName, "复盘") && !strings.Contains(item.StockName, "大盘") {
				if _, exists := draftsMap[cleanCode]; !exists {
					draftsMap[cleanCode] = &StockRecommendationDraft{
						Code:      cleanCode,
						Name:      item.StockName,
						ModelName: item.ModelName,
						Reason:    fmt.Sprintf("历史深度分析总结: %s", item.Question),
						Rating:    "推荐",
					}
				}
			}
		}
	}

	if len(draftsMap) == 0 {
		return 0, nil
	}

	var drafts []*StockRecommendationDraft
	for _, d := range draftsMap {
		drafts = append(drafts, d)
	}

	saved, err := saveDraftsToDatabase(drafts)
	if err != nil {
		return 0, err
	}
	return len(saved), nil
}

// saveDraftsToDatabase 补全行情、计算止盈止损区间并去重落库
func saveDraftsToDatabase(drafts []*StockRecommendationDraft) ([]*models.AiRecommendStocks, error) {
	if len(drafts) == 0 {
		return nil, nil
	}

	var codes []string
	for _, d := range drafts {
		codes = append(codes, d.Code)
	}

	// 批量拉取实时行情以获得准确价格
	stockDataApi := data.NewStockDataApi()
	stockInfos, _ := stockDataApi.GetStockCodeRealTimeData(codes...)
	infoMap := make(map[string]data.StockInfo)
	if stockInfos != nil {
		for _, info := range *stockInfos {
			clean := models.CleanStockCode(info.Code)
			infoMap[clean] = info
		}
	}

	todayStr := time.Now().Format("2006-01-02")
	var savedList []*models.AiRecommendStocks

	for _, d := range drafts {
		// 检查今日是否已存在该股票的推荐记录，避免频繁重复添加
		var count int64
		if db.Dao != nil {
			db.Dao.Model(&models.AiRecommendStocks{}).
				Where("stock_code = ? AND date(created_at) = ?", d.Code, todayStr).
				Count(&count)
			if count > 0 {
				continue
			}
		}

		stockName := d.Name
		curPrice := 0.0
		preClose := 0.0
		curPriceStr := "0.00"
		prePriceStr := "0.00"

		if info, ok := infoMap[d.Code]; ok {
			if stockName == "" || stockName == d.Code {
				stockName = info.Name
			}
			curPrice, _ = strconv.ParseFloat(info.Price, 64)
			preClose, _ = strconv.ParseFloat(info.PreClose, 64)
			curPriceStr = info.Price
			prePriceStr = info.PreClose
		}

		if curPrice <= 0 {
			curPrice = preClose
		}
		if curPrice <= 0 {
			curPrice = 10.0 // 兜底基准价格防除零
		}

		// 量化推演建议买入区间、止损价（-4%）与止盈价（+8%）
		buyMin := curPrice * 0.985
		buyMax := curPrice * 1.005
		stopLoss := curPrice * 0.955
		stopProfitMin := curPrice * 1.06
		stopProfitMax := curPrice * 1.12

		rec := &models.AiRecommendStocks{
			ModelName:                   d.ModelName,
			Rating:                      d.Rating,
			StockCode:                   d.Code,
			StockName:                   stockName,
			BkName:                      d.BkName,
			StockPrice:                  curPriceStr,
			StockCurrentPrice:           curPriceStr,
			StockCurrentPriceTime:       time.Now().Format("2006-01-02 15:04:05"),
			StockClosePrice:             curPriceStr,
			StockPrePrice:               prePriceStr,
			RecommendReason:             d.Reason,
			RecommendBuyPrice:           fmt.Sprintf("%.2f ~ %.2f", buyMin, buyMax),
			RecommendBuyPriceMin:        buyMin,
			RecommendBuyPriceMax:        buyMax,
			RecommendStopProfitPrice:    fmt.Sprintf("%.2f ~ %.2f", stopProfitMin, stopProfitMax),
			RecommendStopProfitPriceMin: stopProfitMin,
			RecommendStopProfitPriceMax: stopProfitMax,
			RecommendStopLossPrice:      fmt.Sprintf("%.2f", stopLoss),
			RiskRemarks:                 "若跌破建议止损价位请严格执行纪律；注意大盘整体情绪共振",
			Remarks:                     "由 blades-stock 智能研报与量化矩阵自动汇聚生成",
			EnableAlert:                 true,
		}

		if err := data.NewAiRecommendStocksService().CreateAiRecommendStocks(rec); err != nil {
			logger.SugaredLogger.Warnf("保存推荐股票 %s 失败: %v", d.Code, err)
		} else {
			savedList = append(savedList, rec)
		}
	}

	return savedList, nil
}
