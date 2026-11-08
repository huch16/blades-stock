package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
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
	TushareToken              string  `json:"tushareToken"`
	LocalPushEnable           bool    `json:"localPushEnable"`
	DingPushEnable            bool    `json:"dingPushEnable"`
	DingRobot                 string  `json:"dingRobot"`
	UpdateBasicInfoOnStart    bool    `json:"updateBasicInfoOnStart"`
	RefreshInterval           int64   `json:"refreshInterval"`
	OpenAiEnable              bool    `json:"openAiEnable"`
	AiConfigID                uint    `json:"aiConfigId" gorm:"column:ai_config_id;default:0"`
	Prompt                    string  `json:"prompt"`
	CheckUpdate               bool    `json:"checkUpdate"`
	UpdateChannel             string  `json:"updateChannel"`
	QuestionTemplate          string  `json:"questionTemplate"`
	CrawlTimeOut              int64   `json:"crawlTimeOut"`
	KDays                     int64   `json:"kDays"`
	BrowserPath               string  `json:"browserPath"`
	EnableNews                bool    `json:"enableNews"`
	DarkTheme                 bool    `json:"darkTheme"`
	EnableFund                bool    `json:"enableFund"`
	EnablePushNews            bool    `json:"enablePushNews"`
	EnableOnlyPushRedNews     bool    `json:"enableOnlyPushRedNews"`
	HttpProxy                 string  `json:"httpProxy"`
	HttpProxyEnabled          bool    `json:"httpProxyEnabled"`
	EnableAgent               bool    `json:"enableAgent"`
	DisableBrowserSearch      bool    `json:"disableBrowserSearch" gorm:"column:disable_browser_search;default:false"`
	QgqpBId                   string  `json:"qgqpBId" gorm:"column:qgqp_b_id"`
	TickFlowAPIKey            string  `json:"tickFlowAPIKey" gorm:"column:tickflow_api_key"`
	EmApiKey                  string  `json:"emApiKey" gorm:"column:em_api_key"`
	WindowWidth               int     `json:"windowWidth"`
	WindowHeight              int     `json:"windowHeight"`
	PromptPlazaApiBase        string  `json:"promptPlazaApiBase" gorm:"column:prompt_plaza_api_base"`
	BacktestAccuracyThreshold float64 `json:"backtestAccuracyThreshold" gorm:"column:backtest_accuracy_threshold;default:5.0"`
	BacktestVerifyDays        int64   `json:"backtestVerifyDays" gorm:"column:backtest_verify_days;default:2"`
	BacktestEnabled           bool    `json:"backtestEnabled" gorm:"column:backtest_enabled;default:true"`
	AkshareServiceUrl         string  `json:"akshareServiceUrl" gorm:"column:akshare_service_url;default:http://127.0.0.1:9000"`
	ADataServiceUrl           string  `json:"aDataServiceUrl" gorm:"column:adata_service_url;default:http://127.0.0.1:9100"`
	DefaultDataSource         string  `json:"defaultDataSource" gorm:"column:default_data_source;default:eastmoney"`
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
	ID                  uint `gorm:"primarykey"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Name                string  `json:"name"`
	BaseUrl             string  `json:"baseUrl"`
	ApiKey              string  `json:"apiKey" `
	ModelName           string  `json:"modelName"`
	MaxTokens           int     `json:"maxTokens"`
	Temperature         float64 `json:"temperature"`
	TimeOut             int     `json:"timeOut"`
	HttpProxy           string  `json:"httpProxy"`
	HttpProxyEnabled    bool    `json:"httpProxyEnabled"`
	SessionId           string  `json:"sessionId" gorm:"index;size:64"`
	Thinking            bool    `json:"thinking"`
	EmbeddingEnabled    bool    `json:"embeddingEnabled"`
	EmbeddingModelName  string  `json:"embeddingModelName"`
	EmbeddingBaseUrl    string  `json:"embeddingBaseUrl"`
	LLMConcurrencyLimit int     `json:"llmConcurrencyLimit"` // 并发限制，默认 2
	SubAgentTimeout     int     `json:"subAgentTimeout"`     // 子Agent超时秒，默认 20
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

	// submittedFields 记录客户端 JSON 载荷中**实际出现**的顶层字段名。
	// 仅用于区分「字段未提交」与「显式提交 false/零值」两种语义，
	// 不参与序列化，也不会出现在 Wails 绑定 / TypeScript 模型中
	//（生成器会跳过未导出字段，见 wails internal/binding、typescriptify）。
	submittedFields map[string]bool
}

// UnmarshalJSON 自定义反序列化，同时兼顾两个职责：
//
//  1. 保持 v3.0.0 约定的**扁平** JSON 形态（匿名嵌入的 *Settings 字段被提升到顶层），
//     并保持「载荷为嵌套 {settings:{...}} 时 Settings 保持 nil」的既有语义；
//
//  2. 记录载荷中**实际出现**的顶层字段名（submittedFields）。
//
// 为什么需要第 2 点：后端 UpdateConfig 使用 GORM `Updates(map)` **显式写入每一列**，
// 若无法区分「字段未提交」与「显式提交 false」，则 embeddingEnabled / backtestEnabled
// 这类「关闭值就是 false」的开关永远无法被关闭（实测 backtestEnabled=false 无法落库；
// embeddingEnabled=false 侥幸通过，仅仅是因为数据库默认值恰好也是 false）。
func (c *SettingConfig) UnmarshalJSON(data []byte) error {
	// 使用别名类型，避免递归调用本方法
	type settingConfigAlias SettingConfig
	var aux settingConfigAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*c = SettingConfig(aux)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.submittedFields = make(map[string]bool, len(raw))
	for key := range raw {
		c.submittedFields[key] = true
	}
	return nil
}

// fieldSubmitted 判断客户端是否显式提交了某个顶层字段。
// 非 JSON 来源（例如桌面端 beforeClose 用 GetSettingConfig() 原样回写）没有
// submittedFields 标记，此时按「全量提交」处理，保持既有行为不变。
func (c *SettingConfig) fieldSubmitted(key string) bool {
	if c == nil || c.submittedFields == nil {
		return true
	}
	return c.submittedFields[key]
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
	CleanupLogs        = "logs"
	CleanupCache       = "cache"
	CleanupChat        = "chat_sessions"
	CleanupAIResults   = "ai_results"
	CleanupTestImages  = "test_images"
	CleanupBrowserTemp = "browser_temp"
	CleanupAll         = "all"
)

// CleanupData 清理应用产生的中间/临时/历史数据
func (s *SettingsApi) CleanupData(cleanupType string) string {
	logger.SugaredLogger.Infof("Cleaning up data type: %s", cleanupType)

	cleanupFuncs := map[string]func() string{
		CleanupLogs:        cleanupLogs,
		CleanupCache:       cleanupCache,
		CleanupChat:        cleanupChatSessions,
		CleanupAIResults:   cleanupAIResults,
		CleanupTestImages:  cleanupTestImages,
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
	return EnsureBackupDir()
}

// GetBackupOverview 获取系统数据资产概览
func (s *SettingsApi) GetBackupOverview() (*BackupOverview, error) {
	return NewBackupManager().GetBackupOverview()
}

// ListBackups 列出所有历史备份归档
func (s *SettingsApi) ListBackups() ([]BackupFileItem, error) {
	return NewBackupManager().ListBackups()
}

// CreateIntelligentBackup 一键创建全量智能资产备份包
func (s *SettingsApi) CreateIntelligentBackup(note string) string {
	res, err := NewBackupManager().CreateIntelligentBackup(note)
	if err != nil {
		return "❌ 备份失败: " + err.Error()
	}
	return res
}

// CreateDatabaseSnapshot 一键创建底层数据库物理快照
func (s *SettingsApi) CreateDatabaseSnapshot() string {
	res, err := NewBackupManager().CreateDatabaseSnapshot()
	if err != nil {
		return "❌ 创建快照失败: " + err.Error()
	}
	return res
}

// RestoreBackupFile 精确恢复指定文件名的备份
func (s *SettingsApi) RestoreBackupFile(fileName string) string {
	res, err := NewBackupManager().RestoreBackupFile(fileName)
	if err != nil {
		return "❌ 恢复失败: " + err.Error()
	}
	return res
}

// DeleteBackupFile 删除指定的历史备份文件
func (s *SettingsApi) DeleteBackupFile(fileName string) string {
	err := NewBackupManager().DeleteBackupFile(fileName)
	if err != nil {
		return "❌ 删除失败: " + err.Error()
	}
	return "✅ 已成功删除备份文件: " + fileName
}

// GetBackupContent 获取备份文件内容（用于前端直接下载保存）
func (s *SettingsApi) GetBackupContent(fileName string) string {
	content, err := NewBackupManager().GetBackupContent(fileName)
	if err != nil {
		return "ERROR:" + err.Error()
	}
	return content
}

// ImportIntelligentBackup 导入全量智能资产包 (JSON) 并安全合并写入
func (s *SettingsApi) ImportIntelligentBackup(content string) string {
	res, err := NewBackupManager().ImportIntelligentBackup(content)
	if err != nil {
		return "❌ 导入恢复失败: " + err.Error()
	}
	return res
}

// VerifyBackupIntegrity 对指定的备份文件执行工业级完整性与格式有效性校验
func (s *SettingsApi) VerifyBackupIntegrity(fileName string) string {
	err := NewBackupManager().VerifyBackupIntegrity(fileName)
	if err != nil {
		return fmt.Sprintf("❌ 校验失败: %v", err)
	}
	return "✅ 校验通过：文件结构完整合法，数据表与索引一致性良好"
}

// PruneOldBackups 智能滚动修剪过期的历史备份
func (s *SettingsApi) PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays int) string {
	pruned, err := NewBackupManager().PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays)
	if err != nil {
		return fmt.Sprintf("❌ 清理失败: %v", err)
	}
	return fmt.Sprintf("✅ 历史备份清理完成，共自动回收 %d 个陈旧/超额备份文件", pruned)
}

// BackupData 导出用户数据到 CSV，返回生成的文件路径
func (s *SettingsApi) BackupData(backupType string) string {
	logger.SugaredLogger.Infof("Backing up data type: %s", backupType)

	backupDir := backupDirPath()
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "❌ 创建备份目录失败: " + err.Error()
	}

	ts := time.Now().Format("20060102_150405")
	baseFileName := fmt.Sprintf("%s_%s.csv", backupType, ts)
	fileName := filepath.Join(backupDir, baseFileName)

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
		return "❌ 未知备份类型: " + backupType
	}
	if strings.Contains(result, "失败") {
		return "❌ " + result
	}
	return fmt.Sprintf("✅ %s\nFILE:%s\n备份目录: %s", result, baseFileName, backupDir)
}

// RestoreData 从 CSV 文件恢复数据到数据库
func (s *SettingsApi) RestoreData(backupType string) string {
	logger.SugaredLogger.Infof("Restoring data type: %s", backupType)

	backupDir := backupDirPath()
	pattern := filepath.Join(backupDir, backupType+"_*.csv")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "❌ 查找备份文件失败: " + err.Error()
	}
	if len(matches) == 0 {
		return "❌ 未找到 " + backupType + " 的备份文件，请先执行备份\n备份目录: " + backupDir
	}

	// 使用最新的文件
	fileName := matches[len(matches)-1]

	var res string
	switch backupType {
	case BackupFollowedStocks:
		res = restoreFollowedStocks(fileName)
	case BackupRecommendStocks:
		res = restoreRecommendStocks(fileName)
	case BackupAIReports:
		res = restoreAIReports(fileName)
	case BackupFollowedFunds:
		res = restoreFollowedFunds(fileName)
	case BackupBacktestResults:
		res = restoreBacktestResults(fileName)
	case BackupStockChangeHistory:
		res = restoreStockChangeHistory(fileName)
	default:
		return "❌ 未知恢复类型: " + backupType
	}
	if strings.Contains(res, "失败") {
		return "❌ " + res
	}
	return "✅ " + res
}

func backupFollowedStocks(fileName string) string {
	rows, err := db.Dao.Raw("SELECT stock_code, name, volume, cost_price, price, change_percent, alarm_change_percent, alarm_price, time, sort, ai_config_id, entry_price, take_profit_price, stop_loss_price FROM followed_stock WHERE is_del=0 ORDER BY sort, stock_code").Rows()
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
		code := strings.TrimSpace(r["股票代码"])
		if code == "" {
			continue
		}
		var count int64
		tx.Model(&FollowedStock{}).Where("stock_code = ?", code).Count(&count)
		if count > 0 {
			err := tx.Model(&FollowedStock{}).Where("stock_code = ?", code).Updates(map[string]any{
				"name":                 r["名称"],
				"volume":               r["持仓量"],
				"cost_price":           r["成本价"],
				"price":                r["当前价"],
				"change_percent":       r["涨跌幅%"],
				"alarm_change_percent": r["预警涨跌幅%"],
				"alarm_price":          r["预警价"],
				"time":                 r["时间"],
				"sort":                 r["排序"],
				"ai_config_id":         r["AI配置ID"],
				"entry_price":          r["入场价"],
				"take_profit_price":    r["止盈价"],
				"stop_loss_price":      r["止损价"],
				"is_del":               0,
			}).Error
			if err != nil {
				tx.Rollback()
				return "恢复自选股票失败: " + err.Error()
			}
		} else {
			err := tx.Exec("INSERT INTO followed_stock (stock_code, name, volume, cost_price, price, change_percent, alarm_change_percent, alarm_price, time, sort, ai_config_id, entry_price, take_profit_price, stop_loss_price, is_del) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,0)",
				code, r["名称"], r["持仓量"], r["成本价"], r["当前价"], r["涨跌幅%"], r["预警涨跌幅%"], r["预警价"], r["时间"], r["排序"], r["AI配置ID"], r["入场价"], r["止盈价"], r["止损价"],
			).Error
			if err != nil {
				tx.Rollback()
				return "恢复自选股票失败: " + err.Error()
			}
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
	masked := GetMaskedSettingConfig()
	d, err := json.MarshalIndent(masked, "", "    ")
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
	Type           string `json:"type"`
	Label          string `json:"label"`
	Description    string `json:"description"`
	BaseUrl        string `json:"baseUrl"`
	ModelName      string `json:"modelName"`
	ApiKeyRequired bool   `json:"apiKeyRequired"`
	EnvHint        string `json:"envHint"`
}

// GetAiConfigTemplates 返回预设的 AI 配置模板，供前端向导使用
func (s *SettingsApi) GetAiConfigTemplates() []*AiConfigTemplate {
	return []*AiConfigTemplate{
		{
			Type:           "cloud",
			Label:          "MiniMax 官方直连（推荐·投研专用）",
			Description:    "支持 MiniMax-M3 / MiniMax-Text-01，国内直连低延迟，具备优秀的量化语义推导能力",
			BaseUrl:        "https://api.minimax.chat/v1",
			ModelName:      "MiniMax-M3",
			ApiKeyRequired: true,
			EnvHint:        "在 https://platform.minimaxi.com 注册并获取 API Key",
		},
		{
			Type:           "cloud",
			Label:          "NVIDIA NIM 极速推理平台",
			Description:    "英伟达托管的高性能企业级微服务，支持 GLM-5.3-Flash / Llama-3.3-70B 极速推理",
			BaseUrl:        "https://integrate.api.nvidia.com/v1",
			ModelName:      "z-ai/glm-5.3-flash",
			ApiKeyRequired: true,
			EnvHint:        "在 https://build.nvidia.com 免费领取 1000 次 API Key",
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
	cfg.BaseUrl = strings.TrimRight(strings.TrimSpace(cfg.BaseUrl), "/")
	if cfg.BaseUrl == "" {
		return "AI 配置 Base URL 不能为空"
	}
	if !strings.HasPrefix(cfg.BaseUrl, "http://") && !strings.HasPrefix(cfg.BaseUrl, "https://") {
		return "AI 配置 Base URL 格式错误，必须以 http:// 或 https:// 开头"
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

	// 固化运行时调度参数为纯 Go 确定性默认值
	if cfg.LLMConcurrencyLimit <= 0 {
		cfg.LLMConcurrencyLimit = 2
	}
	if cfg.SubAgentTimeout <= 0 {
		cfg.SubAgentTimeout = 20
	}

	if cfg.EmbeddingEnabled && strings.TrimSpace(cfg.EmbeddingModelName) == "" {
		cfg.EmbeddingModelName = "text-embedding-3-small"
	}
	return ""
}

// MaskSecret 对敏感商业凭据进行掩码处理（例如 sk-2897****3d9b）
func MaskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// MaskWebhook 对 Webhook 链接中的敏感 token 参数脱敏
func MaskWebhook(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "access_token="); idx != -1 {
		prefix := s[:idx+len("access_token=")]
		token := s[idx+len("access_token="):]
		if len(token) > 8 {
			return prefix + token[:4] + "****" + token[len(token)-4:]
		}
		return prefix + "****"
	}
	return MaskSecret(s)
}

// IsMaskedSecret 判断字符串是否包含掩码特征
func IsMaskedSecret(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, "****") {
		return true
	}
	trimmed := strings.ReplaceAll(s, "*", "")
	return len(trimmed) == 0
}

func UpdateConfig(s *SettingConfig) string {
	if s == nil || s.Settings == nil {
		return "保存失败: 配置数据为空"
	}

	// 敏感凭据掩码恢复防护：若前端提交的是脱敏掩码，保留数据库现有真实凭据，防止被掩码覆盖
	if db.Dao != nil {
		var exist Settings
		if err := db.Dao.Model(&Settings{}).First(&exist).Error; err == nil {
			if IsMaskedSecret(s.TushareToken) {
				s.TushareToken = exist.TushareToken
			}
			if IsMaskedSecret(s.EmbeddingApiKey) {
				s.EmbeddingApiKey = exist.EmbeddingApiKey
			}
			if IsMaskedSecret(s.TickFlowAPIKey) {
				s.TickFlowAPIKey = exist.TickFlowAPIKey
			}
			if IsMaskedSecret(s.EmApiKey) {
				s.EmApiKey = exist.EmApiKey
			}
			if IsMaskedSecret(s.DingRobot) {
				s.DingRobot = exist.DingRobot
			}
		}
	}

	// ── 防御性保护（v3.0.0）— 必须在 validateAIConfig **之前**执行 ───────────────
	//
	// `updateAiConfigsTx` 使用 GORM Updates(map) **显式写入每一列**，因此调用方未提交的
	// 字段会以零值落库。而 `validateAIConfig` 本身会用默认值（4096 / 300 / 2 / 20）
	// 把零值填掉，从而掩盖“该字段根本没被提交”这一事实。所以必须先按数据库原值补齐，
	// 再交给 validateAIConfig 校验（此时值已合法，不会再被默认值覆盖）。
	//
	// 补齐范围：仅限**界面上没有输入入口**的调度/扩展字段（新增或清零本无意义），
	// 以及布尔类中无界面入口的 httpProxyEnabled / embeddingEnabled。
	// 明确**不**补齐：name / baseUrl / apiKey / modelName / thinking —— 这五项由界面管理，
	// 必须允许显式改空或置 false。
	if db.Dao != nil {
		for _, cfg := range s.AiConfigs {
			if cfg == nil || cfg.ID == 0 {
				continue
			}
			var old AIConfig
			if err := db.Dao.Where("id = ?", cfg.ID).First(&old).Error; err != nil {
				continue // 不存在的 ID 会在 upsert 中按新增处理
			}
			if cfg.MaxTokens <= 0 {
				cfg.MaxTokens = old.MaxTokens
			}
			if cfg.Temperature <= 0 {
				cfg.Temperature = old.Temperature
			}
			if cfg.TimeOut <= 0 {
				cfg.TimeOut = old.TimeOut
			}
			if cfg.LLMConcurrencyLimit <= 0 {
				cfg.LLMConcurrencyLimit = old.LLMConcurrencyLimit
			}
			if cfg.SubAgentTimeout <= 0 {
				cfg.SubAgentTimeout = old.SubAgentTimeout
			}
			if !cfg.HttpProxyEnabled {
				cfg.HttpProxyEnabled = old.HttpProxyEnabled
			}
			if !cfg.EmbeddingEnabled {
				cfg.EmbeddingEnabled = old.EmbeddingEnabled
			}
			if strings.TrimSpace(cfg.HttpProxy) == "" {
				cfg.HttpProxy = old.HttpProxy
			}
			if strings.TrimSpace(cfg.SessionId) == "" {
				cfg.SessionId = old.SessionId
			}
			if strings.TrimSpace(cfg.EmbeddingModelName) == "" {
				cfg.EmbeddingModelName = old.EmbeddingModelName
			}
			if strings.TrimSpace(cfg.EmbeddingBaseUrl) == "" {
				cfg.EmbeddingBaseUrl = old.EmbeddingBaseUrl
			}
			if cfg.RateLimitPerSecond <= 0 {
				cfg.RateLimitPerSecond = old.RateLimitPerSecond
			}
			if cfg.Burst <= 0 {
				cfg.Burst = old.Burst
			}
			if cfg.DailyRequestQuota <= 0 {
				cfg.DailyRequestQuota = old.DailyRequestQuota
			}
			if cfg.DailyCostQuota <= 0 {
				cfg.DailyCostQuota = old.DailyCostQuota
			}
		}
	}

	for i, cfg := range s.AiConfigs {
		if cfg == nil {
			continue
		}
		cfg.BaseUrl = strings.TrimRight(strings.TrimSpace(cfg.BaseUrl), "/")
		cfg.Name = strings.TrimSpace(cfg.Name)
		cfg.ModelName = strings.TrimSpace(cfg.ModelName)
		cfg.ApiKey = strings.TrimSpace(cfg.ApiKey)
		if msg := validateAIConfig(cfg); msg != "" {
			return fmt.Sprintf("保存失败: 第 %d 项 %s", i+1, msg)
		}
	}
	if s.EmbeddingEnabled && strings.TrimSpace(s.EmbeddingModelName) == "" {
		s.EmbeddingModelName = "text-embedding-3-small"
	}
	var err error
	maxRetries := 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = db.Dao.Transaction(func(tx *gorm.DB) error {
			count := int64(0)
			tx.Model(&Settings{}).Count(&count)
			if count > 0 {
				var exist Settings
				var query *gorm.DB
				var targetID uint
				if err := tx.First(&exist).Error; err == nil && exist.ID > 0 {
					targetID = exist.ID
					query = tx.Model(&Settings{}).Where("id = ?", exist.ID)
				} else if s.ID > 0 {
					targetID = s.ID
					query = tx.Model(&Settings{}).Where("id = ?", s.ID)
				} else {
					query = tx.Model(&Settings{}).Where("1 = 1")
				}
				// ── 字段提交语义（v3.0.0 P0 修复）───────────────────────────────
				//
				// 本函数使用 Updates(map) **显式写入每一列**，因此后端必须能区分两种情况：
				//   A. 客户端**未提交**该字段 → 保留数据库原值（兼容部分载荷 / 老版本缓存分块）
				//   B. 客户端**显式提交**零值/false → 尊重客户端意图（false 是合法取值）
				//
				// 旧实现用「零值即保留」近似处理，这对 embedding_enabled / backtest_enabled
				// 是**错误**的：这两个能力开关的关闭值就是 false，导致用户永远无法关闭它们。
				// submittedFields 由 SettingConfig.UnmarshalJSON 记录；非 JSON 来源
				// （桌面端 beforeClose 原样回写 GetSettingConfig()）没有该标记，按全量提交处理。
				keepStr := func(key, newV, oldV string) string {
					if !s.fieldSubmitted(key) {
						return oldV
					}
					return newV
				}
				// keepStrNonEmpty：显式提交且非空才覆盖（空串对这些字段永不是有意义的取值，
				// 例如 prompt / browserPath 都有后端兜底默认）。
				keepStrNonEmpty := func(key, newV, oldV string) string {
					if !s.fieldSubmitted(key) || strings.TrimSpace(newV) == "" {
						return oldV
					}
					return newV
				}
				keepInt := func(key string, newV, oldV int64) int64 {
					if !s.fieldSubmitted(key) {
						return oldV
					}
					return newV
				}
				keepFloat := func(key string, newV, oldV float64) float64 {
					if !s.fieldSubmitted(key) {
						return oldV
					}
					return newV
				}
				keepBool := func(key string, newV, oldV bool) bool {
					if !s.fieldSubmitted(key) {
						return oldV
					}
					return newV
				}
				// 窗口尺寸由桌面端 beforeClose 显式写入，保留零值以免 headless 场景误清
				keepPositiveInt := func(newV, oldV int64) int64 {
					if newV <= 0 {
						return oldV
					}
					return newV
				}

				result := query.Updates(map[string]any{
					"local_push_enable":           keepBool("localPushEnable", s.LocalPushEnable, exist.LocalPushEnable),
					"ding_push_enable":            keepBool("dingPushEnable", s.DingPushEnable, exist.DingPushEnable),
					"ding_robot":                  keepStr("dingRobot", s.DingRobot, exist.DingRobot),
					"update_basic_info_on_start":  keepBool("updateBasicInfoOnStart", s.UpdateBasicInfoOnStart, exist.UpdateBasicInfoOnStart),
					"refresh_interval":            keepInt("refreshInterval", s.RefreshInterval, exist.RefreshInterval),
					"open_ai_enable":              keepBool("openAiEnable", s.OpenAiEnable, exist.OpenAiEnable),
					"ai_config_id":                s.AiConfigID,
					"tushare_token":               keepStr("tushareToken", s.TushareToken, exist.TushareToken),
					"prompt":                      keepStrNonEmpty("prompt", s.Prompt, exist.Prompt),
					"check_update":                keepBool("checkUpdate", s.CheckUpdate, exist.CheckUpdate),
					"update_channel":              keepStrNonEmpty("updateChannel", s.UpdateChannel, exist.UpdateChannel),
					"question_template":           keepStrNonEmpty("questionTemplate", s.QuestionTemplate, exist.QuestionTemplate),
					"crawl_time_out":              keepInt("crawlTimeOut", s.CrawlTimeOut, exist.CrawlTimeOut),
					"k_days":                      keepInt("kDays", s.KDays, exist.KDays),
					"browser_path":                keepStrNonEmpty("browserPath", s.BrowserPath, exist.BrowserPath),
					"enable_news":                 keepBool("enableNews", s.EnableNews, exist.EnableNews),
					"dark_theme":                  keepBool("darkTheme", s.DarkTheme, exist.DarkTheme),
					"enable_fund":                 keepBool("enableFund", s.EnableFund, exist.EnableFund),
					"enable_push_news":            keepBool("enablePushNews", s.EnablePushNews, exist.EnablePushNews),
					"enable_only_push_red_news":   keepBool("enableOnlyPushRedNews", s.EnableOnlyPushRedNews, exist.EnableOnlyPushRedNews),
					"http_proxy":                  keepStr("httpProxy", s.HttpProxy, exist.HttpProxy),
					"http_proxy_enabled":          keepBool("httpProxyEnabled", s.HttpProxyEnabled, exist.HttpProxyEnabled),
					"enable_agent":                keepBool("enableAgent", s.EnableAgent, exist.EnableAgent),
					"disable_browser_search":      keepBool("disableBrowserSearch", s.DisableBrowserSearch, exist.DisableBrowserSearch),
					"qgqp_b_id":                   keepStrNonEmpty("qgqpBId", s.QgqpBId, exist.QgqpBId),
					"em_api_key":                  keepStr("emApiKey", s.EmApiKey, exist.EmApiKey),
					"tickflow_api_key":            keepStr("tickFlowAPIKey", s.TickFlowAPIKey, exist.TickFlowAPIKey),
					"akshare_service_url":         keepStrNonEmpty("akshareServiceUrl", s.AkshareServiceUrl, exist.AkshareServiceUrl),
					"adata_service_url":           keepStrNonEmpty("aDataServiceUrl", s.ADataServiceUrl, exist.ADataServiceUrl),
					"default_data_source":         keepStrNonEmpty("defaultDataSource", s.DefaultDataSource, exist.DefaultDataSource),
					"window_width":                int(keepPositiveInt(int64(s.WindowWidth), int64(exist.WindowWidth))),
					"window_height":               int(keepPositiveInt(int64(s.WindowHeight), int64(exist.WindowHeight))),
					"prompt_plaza_api_base":       keepStrNonEmpty("promptPlazaApiBase", s.PromptPlazaApiBase, exist.PromptPlazaApiBase),
					"embedding_enabled":           keepBool("embeddingEnabled", s.EmbeddingEnabled, exist.EmbeddingEnabled),
					"embedding_model_name":        keepStrNonEmpty("embeddingModelName", s.EmbeddingModelName, exist.EmbeddingModelName),
					"embedding_base_url":          keepStrNonEmpty("embeddingBaseUrl", s.EmbeddingBaseUrl, exist.EmbeddingBaseUrl),
					"embedding_api_key":           keepStr("embeddingApiKey", s.EmbeddingApiKey, exist.EmbeddingApiKey),
					"backtest_accuracy_threshold": keepFloat("backtestAccuracyThreshold", s.BacktestAccuracyThreshold, exist.BacktestAccuracyThreshold),
					"backtest_verify_days":        keepInt("backtestVerifyDays", s.BacktestVerifyDays, exist.BacktestVerifyDays),
					"backtest_enabled":            keepBool("backtestEnabled", s.BacktestEnabled, exist.BacktestEnabled),
					"mcp_timeout":                 keepInt("mcpTimeout", s.McpTimeout, exist.McpTimeout),
					"tool_run_budget":             keepInt("toolRunBudget", s.ToolRunBudget, exist.ToolRunBudget),
					"tool_daily_budget":           keepInt("toolDailyBudget", s.ToolDailyBudget, exist.ToolDailyBudget),
					"tool_daily_cost_usd":         keepFloat("toolDailyCostUSD", s.ToolDailyCostUSD, exist.ToolDailyCostUSD),
				})
				if result.Error != nil {
					return result.Error
				}
				if err := updateAiConfigsTx(tx, s.AiConfigs); err != nil {
					return err
				}
				// 全局默认模型指针必须在**同一事务内**修复：
				// 提交的 aiConfigId 可能指向一张已被删除（或从未存在）的卡片，
				// 若不修复，settings.ai_config_id 就会悬空，Agent 解析时只能回落到
				// 「第一张卡片」并在日志里持续刷「ai config %d not found」。
				effectiveID, err := resolveDefaultAiConfigID(tx, s.AiConfigID)
				if err != nil {
					return err
				}
				if effectiveID != s.AiConfigID {
					fix := tx.Model(&Settings{}).Where("1 = 1")
					if targetID > 0 {
						fix = tx.Model(&Settings{}).Where("id = ?", targetID)
					}
					if err := fix.Update("ai_config_id", effectiveID).Error; err != nil {
						return err
					}
					s.AiConfigID = effectiveID
				}
				return nil
			}
			result := tx.Model(&Settings{}).Create(s.Settings)
			if result.Error != nil {
				return result.Error
			}
			if err := updateAiConfigsTx(tx, s.AiConfigs); err != nil {
				return err
			}
			effectiveID, err := resolveDefaultAiConfigID(tx, s.AiConfigID)
			if err != nil {
				return err
			}
			if effectiveID != s.AiConfigID {
				if err := tx.Model(&Settings{}).Where("1 = 1").Update("ai_config_id", effectiveID).Error; err != nil {
					return err
				}
				s.AiConfigID = effectiveID
			}
			return nil
		})

		if err == nil {
			break
		}
		// 若因并发锁竞争导致 SQLITE_BUSY，退避重试
		if strings.Contains(err.Error(), "locked") || strings.Contains(err.Error(), "busy") {
			logger.SugaredLogger.Warnf("UpdateConfig 遇数据库锁 (尝试 %d/%d): %v", attempt, maxRetries, err)
			time.Sleep(time.Duration(100*attempt) * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		logger.SugaredLogger.Errorf("更新配置失败: %v", err)
		return "保存失败: " + err.Error()
	}

	// 用**数据库中的最终生效值**配置共享 HTTP 客户端（代理 / 超时），而不是用提交值：
	// 部分载荷提交时，提交值并不等于最终落库值
	//（实例：只提交 prompt 的载荷会把 httpProxy 置空，从而误关全局代理）。
	if db.Dao != nil {
		var final Settings
		if err := db.Dao.First(&final).Error; err == nil {
			ConfigureFromSettings(&SettingConfig{Settings: &final})
		} else {
			ConfigureFromSettings(s)
		}
	} else {
		ConfigureFromSettings(s)
	}

	return "保存成功！"
}

// resolveDefaultAiConfigID 在模型配置 upsert 之后校验全局默认模型指针。
//
// 背景：settings.ai_config_id 指向 ai_config.id。客户端提交的 aiConfigId 可能：
//   - 指向一张刚被删除的卡片（用户删掉了默认卡片，却仍提交旧的 ID）；
//   - 指向一张从未存在过的卡片（陈旧前端 / 手改载荷）。
//
// 两种情况都会留下悬空指针，导致 Agent 每次请求都回落到“第一张卡片”并刷错误日志。
//
// 语义：提交的 ID 仍然有效 → 原样保留；否则回落到库里 ID 最小的存活卡片；
// 一张都不剩 → 置 0（表示“未指定默认”）。
func resolveDefaultAiConfigID(tx *gorm.DB, submittedID uint) (uint, error) {
	var ids []uint
	if err := tx.Model(&AIConfig{}).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if submittedID > 0 {
		for _, id := range ids {
			if id == submittedID {
				return submittedID, nil
			}
		}
	}
	if len(ids) > 0 {
		return ids[0], nil
	}
	return 0, nil
}

func updateAiConfigsTx(tx *gorm.DB, aiConfigs []*AIConfig) error {
	// 防御性保护（v3.0.0）：
	// 空列表会删除**全部**模型配置。只有当数据库中确实存在配置时，才需要真正执行删除；
	// 若库中本来就为空，直接返回即可，避免无谓的 DELETE 与 sqlite_sequence 重排。
	if len(aiConfigs) == 0 {
		var existing int64
		if err := tx.Model(&AIConfig{}).Count(&existing).Error; err != nil {
			return err
		}
		if existing == 0 {
			return nil
		}
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
			// 新增条目补齐调度默认值：前端界面只暴露 5 项核心身份字段，
			// 其余调度参数可能以零值到达，必须在落库前确定性补齐，
			// 否则会被写入 0（例如 max_tokens=0 会让推理模型直接被截断）。
			item.SetDefaults()
			addAiConfigs = append(addAiConfigs, item)
		} else {
			notDeleteIds = append(notDeleteIds, item.ID)
			// 若前端提交的 ApiKey 包含掩码，保留数据库原有的真实明文 ApiKey
			if IsMaskedSecret(item.ApiKey) {
				var oldCfg AIConfig
				if err := tx.Model(&AIConfig{}).Select("api_key").Where("id = ?", item.ID).First(&oldCfg).Error; err == nil && oldCfg.ApiKey != "" {
					item.ApiKey = oldCfg.ApiKey
				}
			}
			e = tx.Model(&AIConfig{}).Where("id=?", item.ID).Updates(map[string]interface{}{
				"name":                  item.Name,
				"base_url":              item.BaseUrl,
				"api_key":               item.ApiKey,
				"model_name":            item.ModelName,
				"max_tokens":            item.MaxTokens,
				"temperature":           item.Temperature,
				"time_out":              item.TimeOut,
				"http_proxy":            item.HttpProxy,
				"http_proxy_enabled":    item.HttpProxyEnabled,
				"session_id":            item.SessionId,
				"thinking":              item.Thinking,
				"llm_concurrency_limit": item.LLMConcurrencyLimit,
				"sub_agent_timeout":     item.SubAgentTimeout,
				"embedding_enabled":     item.EmbeddingEnabled,
				"embedding_model_name":  item.EmbeddingModelName,
				"embedding_base_url":    item.EmbeddingBaseUrl,
				"rate_limit_per_second": item.RateLimitPerSecond,
				"burst":                 item.Burst,
				"daily_request_quota":   item.DailyRequestQuota,
				"daily_cost_quota":      item.DailyCostQuota,
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
	} else if len(addAiConfigs) > 0 {
		// 所有传入配置均为全新项，先清空旧配置，避免历史残留配置无法被覆盖删除
		err = tx.Exec("DELETE FROM ai_config").Error
		if err != nil {
			return err
		}
	}
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
	// 按 ID 升序返回，保证前端卡片顺序与“第一张卡片即默认回落项”的语义一致
	result := db.Dao.Model(&AIConfig{}).Order("id ASC").Find(&aiConfigs)
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
	// 跨平台自动纠错：若当前存储的 BrowserPath 在本地文件系统不存在（例如 Windows 路径同步到 Linux 上），重新探测
	if settings.BrowserPath != "" {
		if _, err := os.Stat(settings.BrowserPath); err != nil {
			if detected, ok := CheckBrowser(); ok && detected != "" {
				settings.BrowserPath = detected
			} else {
				settings.BrowserPath = ""
			}
		}
	} else {
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
	// 兜底防护：若提示词为空或受压测脏数据污染，自动采用买方投研标准模板
	if strings.TrimSpace(settings.Prompt) == "" || strings.HasPrefix(settings.Prompt, "并发测试") {
		settings.Prompt = DefaultResearchSystemPrompt
	}
	if strings.TrimSpace(settings.QuestionTemplate) == "" || strings.TrimSpace(settings.QuestionTemplate) == "{{stockName}}分析和总结" {
		settings.QuestionTemplate = DefaultStockAnalysisQuestionTemplate
	}
	settingConfig.Settings = settings
	settingConfig.AiConfigs = aiConfigs

	return settingConfig
}

// GetMaskedSettingConfig 获取脱敏后的系统配置，供前端展示与对外 RPC 返回
func GetMaskedSettingConfig() *SettingConfig {
	raw := GetSettingConfig()
	if raw == nil || raw.Settings == nil {
		return raw
	}

	// 浅拷贝 Settings，并对其敏感凭据打码
	maskedSettings := *raw.Settings
	maskedSettings.TushareToken = MaskSecret(maskedSettings.TushareToken)
	maskedSettings.EmbeddingApiKey = MaskSecret(maskedSettings.EmbeddingApiKey)
	maskedSettings.TickFlowAPIKey = MaskSecret(maskedSettings.TickFlowAPIKey)
	maskedSettings.EmApiKey = MaskSecret(maskedSettings.EmApiKey)
	maskedSettings.DingRobot = MaskWebhook(maskedSettings.DingRobot)

	// 拷贝 AIConfigs，并对每个 ApiKey 打码
	maskedAiConfigs := make([]*AIConfig, len(raw.AiConfigs))
	for i, cfg := range raw.AiConfigs {
		if cfg != nil {
			clone := *cfg
			clone.ApiKey = MaskSecret(clone.ApiKey)
			maskedAiConfigs[i] = &clone
		}
	}

	return &SettingConfig{
		Settings:  &maskedSettings,
		AiConfigs: maskedAiConfigs,
	}
}
