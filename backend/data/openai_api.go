package data

import (
    "context"
    "fmt"
    "strings"

    "github.com/samber/lo"
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

func (o *OpenAi) Ctx() context.Context     { return o.ctx }
func (o *OpenAi) GetBaseURL() string       { return o.BaseUrl }
func (o *OpenAi) GetAPIKey() string        { return o.ApiKey }
func (o *OpenAi) GetModel() string         { return o.Model }
func (o *OpenAi) GetMaxTokens() int        { return o.MaxTokens }
func (o *OpenAi) GetTemperature() float64  { return o.Temperature }
func (o *OpenAi) GetTimeout() int          { return o.TimeOut }
func (o *OpenAi) IsHttpProxyEnabled() bool { return o.HttpProxyEnabled }
func (o *OpenAi) GetHttpProxy() string     { return o.HttpProxy }

func (o OpenAi) String() string {
    return fmt.Sprintf("OpenAi{BaseUrl: %s, Model: %s, MaxTokens: %d, Temperature: %.2f, Prompt: %s, TimeOut: %d, QuestionTemplate: %s, CrawlTimeOut: %d, KDays: %d, BrowserPath: %s, ApiKey: [MASKED]}",
        o.BaseUrl, o.Model, o.MaxTokens, o.Temperature, o.Prompt, o.TimeOut, o.QuestionTemplate, o.CrawlTimeOut, o.KDays, o.BrowserPath)
}

// DefaultResearchSystemPrompt 是机构级专业投研导师系统提示词，强化 Tushare MCP 黄金工具与三维风控核查。
const DefaultResearchSystemPrompt = `你是一位拥有20年证券实操经验的资深量化投研专家与投资导师，秉持“风险控制第一，数据穿透为本”的原则。
在分析个股、板块或宏观市场时，必须通过工具调用获取真实客观的数据（严禁凭记忆编造数据）：
1. 涉及基本面体检与排雷时，优先组合调用 Tushare MCP 黄金工具：质押平仓风险（pledge_stat）、未来大额限售解禁（share_float）、真实ROE与销售毛利率（fina_indicator）、经营现金流质量（cashflow）、非标审计意见（fina_audit）；
2. 涉及资金与机构筹码时，优先调用：股东户数连续季度吸筹/分散趋势（stk_holdernumber）、两融杠杆买入（margin_detail）、北向资金（moneyflow_hsgt / hsgt_top10），异动个股穿透龙虎榜机构专用席位买卖净额（top_list / top_inst）；
3. 涉及财报前瞻与预期差时，调用业绩预告（forecast）与业绩快报（express）挖掘超预期增长潜能；
4. 任何操作建议均须附带第一支撑位、止损位与核心风险警示。`

// DefaultStockAnalysisQuestionTemplate 是全维度深度个股分析与排雷提示词模板（涵盖基本面与排雷体检、资金与机构席位追踪、财报超预期挖掘）。
const DefaultStockAnalysisQuestionTemplate = `请对 {{stockName}}[{{stockCode}}] 进行全维度深度量化投研与排雷体检：
1. 基本面与排雷体检：调用 Tushare pledge_stat 核查控股股东质押比例，share_float 检查未来一年限售解禁日程，fina_indicator 查询近3年真实 ROE 与毛利率趋势，cashflow 检验经营现金流健康度，排除财务暴雷隐患；
2. 资金与机构席位追踪：调用 stk_holdernumber 穿透近3季度股东户数是集中还是发散（主力吸筹/出逃判定），margin_detail 查看两融杠杆多空动态，异动时调用 top_list 与 top_inst 穿透龙虎榜机构专用席位买卖净额；
3. 财报预期差与前瞻：调用 forecast 与 express 挖掘近期业绩预告是否超预期，结合 K 线趋势给出量化评分、操作建议与第一止损位。`

// NewDeepSeekOpenAi creates an OpenAi client with a sensible default system prompt.
func NewDeepSeekOpenAi(ctx context.Context, aiConfigId int) *OpenAi {
    settingConfig := GetSettingConfig()
    aiConfig, find := lo.Find(settingConfig.AiConfigs, func(item *AIConfig) bool {
        return uint(aiConfigId) == item.ID
    })
    if !find {
        aiConfig = &AIConfig{}
    }

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
        ctx:               ctx,
        BaseUrl:           aiConfig.BaseUrl,
        ApiKey:            aiConfig.ApiKey,
        Model:             aiConfig.ModelName,
        MaxTokens:         aiConfig.MaxTokens,
        Temperature:       aiConfig.Temperature,
        TimeOut:           aiConfig.TimeOut,
        HttpProxy:         aiConfig.HttpProxy,
        HttpProxyEnabled:  aiConfig.HttpProxyEnabled,
        Prompt:            prompt,
        QuestionTemplate: qTemplate,
        CrawlTimeOut:      settingConfig.CrawlTimeOut,
        KDays:             settingConfig.KDays,
        BrowserPath:       settingConfig.BrowserPath,
    }
    return o
}
