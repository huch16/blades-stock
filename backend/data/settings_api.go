package data

import (
	"encoding/csv"
	"encoding/json"
	"database/sql"
	"errors"
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/paths"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

type Settings struct {
	gorm.Model
	TushareToken           string `json:"tushareToken"`
	LocalPushEnable        bool   `json:"localPushEnable"`
	DingPushEnable         bool   `json:"dingPushEnable"`
	DingRobot              string `json:"dingRobot"`
	UpdateBasicInfoOnStart bool   `json:"updateBasicInfoOnStart"`
	RefreshInterval        int64  `json:"refreshInterval"`
	OpenAiEnable           bool   `json:"openAiEnable"`
	AiConfigID             uint   `json:"aiConfigId" gorm:"column:ai_config_id;default:0"`
	Prompt                 string `json:"prompt"`
	CheckUpdate            bool   `json:"checkUpdate"`
	UpdateChannel          string `json:"updateChannel"`
	QuestionTemplate       string `json:"questionTemplate"`
	CrawlTimeOut           int64  `json:"crawlTimeOut"`
	KDays                  int64  `json:"kDays"`
	EnableDanmu            bool   `json:"enableDanmu"`
	BrowserPath            string `json:"browserPath"`
	EnableNews             bool   `json:"enableNews"`
	DarkTheme              bool   `json:"darkTheme"`
	EnableFund             bool   `json:"enableFund"`
	EnablePushNews         bool   `json:"enablePushNews"`
	EnableOnlyPushRedNews  bool   `json:"enableOnlyPushRedNews"`
	HttpProxy              string `json:"httpProxy"`
	HttpProxyEnabled       bool   `json:"httpProxyEnabled"`
	EnableAgent            bool   `json:"enableAgent"`
	DisableBrowserSearch   bool   `json:"disableBrowserSearch" gorm:"column:disable_browser_search;default:false"`
	QgqpBId                string `json:"qgqpBId" gorm:"column:qgqp_b_id"`
	TickFlowAPIKey        string `json:"tickFlowAPIKey" gorm:"column:tickflow_api_key"`
	EmApiKey               string `json:"emApiKey" gorm:"column:em_api_key"`
	WindowWidth            int    `json:"windowWidth"`
	WindowHeight           int    `json:"windowHeight"`
	PromptPlazaApiBase     string  `json:"promptPlazaApiBase" gorm:"column:prompt_plaza_api_base"`
	BacktestAccuracyThreshold float64 `json:"backtestAccuracyThreshold" gorm:"column:backtest_accuracy_threshold;default:5.0"`
	BacktestVerifyDays        int64   `json:"backtestVerifyDays" gorm:"column:backtest_verify_days;default:2"`
	BacktestEnabled           bool    `json:"backtestEnabled" gorm:"column:backtest_enabled;default:true"`
	AkshareServiceUrl         string  `json:"akshareServiceUrl" gorm:"column:akshare_service_url;default:http://115.190.72.22:8089"`
	ADataServiceUrl           string  `json:"aDataServiceUrl" gorm:"column:adata_service_url;default:http://115.190.72.22:8000"`
	DefaultDataSource         string  `json:"defaultDataSource" gorm:"column:default_data_source;default:adata"`
	EmbeddingEnabled          bool    `json:"embeddingEnabled" gorm:"column:embedding_enabled;default:false"`
	EmbeddingModelName        string  `json:"embeddingModelName" gorm:"column:embedding_model_name"`
	EmbeddingBaseUrl          string  `json:"embeddingBaseUrl" gorm:"column:embedding_base_url"`
	EmbeddingApiKey           string  `json:"embeddingApiKey" gorm:"column:embedding_api_key"`
	McpTimeout                int64   `json:"mcpTimeout" gorm:"column:mcp_timeout;default:8"`
	ToolRunBudget             int64   `json:"toolRunBudget" gorm:"column:tool_run_budget;default:15"`
	ToolDailyBudget           int64   `json:"toolDailyBudget" gorm:"column:tool_daily_budget;default:200"`
	ToolDailyCostUSD          float64 `json:"toolDailyCostUSD" gorm:"column:tool_daily_cost_usd;default:2.0"`
}

func (receiver Settings) TableName() string {
	return "settings"
}

type AIConfig struct {
	ID                 uint `gorm:"primarykey"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Name               string  `json:"name"`
	BaseUrl            string  `json:"baseUrl"`
	ApiKey             string  `json:"apiKey" `
	ModelName          string  `json:"modelName"`
	MaxTokens          int     `json:"maxTokens"`
	Temperature        float64 `json:"temperature"`
	TimeOut            int     `json:"timeOut"`
	HttpProxy          string  `json:"httpProxy"`
	HttpProxyEnabled   bool    `json:"httpProxyEnabled"`
	SessionId          string  `json:"sessionId" gorm:"index;size:64"`
	Thinking           bool    `json:"thinking"`
	EmbeddingEnabled   bool    `json:"embeddingEnabled"`
	EmbeddingModelName string  `json:"embeddingModelName"`
	EmbeddingBaseUrl   string  `json:"embeddingBaseUrl"`
	LLMConcurrencyLimit int    `json:"llmConcurrencyLimit"` // 并发限制，默认 2
	SubAgentTimeout    int     `json:"subAgentTimeout"`    // 子Agent超时秒，默认 20
	// Rate limiting and quota management
	RateLimitPerSecond float64 `json:"rateLimitPerSecond"` // 每秒最大请求数，0 表示不限制
	Burst              int     `json:"burst"`              // 令牌桶突发大小，0 表示使用默认值
	DailyRequestQuota  int     `json:"dailyRequestQuota"`  // 每日最大请求数，0 表示不限制
	DailyCostQuota     float64 `json:"dailyCostQuota"`     // 每日最大成本（USD），0 表示不限制
}

func (AIConfig) TableName() string {
	return "ai_config"
}

func (c *AIConfig) SetDefaults() {
	if c.MaxTokens <= 0 {
		c.MaxTokens = 4096
	}
	if c.Temperature <= 0 {
		c.Temperature = 0.7
	}
	if c.TimeOut <= 0 {
		c.TimeOut = 300
	}
	if c.LLMConcurrencyLimit <= 0 {
		c.LLMConcurrencyLimit = 2
	}
	if c.SubAgentTimeout <= 0 {
		c.SubAgentTimeout = 20
	}
	// Rate limiting and quota defaults: 0 means unlimited
	if c.RateLimitPerSecond < 0 {
		c.RateLimitPerSecond = 0
	}
	if c.Burst < 0 {
		c.Burst = 0
	}
	if c.DailyRequestQuota < 0 {
		c.DailyRequestQuota = 0
	}
	if c.DailyCostQuota < 0 {
		c.DailyCostQuota = 0
	}
}

type SettingConfig struct {
	*Settings
	AiConfigs []*AIConfig `json:"aiConfigs"`
}

func (c *SettingConfig) GetAIConfigThinking(aiConfigId int) bool {
	if aiConfigId <= 0 && len(c.AiConfigs) > 0 {
		return c.AiConfigs[0].Thinking
	}
	for _, cfg := range c.AiConfigs {
		if int(cfg.ID) == aiConfigId {
			return cfg.Thinking
		}
	}
	return false
}

// GetMcpTimeout 获取 MCP 客户端超时时间，优先读取环境变量 MCP_TIMEOUT_SECONDS，次选 Settings.McpTimeout，兜底 8s
func (c *SettingConfig) GetMcpTimeout() time.Duration {
	if env := os.Getenv("MCP_TIMEOUT_SECONDS"); env != "" {
		if s, err := strconv.Atoi(env); err == nil && s > 0 {
			return time.Duration(s) * time.Second
		}
	}
	if c != nil && c.Settings != nil && c.McpTimeout > 0 {
		return time.Duration(c.McpTimeout) * time.Second
	}
	return 8 * time.Second
}

func (c *SettingConfig) GetToolRunBudget() int64 {
	if env := os.Getenv("AGENT_TOOL_RUN_BUDGET"); env != "" {
		if v, err := strconv.ParseInt(env, 10, 64); err == nil && v > 0 {
			return v
		}
	}
	if c != nil && c.Settings != nil && c.ToolRunBudget > 0 {
		return c.ToolRunBudget
	}
	return 15
}

func (c *SettingConfig) GetToolDailyBudget() int64 {
	if env := os.Getenv("AGENT_TOOL_DAILY_BUDGET"); env != "" {
		if v, err := strconv.ParseInt(env, 10, 64); err == nil && v > 0 {
			return v
		}
	}
	if c != nil && c.Settings != nil && c.ToolDailyBudget > 0 {
		return c.ToolDailyBudget
	}
	return 200
}

func (c *SettingConfig) GetToolDailyCostUSD() float64 {
	if env := os.Getenv("AGENT_TOOL_DAILY_COST_USD"); env != "" {
		if v, err := strconv.ParseFloat(env, 64); err == nil && v > 0 {
			return v
		}
	}
	if c != nil && c.Settings != nil && c.ToolDailyCostUSD > 0 {
		return c.ToolDailyCostUSD
	}
	return 2.0
}

type SettingsApi struct {
	Config *SettingConfig
}

func NewSettingsApi() *SettingsApi {
	return &SettingsApi{
		Config: GetSettingConfig(),
	}
}

// CleanupTypes 定义支持的清理类型

// BackupTypes 定义支持的数据备份类型
const (
	BackupFollowedStocks     = "followed_stocks"
	BackupRecommendStocks    = "recommend_stocks"
	BackupAIReports          = "ai_reports"
	BackupFollowedFunds      = "followed_funds"
	BackupBacktestResults    = "backtest_results"
	BackupStockChangeHistory = "stock_change_history"
)
const (
	CleanupLogs       = "logs"
	CleanupCache      = "cache"
	CleanupChat       = "chat_sessions"
	CleanupAIResults  = "ai_results"
	CleanupTestImages = "test_images"
	CleanupBrowserTemp = "browser_temp"
	CleanupAll        = "all"
)

// CleanupData 清理应用产生的中间/临时/历史数据
func (s *SettingsApi) CleanupData(cleanupType string) string {
	logger.SugaredLogger.Infof("Cleaning up data type: %s", cleanupType)

	cleanupFuncs := map[string]func() string{
		CleanupLogs:       cleanupLogs,
		CleanupCache:      cleanupCache,
		CleanupChat:       cleanupChatSessions,
		CleanupAIResults:  cleanupAIResults,
		CleanupTestImages: cleanupTestImages,
		CleanupBrowserTemp: cleanupBrowserTemp,
	}

	if cleanupType == CleanupAll {
		var results []string
		for _, key := range []string{CleanupLogs, CleanupCache, CleanupChat, CleanupAIResults, CleanupTestImages, CleanupBrowserTemp} {
			if fn := cleanupFuncs[key]; fn != nil {
				results = append(results, fn())
			}
		}
		return strings.Join(results, "\n")
	}

	fn, ok := cleanupFuncs[cleanupType]
	if !ok {
		return "未知清理类型: " + cleanupType
	}
	return fn()
}

func cleanupLogs() string {
	logDir := "logs"
	files, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "日志目录不存在，跳过"
		}
		return "清理日志失败: " + err.Error()
	}
	var removed int
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".log") {
			path := filepath.Join(logDir, f.Name())
			if err := os.Truncate(path, 0); err != nil {
				logger.SugaredLogger.Warnf("truncate log %s failed: %v", path, err)
			} else {
				removed++
			}
		}
	}
	return fmt.Sprintf("已清理 %d 个日志文件", removed)
}

func cleanupCache() string {
	return "缓存清理已注册到应用层，请使用设置页清理按钮或重启应用"
}

func cleanupChatSessions() string {
	result := db.Dao.Exec("DELETE FROM ai_assistant_sessions")
	if result.Error != nil {
		return "清理AI对话失败: " + result.Error.Error()
	}
	return fmt.Sprintf("已清理 %d 条AI对话记录", result.RowsAffected)
}

func cleanupAIResults() string {
	result := db.Dao.Exec("DELETE FROM ai_response_result")
	if result.Error != nil {
		return "清理AI分析结果失败: " + result.Error.Error()
	}
	return fmt.Sprintf("已清理 %d 条AI分析结果", result.RowsAffected)
}

func cleanupTestImages() string {
	dir := "backend/data/test_images"
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "测试图片目录不存在，跳过"
		}
		return "清理测试图片失败: " + err.Error()
	}
	var removed int
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		path := filepath.Join(dir, f.Name())
		if err := os.Remove(path); err != nil {
			logger.SugaredLogger.Warnf("remove test image %s failed: %v", path, err)
		} else {
			removed++
		}
	}
	return fmt.Sprintf("已清理 %d 个测试图片", removed)
}

func cleanupBrowserTemp() string {
	tempDir := os.TempDir()
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "清理浏览器临时目录失败: " + err.Error()
	}
	var removed int
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "chromedp") {
			path := filepath.Join(tempDir, e.Name())
			if err := os.RemoveAll(path); err != nil {
				logger.SugaredLogger.Warnf("remove chromedp temp dir %s failed: %v", path, err)
			} else {
				removed++
			}
		}
	}
	return fmt.Sprintf("已清理 %d 个浏览器临时目录", removed)
}
func backupDirPath() string {
	return filepath.Join(paths.BaseDir(), "backups")
}

// BackupData 导出用户数据到 CSV，返回生成的文件路径
func (s *SettingsApi) BackupData(backupType string) string {
	logger.SugaredLogger.Infof("Backing up data type: %s", backupType)

	backupDir := backupDirPath()
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "创建备份目录失败: " + err.Error()
	}

	ts := time.Now().Format("20060102_150405")
	fileName := filepath.Join(backupDir, fmt.Sprintf("%s_%s.csv", backupType, ts))

	var result string
	switch backupType {
	case BackupFollowedStocks:
		result = backupFollowedStocks(fileName)
	case BackupRecommendStocks:
		result = backupRecommendStocks(fileName)
	case BackupAIReports:
		result = backupAIReports(fileName)
	case BackupFollowedFunds:
		result = backupFollowedFunds(fileName)
	case BackupBacktestResults:
		result = backupBacktestResults(fileName)
	case BackupStockChangeHistory:
		result = backupStockChangeHistory(fileName)
	default:
		return "未知备份类型: " + backupType
	}
	return fmt.Sprintf("%s\n备份目录: %s", result, backupDir)
}

// RestoreData 从 CSV 文件恢复数据到数据库
func (s *SettingsApi) RestoreData(backupType string) string {
	logger.SugaredLogger.Infof("Restoring data type: %s", backupType)

	backupDir := backupDirPath()
	pattern := filepath.Join(backupDir, backupType+"_*.csv")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "查找备份文件失败: " + err.Error()
	}
	if len(matches) == 0 {
		return "未找到 " + backupType + " 的备份文件，请先执行备份\n备份目录: " + backupDir
	}

	// 使用最新的文件
	fileName := matches[len(matches)-1]

	switch backupType {
	case BackupFollowedStocks:
		return restoreFollowedStocks(fileName)
	case BackupRecommendStocks:
		return restoreRecommendStocks(fileName)
	case BackupAIReports:
		return restoreAIReports(fileName)
	case BackupFollowedFunds:
		return restoreFollowedFunds(fileName)
	case BackupBacktestResults:
		return restoreBacktestResults(fileName)
	case BackupStockChangeHistory:
		return restoreStockChangeHistory(fileName)
	default:
		return "未知恢复类型: " + backupType
	}
}

func backupFollowedStocks(fileName string) string {
	rows, err := db.Dao.Raw("SELECT stock_code, name, volume, cost_price, price, change_percent, alarm_change_percent, alarm_price, time, sort, ai_config_id, entry_price, take_profit_price, stop_loss_price FROM followed_stock WHERE is_del=0 ORDER BY sort, id").Rows()
	if err != nil {
		return "导出自选股票失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"股票代码", "名称", "持仓量", "成本价", "当前价", "涨跌幅%", "预警涨跌幅%", "预警价", "时间", "排序", "AI配置ID", "入场价", "止盈价", "止损价"})
}

func backupRecommendStocks(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, data_time, model_name, rating, stock_code, stock_name, bk_name, stock_price, stock_current_price, stock_current_price_time, stock_close_price, stock_pre_price, recommend_reason, recommend_buy_price, recommend_buy_price_min, recommend_buy_price_max, recommend_stop_profit_price, recommend_stop_profit_price_min, recommend_stop_profit_price_max, recommend_stop_loss_price, risk_remarks, remarks, enable_alert FROM ai_recommend_stocks ORDER BY id DESC").Rows()
	if err != nil {
		return "导出推荐股票失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "数据时间", "模型", "评级", "股票代码", "股票名称", "板块", "推荐价", "当前价", "当前价时间", "收盘价", "昨收", "推荐理由", "建议买入价", "建议买入最低价", "建议买入最高价", "建议止盈价", "建议止盈最低价", "建议止盈最高价", "建议止损价", "风险提示", "备注", "启用提醒"})
}

func backupAIReports(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, created_at, model_name, stock_code, stock_name, question, content, is_del FROM ai_response_result ORDER BY id DESC").Rows()
	if err != nil {
		return "导出AI分析报告失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "创建时间", "模型", "股票代码", "股票名称", "问题", "内容", "是否删除"})
}

func backupFollowedFunds(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, created_at, code, name, net_unit_value, net_unit_value_date, net_estimated_unit, net_estimated_time, net_accumulated, net_estimated_rate, net_unit_value_prev, net_actual_rate FROM followed_fund ORDER BY id DESC").Rows()
	if err != nil {
		return "导出关注基金失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "创建时间", "基金代码", "基金名称", "单位净值", "净值日期", "估算净值", "估算时间", "累计净值", "估算涨跌幅", "昨日净值", "实际涨跌幅"})
}

func backupBacktestResults(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, created_at, report_id, stock_code, stock_name, analysis_date, direction, target_price, verify_date, verify_price, price_change, max_drawdown, max_upside, is_accurate, confidence, summary FROM backtest_results ORDER BY id DESC").Rows()
	if err != nil {
		return "导出回测结果失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "创建时间", "报告ID", "股票代码", "股票名称", "分析日期", "方向", "目标价", "验证日期", "验证价", "价格变化", "最大回撤", "最大上涨", "是否准确", "置信度", "摘要"})
}

func backupStockChangeHistory(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, change_date, change_time, stock_code, stock_name, market, change_type, type_name, volume, price, change_rate, amount, industry, concept FROM stock_change_history ORDER BY id DESC LIMIT 10000").Rows()
	if err != nil {
		return "导出异动历史失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "日期", "时间", "股票代码", "股票名称", "市场", "异动类型", "类型名", "成交量", "价格", "涨跌幅%", "金额", "行业", "概念"})
}

func dumpRowsToCSV(fileName string, rows *sql.Rows, headers []string) string {
	f, err := os.Create(fileName)
	if err != nil {
		return "创建备份文件失败: " + err.Error()
	}
	defer f.Close()

	// UTF-8 BOM for Excel
	f.Write([]byte{0xEF, 0xBB, 0xBF})

	columns, err := rows.Columns()
	if err != nil {
		return "读取列信息失败: " + err.Error()
	}

	w := csv.NewWriter(f)
	w.Write(headers)

	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var count int
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			logger.SugaredLogger.Warnf("scan row failed: %v", err)
			continue
		}
		record := make([]string, len(columns))
		for i, v := range values {
			record[i] = fmt.Sprintf("%v", v)
		}
		w.Write(record)
		count++
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "写入CSV失败: " + err.Error()
	}
	return fmt.Sprintf("已备份 %d 条记录到 %s", count, fileName)
}

func readCSV(fileName string) ([]map[string]string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	allRows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(allRows) < 2 {
		return nil, nil
	}
	headers := allRows[0]
	var records []map[string]string
	for _, row := range allRows[1:] {
		rec := make(map[string]string)
		for i, h := range headers {
			if i < len(row) {
				rec[h] = row[i]
			}
		}
		records = append(records, rec)
	}
	return records, nil
}

func restoreFollowedStocks(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO followed_stock (stock_code, name, volume, cost_price, price, change_percent, alarm_change_percent, alarm_price, time, sort, ai_config_id, entry_price, take_profit_price, stop_loss_price) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			r["股票代码"], r["名称"], r["持仓量"], r["成本价"], r["当前价"], r["涨跌幅%"], r["预警涨跌幅%"], r["预警价"], r["时间"], r["排序"], r["AI配置ID"], r["入场价"], r["止盈价"], r["止损价"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复自选股票失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条自选股票记录", len(records))
}

func restoreRecommendStocks(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO ai_recommend_stocks (id, data_time, model_name, rating, stock_code, stock_name, bk_name, stock_price, stock_current_price, stock_current_price_time, stock_close_price, stock_pre_price, recommend_reason, recommend_buy_price, recommend_buy_price_min, recommend_buy_price_max, recommend_stop_profit_price, recommend_stop_profit_price_min, recommend_stop_profit_price_max, recommend_stop_loss_price, risk_remarks, remarks, enable_alert) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			r["ID"], r["数据时间"], r["模型"], r["评级"], r["股票代码"], r["股票名称"], r["板块"], r["推荐价"], r["当前价"], r["当前价时间"], r["收盘价"], r["昨收"], r["推荐理由"], r["建议买入价"], r["建议买入最低价"], r["建议买入最高价"], r["建议止盈价"], r["建议止盈最低价"], r["建议止盈最高价"], r["建议止损价"], r["风险提示"], r["备注"], r["启用提醒"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复推荐股票失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条推荐股票记录", len(records))
}

func restoreAIReports(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO ai_response_result (id, created_at, model_name, stock_code, stock_name, question, content, is_del) VALUES (?,?,?,?,?,?,?,?)",
			r["ID"], r["创建时间"], r["模型"], r["股票代码"], r["股票名称"], r["问题"], r["内容"], r["是否删除"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复AI分析报告失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条AI分析报告", len(records))
}

func restoreFollowedFunds(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO followed_fund (id, created_at, code, name, net_unit_value, net_unit_value_date, net_estimated_unit, net_estimated_time, net_accumulated, net_estimated_rate, net_unit_value_prev, net_actual_rate) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
			r["ID"], r["创建时间"], r["基金代码"], r["基金名称"], r["单位净值"], r["净值日期"], r["估算净值"], r["估算时间"], r["累计净值"], r["估算涨跌幅"], r["昨日净值"], r["实际涨跌幅"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复关注基金失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条关注基金记录", len(records))
}

func restoreBacktestResults(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO backtest_results (id, created_at, report_id, stock_code, stock_name, analysis_date, direction, target_price, verify_date, verify_price, price_change, max_drawdown, max_upside, is_accurate, confidence, summary) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			r["ID"], r["创建时间"], r["报告ID"], r["股票代码"], r["股票名称"], r["分析日期"], r["方向"], r["目标价"], r["验证日期"], r["验证价"], r["价格变化"], r["最大回撤"], r["最大上涨"], r["是否准确"], r["置信度"], r["摘要"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复回测结果失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条回测结果记录", len(records))
}

func restoreStockChangeHistory(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO stock_change_history (id, change_date, change_time, stock_code, stock_name, market, change_type, type_name, volume, price, change_rate, amount, industry, concept) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			r["ID"], r["日期"], r["时间"], r["股票代码"], r["股票名称"], r["市场"], r["异动类型"], r["类型名"], r["成交量"], r["价格"], r["涨跌幅%"], r["金额"], r["行业"], r["概念"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复异动历史失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条异动历史记录", len(records))
}

func (s *SettingsApi) Export() string {
	d, err := json.MarshalIndent(s.Config, "", "    ")
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置序列化失败: %v", err)
		return ""
	}
	return string(d)
}

// AiConfigTemplate 预设配置模板，用于向导式配置
// 字段说明：
// - Type: "local" | "cloud" | "enterprise"
// - Label: 显示名称
// - Description: 简短说明
// - BaseUrl/ModelName/ApiKeyRequired: 预填字段
// - EnvVars: 需要用户配置的环境变量/密钥提示
type AiConfigTemplate struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Description string `json:"description"`
	BaseUrl     string `json:"baseUrl"`
	ModelName   string `json:"modelName"`
	ApiKeyRequired bool  `json:"apiKeyRequired"`
	EnvHint     string `json:"envHint"`
}

// GetAiConfigTemplates 返回预设的 AI 配置模板，供前端向导使用
func (s *SettingsApi) GetAiConfigTemplates() []*AiConfigTemplate {
	return []*AiConfigTemplate{
		{
			Type:           "local",
			Label:          "Ollama 本地部署",
			Description:    "在本机运行的开源大模型（如 llama3, qwen2, deepseek-r1 等），无需 API Key，数据不出本地",
			BaseUrl:        "http://localhost:11434/v1",
			ModelName:      "",
			ApiKeyRequired: false,
			EnvHint:        "需先安装 Ollama 并拉取模型：`ollama pull qwen2:7b`",
		},
		{
			Type:           "local",
			Label:          "LM Studio 本地部署",
			Description:    "图形化本地大模型运行工具，兼容 OpenAI 接口",
			BaseUrl:        "http://localhost:1234/v1",
			ModelName:      "",
			ApiKeyRequired: false,
			EnvHint:        "在 LM Studio 中启动本地服务器（默认端口 1234）",
		},
		{
			Type:           "cloud",
			Label:          "DeepSeek 官方 API",
			Description:    "高性价比云端推理，支持 deepseek-chat / deepseek-reasoner",
			BaseUrl:        "https://api.deepseek.com",
			ModelName:      "deepseek-reasoner",
			ApiKeyRequired: true,
			EnvHint:        "在 https://platform.deepseek.com 获取 API Key",
		},
		{
			Type:           "cloud",
			Label:          "硅基流动 SiliconFlow",
			Description:    "国产模型托管平台，支持 Qwen、GLM、Yi 等多种模型",
			BaseUrl:        "https://api.siliconflow.cn/v1",
			ModelName:      "Qwen/Qwen2.5-7B-Instruct",
			ApiKeyRequired: true,
			EnvHint:        "在 https://cloud.siliconflow.cn 获取 API Key",
		},
		{
			Type:           "cloud",
			Label:          "智谱 AI (GLM)",
			Description:    "国产大模型厂商，GLM-4 系列性能优秀",
			BaseUrl:        "https://open.bigmodel.cn/api/paas/v4",
			ModelName:      "glm-4-flash",
			ApiKeyRequired: true,
			EnvHint:        "在 https://open.bigmodel.cn 获取 API Key",
		},
		{
			Type:           "cloud",
			Label:          "字节豆包 (火山引擎)",
			Description:    "字节跳动旗下模型服务，Doubao 系列模型",
			BaseUrl:        "https://ark.cn-beijing.volces.com/api/v3",
			ModelName:      "doubao-pro-32k",
			ApiKeyRequired: true,
			EnvHint:        "在 https://console.volcengine.com/ark 获取 API Key 与 Endpoint ID",
		},
		{
			Type:           "cloud",
			Label:          "阿里云百炼",
			Description:    "阿里云模型服务，Qwen 系列全谱系",
			BaseUrl:        "https://dashscope.aliyuncs.com/compatible-mode/v1",
			ModelName:      "qwen-plus",
			ApiKeyRequired: true,
			EnvHint:        "在 https://dashscope.aliyuncs.com 获取 API Key",
		},
		{
			Type:           "cloud",
			Label:          "OpenAI 官方",
			Description:    "GPT-4o / GPT-4o-mini 等顶级模型",
			BaseUrl:        "https://api.openai.com/v1",
			ModelName:      "gpt-4o-mini",
			ApiKeyRequired: true,
			EnvHint:        "在 https://platform.openai.com 获取 API Key",
		},
		{
			Type:           "cloud",
			Label:          "OpenRouter (聚合)",
			Description:    "统一接口调用 300+ 模型，适合多模型对比",
			BaseUrl:        "https://openrouter.ai/api/v1",
			ModelName:      "anthropic/claude-3.5-sonnet",
			ApiKeyRequired: true,
			EnvHint:        "在 https://openrouter.ai 获取 API Key",
		},
		{
			Type:           "enterprise",
			Label:          "企业内网/私有化部署",
			Description:    "企业内网部署的兼容 OpenAI 接口服务（如 vLLM, TGI, Xinference 等）",
			BaseUrl:        "",
			ModelName:      "",
			ApiKeyRequired: true,
			EnvHint:        "请咨询运维获取内网 API 地址与密钥",
		},
	}
}

func validateAIConfig(cfg *AIConfig) string {
	if strings.TrimSpace(cfg.Name) == "" {
		return "AI 配置名称不能为空"
	}
	if strings.TrimSpace(cfg.BaseUrl) == "" {
		return "AI 配置 Base URL 不能为空"
	}
	if strings.TrimSpace(cfg.ModelName) == "" {
		return "AI 配置模型名称不能为空"
	}

	// 本地模型或特定网络端点允许 API Key 为空，远程 API 若为空自动填充默认占位符 "none" 避免报错
	if strings.TrimSpace(cfg.ApiKey) == "" {
		cfg.ApiKey = "none"
	}

	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		return "Temperature 必须在 0~2 之间"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	} else if cfg.MaxTokens > 256000 {
		return "MaxTokens 不能超过 256000"
	}

	if cfg.TimeOut <= 0 {
		cfg.TimeOut = 300
	} else if cfg.TimeOut < 5 {
		return "TimeOut 必须大于 5 秒"
	}

	if cfg.EmbeddingEnabled && strings.TrimSpace(cfg.EmbeddingModelName) == "" {
		return "Embedding 已启用但未配置模型名称"
	}
	return ""
}

func UpdateConfig(s *SettingConfig) string {
	if s.Settings == nil {
		return "保存失败: 配置数据为空"
	}
	for i, cfg := range s.AiConfigs {
		if msg := validateAIConfig(cfg); msg != "" {
			return fmt.Sprintf("保存失败: 第 %d 项 %s", i+1, msg)
		}
	}
	if s.EmbeddingEnabled && strings.TrimSpace(s.EmbeddingModelName) == "" {
		return "保存失败: 向量模型已启用但未配置模型名称"
	}
	err := db.Dao.Transaction(func(tx *gorm.DB) error {
		count := int64(0)
		// 清理已废弃字段，避免旧数据污染
		tx.Model(&Settings{}).Where("1=1").Updates(map[string]any{"iwencai_api_key": ""})
		tx.Model(&Settings{}).Count(&count)
		if count > 0 {
			result := tx.Model(&Settings{}).Where("id=?", s.ID).Updates(map[string]any{
				"local_push_enable":          s.LocalPushEnable,
				"ding_push_enable":           s.DingPushEnable,
				"ding_robot":                 s.DingRobot,
				"update_basic_info_on_start": s.UpdateBasicInfoOnStart,
				"refresh_interval":           s.RefreshInterval,
				"open_ai_enable":             s.OpenAiEnable,
				"ai_config_id":               s.AiConfigID,
				"tushare_token":              s.TushareToken,
				"prompt":                     s.Prompt,
				"check_update":               s.CheckUpdate,
				"update_channel":             s.UpdateChannel,
				"question_template":          s.QuestionTemplate,
				"crawl_time_out":             s.CrawlTimeOut,
				"k_days":                     s.KDays,
				"enable_danmu":               s.EnableDanmu,
				"browser_path":               s.BrowserPath,
				"enable_news":                s.EnableNews,
				"dark_theme":                 s.DarkTheme,
				"enable_fund":                s.EnableFund,
				"enable_push_news":           s.EnablePushNews,
				"enable_only_push_red_news":  s.EnableOnlyPushRedNews,
				"http_proxy":                 s.HttpProxy,
				"http_proxy_enabled":         s.HttpProxyEnabled,
				"enable_agent":               s.EnableAgent,
				"disable_browser_search":     s.DisableBrowserSearch,
				"qgqp_b_id":                  s.QgqpBId,
				"em_api_key":                 s.EmApiKey,
			"tickflow_api_key":           s.TickFlowAPIKey,
			"akshare_service_url":       s.AkshareServiceUrl,
			"adata_service_url":         s.ADataServiceUrl,
			"default_data_source":       s.DefaultDataSource,
				"window_width":               s.WindowWidth,
				"window_height":              s.WindowHeight,
				"prompt_plaza_api_base":      s.PromptPlazaApiBase,
				"embedding_enabled":          s.EmbeddingEnabled,
				"embedding_model_name":       s.EmbeddingModelName,
				"embedding_base_url":         s.EmbeddingBaseUrl,
				"embedding_api_key":          s.EmbeddingApiKey,
			})
			if result.Error != nil {
				return result.Error
			}
			return updateAiConfigsTx(tx, s.AiConfigs)
		}
		result := tx.Model(&Settings{}).Create(s.Settings)
		if result.Error != nil {
			return result.Error
		}
		return updateAiConfigsTx(tx, s.AiConfigs)
	})
	if err != nil {
		logger.SugaredLogger.Errorf("更新配置失败: %v", err)
		return "保存失败: " + err.Error()
	}

	ConfigureFromSettings(s)

	return "保存成功！"
}

func updateAiConfigsTx(tx *gorm.DB, aiConfigs []*AIConfig) error {
	if len(aiConfigs) == 0 {
		err := tx.Exec("DELETE FROM ai_config").Error
		if err != nil {
			return err
		}
		return tx.Exec("DELETE FROM sqlite_sequence WHERE name='ai_config'").Error
	}
	// 仅收集大于 0 的 ID，用于识别已存在的配置；
	// ID<=0 视为“新配置”，强制走插入逻辑，避免多个 ID 为 0 的配置互相覆盖。
	var ids []uint
	lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	})
	var existAiConfigs []*AIConfig
	err := tx.Model(&AIConfig{}).Select("id").Where("id IN ?", ids).Find(&existAiConfigs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	idMap := make(map[uint]bool)
	lo.ForEach(existAiConfigs, func(item *AIConfig, index int) {
		idMap[item.ID] = true
	})
	var addAiConfigs []*AIConfig
	var notDeleteIds []uint
	var e error
	lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
		if e != nil {
			return
		}
		// ID<=0 或数据库中不存在该 ID，一律视为新配置，走插入逻辑（清除临时ID由数据库自增主键）
		if item.ID <= 0 || !idMap[item.ID] {
			item.ID = 0
			addAiConfigs = append(addAiConfigs, item)
		} else {
			notDeleteIds = append(notDeleteIds, item.ID)
			e = tx.Model(&AIConfig{}).Where("id=?", item.ID).Updates(map[string]interface{}{
				"name":                 item.Name,
				"base_url":             item.BaseUrl,
				"api_key":              item.ApiKey,
				"model_name":           item.ModelName,
				"max_tokens":           item.MaxTokens,
				"temperature":          item.Temperature,
				"time_out":             item.TimeOut,
				"http_proxy":           item.HttpProxy,
				"http_proxy_enabled":   item.HttpProxyEnabled,
				"session_id":           item.SessionId,
				"thinking":             item.Thinking,
				"llm_concurrency_limit": item.LLMConcurrencyLimit,
				"sub_agent_timeout":     item.SubAgentTimeout,
				"embedding_enabled":    item.EmbeddingEnabled,
				"embedding_model_name": item.EmbeddingModelName,
				"embedding_base_url":   item.EmbeddingBaseUrl,
			}).Error
			if e != nil {
				return
			}
		}
	})
	if e != nil {
		return e
	}
	//删除旧的配置
	if len(notDeleteIds) > 0 {
		err = tx.Exec("DELETE FROM ai_config WHERE id NOT IN ?", notDeleteIds).Error
		if err != nil {
			return err
		}
	}
	//logger.SugaredLogger.Infof("更新aiConfigs +%d", len(addAiConfigs))
	//批量新增的配置
	err = tx.CreateInBatches(addAiConfigs, len(addAiConfigs)).Error
	return err
}

func GetSettingConfig() *SettingConfig {
	if db.Dao == nil {
		return &SettingConfig{
			Settings:  &Settings{},
			AiConfigs: []*AIConfig{},
		}
	}

	settingConfig := &SettingConfig{}
	settings := &Settings{}
	aiConfigs := make([]*AIConfig, 0)
	// 处理数据库查询可能返回的空结果
	db.Dao.Model(&Settings{}).First(settings)
	// 始终加载 AI 配置列表，无论 OpenAiEnable 是否开启
	// 这样前端配置选择器（如市场资讯总结）能始终展示已有配置
	result := db.Dao.Model(&AIConfig{}).Find(&aiConfigs)
	if result.Error != nil {
		logger.SugaredLogger.Error("查询AI配置失败:", result.Error)
	} else if len(aiConfigs) > 0 {
		lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
			if item.TimeOut <= 0 {
				item.TimeOut = 60 * 5
			}
			// ✅ 修复：BaseUrl/ModelName/Name 为空时填充默认值，避免前端下拉框空白
			if strings.TrimSpace(item.BaseUrl) == "" {
				item.BaseUrl = "https://api.deepseek.com"
			}
			if strings.TrimSpace(item.ModelName) == "" {
				item.ModelName = "deepseek-reasoner"
			}
			if strings.TrimSpace(item.Name) == "" {
				item.Name = "AI 配置"
			}
			if item.MaxTokens <= 0 {
				item.MaxTokens = 4096
			}
			if item.Temperature <= 0 {
				item.Temperature = 0.7
			}
			if item.LLMConcurrencyLimit <= 0 {
				item.LLMConcurrencyLimit = 2
			}
			if item.SubAgentTimeout <= 0 {
				item.SubAgentTimeout = 20
			}
		})
	}
	if settings.OpenAiEnable {
		if settings.CrawlTimeOut <= 0 {
			settings.CrawlTimeOut = 60
		}
		if settings.KDays < 30 {
			settings.KDays = 60
		}
	}
	if settings.BrowserPath == "" {
		settings.BrowserPath, _ = CheckBrowser()
	}
	if settings.ToolRunBudget <= 0 {
		settings.ToolRunBudget = 15
	}
	if settings.ToolDailyBudget <= 0 {
		settings.ToolDailyBudget = 200
	}
	if settings.ToolDailyCostUSD <= 0 {
		settings.ToolDailyCostUSD = 2.0
	}
	settingConfig.Settings = settings
	settingConfig.AiConfigs = aiConfigs

	return settingConfig
}
