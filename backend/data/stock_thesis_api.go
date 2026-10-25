package data

import (
	"regexp"
	"strings"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

type StockThesisApi struct{}

func NewStockThesisApi() *StockThesisApi {
	return &StockThesisApi{}
}

// NormalizeStockCode 统一股票代码格式（小写去空格）
func (api *StockThesisApi) NormalizeStockCode(code string) string {
	clean := strings.TrimSpace(strings.ToLower(code))
	return clean
}

// GetStockThesis 查询单只标的当前的买方认知假说与最新判定
func (api *StockThesisApi) GetStockThesis(stockCode string) (*models.StockThesisLedger, error) {
	if db.Dao == nil {
		return nil, nil
	}
	code := api.NormalizeStockCode(stockCode)
	if code == "" {
		return nil, nil
	}

	var ledger models.StockThesisLedger
	// 兼容带前缀 (sh600584) 与不带前缀 (600584) 的匹配
	pureNum := regexp.MustCompile(`\d+`).FindString(code)
	err := db.Dao.Model(&models.StockThesisLedger{}).
		Where("stock_code = ? OR stock_code LIKE ?", code, "%"+pureNum+"%").
		Order("updated_at desc").
		First(&ledger).Error

	if err != nil {
		return nil, err
	}
	return &ledger, nil
}

// GetStockThesisHistory 获取标的历史认知演化时间线
func (api *StockThesisApi) GetStockThesisHistory(stockCode string, limit int) ([]models.StockVerdictHistory, error) {
	if db.Dao == nil {
		return nil, nil
	}
	code := api.NormalizeStockCode(stockCode)
	if code == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	var history []models.StockVerdictHistory
	pureNum := regexp.MustCompile(`\d+`).FindString(code)
	err := db.Dao.Model(&models.StockVerdictHistory{}).
		Where("stock_code = ? OR stock_code LIKE ?", code, "%"+pureNum+"%").
		Order("created_at desc").
		Limit(limit).
		Find(&history).Error

	if err != nil {
		return nil, err
	}
	return history, nil
}

// GetAllStockTheses 获取所有有认知档案的标的映射表（代码 -> 认知账本）
func (api *StockThesisApi) GetAllStockTheses() (map[string]models.StockThesisLedger, error) {
	result := make(map[string]models.StockThesisLedger)
	if db.Dao == nil {
		return result, nil
	}

	var list []models.StockThesisLedger
	err := db.Dao.Model(&models.StockThesisLedger{}).Order("updated_at desc").Find(&list).Error
	if err != nil {
		return result, err
	}

	for _, item := range list {
		norm := api.NormalizeStockCode(item.StockCode)
		result[norm] = item
		pure := regexp.MustCompile(`\d+`).FindString(norm)
		if pure != "" && pure != norm {
			if _, exists := result[pure]; !exists {
				result[pure] = item
			}
		}
	}
	return result, nil
}

// SaveStockThesis 手动保存或调整认知假说
func (api *StockThesisApi) SaveStockThesis(thesis *models.StockThesisLedger) error {
	if db.Dao == nil || thesis == nil {
		return nil
	}
	thesis.StockCode = api.NormalizeStockCode(thesis.StockCode)
	if thesis.StockCode == "" {
		return nil
	}

	now := time.Now()
	var existing models.StockThesisLedger
	err := db.Dao.Where("stock_code = ?", thesis.StockCode).First(&existing).Error

	if err == nil && existing.ID > 0 {
		thesis.ID = existing.ID
		thesis.UpdatedAt = now
		if err := db.Dao.Save(thesis).Error; err != nil {
			return err
		}
	} else {
		thesis.CreatedAt = now
		thesis.UpdatedAt = now
		if err := db.Dao.Create(thesis).Error; err != nil {
			return err
		}
	}

	// 同时沉淀一条历史快照
	hist := models.StockVerdictHistory{
		StockCode:    thesis.StockCode,
		StockName:    thesis.StockName,
		Verdict:      thesis.Verdict,
		VerdictText:  thesis.VerdictText,
		CoreThesis:   thesis.CoreThesis,
		KeyCatalysts: thesis.KeyCatalysts,
		RiskFactors:  thesis.RiskFactors,
		ModelName:    thesis.ModelName,
		ReportID:     thesis.LastReportID,
		TriggerEvent: "手动调整/确认假说",
		CreatedAt:    now,
	}
	_ = db.Dao.Create(&hist)

	return nil
}

// CleanExtractText 清理抽取到的文本（去除 markdown 字符、列表符号、emoji 及冗余标点）
func (api *StockThesisApi) CleanExtractText(raw string) string {
	raw = strings.TrimSpace(raw)
	// 去除行首的引用号、破折号、星号、有序列表序号（例如 "1. ", "2) ", "- ", "> "）
	// 注意避免错误去除形如 "800G光模块" 或 "3D先进封装" 中的数字
	rePrefix := regexp.MustCompile(`^(?:[>\-*•\+]|\d+[\.\)）]|\[\s*\]|\s)+`)
	raw = rePrefix.ReplaceAllString(raw, "")
	// 剔除常见的判定状态 emoji 与修饰符
	reEmoji := regexp.MustCompile(`[🟢🔴🟡🔵🟠🎯✨🔥⚠️✅❌]`)
	raw = reEmoji.ReplaceAllString(raw, "")
	raw = strings.Trim(raw, "*_`'\"[]【】()（）:： \t\r\n")
	return strings.TrimSpace(raw)
}

// ExtractThesisFromContent 从 AI 研报中解析买方认知结论
func (api *StockThesisApi) ExtractThesisFromContent(content string) (verdict, verdictText, coreThesis, keyCatalysts, riskFactors string) {
	if strings.TrimSpace(content) == "" {
		return models.VerdictNoChange, "维持现状", "", "", ""
	}

	// 1. 尝试使用正则定位结构化模块
	// 匹配: - **本次判定**：[XXX] 或 本次判定: 🟢 逻辑强化 等多格式变体
	reVerdict := regexp.MustCompile(`(?i)(?:本次判定|判定结果|逻辑判定|定性判定|假说判定|投资结论|最终判定)[*：:\s]+[\[【（(>]*([^\n\r，。\]】）]+)`)
	if match := reVerdict.FindStringSubmatch(content); len(match) > 1 {
		raw := api.CleanExtractText(match[1])
		verdict, verdictText = api.mapVerdictCode(raw)
	}

	// 匹配核心逻辑
	reThesis := regexp.MustCompile(`(?i)(?:核心逻辑|投资假说|核心假说|驱动逻辑|核心投资逻辑|主要逻辑|核心驱动)[*：:\s]+([^\n\r]+)`)
	if match := reThesis.FindStringSubmatch(content); len(match) > 1 {
		coreThesis = api.CleanExtractText(match[1])
	}

	// 匹配验证催化
	reCatalyst := regexp.MustCompile(`(?i)(?:验证催化|关键催化|重点跟踪|催化剂|后续催化|跟踪催化|催化因素)[*：:\s]+([^\n\r]+)`)
	if match := reCatalyst.FindStringSubmatch(content); len(match) > 1 {
		keyCatalysts = api.CleanExtractText(match[1])
	}

	// 匹配证伪条件
	reRisk := regexp.MustCompile(`(?i)(?:证伪条件|核心风险|破坏条件|证伪指标|风险与证伪|证伪信号|风险提示)[*：:\s]+([^\n\r]+)`)
	if match := reRisk.FindStringSubmatch(content); len(match) > 1 {
		riskFactors = api.CleanExtractText(match[1])
	}

	// 2. 兜底判定识别
	if verdict == "" {
		switch {
		case strings.Contains(content, "逻辑强化") || strings.Contains(content, "强化逻辑") || strings.Contains(content, "多头强化"):
			verdict = models.VerdictThesisStrengthen
			verdictText = "逻辑强化"
		case strings.Contains(content, "逻辑削弱") || strings.Contains(content, "削弱逻辑") || strings.Contains(content, "动能衰减") || strings.Contains(content, "风险积聚"):
			verdict = models.VerdictThesisWeaken
			verdictText = "逻辑削弱"
		case strings.Contains(content, "逻辑证伪") || strings.Contains(content, "假说证伪") || strings.Contains(content, "彻底证伪") || strings.Contains(content, "逻辑打破"):
			verdict = models.VerdictThesisBroken
			verdictText = "逻辑证伪"
		case strings.Contains(content, "新论点") || strings.Contains(content, "首次建档") || strings.Contains(content, "首次覆盖") || strings.Contains(content, "新假说"):
			verdict = models.VerdictNewThesis
			verdictText = "新论点"
		default:
			verdict = models.VerdictNoChange
			verdictText = "维持现状"
		}
	}

	// 3. 核心逻辑若为空，从内容结论段落提取简述
	if coreThesis == "" {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "###") && (strings.Contains(trimmed, "结论") || strings.Contains(trimmed, "总结") || strings.Contains(trimmed, "研判")) {
				continue
			}
			if len(trimmed) > 15 && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "|") {
				coreThesis = trimmed
				if len(coreThesis) > 150 {
					coreThesis = coreThesis[:150] + "..."
				}
				break
			}
		}
	}

	return verdict, verdictText, coreThesis, keyCatalysts, riskFactors
}

func (api *StockThesisApi) mapVerdictCode(raw string) (string, string) {
	raw = api.CleanExtractText(raw)
	switch {
	case strings.Contains(raw, "强化"):
		return models.VerdictThesisStrengthen, "逻辑强化"
	case strings.Contains(raw, "削弱"):
		return models.VerdictThesisWeaken, "逻辑削弱"
	case strings.Contains(raw, "证伪") || strings.Contains(raw, "打破") || strings.Contains(raw, "破位"):
		return models.VerdictThesisBroken, "逻辑证伪"
	case strings.Contains(raw, "新论点") || strings.Contains(raw, "新假说") || strings.Contains(raw, "首次") || strings.Contains(raw, "建档"):
		return models.VerdictNewThesis, "新论点"
	default:
		return models.VerdictNoChange, "维持现状"
	}
}

// ExtractAndUpdateThesisFromReport 在单股研报生成后，异步解析认知结论并持久化沉淀
func (api *StockThesisApi) ExtractAndUpdateThesisFromReport(stockCode, stockName, content, modelName string, reportID uint) (*models.StockThesisLedger, error) {
	if db.Dao == nil {
		return nil, nil
	}
	code := api.NormalizeStockCode(stockCode)
	if code == "" || !models.IsStockReportCode(code) {
		return nil, nil
	}

	verdict, verdictText, coreThesis, keyCatalysts, riskFactors := api.ExtractThesisFromContent(content)

	now := time.Now()
	var ledger models.StockThesisLedger
	err := db.Dao.Where("stock_code = ?", code).First(&ledger).Error

	if err == nil && ledger.ID > 0 {
		ledger.StockName = stockName
		ledger.Verdict = verdict
		ledger.VerdictText = verdictText
		if coreThesis != "" {
			ledger.CoreThesis = coreThesis
		}
		if keyCatalysts != "" {
			ledger.KeyCatalysts = keyCatalysts
		}
		if riskFactors != "" {
			ledger.RiskFactors = riskFactors
		}
		ledger.ModelName = modelName
		ledger.LastReportID = reportID
		ledger.UpdatedAt = now
		if err := db.Dao.Save(&ledger).Error; err != nil {
			logger.SugaredLogger.Errorf("[Thesis] 保存更新标的认知档案失败 code=%s: %v", code, err)
			return nil, err
		}
	} else {
		ledger = models.StockThesisLedger{
			StockCode:    code,
			StockName:    stockName,
			Verdict:      verdict,
			VerdictText:  verdictText,
			CoreThesis:   coreThesis,
			KeyCatalysts: keyCatalysts,
			RiskFactors:  riskFactors,
			ModelName:    modelName,
			LastReportID: reportID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.Dao.Create(&ledger).Error; err != nil {
			logger.SugaredLogger.Errorf("[Thesis] 创建标的初始认知档案失败 code=%s: %v", code, err)
			return nil, err
		}
	}

	// 记录历史轨迹
	hist := models.StockVerdictHistory{
		StockCode:    code,
		StockName:    stockName,
		Verdict:      verdict,
		VerdictText:  verdictText,
		CoreThesis:   coreThesis,
		KeyCatalysts: keyCatalysts,
		RiskFactors:  riskFactors,
		ModelName:    modelName,
		ReportID:     reportID,
		TriggerEvent: "AI单股综合诊断",
		CreatedAt:    now,
	}
	if err := db.Dao.Create(&hist).Error; err != nil {
		logger.SugaredLogger.Warnf("[Thesis] 沉淀标的历史轨迹记录失败 code=%s: %v", code, err)
	} else {
		logger.SugaredLogger.Infof("[Thesis] 成功沉淀买方认知档案 code=%s [%s] verdict=%s", code, stockName, verdictText)
	}

	return &ledger, nil
}
