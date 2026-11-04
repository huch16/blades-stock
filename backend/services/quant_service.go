package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"
)

// QuantService 量化投研与回测服务
type QuantService struct {
	diagnoseMu   sync.Mutex
	diagnoseDone map[string]time.Time
}

// NewQuantService 创建量化服务实例
func NewQuantService() *QuantService {
	return &QuantService{
		diagnoseDone: make(map[string]time.Time),
	}
}

// DiagnoseStock 运行 3D 个股量化综合诊断（包含 30 秒幂等保护）
func (s *QuantService) DiagnoseStock(stockCode string) (*quant.StockDiagnosisResult, error) {
	clean := models.CleanStockCode(stockCode)
	if clean == "" {
		return nil, fmt.Errorf("无效股票代码: %s", stockCode)
	}

	// 快速检查：是否刚完成过（无锁读取）
	s.diagnoseMu.Lock()
	if t, ok := s.diagnoseDone[clean]; ok && time.Since(t) < 30*time.Second {
		s.diagnoseMu.Unlock()
		return nil, fmt.Errorf("诊断刚完成，请稍后重试（幂等保护）")
	}
	s.diagnoseMu.Unlock()

	res, err := quant.DiagnoseStock(clean)

	// 记录完成时间（无论成功失败，防止错误重试风暴）
	s.diagnoseMu.Lock()
	s.diagnoseDone[clean] = time.Now()

	// 定期清理过期键（每 100 次调用清理一次）
	if len(s.diagnoseDone)%100 == 0 {
		cutoff := time.Now().Add(-5 * time.Minute)
		for k, v := range s.diagnoseDone {
			if v.Before(cutoff) {
				delete(s.diagnoseDone, k)
			}
		}
	}
	s.diagnoseMu.Unlock()

	return res, err
}

// TriggerAIReportBacktest 手动或自动触发 AI 报告历史回测与胜率检验
func (s *QuantService) TriggerAIReportBacktest(limit int) (*quant.AIReportBacktestResult, error) {
	if limit <= 0 {
		limit = 50
	}
	mgr := quant.NewBacktestManager()
	return mgr.RunAIReportBacktest(context.Background(), limit)
}

// RunSkillABTest 运行双技能 A/B 测试对比检验
func (s *QuantService) RunSkillABTest(skillID1, skillID2 uint, limit int) (*quant.SkillABTestResult, error) {
	if limit <= 0 {
		limit = 50
	}
	mgr := quant.NewBacktestManager()
	return mgr.RunSkillABTest(context.Background(), skillID1, skillID2, limit)
}

// RunVectorBacktest 运行单次 VectorBT 回测
func (s *QuantService) RunVectorBacktest(stockCode, strategyType string, params map[string]any) map[string]any {
	resp, err := quant.VectorBTHandler(stockCode, strategyType, params)
	if err != nil {
		return map[string]any{"code": 0, "message": err.Error()}
	}
	return map[string]any{
		"code":    resp.Code,
		"message": resp.Message,
		"data": map[string]any{
			"report":   resp.Report,
			"stats":    resp.Stats,
			"strategy": resp.StrategyName,
			"params":   params,
		},
	}
}

// RunVectorGridSearch 运行 VectorBT 参数网格扫描
func (s *QuantService) RunVectorGridSearch(stockCode, strategyType string) map[string]any {
	resp, err := quant.VectorBTGridSearchHandler(stockCode, strategyType)
	if err != nil {
		return map[string]any{"code": 0, "message": err.Error()}
	}
	return map[string]any{
		"code":                  resp.Code,
		"message":               resp.Message,
		"total_combinations":    resp.TotalCombinations,
		"best_strategy":         resp.BestStrategy,
		"best_params":           resp.BestParams,
		"best_stats":            resp.BestStats,
		"duration":              resp.Duration,
		"in_sample_return":      resp.InSampleReturn,
		"out_sample_return":     resp.OutSampleReturn,
		"degradation_ratio":     resp.DegradationRatio,
		"overfitting_risk":      resp.OverfittingRisk,
		"train_test_split_desc": resp.TrainTestSplitDesc,
	}
}

// ScanHoldingRisk 执行盘前持仓确定性多维度风控扫描 (零 Token 纯 Go 计算)
func (s *QuantService) ScanHoldingRisk(items []quant.HoldingScanItem) (*quant.HoldingRiskScanResult, error) {
	return quant.ScanHoldingRisk(items)
}

// ScanWatchlistRisk 根据自选分组 ID 执行盘前持仓确定性风控扫描
func (s *QuantService) ScanWatchlistRisk(groupId int) (*quant.HoldingRiskScanResult, error) {
	followList := data.NewStockDataApi().GetFollowList(groupId)
	if followList == nil || len(*followList) == 0 {
		return quant.ScanHoldingRisk(nil)
	}
	var items []quant.HoldingScanItem
	for _, f := range *followList {
		profitRate := 0.0
		if f.CostPrice > 0 && f.Price > 0 {
			profitRate = (f.Price - f.CostPrice) / f.CostPrice * 100.0
		}
		items = append(items, quant.HoldingScanItem{
			StockCode:     f.StockCode,
			StockName:     f.Name,
			CurrentPrice:  f.Price,
			ChangePercent: f.ChangePercent,
			CostPrice:     f.CostPrice,
			CostVolume:    float64(f.Volume),
			ProfitRate:    profitRate,
		})
	}
	return quant.ScanHoldingRisk(items)
}
