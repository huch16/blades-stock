package data

import (
	"context"
	"fmt"
	"strings"

	"blades-stock/backend/logger"
)

// @Author spark
// @Date 2025/1/16 13:19
// @Desc
// This OpenAI wrapper now supports generic conversation when Prompt is empty, providing a friendly system prompt for everyday topics.

// -----------------------------------------------------------------------------------
type OpenAi struct {
	ctx              context.Context
	BaseUrl          string  `json:"base_url"`
	ApiKey           string  `json:"api_key"`
	Model            string  `json:"model"`
	MaxTokens        int     `json:"max_tokens"`
	Temperature      float64 `json:"temperature"`
	Prompt           string  `json:"prompt"`
	TimeOut          int     `json:"time_out"`
	QuestionTemplate string  `json:"question_template"`
	CrawlTimeOut     int64   `json:"crawl_time_out"`
	KDays            int64   `json:"kDays"`
	BrowserPath      string  `json:"browser_path"`
	HttpProxy        string  `json:"httpProxy"`
	HttpProxyEnabled bool    `json:"httpProxyEnabled"`
	ChatSource       string  `json:"-"`
}

func (o OpenAi) String() string {
	return fmt.Sprintf("OpenAi{BaseUrl: %s, Model: %s, MaxTokens: %d, Temperature: %.2f, Prompt: %s, TimeOut: %d, QuestionTemplate: %s, CrawlTimeOut: %d, KDays: %d, BrowserPath: %s, ApiKey: [MASKED]}",
		o.BaseUrl, o.Model, o.MaxTokens, o.Temperature, o.Prompt, o.TimeOut, o.QuestionTemplate, o.CrawlTimeOut, o.KDays, o.BrowserPath)
}

// DefaultResearchSystemPrompt 是机构级买方量化投研导师系统提示词，强化确定性数据底座、VectorBT量化诊断、买方认知假说、模拟盘净值问责与全天候自主闭环。
const DefaultResearchSystemPrompt = `你是一位拥有20年证券实操经验的资深买方量化投研专家与投资导师，秉持“风险控制第一，数据穿透为本，认知沉淀为核”的原则。
在分析个股、板块或宏观市场时，必须通过工具调用获取真实客观的数据（严禁凭记忆编造数据）：
1. 涉及基本面体检与排雷时，全面核查控股股东质押平仓风险、未来大额限售解禁日程、近3年真实ROE与销售毛利率、经营现金流质量及审计意见，排除财务暴雷隐患；
2. 涉及资金与机构筹码时，穿透近3季度股东户数吸筹/发散趋势（主力吸筹/出货判定）、两融杠杆多空动态，异动个股穿透龙虎榜机构专用席位买卖净额与游资风格；
3. 涉及量化走势与策略时，结合 VectorBT 矩阵回测评分、支撑阻力位与量价形态给出量化胜率和第一止损位；
4. 严格遵循买方假说沉淀范式：对核心逻辑给出明确判定（逻辑强化、发现新逻辑、逻辑无变化、逻辑边际走弱、核心逻辑破裂），明确跟踪催化剂与证伪条件；
5. 收益率问责与模拟盘头寸管理：必要时可通过 GetPaperPortfolioSummary 查看个人模拟盘净值(NAV)与仓位，或通过 ExecutePaperTrade 挂单虚拟交易并设定严密止盈止损线；
6. 闭环回溯与经验自沉淀：主动运用 TriggerPostMortemAudit 对过往推荐进行死后剖析，击穿止损时反思教训并沉淀经验结晶；
7. 跨市场事件级联推演：海外龙头或宏观突发催化时，通过 QuerySupplyChainCascade 沿着产业知识图谱穿透至A股受拉动核心梯队。`

// DefaultStockAnalysisQuestionTemplate 是全维度深度个股分析与排雷提示词模板（涵盖基本面与排雷体检、资金与机构席位追踪、买方假说沉淀）。
const DefaultStockAnalysisQuestionTemplate = `请对 {{stockName}}[{{stockCode}}] 进行全维度深度量化投研、排雷体检与买方假说沉淀：
1. 基本面与排雷体检：核查控股股东质押比例、未来一年限售解禁日程、近3年真实 ROE 与毛利率趋势、经营现金流健康度，排除财务暴雷隐患；
2. 资金与机构席位追踪：穿透近3季度股东户数集中度（主力吸筹/出逃判定）、两融杠杆资金动向，异动时穿透龙虎榜机构专用席位买卖净额与游资动向；
3. 智能量化与买方假说沉淀：结合 VectorBT 量化评分、均线趋势与量价关系给出量化建议与止损位，并在报告末尾提炼买方核心假说、重点跟踪催化与关键证伪条件。`

// NewDeepSeekOpenAi creates an OpenAi client with a sensible default system prompt.
func NewDeepSeekOpenAi(ctx context.Context, aiConfigId int) *OpenAi {
	settingConfig := GetSettingConfig()

	// 统一走 ResolveModel：与 Agent / 摘要 / 价格预警使用**同一套**解析规则。
	// 旧实现找不到 ID 时直接 `aiConfig = &AIConfig{}`（空配置，无任何回落），
	// 于是同一个 aiConfigId 在不同链路上会落到不同的模型（甚至什么都没有）。
	res, resolveErr := ResolveModel(aiConfigId)
	if resolveErr != nil {
		logger.SugaredLogger.Warnf("[OpenAi] 模型解析失败，降级为空配置: %v", resolveErr)
	}
	aiConfig := &res.Config

	if settingConfig.OpenAiEnable {
		if aiConfig.TimeOut <= 0 {
			aiConfig.TimeOut = 60 * 5
		}
		if settingConfig.CrawlTimeOut <= 0 {
			settingConfig.CrawlTimeOut = 60
		}
		if settingConfig.KDays < 30 {
			settingConfig.KDays = 60
		}
	}

	// Use professional research prompt if none configured.
	prompt := strings.TrimSpace(settingConfig.Prompt)
	if prompt == "" {
		prompt = DefaultResearchSystemPrompt
	}

	qTemplate := strings.TrimSpace(settingConfig.QuestionTemplate)
	if qTemplate == "" {
		qTemplate = DefaultStockAnalysisQuestionTemplate
	}

	o := &OpenAi{
		ctx:              ctx,
		BaseUrl:          aiConfig.BaseUrl,
		ApiKey:           aiConfig.ApiKey,
		Model:            aiConfig.ModelName,
		MaxTokens:        aiConfig.MaxTokens,
		Temperature:      aiConfig.Temperature,
		TimeOut:          aiConfig.TimeOut,
		HttpProxy:        aiConfig.HttpProxy,
		HttpProxyEnabled: aiConfig.HttpProxyEnabled,
		Prompt:           prompt,
		QuestionTemplate: qTemplate,
		CrawlTimeOut:     settingConfig.CrawlTimeOut,
		KDays:            settingConfig.KDays,
		BrowserPath:      settingConfig.BrowserPath,
	}
	return o
}
