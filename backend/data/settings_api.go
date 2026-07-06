package data

import (
	"encoding/csv"
	"encoding/json"
	"database/sql"
	"errors"
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"os"
	"path/filepath"
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
	BrowserPoolSize        int    `json:"browserPoolSize"`
	EnableFund             bool   `json:"enableFund"`
	EnablePushNews         bool   `json:"enablePushNews"`
	EnableOnlyPushRedNews  bool   `json:"enableOnlyPushRedNews"`
	HttpProxy              string `json:"httpProxy"`
	HttpProxyEnabled       bool   `json:"httpProxyEnabled"`
	EnableAgent            bool   `json:"enableAgent"`
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
	result := db.Dao.Exec("DELETE FROM ai_assistant_session")
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
// BackupData 导出用户数据到 CSV，返回生成的文件路径
func (s *SettingsApi) BackupData(backupType string) string {
	logger.SugaredLogger.Infof("Backing up data type: %s", backupType)

	backupDir := "backups"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "创建备份目录失败: " + err.Error()
	}

	ts := time.Now().Format("20060102_150405")
	fileName := filepath.Join(backupDir, fmt.Sprintf("%s_%s.csv", backupType, ts))

	switch backupType {
	case BackupFollowedStocks:
		return backupFollowedStocks(fileName)
	case BackupRecommendStocks:
		return backupRecommendStocks(fileName)
	case BackupAIReports:
		return backupAIReports(fileName)
	case BackupFollowedFunds:
		return backupFollowedFunds(fileName)
	case BackupBacktestResults:
		return backupBacktestResults(fileName)
	case BackupStockChangeHistory:
		return backupStockChangeHistory(fileName)
	default:
		return "未知备份类型: " + backupType
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

func (s *SettingsApi) Export() string {
	d, err := json.MarshalIndent(s.Config, "", "    ")
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置序列化失败: %v", err)
		return ""
	}
	return string(d)
}

func validateAIConfig(cfg *AIConfig) string {
	if strings.TrimSpace(cfg.Name) == "" {
		return "AI 配置名称不能为空"
	}
	if strings.TrimSpace(cfg.BaseUrl) == "" {
		return "AI 配置 Base URL 不能为空"
	}
	if strings.TrimSpace(cfg.ApiKey) == "" {
		return "AI 配置 API Key 不能为空"
	}
	if strings.TrimSpace(cfg.ModelName) == "" {
		return "AI 配置模型名称不能为空"
	}
	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		return "Temperature 必须在 0~2 之间"
	}
	if cfg.MaxTokens < 1 || cfg.MaxTokens > 128000 {
		return "MaxTokens 必须在 1~128000 之间"
	}
	if cfg.TimeOut < 60 {
		return "TimeOut 不能小于 60 秒"
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
				"qgqp_b_id":                  s.QgqpBId,
				"em_api_key":                 s.EmApiKey,
			"tickflow_api_key":           s.TickFlowAPIKey,
			"akshare_service_url":       s.AkshareServiceUrl,
			"adata_service_url":         s.ADataServiceUrl,
			"default_data_source":       s.DefaultDataSource,
				"window_width":               s.WindowWidth,
				"window_height":              s.WindowHeight,
				"prompt_plaza_api_base":      s.PromptPlazaApiBase,
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
		// ID<=0 一律视为新配置，走插入逻辑；否则根据是否已存在决定更新或新增
		if item.ID <= 0 || !idMap[item.ID] {
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
	db.Dao.AutoMigrate(&AIConfig{})

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
	if settings.BrowserPoolSize <= 0 {
		settings.BrowserPoolSize = 1
	}
	settingConfig.Settings = settings
	settingConfig.AiConfigs = aiConfigs

	return settingConfig
}
