package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
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
	SearxngUrl             string `json:"searxngUrl"`
	SearxngJsonFormat      bool   `json:"searxngJsonFormat"`
	QgqpBId                string `json:"qgqpBId" gorm:"column:qgqp_b_id"`
	IwencaiApiKey          string `json:"iwencaiApiKey" gorm:"column:iwencai_api_key"`
	EmApiKey               string `json:"emApiKey" gorm:"column:em_api_key"`
	WindowWidth            int    `json:"windowWidth"`
	WindowHeight           int    `json:"windowHeight"`
	PromptPlazaApiBase     string `json:"promptPlazaApiBase" gorm:"column:prompt_plaza_api_base"`
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
				"iwencai_api_key":            s.IwencaiApiKey,
				"em_api_key":                 s.EmApiKey,
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
