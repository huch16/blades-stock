package agent

import (
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"fmt"
	"strings"
)

// AnalysisMemoryService 加载历史 AI 分析报告并转化为智能体记忆
type AnalysisMemoryService struct{}

func NewAnalysisMemoryService() *AnalysisMemoryService {
	return &AnalysisMemoryService{}
}

// LoadRelevantReports 查询与当前问题相关的历史分析报告
// 注意：未提供股票代码/名称（通用性问题，如"推荐ETF"）时不注入个股记忆，
// 否则会把最近一次个股分析报告错误地塞给无关问题，导致答非所问。
func (s *AnalysisMemoryService) LoadRelevantReports(stockCode, stockName string, limit int) []models.AIResponseResult {
	if limit <= 0 {
		limit = 5
	}
	if stockCode == "" && stockName == "" {
		return nil
	}
	var results []models.AIResponseResult
	q := db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where(models.StockReportCodeSQL())

	if stockCode != "" {
		q = q.Where("stock_code = ?", stockCode)
	}
	if stockName != "" {
		q = q.Where("stock_name LIKE ?", "%"+stockName+"%")
	}

	q.Order("id desc").Limit(limit).Find(&results)

	return results
}

// BuildMemoryContext 将多份分析报告压缩成一段记忆上下文文本
func (s *AnalysisMemoryService) BuildMemoryContext(reports []models.AIResponseResult) string {
	if len(reports) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("【历史分析记忆】以下是你过去对相关股票的分析判断，回顾它们有助于保持判断一致性：\n\n")

	for i, r := range reports {
		summary := r.Content
		if len([]rune(summary)) > 500 {
			runes := []rune(summary)
			summary = string(runes[:500]) + "…（以下省略）"
		}
		dateStr := r.CreatedAt.Format("2006-01-02")
		b.WriteString(fmt.Sprintf("【记忆 %d】%s | %s(%s) | %s\n", i+1, dateStr, r.StockName, r.StockCode, r.Question))
		b.WriteString(summary)
		b.WriteString("\n\n")
	}

	b.WriteString("（以上记忆由系统自动提取，供你参考。若与当前查询无关可忽略。）")
	return b.String()
}

// BuildAutoInjectContext 一键完成查询 + 压缩
func (s *AnalysisMemoryService) BuildAutoInjectContext(stockCode, stockName string) string {
	reports := s.LoadRelevantReports(stockCode, stockName, 5)
	return s.BuildMemoryContext(reports)
}
