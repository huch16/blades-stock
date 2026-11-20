package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

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
