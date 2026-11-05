package models

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type AIResponseResult struct {
	gorm.Model
	ChatId    string                `json:"chatId"`
	ModelName string                `json:"modelName"`
	StockCode string                `json:"stockCode"`
	StockName string                `json:"stockName"`
	Question  string                `json:"question"`
	Content   string                `json:"content"`
	IsDel     soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

func (receiver AIResponseResult) TableName() string {
	return "ai_response_result"
}

// AIResponseResultQuery 分页查询参数
type AIResponseResultQuery struct {
	Page      int    `form:"page" json:"page"`           // 页码
	PageSize  int    `form:"pageSize" json:"pageSize"`   // 每页大小
	ChatId    string `form:"chatId" json:"chatId"`       // 聊天ID筛选
	ModelName string `form:"modelName" json:"modelName"` // 模型名称筛选
	StockCode string `form:"stockCode" json:"stockCode"` // 股票代码筛选
	StockName string `form:"stockName" json:"stockName"` // 股票名称筛选
	Question  string `form:"question" json:"question"`   // 问题内容模糊搜索
	StartDate string `form:"startDate" json:"startDate"` // 开始日期
	EndDate   string `form:"endDate" json:"endDate"`     // 结束日期
}

// AIResponseResultPageResp 分页查询响应
type AIResponseResultPageResp struct {
	Code    int                      `json:"code"`
	Message string                   `json:"message"`
	Data    AIResponseResultPageData `json:"data"`
}

type AIResponseResultPageData struct {
	List       []AIResponseResult `json:"list"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	TotalPages int                `json:"totalPages"`
}

// ============================================================================
// 回测准确度追踪的"个股报告"可验证性判断
// ============================================================================
//
// 背景：ai_response_result 中除个股分析外，还存在"市场分析"等泛化报告
// （StockCode 可能是 "市场分析"、"agent" 或空串），它们没有可回测的个股标的，
// 若混入回测/准确度统计会编造出虚假的预测准确度数据（如 id=35 类记录）。
// 因此所有进入回测准确度追踪的查询都必须过滤为"可回测的个股报告"。

var (
	reSixDigitStockCode = regexp.MustCompile(`^[0-9]{6}$`)
	rePrefixedStockCode = regexp.MustCompile(`^(?i:sh|sz|bj)[0-9]{6}$`)
	reHKFiveDigitCode   = regexp.MustCompile(`^[0-9]{5}$`)
	reUSTickerCode      = regexp.MustCompile(`^[A-Z]{1,6}$`)
)

// IsStockReportCode 判断 stockCode 是否为可回测的个股代码：
//   - 6 位数字：A股 / 场内基金
//   - sh/sz/bj 前缀 + 6 位数字：A股（大小写不限）
//   - 5 位数字：港股
//   - 1-6 位大写字母：美股代码
//
// 注意美股只认大写字母，避免把 "agent"、"market" 等伪代码误判为个股。
func CleanStockCode(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	matches := reSixDigitStockCode.FindStringSubmatch(raw)
	if len(matches) > 0 {
		return matches[0]
	}
	// 尝试从包含名称或后缀的字符串提取6位数字
	match := regexp.MustCompile(`([0-9]{6})`).FindStringSubmatch(raw)
	if len(match) > 1 {
		return match[1]
	}
	// 处理纯数字但丢失前导零（1~5位，如 2906 -> 002906）
	isDigits := true
	for _, c := range raw {
		if c < '0' || c > '9' {
			isDigits = false
			break
		}
	}
	if isDigits && len(raw) > 0 && len(raw) < 6 {
		return fmt.Sprintf("%06s", raw)
	}
	return raw
}

func IsStockReportCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	return reSixDigitStockCode.MatchString(code) ||
		rePrefixedStockCode.MatchString(code) ||
		reHKFiveDigitCode.MatchString(code) ||
		reUSTickerCode.MatchString(code)
}

// StockReportCodeSQL 生成用于 gorm 的 SQL WHERE 条件片段（不含 WHERE 关键字），
// 与 IsStockReportCode 语义一致，用于在数据库层面过滤出可回测的个股报告：
//
//	db.Where(StockReportCodeSQL()).Find(&reports)
func StockReportCodeSQL() string {
	return `(
stock_code GLOB '[0-9][0-9][0-9][0-9][0-9][0-9]'
OR stock_code GLOB 'SH[0-9][0-9][0-9][0-9][0-9][0-9]'
OR stock_code GLOB 'SZ[0-9][0-9][0-9][0-9][0-9][0-9]'
OR stock_code GLOB 'BJ[0-9][0-9][0-9][0-9][0-9][0-9]'
OR stock_code GLOB 'sh[0-9][0-9][0-9][0-9][0-9][0-9]'
OR stock_code GLOB 'sz[0-9][0-9][0-9][0-9][0-9][0-9]'
OR stock_code GLOB 'bj[0-9][0-9][0-9][0-9][0-9][0-9]'
OR (length(stock_code) = 5 AND stock_code GLOB '[0-9][0-9][0-9][0-9][0-9]')
OR (length(stock_code) BETWEEN 1 AND 6 AND stock_code GLOB '[A-Z][A-Z]*')
)`
}
