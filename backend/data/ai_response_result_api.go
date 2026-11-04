package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/datetime"
)

type AIResponseResultService struct{}

func NewAIResponseResultService() *AIResponseResultService {
	return &AIResponseResultService{}
}

// GetAIResponseResultList 分页查询AI响应结果
func (s *AIResponseResultService) GetAIResponseResultList(query models.AIResponseResultQuery) (*models.AIResponseResultPageData, error) {
	var list []models.AIResponseResult
	var total int64

	q := db.Dao.Model(&models.AIResponseResult{})

	// 构建查询条件
	if query.ChatId != "" || query.ModelName != "" || query.StockCode != "" || query.StockName != "" || query.Question != "" {
		sub := db.Dao.Where("1 = 0")
		if query.ChatId != "" {
			sub = sub.Or("chat_id LIKE ?", "%"+query.ChatId+"%")
		}
		if query.ModelName != "" {
			sub = sub.Or("model_name LIKE ?", "%"+query.ModelName+"%")
		}
		if query.StockCode != "" {
			sub = sub.Or("stock_code LIKE ?", "%"+query.StockCode+"%")
		}
		if query.StockName != "" {
			sub = sub.Or("stock_name LIKE ?", "%"+query.StockName+"%")
		}
		if query.Question != "" {
			sub = sub.Or("question LIKE ?", "%"+query.Question+"%")
		}
		q = q.Where(sub)
	}

	if query.StartDate != "" && query.EndDate != "" {
		parseDate := func(raw string) (time.Time, bool) {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return time.Time{}, false
			}
			// 支持常见时间格式
			layouts := []string{
				time.RFC3339,
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05",
				"2006-01-02",
			}
			for _, layout := range layouts {
				if t, err := time.Parse(layout, raw); err == nil {
					return t, true
				}
			}
			// 尝试前10字符作为 YYYY-MM-DD
			if len(raw) >= 10 {
				if t, err := time.Parse("2006-01-02", raw[:10]); err == nil {
					return t, true
				}
			}
			return time.Time{}, false
		}

		startDate, okStart := parseDate(query.StartDate)
		endDate, okEnd := parseDate(query.EndDate)
		if okStart && okEnd {
			q = q.Where("created_at BETWEEN ? AND ?", datetime.BeginOfDay(startDate), datetime.EndOfDay(endDate))
		}
	}

	// 计算总数
	err := q.Count(&total).Error
	if err != nil {
		return nil, err
	}

	// 设置默认分页参数
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	// 执行分页查询
	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.AIResponseResultPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// DeleteAIResponseResult 根据ID删除AI响应结果
func (s *AIResponseResultService) DeleteAIResponseResult(id uint) error {

	// 使用软删除
	result := db.Dao.Where("id = ?", id).Delete(&models.AIResponseResult{})

	return result.Error
}

// BatchDeleteAIResponseResult 批量删除AI响应结果
func (s *AIResponseResultService) BatchDeleteAIResponseResult(ids []uint) error {
	// 使用软删除
	result := db.Dao.Where("id IN ?", ids).Delete(&models.AIResponseResult{})

	return result.Error
}
