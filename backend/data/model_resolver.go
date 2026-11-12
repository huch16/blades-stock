package data

import (
	"errors"
	"fmt"
	"strings"
)

// ── 模型解析：唯一入口 ──────────────────────────────────────────────────────
//
// 为什么需要把“选哪个模型”收敛成一个函数：
//
//	改造前仓库里同时存在 **4 套互不相同的解析行为**：
//	  1. agent/agent_api.go        ID → 全局默认 → 第一张非 nil（并直接改写 Thinking）
//	  2. data/NewDeepSeekOpenAi     ID → **空配置**（无任何回落）
//	  3. agent/llm_summarizer.go   自定义内联解析，额外要求 ApiKey 非空
//	  4. agent/price_alert_watcher.go 又一套内联解析
//	于是同一次请求在不同链路上“实际用的模型”可能不同，而界面上永远只显示
//	**请求的**那个 —— 这正是用户反复遇到的“到底哪个模型在跑”。
//
// 现在所有链路都必须调用 ResolveModel，并且：
//   · 返回的是配置**副本**，调用方可安全改写（请求级 thinking 覆盖不再污染存储）；
//   · 同时给出“为什么选它”的完整依据（Source / Reason / FallbackUsed），
//     可直接写进日志、运行记录与界面。

// 模型解析依据
const (
	ModelSourceExplicit      = "explicit"        // 调用方明确指定了 aiConfigId
	ModelSourceGlobalDefault = "global_default"  // 采用设置里的全局默认卡片
	ModelSourceFirstAvail    = "first_available" // 指定与默认都不可用，回落到第一张可用卡片
)

// ModelResolution 一次模型解析的完整结果与依据。
type ModelResolution struct {
	// Config 是解析后的配置**副本**。绝不要把它写回存储。
	Config      AIConfig
	RequestedID int
	ResolvedID  uint
	// Source 取值见 ModelSource* 常量
	Source string
	// Reason 人类可读的解析依据（用于日志 / 运行记录 / 界面）
	Reason string
	// FallbackUsed 表示“没有用调用方请求的那张卡片”
	FallbackUsed bool
	// AvailableIDs 解析时刻库里存在的卡片 ID，便于排查“卡片被删了”这类问题
	AvailableIDs []uint
}

// Resolved 是否解析到了一个真实存在的卡片。
func (r ModelResolution) Resolved() bool { return r.ResolvedID > 0 }

// Usable 是否具备发起一次请求的最低条件（BaseUrl + ModelName 都非空）。
func (r ModelResolution) Usable() bool {
	return strings.TrimSpace(r.Config.BaseUrl) != "" && strings.TrimSpace(r.Config.ModelName) != ""
}

// WithThinking 返回一个 thinking 被覆盖后的**副本**。
//
// 关键修复：旧实现写的是 `aiConfig.Thinking = thinkingMode`，直接改写了
// 从 GetSettingConfig() 取出的结构体。请求级的 UI 开关因此会“粘”在配置上，
// 造成“卡片里写着 A、实际按 B 在跑”。
func (r ModelResolution) WithThinking(thinking bool) ModelResolution {
	r.Config.Thinking = thinking
	return r
}

// Describe 一句话说明本次解析结论，可直接用于日志与界面。
func (r ModelResolution) Describe() string {
	if !r.Resolved() {
		return "未解析到可用模型"
	}
	return fmt.Sprintf("%s (卡片 #%d)", r.Reason, r.ResolvedID)
}

func aiConfigDisplayName(c *AIConfig) string {
	if c == nil {
		return "未命名配置"
	}
	if n := strings.TrimSpace(c.Name); n != "" {
		return n
	}
	if m := strings.TrimSpace(c.ModelName); m != "" {
		return m
	}
	return "未命名配置"
}

func findAIConfigByID(configs []*AIConfig, id uint) *AIConfig {
	if id == 0 {
		return nil
	}
	for _, c := range configs {
		if c != nil && c.ID == id {
			return c
		}
	}
	return nil
}

// ResolveModel 解析“本次请求到底该用哪张卡片”。这是唯一入口。
//
// 优先级：
//  1. 调用方显式指定的 aiConfigId（存在即用）
//  2. 设置里的全局默认 ai_config_id（调用方没指定，或指定的卡片已被删除）
//  3. 注册表里第一张存在的卡片
//
// aiConfigId <= 0 表示“未指定”，走 2 → 3。
func ResolveModel(aiConfigId int) (ModelResolution, error) {
	cfg := GetSettingConfig()
	if cfg == nil || len(cfg.AiConfigs) == 0 {
		return ModelResolution{}, errors.New("尚未配置任何 AI 模型，请先在「AI 模型注册表」中添加卡片并保存")
	}

	res := ModelResolution{RequestedID: aiConfigId}
	for _, c := range cfg.AiConfigs {
		if c != nil {
			res.AvailableIDs = append(res.AvailableIDs, c.ID)
		}
	}
	if len(res.AvailableIDs) == 0 {
		return ModelResolution{}, errors.New("AI 模型注册表中没有有效卡片")
	}

	// 1) 显式指定
	if aiConfigId > 0 {
		if c := findAIConfigByID(cfg.AiConfigs, uint(aiConfigId)); c != nil {
			res.Config = *c
			res.ResolvedID = c.ID
			res.Source = ModelSourceExplicit
			res.Reason = fmt.Sprintf("按请求指定的卡片「%s」", aiConfigDisplayName(c))
			return res, nil
		}
	}

	// 2) 全局默认
	if cfg.AiConfigID > 0 {
		if c := findAIConfigByID(cfg.AiConfigs, cfg.AiConfigID); c != nil {
			res.Config = *c
			res.ResolvedID = c.ID
			res.Source = ModelSourceGlobalDefault
			res.FallbackUsed = aiConfigId > 0
			if aiConfigId > 0 {
				res.Reason = fmt.Sprintf(
					"请求的卡片 #%d 已不存在（现存卡片 %v），改用全局默认「%s」",
					aiConfigId, res.AvailableIDs, aiConfigDisplayName(c))
			} else {
				res.Reason = fmt.Sprintf("未指定模型，采用全局默认「%s」", aiConfigDisplayName(c))
			}
			return res, nil
		}
	}

	// 3) 第一张可用
	for _, c := range cfg.AiConfigs {
		if c == nil {
			continue
		}
		res.Config = *c
		res.ResolvedID = c.ID
		res.Source = ModelSourceFirstAvail
		res.FallbackUsed = true
		if aiConfigId > 0 {
			res.Reason = fmt.Sprintf(
				"请求的卡片 #%d 与全局默认均不可用（现存卡片 %v），回落到第一张卡片「%s」",
				aiConfigId, res.AvailableIDs, aiConfigDisplayName(c))
		} else {
			res.Reason = fmt.Sprintf("未指定模型且未设置全局默认，回落到第一张卡片「%s」", aiConfigDisplayName(c))
		}
		return res, nil
	}

	return ModelResolution{}, errors.New("AI 模型注册表中没有可用的卡片")
}

// ResolveModelForced 与 ResolveModel 相同，但额外要求 ApiKey 非空。
// 用于那些“没有密钥就必然失败”的链路（摘要、价格预警等），
// 避免它们再各自维护一套内联解析逻辑。
func ResolveModelForced(aiConfigId int, requireApiKey bool) (ModelResolution, error) {
	res, err := ResolveModel(aiConfigId)
	if err != nil {
		return res, err
	}
	if requireApiKey && (IsMaskedSecret(res.Config.ApiKey) || strings.TrimSpace(res.Config.ApiKey) == "") {
		return res, fmt.Errorf("卡片「%s」未配置有效的 API Key（检测到掩码或空密钥）", aiConfigDisplayName(&res.Config))
	}
	return res, nil
}
