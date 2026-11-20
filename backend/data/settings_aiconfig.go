package data

import (
	"errors"
	"strings"

	lo "github.com/samber/lo"
	"gorm.io/gorm"
)

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
