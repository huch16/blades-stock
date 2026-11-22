package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	lo "github.com/samber/lo"
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

type AiConfigTemplate struct {
	Type           string `json:"type"`
	Label          string `json:"label"`
	Description    string `json:"description"`
	BaseUrl        string `json:"baseUrl"`
	ModelName      string `json:"modelName"`
	ApiKeyRequired bool   `json:"apiKeyRequired"`
	EnvHint        string `json:"envHint"`
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
	// 处理数据库查询可能返回的空结果：若不存在任何 Settings 行，创建默认行
	if err := db.Dao.Model(&Settings{}).First(settings).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建默认配置行
			defaultSettings := &Settings{
				OpenAiEnable:              true,
				RefreshInterval:           3,
				CrawlTimeOut:              60,
				KDays:                     60,
				BrowserPath:               "",
				HttpProxyEnabled:          false,
				BacktestEnabled:           true,
				BacktestAccuracyThreshold: 5.0,
				BacktestVerifyDays:        2,
				McpTimeout:                8,
				ToolRunBudget:             15,
				ToolDailyBudget:           200,
				ToolDailyCostUSD:          2.0,
			}
			if createErr := db.Dao.Create(defaultSettings).Error; createErr != nil {
				logger.SugaredLogger.Warnf("创建默认 Settings 行失败: %v", createErr)
			} else {
				*settings = *defaultSettings
				logger.SugaredLogger.Info("数据库未找到 Settings 行，已创建默认配置")
			}
		} else {
			logger.SugaredLogger.Error("查询 Settings 失败:", err)
		}
	}
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
				item.BaseUrl = "https://api.deepseek.com/v1"
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
