package agent

import (
	"context"
	"fmt"
	"math"
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

// isHardRiskBlocked 严密扫描研报中该标的前后上下文，识别硬风控拦截与禁止开仓信号
func isHardRiskBlocked(content, code string) bool {
	if content == "" || code == "" {
		return false
	}
	lowerContent := strings.ToLower(content)
	cleanCode := strings.ToLower(models.CleanStockCode(code))
	idx := strings.Index(lowerContent, cleanCode)
	if idx == -1 {
		return false
	}

	start := idx - 120
	if start < 0 {
		start = 0
	}
	end := idx + 120
	if end > len(content) {
		end = len(content)
	}
	segment := content[start:end]

	riskKeywords := []string{
		"禁止开仓", "硬风控拦截", "风控拦截", "0% 仓位", "0%仓位",
		"规避", "清仓", "空仓", "严禁开仓", "风控触发", "拦截", "暂不开仓",
	}
	for _, kw := range riskKeywords {
		if strings.Contains(segment, kw) {
			return true
		}
	}
	return false
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
				// 严格风控准则：若该标的在报告中已被明确判定为硬风控拦截或禁止开仓，坚决拒收入库
				if isHardRiskBlocked(report.Content, cleanCode) {
					logger.SugaredLogger.Warnf("[RecommendExtractor] 标的 %s 被研报硬风控拦截(禁止开仓/0%%仓位)，坚决拒收入库推荐表", cleanCode)
					continue
				}

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
					if isHardRiskBlocked(report.Content, cleanCode) {
						logger.SugaredLogger.Warnf("[RecommendExtractor] 正则扫描到标的 %s 被研报硬风控拦截(禁止开仓)，坚决跳过", cleanCode)
						continue
					}
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

// CleanDirtyRecommendStocks 清理并修复数据库中历史残留的虚假兜底数据（如 9.85~10.05 模版、0.00 行情、排雷清单误录入等）
func CleanDirtyRecommendStocks() error {
	if db.Dao == nil {
		return nil
	}
	// 1. 删除误将“排雷诊断清单/自选观察池/Prompt大段文字”作为推荐理由的错误记录，并物理清理已被硬风控拦截标的
	db.Dao.Where("recommend_reason LIKE ? OR recommend_reason LIKE ? OR recommend_reason LIKE ? OR risk_remarks LIKE ? OR stock_code IN ('159915', '512480', '159949')",
		"%排雷诊断清单%", "%自选观察池%", "%持仓与自选风险分级清单%", "%硬风控拦截%").
		Delete(&models.AiRecommendStocks{})

	// 2. 找到所有开仓价包含 9.85 ~ 10.05 或现价为 0.00 / 空的记录，尝试拉取真实行情回填，若无法获取则清空虚假指标
	var dirtyList []models.AiRecommendStocks
	db.Dao.Where("stock_price = '0.00' OR stock_price = '' OR stock_current_price = '0.00' OR stock_current_price = '' OR recommend_buy_price LIKE '%9.85 ~ 10.05%'").
		Find(&dirtyList)

	if len(dirtyList) == 0 {
		return nil
	}

	var codes []string
	for _, item := range dirtyList {
		codes = append(codes, data.FormatMarketStockCode(item.StockCode))
	}

	stockDataApi := data.NewStockDataApi()
	stockInfos, _ := stockDataApi.GetStockCodeRealTimeData(codes...)
	infoMap := make(map[string]data.StockInfo)
	if stockInfos != nil {
		for _, info := range *stockInfos {
			clean := models.CleanStockCode(info.Code)
			infoMap[clean] = info
		}
	}

	for _, item := range dirtyList {
		clean := models.CleanStockCode(item.StockCode)
		if info, ok := infoMap[clean]; ok && info.Price != "" && info.Price != "0.00" {
			curPrice, _ := strconv.ParseFloat(info.Price, 64)
			if curPrice > 0 {
				buyMin := math.Round(curPrice*0.985*100) / 100
				buyMax := math.Round(curPrice*1.005*100) / 100
				stopLoss := math.Round(curPrice*0.955*100) / 100
				stopProfitMin := math.Round(curPrice*1.06*100) / 100
				stopProfitMax := math.Round(curPrice*1.12*100) / 100

				item.StockName = info.Name
				item.StockPrice = info.Price
				item.StockCurrentPrice = info.Price
				item.StockClosePrice = info.Price
				item.StockPrePrice = info.PreClose
				item.RecommendBuyPrice = fmt.Sprintf("%.2f ~ %.2f", buyMin, buyMax)
				item.RecommendBuyPriceMin = buyMin
				item.RecommendBuyPriceMax = buyMax
				item.RecommendStopProfitPrice = fmt.Sprintf("%.2f ~ %.2f", stopProfitMin, stopProfitMax)
				item.RecommendStopProfitPriceMin = stopProfitMin
				item.RecommendStopProfitPriceMax = stopProfitMax
				item.RecommendStopLossPrice = fmt.Sprintf("%.2f", stopLoss)
				item.RiskRemarks = fmt.Sprintf("若跌破建议止损价位 %.2f 请严格执行纪律；注意大盘整体情绪共振", stopLoss)
				item.Remarks = "真实行情量化回补"
				db.Dao.Save(&item)
				continue
			}
		}
		// 无法获取真实价格，清空虚假指标
		item.StockPrice = ""
		item.StockCurrentPrice = ""
		item.StockPrePrice = ""
		item.RecommendBuyPrice = ""
		item.RecommendBuyPriceMin = 0
		item.RecommendBuyPriceMax = 0
		item.RecommendStopProfitPrice = ""
		item.RecommendStopProfitPriceMin = 0
		item.RecommendStopProfitPriceMax = 0
		item.RecommendStopLossPrice = ""
		item.RiskRemarks = ""
		item.Remarks = ""
		db.Dao.Save(&item)
	}
	return nil
}

// SyncRecommendStocksFromAllIntelligence 一键从所有已有研报、ETF动量池、市场异动中聚合并补全推荐股票
func SyncRecommendStocksFromAllIntelligence(ctx context.Context) (int, error) {
	// 先执行一次脏数据自动修复与排雷清理
	_ = CleanDirtyRecommendStocks()

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

	// 3. 从数据库已保存的历史 AI 报告 (ai_response_result) 倒序扫描最近 10 条真实个股分析
	var recentResponses []models.AIResponseResult
	if db.Dao != nil {
		db.Dao.Order("id desc").Limit(10).Find(&recentResponses)
		for _, item := range recentResponses {
			cleanCode := models.CleanStockCode(item.StockCode)
			if cleanCode == "" || len(cleanCode) != 6 {
				continue
			}
			// 过滤掉复盘、大盘概览、或者带有“排雷/诊断/自选/持仓”的非推荐报告
			qLower := strings.ToLower(item.Question)
			if strings.Contains(qLower, "排雷") ||
				strings.Contains(qLower, "自选") ||
				strings.Contains(qLower, "持仓") ||
				strings.Contains(qLower, "清单") ||
				strings.Contains(qLower, "复盘") ||
				strings.Contains(qLower, "大盘") ||
				strings.Contains(qLower, "前瞻") ||
				strings.Contains(qLower, "内参") ||
				strings.Contains(qLower, "诊断") ||
				strings.Contains(item.StockName, "复盘") ||
				strings.Contains(item.StockName, "大盘") ||
				len([]rune(item.Question)) > 60 {
				continue
			}

			if _, exists := draftsMap[cleanCode]; !exists {
				reasonText := strings.TrimSpace(item.Question)
				if reasonText == "" {
					reasonText = fmt.Sprintf("AI 专项深度量化分析优选标的：%s", item.StockName)
				} else {
					reasonRunes := []rune(reasonText)
					if len(reasonRunes) > 40 {
						reasonText = string(reasonRunes[:40]) + "..."
					}
					reasonText = fmt.Sprintf("历史深度分析优选: %s", reasonText)
				}
				draftsMap[cleanCode] = &StockRecommendationDraft{
					Code:      cleanCode,
					Name:      item.StockName,
					ModelName: item.ModelName,
					Reason:    reasonText,
					Rating:    "推荐",
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
		codes = append(codes, data.FormatMarketStockCode(d.Code))
	}

	// 批量拉取真实行情以获得准确价格
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
		curPriceStr := ""
		prePriceStr := ""

		if info, ok := infoMap[d.Code]; ok {
			if stockName == "" || stockName == d.Code {
				stockName = info.Name
			}
			curPrice, _ = strconv.ParseFloat(info.Price, 64)
			preClose, _ = strconv.ParseFloat(info.PreClose, 64)
			if curPrice > 0 {
				curPriceStr = fmt.Sprintf("%.2f", curPrice)
			}
			if preClose > 0 {
				prePriceStr = fmt.Sprintf("%.2f", preClose)
			}
		}

		if curPrice <= 0 && preClose > 0 {
			curPrice = preClose
			curPriceStr = prePriceStr
		}

		// 用户准则：“如果不能获得准确数据，就不需要列出这些指标。”
		// 严禁填入虚拟 10.0 兜底价格；若无有效准确行情，开仓/止损/止盈指标一律留空，不展示虚假数据
		var buyMin, buyMax, stopLoss, stopProfitMin, stopProfitMax float64
		var buyPriceStr, stopProfitStr, stopLossStr string

		if curPrice > 0 {
			buyMin = math.Round(curPrice*0.985*100) / 100
			buyMax = math.Round(curPrice*1.005*100) / 100
			stopLoss = math.Round(curPrice*0.955*100) / 100
			stopProfitMin = math.Round(curPrice*1.06*100) / 100
			stopProfitMax = math.Round(curPrice*1.12*100) / 100

			buyPriceStr = fmt.Sprintf("%.2f ~ %.2f", buyMin, buyMax)
			stopProfitStr = fmt.Sprintf("%.2f ~ %.2f", stopProfitMin, stopProfitMax)
			stopLossStr = fmt.Sprintf("%.2f", stopLoss)
		}

		riskRemarks := ""
		if stopLoss > 0 {
			riskRemarks = fmt.Sprintf("若跌破建议止损价位 %.2f 请严格执行纪律；注意大盘整体情绪共振", stopLoss)
		}
		remarks := ""
		if d.BkName != "" {
			remarks = fmt.Sprintf("板块/题材: %s", d.BkName)
		} else if d.ModelName != "" {
			remarks = fmt.Sprintf("由 %s 量化驱动提炼", d.ModelName)
		}

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
			RecommendBuyPrice:           buyPriceStr,
			RecommendBuyPriceMin:        buyMin,
			RecommendBuyPriceMax:        buyMax,
			RecommendStopProfitPrice:    stopProfitStr,
			RecommendStopProfitPriceMin: stopProfitMin,
			RecommendStopProfitPriceMax: stopProfitMax,
			RecommendStopLossPrice:      stopLossStr,
			RiskRemarks:                 riskRemarks,
			Remarks:                     remarks,
			EnableAlert:                 stopLoss > 0,
		}

		if err := data.NewAiRecommendStocksService().CreateAiRecommendStocks(rec); err != nil {
			logger.SugaredLogger.Warnf("保存推荐股票 %s 失败: %v", d.Code, err)
		} else {
			savedList = append(savedList, rec)
		}
	}

	return savedList, nil
}
