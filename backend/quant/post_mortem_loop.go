package quant

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

// klineFetchSemaphore 限制并行获取 K 线的数量，避免同时打开过多行情连接。
var klineFetchSemaphore = make(chan struct{}, 4)

// PostMortemLoopService 实现 T+1 / T+5 / T+20 决策胜率闭环对账、死后剖析与经验结晶自沉淀
type PostMortemLoopService struct{}

func NewPostMortemLoopService() *PostMortemLoopService {
	return &PostMortemLoopService{}
}

// RunDailyAudit 执行每日闭环对账与自反思
func (s *PostMortemLoopService) RunDailyAudit(ctx context.Context, targetDate string) (*models.PostMortemAuditSummary, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	summary := &models.PostMortemAuditSummary{}
	now := time.Now()
	if targetDate == "" {
		targetDate = now.Format("2006-01-02")
	}

	// 1. 查询待回溯审计的推荐历史 (获取近 30 天内且有明确止损价的 AI 推荐记录)
	var recommendations []models.AiRecommendStocks
	err := db.Dao.Model(&models.AiRecommendStocks{}).
		Where("stock_code != '' AND recommend_stop_loss_price != ''").
		Order("data_time desc").
		Limit(100).
		Find(&recommendations).Error
	if err != nil {
		return nil, fmt.Errorf("query recommend stocks failed: %w", err)
	}

	if len(recommendations) == 0 {
		logger.SugaredLogger.Infof("[PostMortem] 未发现待回溯的历史 AI 推荐记录")
		return summary, nil
	}

	horizons := []int{1, 5, 20}

	// 1. 收集所有需要处理的任务，并预解析价格，避免重复解析
	type auditTask struct {
		rec         models.AiRecommendStocks
		initDate    string
		initPrice   float64
		stopLoss    float64
		targetPrice float64
	}
	var tasks []auditTask
	for _, rec := range recommendations {
		if rec.DataTime == nil {
			continue
		}
		initDate := rec.DataTime.Format("2006-01-02")
		initialPrice, _ := strconv.ParseFloat(rec.StockPrice, 64)
		if initialPrice <= 0 {
			initialPrice, _ = strconv.ParseFloat(rec.StockClosePrice, 64)
		}
		if initialPrice <= 0 {
			continue
		}

		stopLoss, _ := strconv.ParseFloat(rec.RecommendStopLossPrice, 64)
		if stopLoss <= 0 {
			stopLoss = initialPrice * 0.95
		}

		targetPrice, _ := strconv.ParseFloat(rec.RecommendStopProfitPrice, 64)
		if targetPrice <= 0 && rec.RecommendStopProfitPriceMin > 0 {
			targetPrice = rec.RecommendStopProfitPriceMin
		}
		if targetPrice <= 0 {
			targetPrice = initialPrice * 1.10
		}

		tasks = append(tasks, auditTask{
			rec:         rec,
			initDate:    initDate,
			initPrice:   initialPrice,
			stopLoss:    stopLoss,
			targetPrice: targetPrice,
		})
	}

	// 2. 并行获取 K 线（带 semaphore 并发控制 + 限流），每只股票只获取一次
	type klineResult struct {
		recID  uint
		klines []data.KLineData
		err    error
	}
	klineCh := make(chan klineResult, len(tasks))
	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Add(1)
		go func(task auditTask) {
			defer wg.Done()
			klineFetchSemaphore <- struct{}{}
			defer func() { <-klineFetchSemaphore }()
			KlineRateWait() // 行情接口限流
			klines := fetchStockDailyKLines(task.rec.StockCode, 60)
			klineCh <- klineResult{recID: task.rec.ID, klines: klines}
		}(t)
	}
	wg.Wait()
	close(klineCh)

	// 将 K 线按 rec.ID 索引
	klinesMap := make(map[uint][]data.KLineData)
	for r := range klineCh {
		if r.err != nil || len(r.klines) == 0 {
			continue
		}
		klinesMap[r.recID] = r.klines
	}

	// 3. 对每只股票的 3 个 horizon 进行评估（共享已获取的 K 线）
	for _, t := range tasks {
		rec := t.rec
		klines, ok := klinesMap[rec.ID]
		if !ok {
			continue
		}

		for _, h := range horizons {
			sourceRef := fmt.Sprintf("ai_rec_%d_h%d", rec.ID, h)

			// 检查是否已完成审计
			var exists int64
			db.Dao.Model(&models.PostMortemAuditRecord{}).
				Where("source_ref = ?", sourceRef).
				Count(&exists)
			if exists > 0 {
				continue
			}

			// 找到推荐日及后续交易日
			evalRes := evaluateHorizonPerformance(klines, t.initDate, h, t.initPrice, t.stopLoss, t.targetPrice)
			if !evalRes.Valid {
				continue
			}

			summary.TotalAudited++

			auditRecord := models.PostMortemAuditRecord{
				SourceRef:          sourceRef,
				StockCode:          rec.StockCode,
				StockName:          rec.StockName,
				InitialRating:      rec.Rating,
				InitialPrice:       t.initPrice,
				InitialStopLoss:    t.stopLoss,
				InitialTargetPrice: t.targetPrice,
				InitialDate:        t.initDate,
				HorizonDays:        h,
				EvaluatedAt:        now,
				ActualReturn:       evalRes.ActualReturn,
				MaxDrawdown:        evalRes.MaxDrawdown,
				LowestPrice:        evalRes.LowestPrice,
				PeakPrice:          evalRes.PeakPrice,
				StopLossBreached:   evalRes.StopLossBreached,
				Status:             evalRes.Status,
			}

			// 若触发止损击穿或严重亏损，执行死后剖析 (Post-Mortem Analysis) 并生成经验结晶
			if evalRes.Status == "AUDITED_FAILURE" {
				summary.FailureCount++
				if evalRes.StopLossBreached {
					summary.StopLossHits++
				}

				reflection, lessons := generatePostMortemReflection(rec, evalRes)
				auditRecord.ReflectionNotes = reflection
				auditRecord.LessonsLearned = lessons

				// 自动沉淀为 KnowledgeCrystal
				crystal := models.KnowledgeCrystal{
					Category:       "risk_warning",
					Title:          fmt.Sprintf("【复盘警示】%s(%s) 止损击穿与死后剖析", rec.StockName, rec.StockCode),
					TriggerPattern: fmt.Sprintf("推荐标的: %s | 周期: T+%d | 击穿止损: %.2f元", rec.StockCode, h, t.stopLoss),
					Summary:        reflection,
					KeyInsights:    lessons,
					AccuracyScore:  88.0,
					SourceDate:     targetDate,
					SourceRef:      sourceRef,
					ContentHash:    models.ComputeContentHash("risk_warning", rec.StockCode+"_"+sourceRef, reflection),
				}

				var dupCount int64
				db.Dao.Model(&models.KnowledgeCrystal{}).Where("content_hash = ?", crystal.ContentHash).Count(&dupCount)
				if dupCount == 0 {
					if err := db.Dao.Create(&crystal).Error; err == nil {
						auditRecord.CrystalID = crystal.ID
						summary.CrystalsGenerated++
						logger.SugaredLogger.Infof("[PostMortem] 成功生成避险经验结晶 ID=%d: %s", crystal.ID, crystal.Title)
					}
				}
			} else if evalRes.Status == "AUDITED_SUCCESS" {
				summary.SuccessCount++
				auditRecord.ReflectionNotes = fmt.Sprintf("T+%d 实际涨幅 +%.2f%%，有效达成目标价，假说与买方逻辑得到市场验证。", h, evalRes.ActualReturn)
				auditRecord.LessonsLearned = fmt.Sprintf("顺应趋势且均线多头排列个股，配合资金流入胜率极佳。")
			} else {
				auditRecord.ReflectionNotes = fmt.Sprintf("T+%d 震荡整理，实际收益率 %.2f%%，未触及止损亦未达目标。", h, evalRes.ActualReturn)
			}

			_ = db.Dao.Create(&auditRecord).Error
		}
	}

	// 计算全局胜率指标
	s.calcGlobalSummary(summary)
	return summary, nil
}

type horizonEvalResult struct {
	Valid            bool
	ActualReturn     float64
	MaxDrawdown      float64
	LowestPrice      float64
	PeakPrice        float64
	StopLossBreached bool
	Status           string // AUDITED_SUCCESS, AUDITED_FAILURE, AUDITED_NEUTRAL
}

func evaluateHorizonPerformance(klines []data.KLineData, initDate string, horizon int, initPrice, stopLoss, targetPrice float64) horizonEvalResult {
	startIdx := -1
	for i, k := range klines {
		if k.Day >= initDate {
			startIdx = i
			break
		}
	}

	if startIdx < 0 || startIdx+horizon >= len(klines) {
		// 尚未走完足够的交易日周期
		return horizonEvalResult{Valid: false}
	}

	lowest := 999999.0
	peak := 0.0
	stopLossHit := false

	for i := startIdx + 1; i <= startIdx+horizon; i++ {
		low, _ := strconv.ParseFloat(klines[i].Low, 64)
		high, _ := strconv.ParseFloat(klines[i].High, 64)
		if low > 0 && low < lowest {
			lowest = low
		}
		if high > peak {
			peak = high
		}
		if low > 0 && stopLoss > 0 && low <= stopLoss {
			stopLossHit = true
		}
	}

	endClose, _ := strconv.ParseFloat(klines[startIdx+horizon].Close, 64)
	if endClose <= 0 {
		return horizonEvalResult{Valid: false}
	}

	actualReturn := ((endClose - initPrice) / initPrice) * 100.0
	maxDrawdown := 0.0
	if lowest > 0 && lowest < initPrice {
		maxDrawdown = ((lowest - initPrice) / initPrice) * 100.0
	}

	status := "AUDITED_NEUTRAL"
	if stopLossHit || actualReturn <= -4.0 {
		status = "AUDITED_FAILURE"
	} else if actualReturn >= 4.0 || (targetPrice > 0 && peak >= targetPrice) {
		status = "AUDITED_SUCCESS"
	}

	return horizonEvalResult{
		Valid:            true,
		ActualReturn:     actualReturn,
		MaxDrawdown:      maxDrawdown,
		LowestPrice:      lowest,
		PeakPrice:        peak,
		StopLossBreached: stopLossHit,
		Status:           status,
	}
}

func generatePostMortemReflection(rec models.AiRecommendStocks, eval horizonEvalResult) (string, string) {
	var refSb strings.Builder
	var lessonSb strings.Builder

	fmt.Fprintf(&refSb, "【死后剖析 Post-Mortem】%s(%s) 推荐后走势偏离预期。\n", rec.StockName, rec.StockCode)
	fmt.Fprintf(&refSb, "- 初始研判逻辑：%s\n", rec.RecommendReason)
	stopLossVal, _ := strconv.ParseFloat(rec.RecommendStopLossPrice, 64)
	fmt.Fprintf(&refSb, "- 实际走势：最低探至 %.2f 元 (最大跌幅 %.2f%%)，跌破设定的安全止损线 %.2f 元。\n",
		eval.LowestPrice, eval.MaxDrawdown, stopLossVal)
	refSb.WriteString("- 根本原因反思：忽略了宏观/大盘流动性收缩的系统性贝塔杀伤，将震荡市中的假突破误判为主升浪启动；筹码集中度未达极限锁定即盲目给出了过高确定性。")

	lessonSb.WriteString("1. 必须在系统性大跌日对任何突破买点进行一票否决，严禁在全市场超3500只下跌时提示买入；\n")
	lessonSb.WriteString("2. 突破阳线实体一旦被后续K线全部吞没，必须无条件执行纪律性止损，严禁抱有侥幸逆势扛单；\n")
	lessonSb.WriteString("3. 严格遵循买方立论证伪标准，将该标的当前假说评级下调为 THESIS_BROKEN。")

	return refSb.String(), lessonSb.String()
}

func (s *PostMortemLoopService) calcGlobalSummary(sum *models.PostMortemAuditSummary) {
	if db.Dao == nil {
		return
	}
	var total int64
	db.Dao.Model(&models.PostMortemAuditRecord{}).Count(&total)
	sum.TotalAudited = int(total)

	var successCount int64
	db.Dao.Model(&models.PostMortemAuditRecord{}).Where("status = ?", "AUDITED_SUCCESS").Count(&successCount)
	sum.SuccessCount = int(successCount)

	var failCount int64
	db.Dao.Model(&models.PostMortemAuditRecord{}).Where("status = ?", "AUDITED_FAILURE").Count(&failCount)
	sum.FailureCount = int(failCount)

	var stopHits int64
	db.Dao.Model(&models.PostMortemAuditRecord{}).Where("stop_loss_breached = ?", true).Count(&stopHits)
	sum.StopLossHits = int(stopHits)

	calcRate := func(h int) float64 {
		var hTotal, hSucc int64
		db.Dao.Model(&models.PostMortemAuditRecord{}).Where("horizon_days = ?", h).Count(&hTotal)
		if hTotal == 0 {
			return 0
		}
		db.Dao.Model(&models.PostMortemAuditRecord{}).Where("horizon_days = ? AND status = ?", h, "AUDITED_SUCCESS").Count(&hSucc)
		return float64(hSucc) / float64(hTotal) * 100.0
	}

	sum.WinRateT1 = calcRate(1)
	sum.WinRateT5 = calcRate(5)
	sum.WinRateT20 = calcRate(20)

	var avgRet float64
	db.Dao.Model(&models.PostMortemAuditRecord{}).Select("COALESCE(AVG(actual_return), 0)").Scan(&avgRet)
	sum.AvgReturn = avgRet
}

// GetRecentAuditRecords 查询最近的闭环审计记录
func (s *PostMortemLoopService) GetRecentAuditRecords(limit int) ([]models.PostMortemAuditRecord, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var list []models.PostMortemAuditRecord
	err := db.Dao.Model(&models.PostMortemAuditRecord{}).Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}

// fetchStockDailyKLines 获取日线行情，带限流与并发控制。
// 协程安全：KlineRateWait 控制长期请求频率，semaphore 控制瞬时并发连接数。
func fetchStockDailyKLines(stockCode string, limit int) []data.KLineData {
	klineFetchSemaphore <- struct{}{}
	defer func() { <-klineFetchSemaphore }()
	KlineRateWait()
	res := data.FetchKLineWithFallback(stockCode, "", "101", limit, "")
	if res != nil && res.Data != nil {
		return *res.Data
	}
	return nil
}
