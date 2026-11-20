package data

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/models"
	"blades-stock/backend/util"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func RegisterChangesTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetStockChanges",
		"获取股票异动数据，包括火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板等异动类型。",
		map[string]*types.ParamDef{
			"changeTypes": {
				Type:     "string",
				Desc:     "异动类型，多个用逗号分隔：火箭发射=8201,快速反弹=8202,大笔买入=8193,封涨停板=4,打开跌停板=32,有大买盘=64,竞价上涨=8207,高开5日线=8209,向上缺口=8211,60日新高=8213,60日大幅上涨=8215,加速下跌=8204,高台跳水=8203,大笔卖出=8194,封跌停板=8,打开涨停板=16,有大卖盘=128,竞价下跌=8208,低开5日线=8210,向下缺口=8212,60日新低=8214,60日大幅下跌=8216。默认查询火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			changeTypesStr := gjson.Get(args, "changeTypes").String()
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if pageSize <= 0 {
				pageSize = 20
			}

			var changeTypes []int
			if changeTypesStr != "" {
				typeMap := map[string]int{
					"火箭发射": 8201, "快速反弹": 8202, "大笔买入": 8193, "封涨停板": 4,
					"打开跌停板": 32, "有大买盘": 64, "竞价上涨": 8207, "高开5日线": 8209,
					"向上缺口": 8211, "60日新高": 8213, "60日大幅上涨": 8215,
					"加速下跌": 8204, "高台跳水": 8203, "大笔卖出": 8194, "封跌停板": 8,
					"打开涨停板": 16, "有大卖盘": 128, "竞价下跌": 8208, "低开5日线": 8210,
					"向下缺口": 8212, "60日新低": 8214, "60日大幅下跌": 8216,
				}
				for _, t := range strings.Split(changeTypesStr, ",") {
					t = strings.TrimSpace(t)
					if code, ok := typeMap[t]; ok {
						changeTypes = append(changeTypes, code)
					} else if code, err := strconv.Atoi(t); err == nil {
						changeTypes = append(changeTypes, code)
					}
				}
			}

			return data.NewStockChangesApi().GetStockChangesReadable(changeTypes, 0, pageSize), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockChangeHistoryList",
		"查询股票异动历史记录。可以根据股票代码、股票名称、异动类型、日期范围、成交量、金额、涨跌幅、行业、概念等条件筛选历史异动数据。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码筛选（可选），支持模糊匹配",
				Required: false,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称筛选（可选），支持模糊匹配",
				Required: false,
			},
			"changeType": {
				Type:     "integer",
				Desc:     "异动类型代码筛选（可选）。完整列表：8201=火箭发射,8202=快速反弹,8193=大笔买入,4=封涨停板,32=打开跌停板,64=有大买盘,8207=竞价上涨,8209=高开5日线,8211=向上缺口,8213=60日新高,8215=60日大幅上涨,8204=加速下跌,8203=高台跳水,8194=大笔卖出,8=封跌停板,16=打开涨停板,128=有大卖盘,8208=竞价下跌,8210=低开5日线,8212=向下缺口,8214=60日新低,8216=60日大幅下跌",
				Required: false,
			},
			"typeName": {
				Type:     "string",
				Desc:     "异动类型名称筛选（可选）。完整列表：火箭发射、快速反弹、大笔买入、封涨停板、打开跌停板、有大买盘、竞价上涨、高开5日线、向上缺口、60日新高、60日大幅上涨、加速下跌、高台跳水、大笔卖出、封跌停板、打开涨停板、有大卖盘、竞价下跌、低开5日线、向下缺口、60日新低、60日大幅下跌",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式：YYYY-MM-DD（可选）",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式：YYYY-MM-DD（可选）",
				Required: true,
			},
			"startTime": {
				Type:     "string",
				Desc:     "开始时间，格式：HH:MM:SS（可选）",
				Required: true,
			},
			"endTime": {
				Type:     "string",
				Desc:     "结束时间，格式：HH:MM:SS（可选）",
				Required: true,
			},
			"minVolume": {
				Type:     "integer",
				Desc:     "最小成交量筛选（股），如50000表示大于500手（可选）",
				Required: false,
			},
			"minAmount": {
				Type:     "number",
				Desc:     "最小金额筛选（元），如10000000表示大于1000万（可选）",
				Required: false,
			},
			"minChangeRate": {
				Type:     "number",
				Desc:     "最小涨跌幅筛选（%），如5表示涨幅大于5%（可选）",
				Required: false,
			},
			"maxChangeRate": {
				Type:     "number",
				Desc:     "最大涨跌幅筛选（%），如-5表示跌幅小于-5%（可选）",
				Required: false,
			},
			"industry": {
				Type:     "string",
				Desc:     "行业关键词筛选（可选），支持模糊匹配",
				Required: false,
			},
			"concept": {
				Type:     "string",
				Desc:     "概念关键词筛选（可选），支持模糊匹配",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: true,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			stockName := gjson.Get(args, "stockName").String()
			changeType := int(gjson.Get(args, "changeType").Int())
			typeName := gjson.Get(args, "typeName").String()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			startTime := gjson.Get(args, "startTime").String()
			endTime := gjson.Get(args, "endTime").String()
			minVolume := gjson.Get(args, "minVolume").Int()
			minAmount := gjson.Get(args, "minAmount").Float()
			minChangeRate := gjson.Get(args, "minChangeRate").Float()
			maxChangeRate := gjson.Get(args, "maxChangeRate").Float()
			industry := gjson.Get(args, "industry").String()
			concept := gjson.Get(args, "concept").String()

			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			query := models.StockChangeHistoryQuery{
				StockCode:     stockCode,
				StockName:     stockName,
				ChangeType:    changeType,
				TypeName:      typeName,
				StartDate:     startDate,
				EndDate:       endDate,
				Page:          page,
				PageSize:      pageSize,
				StartTime:     startTime,
				EndTime:       endTime,
				MinVolume:     minVolume,
				MinAmount:     minAmount,
				MinChangeRate: minChangeRate,
				MaxChangeRate: maxChangeRate,
				Industry:      industry,
				Concept:       concept,
			}

			pageData, err := data.NewStockChangeHistoryService().GetHistoryList(query)
			if err != nil {
				return "", err
			}

			if pageData == nil || len(pageData.List) == 0 {
				return "未找到符合条件的异动历史记录", nil
			}

			type historyRow struct {
				ChangeTime string  `md:"异动时间"`
				ChangeDate string  `md:"异动日期"`
				StockCode  string  `md:"股票代码"`
				StockName  string  `md:"股票名称"`
				TypeName   string  `md:"异动类型"`
				Volume     int64   `md:"成交量(股)"`
				Price      float64 `md:"价格"`
				ChangeRate float64 `md:"涨跌幅(%)"`
				Amount     float64 `md:"金额"`
				Industry   string  `md:"所属行业"`
				Concept    string  `md:"所属概念"`
			}

			var rows []historyRow
			for _, item := range pageData.List {
				rows = append(rows, historyRow{
					ChangeTime: item.ChangeTime,
					ChangeDate: item.ChangeDate,
					StockCode:  item.StockCode,
					StockName:  item.StockName,
					TypeName:   item.TypeName,
					Volume:     item.Volume,
					Price:      item.Price,
					ChangeRate: item.ChangeRate,
					Amount:     item.Amount,
					Industry:   item.Industry,
					Concept:    item.Concept,
				})
			}

			summary := fmt.Sprintf("共找到 %d 条异动历史记录，当前第 %d/%d 页", pageData.Total, page, pageData.TotalPages)
			return summary + "\n\n" + util.MarkdownTableWithTitle("股票异动历史记录", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetDailyChangeStats",
		"获取近N日每日异动统计趋势，包括每天的上涨异动数、下跌异动数、封涨停数、封跌停数和总异动数。用于分析市场异动活跃度的变化趋势。",
		map[string]*types.ParamDef{
			"days": {
				Type:     "integer",
				Desc:     "查询天数，如7表示近7日，30表示近30日，默认30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 30
			}
			result, err := data.NewStockChangeHistoryService().GetDailyChangeStats(days)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return "暂无异动统计数据", nil
			}
			type row struct {
				ChangeDate string `md:"日期"`
				TotalCount int64  `md:"总异动数"`
				UpCount    int64  `md:"上涨异动"`
				DownCount  int64  `md:"下跌异动"`
				LimitUp    int64  `md:"封涨停"`
				LimitDown  int64  `md:"封跌停"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					ChangeDate: d.ChangeDate,
					TotalCount: d.TotalCount,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					LimitUp:    d.LimitUp,
					LimitDown:  d.LimitDown,
				})
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("近%d日每日异动统计", days), rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetChangeRank",
		"获取异动次数排行榜，支持按股票、行业、概念三个维度排名，区分利好异动（封涨停板、火箭发射、快速反弹等）和利空异动（封跌停板、高台跳水、加速下跌等）。",
		map[string]*types.ParamDef{
			"days": {
				Type:     "integer",
				Desc:     "查询天数，1=当日，3=近3日，5=近5日，10=近10日，默认1",
				Required: false,
			},
			"topN": {
				Type:     "integer",
				Desc:     "返回排名前N个，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 1
			}
			topN := int(gjson.Get(args, "topN").Int())
			if topN <= 0 {
				topN = 20
			}
			result, err := data.NewStockChangeHistoryService().GetChangeRank(days, topN)
			if err != nil {
				return "", err
			}
			periodLabel := "当日"
			if days > 1 {
				periodLabel = fmt.Sprintf("近%d日", days)
			}
			var sb strings.Builder
			if len(result.TopStocks) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					StockName string `md:"股票名称"`
					StockCode string `md:"股票代码"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopStocks {
					rows = append(rows, row{Rank: i + 1, StockName: d.Name, StockCode: d.Code, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的股票", rows))
				sb.WriteString("\n\n")
			}
			if len(result.TopIndustries) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					Industry  string `md:"行业"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopIndustries {
					rows = append(rows, row{Rank: i + 1, Industry: d.Name, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的行业", rows))
				sb.WriteString("\n\n")
			}
			if len(result.TopConcepts) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					Concept   string `md:"概念"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopConcepts {
					rows = append(rows, row{Rank: i + 1, Concept: d.Name, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的概念", rows))
			}
			output := sb.String()
			if output == "" {
				return "暂无异动排行数据", nil
			}
			return output, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetDailyDimensionStats",
		"按维度查询近N日每日异动趋势，支持按股票、行业、概念、异动类型四个维度查询，返回每天的利好异动数、利空异动数和总异动数。用于分析某个股票/行业/概念/异动类型在一段时间内的异动变化趋势。",
		map[string]*types.ParamDef{
			"dimension": {
				Type:     "string",
				Desc:     "查询维度：stock=按股票，industry=按行业，concept=按概念，type=按异动类型",
				Required: true,
			},
			"name": {
				Type:     "string",
				Desc:     "维度名称，如股票名称/代码、行业名称、概念名称、异动类型名称",
				Required: true,
			},
			"days": {
				Type:     "integer",
				Desc:     "查询天数，默认30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			dimension := gjson.Get(args, "dimension").String()
			name := gjson.Get(args, "name").String()
			days := int(gjson.Get(args, "days").Int())
			if dimension == "" || name == "" {
				return "请提供dimension和name参数", nil
			}
			if days <= 0 {
				days = 30
			}
			result, err := data.NewStockChangeHistoryService().GetDailyDimensionStats(dimension, name, days)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return fmt.Sprintf("未找到%s[%s]的异动趋势数据", dimension, name), nil
			}
			type row struct {
				ChangeDate string `md:"日期"`
				UpCount    int64  `md:"利好异动"`
				DownCount  int64  `md:"利空异动"`
				TotalCount int64  `md:"总异动数"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					ChangeDate: d.ChangeDate,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					TotalCount: d.TotalCount,
				})
			}
			dimLabels := map[string]string{"stock": "股票", "industry": "行业", "concept": "概念", "type": "异动类型"}
			title := fmt.Sprintf("%s[%s]近%d日异动趋势", dimLabels[dimension], name, days)
			return util.MarkdownTableWithTitle(title, rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetTypeStatsByDate",
		"查询某一天的异动类型分布统计，返回该天每种异动类型的利好/利空次数和总次数。用于分析某天的市场异动结构。",
		map[string]*types.ParamDef{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：YYYY-MM-DD，如2025-04-13",
				Required: true,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			if date == "" {
				return "请提供date参数", nil
			}
			result, err := data.NewStockChangeHistoryService().GetTypeStatsByDate(date)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return fmt.Sprintf("未找到%s的异动类型分布数据", date), nil
			}
			type row struct {
				TypeName   string `md:"异动类型"`
				UpCount    int64  `md:"利好异动"`
				DownCount  int64  `md:"利空异动"`
				TotalCount int64  `md:"总次数"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					TypeName:   d.TypeName,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					TotalCount: d.TotalCount,
				})
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("%s异动类型分布", date), rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"StockEarningsReview",
		"个股业绩点评。获取业绩点评报告，含营收/利润分析、财务指标解读。支持沪深京港美股。用于个股业绩点评、财报分析、业绩报告类问题。",
		map[string]*types.ParamDef{
			"query": {
				Type:     "string",
				Desc:     "股票名称或代码，如：贵州茅台、600519、宁德时代等",
				Required: true,
			},
			"reportDate": {
				Type:     "string",
				Desc:     "报告期，格式YYYY-MM-DD，如：2024-12-31。不填则使用最新报告期",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			reportDate := gjson.Get(args, "reportDate").String()
			if query == "" {
				return "请输入股票名称或代码", nil
			}
			result := data.NewEmAPI().EarningsReviewToMarkdown(query, reportDate)
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"FinancialQA",
		"金融智能问答。覆盖数据查询、资讯搜索、宏观分析、选股选基、金融百科、市场分析、热点解读。用于自然语言金融问题：查数据、分析、选股、XX是什么、最新政策、宏观数据等。",
		map[string]*types.ParamDef{
			"question": {
				Type:     "string",
				Desc:     "用户自然语言问题，如：今天A股市场表现如何、贵州茅台最新估值、近三年ROE最高的消费股有哪些等",
				Required: true,
			},
			"deepThink": {
				Type:     "boolean",
				Desc:     "是否开启深度思考模式，当用户明确要求深度分析、详细分析、仔细想想时设为true",
				Required: false,
			},
		},
		func(args string) (string, error) {
			question := gjson.Get(args, "question").String()
			deepThink := gjson.Get(args, "deepThink").Bool()
			if question == "" {
				return "请输入您想问的问题", nil
			}
			result := data.NewEmAPI().FinancialQAToMarkdown(question, deepThink)
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"IndustryResearch",
		"行业研究报告生成。按行业关键词生成深度报告：行业概况、市场规模、竞争格局、发展趋势、投资建议。用于行业研究报告、深度分析、产业/趋势分析。",
		map[string]*types.ParamDef{
			"query": {
				Type:     "string",
				Desc:     "行业关键词，如：半导体、新能源汽车、AI芯片、消费电子等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入行业关键词", nil
			}
			result := data.NewEmAPI().IndustryResearchToMarkdown(query)
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"TrackingReport",
		"个股/行业跟踪报告。按股票或行业关键词生成跟踪报告：最新动态、核心观点、关键指标变化、重要事件。支持A/港/美股及行业板块。用于跟踪报告、最新动态、个股/行业跟踪。",
		map[string]*types.ParamDef{
			"query": {
				Type:     "string",
				Desc:     "股票名称/代码或行业关键词，如：贵州茅台、600519、半导体行业等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入股票名称/代码或行业关键词", nil
			}
			result := data.NewEmAPI().TrackingReportToMarkdown(query)
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"FinanceDataQuery",
		"金融数据查询。自然语言查询金融结构化数据：A股/港股/美股/基金/债券，含实时行情、公司信息、估值指标、财务报表。单次最多5个实体。用于查询具体数据/指标/财务/行情。",
		map[string]*types.ParamDef{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询，如：贵州茅台最近一年的营业收入和净利润、沪深300当前点位和成交额、东方财富和拼多多最近一年的营收等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入查询内容", nil
			}
			result := data.NewEmAPI().FinanceDataQueryToMarkdown(query)
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"FinanceSearch",
		"金融资讯搜索。自然语言搜索最新公告、研报、财经新闻、交易所动态及官方政策，覆盖全球标的。用于热点捕捉、舆情监控、研报速览、公告精读、投资决策。",
		map[string]*types.ParamDef{
			"query": {
				Type:     "string",
				Desc:     "自然语言搜索查询，如：格力电器最新研报与公告、商业航天板块近期新闻、美联储加息对A股影响等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入搜索内容", nil
			}
			result := data.NewEmAPI().FinanceSearchToMarkdown(query)
			return result, nil
		},
	))

	f10Tools := []struct {
		name      string
		desc      string
		paramDesc string
		handler   func(string) string
	}{
		{"GetStockLatestFinance", "获取股票最新财务主要数据，包括每股收益(EPS)、每股净资产(BPS)、净资产收益率(ROE)、营业收入、净利润及同比/环比增速等。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockLatestFinanceToMarkdown},
		{"GetStockQtrMainFinance", "获取股票季度主要财务指标，包括EPS、BPS、营业收入、净利润、同比增长率、ROE、毛利率等按季度列示。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockQtrMainFinanceToMarkdown},
		{"GetStockOrgPredict", "获取股票机构预测数据，包括各券商/机构对未来数年的EPS和PE预测明细。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockOrgPredictToMarkdown},
		{"GetStockPredictSummary", "获取股票机构预测汇总，按年度汇总多家机构的EPS预测均值、增长率和PE估值。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockPredictSummaryToMarkdown},
		{"GetStockValuationPercentile", "获取股票估值百分位数据，展示当前PE在历史30%/50%/70%分位的值，判断估值高低。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockValuationPercentileToMarkdown},
		{"GetStockMarginTrading", "获取股票融资融券数据，包括融资买入额、融资余额、融券卖出量、融券余额等按日列示。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockMarginTradingToMarkdown},
		{"GetStockBlockTrade", "获取股票大宗交易数据，包括成交价、溢价率、成交金额、买方/卖方营业部等。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockBlockTradeToMarkdown},
		{"GetStockHolderTrend", "获取股票户均持股趋势数据，展示股东户数和户均持股数量随时间的变化趋势。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockHolderTrendToMarkdown},
		{"GetStockBillboard", "获取股票龙虎榜数据，包括上榜日期、上榜原因、买入/卖出总额等。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockBillboardToMarkdown},
		{"GetStockOperationDeptTrade", "获取股票营业部买卖明细，展示各营业部在龙虎榜上的买入/卖出金额和占比。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockOperationDeptTradeToMarkdown},
	}

	for _, t := range f10Tools {
		tool := t
		result = append(result, NewDataToolWrapper(
			tool.name,
			tool.desc,
			map[string]*types.ParamDef{
				"stockCode": {
					Type:     "string",
					Desc:     tool.paramDesc,
					Required: true,
				},
			},
			func(args string) (string, error) {
				stockCode := gjson.Get(args, "stockCode").String()
				if stockCode == "" {
					return "请输入股票代码", nil
				}
				return tool.handler(stockCode), nil
			},
		))
	}

	result = append(result, NewDataToolWrapper(
		"ComparableCompanyAnalysis",
		"可比公司分析(东方财富妙想)。对指定公司进行可比公司分析，包括财务指标对比和估值对比，帮助判断公司相对估值水平。",
		map[string]*types.ParamDef{
			"query": {
				Type:     "string",
				Desc:     "公司名称或股票代码，如：贵州茅台、东方财富",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入公司名称或股票代码", nil
			}
			return data.NewEmAPI().ComparableCompanyAnalysisToMarkdown(query), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"HotspotDiscovery",
		"市场热点发现(东方财富妙想)。发现当前A股市场热点板块和题材，包括热点逻辑分析和相关个股。",
		map[string]*types.ParamDef{
			"question": {
				Type:     "string",
				Desc:     "热点的自然语言描述，如：今日热点、新能源热点、AI概念热点",
				Required: true,
			},
		},
		func(args string) (string, error) {
			question := gjson.Get(args, "question").String()
			if question == "" {
				return "请输入热点描述", nil
			}
			return data.NewEmAPI().HotspotDiscoveryToMarkdown(question), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetUplimitLadder",
		"获取连板梯队数据：连板统计和各层级股票列表（代码、名称、封单比、成交额、市值、概念板块）。用于连板高度、市场情绪、龙头股识别。",
		map[string]*types.ParamDef{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			// Check trading day
			if date == "" && !data.IsTodayTradingDay() {
				return "⚠️ 非交易日（周末或节假日），无涨停板块数据。请指定一个交易日日期（如 2026-07-04）", nil
			}
			if date != "" && !data.IsTradingDay(date) {
				return fmt.Sprintf("⚠️ %s 非交易日，无涨停板块数据", date), nil
			}
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 连板梯队\n\n", date))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			stocksStr, _ := dataMap["stocks"].(string)
			stockList := strings.Split(stocksStr, ",")
			ztCount := 0
			for _, s := range stockList {
				if strings.TrimSpace(s) != "" {
					ztCount++
				}
			}
			maxCount, _ := dataMap["max_count"].(float64)
			sb.WriteString(fmt.Sprintf("**涨停总数**: %d只 | **最高连板**: %d\n\n", ztCount, int(maxCount)))
			banInfo, _ := dataMap["ban_info"].(map[string]any)
			if len(banInfo) > 0 {
				sb.WriteString("## 连板统计\n\n")
				sb.WriteString("| 连板层级 | 数量 |\n|:---:|:---:|\n")
				for i := int(maxCount); i >= 1; i-- {
					if info, ok := banInfo[fmt.Sprintf("%d", i)].(map[string]any); ok {
						cnt, _ := info["count"].(float64)
						sb.WriteString(fmt.Sprintf("| %d连板 | %d |\n", i, int(cnt)))
					}
				}
				sb.WriteString("\n")
			}
			plateStocks, _ := dataMap["plate_stocks"].(map[string]any)
			stockInfo, _ := dataMap["stock_info"].(map[string]any)
			if len(banInfo) > 0 && len(plateStocks) > 0 {
				sb.WriteString("## 连板梯队详情\n\n")
				for i := int(maxCount); i >= 1; i-- {
					if info, ok := banInfo[fmt.Sprintf("%d", i)].(map[string]any); ok {
						cnt, _ := info["count"].(float64)
						if int(cnt) == 0 {
							continue
						}
						sb.WriteString(fmt.Sprintf("### %d连板（%d只）\n\n", i, int(cnt)))
						sb.WriteString("| 代码 | 名称 | 类型 | 描述 | 时间 | 封单比 | 收盘封单 | 成交额 | 市值 | 概念板块 |\n|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
						seen := make(map[string]bool)
						for _, pStocks := range plateStocks {
							for _, s := range pStocks.([]any) {
								sm, _ := s.(map[string]any)
								keepTimes, _ := sm["up_limit_keep_times"].(float64)
								if int(keepTimes) != i {
									continue
								}
								sCode, _ := sm["stock_code"].(string)
								if seen[sCode] {
									continue
								}
								seen[sCode] = true
								sName, _ := sm["stock_name"].(string)
								upType, _ := sm["up_limit_type"].(string)
								upDesc, _ := sm["up_limit_desc"].(string)
								upTime, _ := sm["up_limit_time"].(string)
								fdMax := floatOrDefault(sm["fd_max"])
								fdClose := floatOrDefault(sm["fd_close"])
								amount := floatOrDefault(sm["amount"])
								marketC := floatOrDefault(sm["market_c"])
								platesStr := getPlatesStr(stockInfo, sCode)
								sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %.2f%% | %.2f%% | %.2f亿 | %.2f亿 | %s |\n",
									sCode, sName, upType, upDesc, upTime, fdMax, fdClose, amount, marketC, platesStr))
							}
						}
						sb.WriteString("\n")
					}
				}
			}
			return sb.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetUplimitHotPlates",
		"获取涨停热门板块排名和接力主线数据：板块热度得分、涨停数、炸板数、接力主线。用于板块轮动、市场热点方向、主线题材分析。",
		map[string]*types.ParamDef{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			// Check trading day
			if date == "" && !data.IsTodayTradingDay() {
				return "⚠️ 非交易日（周末或节假日），无涨停板块数据。请指定一个交易日日期（如 2026-07-04）", nil
			}
			if date != "" && !data.IsTradingDay(date) {
				return fmt.Sprintf("⚠️ %s 非交易日，无涨停板块数据", date), nil
			}
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 热门板块\n\n", date))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			plateStocks, _ := dataMap["plate_stocks"].(map[string]any)
			plateStocksZb, _ := dataMap["plate_stocks_zb"].(map[string]any)
			plateArr, _ := dataMap["plate"].([]any)
			if len(plateArr) > 0 {
				sb.WriteString("## 热门板块TOP20\n\n")
				sb.WriteString("| 排名 | 板块 | 热度得分 | 涨停数 | 炸板数 |\n|:---:|:---:|:---:|:---:|:---:|\n")
				for idx, p := range plateArr {
					if arr, ok := p.([]any); ok && len(arr) >= 3 {
						name, _ := arr[0].(string)
						pCode, _ := arr[1].(string)
						score, _ := arr[2].(float64)
						ztN := 0
						if ps, ok := plateStocks[pCode].([]any); ok {
							ztN = len(ps)
						}
						zbN := 0
						if ps, ok := plateStocksZb[pCode].([]any); ok {
							zbN = len(ps)
						}
						sb.WriteString(fmt.Sprintf("| %d | %s | %d | %d | %d |\n", idx+1, name, int(score), ztN, zbN))
					}
				}
				sb.WriteString("\n")
			}
			plateInfo, _ := dataMap["plate_info"].(map[string]any)
			relay, _ := dataMap["relay"].(map[string]any)
			if area, ok := relay["area"].([]any); ok && len(area) > 0 {
				sb.WriteString("## 接力主线\n\n")
				sb.WriteString("| 板块 | 热度 | 涨停数 |\n|:---:|:---:|:---:|\n")
				for _, a := range area {
					am, _ := a.(map[string]any)
					pCode, _ := am["p_code"].(string)
					pScore, _ := am["p_score"].(float64)
					count, _ := am["count"].(float64)
					pName := pCode
					if pi, ok := plateInfo[pCode].(map[string]any); ok {
						pName, _ = pi["name"].(string)
					}
					sb.WriteString(fmt.Sprintf("| %s | %d | %d |\n", pName, int(pScore), int(count)))
				}
				sb.WriteString("\n")
			}
			return sb.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetUplimitHotStocks",
		"获取涨停个股热度排行数据，包括股票代码、名称、热度得分、概念板块等。适用于分析个股受关注程度、市场人气股、热门标的等场景。当用户提到个股热度、热门个股、人气股、关注度等关键词时使用此工具。",
		map[string]*types.ParamDef{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
			"limit": {
				Type:     "number",
				Desc:     "返回数量，默认30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 30
			}
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 个股热度排行\n\n", date))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			stocksHot, _ := dataMap["stocks_hot"].(map[string]any)
			hotN, _ := dataMap["stocks_hot_n"].(float64)
			if len(stocksHot) == 0 {
				sb.WriteString("暂无个股热度数据\n")
				return sb.String(), nil
			}
			sb.WriteString(fmt.Sprintf("热度≥%d为超级热门\n\n", int(hotN)))
			type hotItem struct {
				code  string
				score float64
			}
			var hotList []hotItem
			for code, score := range stocksHot {
				s, _ := score.(float64)
				hotList = append(hotList, hotItem{code, s})
			}
			sort.Slice(hotList, func(i, j int) bool {
				return hotList[i].score > hotList[j].score
			})
			plateStocks, _ := dataMap["plate_stocks"].(map[string]any)
			stockInfo, _ := dataMap["stock_info"].(map[string]any)
			sb.WriteString("| 排名 | 代码 | 名称 | 热度 | 概念板块 |\n|:---:|:---:|:---:|:---:|:---:|\n")
			for idx, item := range hotList {
				if idx >= limit {
					break
				}
				platesStr := getPlatesStr(stockInfo, item.code)
				sName := getStockNameFromPlateStocks(plateStocks, item.code)
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %d | %s |\n", idx+1, item.code, sName, int(item.score), platesStr))
			}
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetUplimitExplodedStocks",
		"获取炸板股数据，即曾经涨停但未能封住的股票列表，包括代码、名称、炸板时间、概念板块等。适用于分析封板失败、市场分歧、抛压较重等场景。当用户提到炸板、封板失败、开板、破板等关键词时使用此工具。",
		map[string]*types.ParamDef{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			// Check trading day
			if date == "" && !data.IsTodayTradingDay() {
				return "⚠️ 非交易日（周末或节假日），无涨停板块数据。请指定一个交易日日期（如 2026-07-04）", nil
			}
			if date != "" && !data.IsTradingDay(date) {
				return fmt.Sprintf("⚠️ %s 非交易日，无涨停板块数据", date), nil
			}
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 炸板股\n\n", date))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			plateStocksZb, _ := dataMap["plate_stocks_zb"].(map[string]any)
			stockInfo, _ := dataMap["stock_info"].(map[string]any)
			var zbTotal []map[string]any
			for _, stocks := range plateStocksZb {
				if arr, ok := stocks.([]any); ok {
					for _, s := range arr {
						if sm, ok := s.(map[string]any); ok {
							zbTotal = append(zbTotal, sm)
						}
					}
				}
			}
			if len(zbTotal) == 0 {
				sb.WriteString("今日无炸板股\n")
				return sb.String(), nil
			}
			sb.WriteString(fmt.Sprintf("共%d只炸板股\n\n", len(zbTotal)))
			sb.WriteString("| 代码 | 名称 | 时间 | 概念板块 |\n|:---:|:---:|:---:|:---:|\n")
			zbSeen := make(map[string]bool)
			for _, sm := range zbTotal {
				sCode, _ := sm["stock_code"].(string)
				if zbSeen[sCode] {
					continue
				}
				zbSeen[sCode] = true
				sName, _ := sm["stock_name"].(string)
				upTime, _ := sm["up_limit_time"].(string)
				platesStr := getPlatesStr(stockInfo, sCode)
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", sCode, sName, upTime, platesStr))
			}
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetUplimitPlateStocks",
		"获取指定板块的涨停股详情：代码、名称、连板数、封单比、成交额、市值、概念板块。用于分析板块涨停个股，需提供板块名称。",
		map[string]*types.ParamDef{
			"plate_name": {
				Type:     "string",
				Desc:     "板块名称，如：人工智能、机器人、芯片等",
				Required: true,
			},
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			plateName := gjson.Get(args, "plate_name").String()
			if plateName == "" {
				return "请提供板块名称参数 plate_name", nil
			}
			date := gjson.Get(args, "date").String()
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 板块【%s】涨停股详情\n\n", date, plateName))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			plateInfo, _ := dataMap["plate_info"].(map[string]any)
			plateStocks, _ := dataMap["plate_stocks"].(map[string]any)
			stockInfo, _ := dataMap["stock_info"].(map[string]any)
			var targetCode string
			for pCode, pi := range plateInfo {
				if piMap, ok := pi.(map[string]any); ok {
					name, _ := piMap["name"].(string)
					if name == plateName {
						targetCode = pCode
						break
					}
				}
			}
			if targetCode == "" {
				plateArr, _ := dataMap["plate"].([]any)
				for _, p := range plateArr {
					if arr, ok := p.([]any); ok && len(arr) >= 3 {
						name, _ := arr[0].(string)
						pCode, _ := arr[1].(string)
						if name == plateName {
							targetCode = pCode
							break
						}
					}
				}
			}
			if targetCode == "" {
				sb.WriteString(fmt.Sprintf("未找到板块【%s】，请检查板块名称是否正确\n", plateName))
				return sb.String(), nil
			}
			stocks, ok := plateStocks[targetCode].([]any)
			if !ok || len(stocks) == 0 {
				sb.WriteString(fmt.Sprintf("板块【%s】暂无涨停股\n", plateName))
				return sb.String(), nil
			}
			sb.WriteString(fmt.Sprintf("共%d只涨停股\n\n", len(stocks)))
			sb.WriteString("| 代码 | 名称 | 连板 | 类型 | 描述 | 时间 | 封单比 | 收盘封单 | 成交额 | 市值 | 概念板块 |\n|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
			for _, s := range stocks {
				sm, _ := s.(map[string]any)
				sCode, _ := sm["stock_code"].(string)
				sName, _ := sm["stock_name"].(string)
				keepTimes, _ := sm["up_limit_keep_times"].(float64)
				upType, _ := sm["up_limit_type"].(string)
				upDesc, _ := sm["up_limit_desc"].(string)
				upTime, _ := sm["up_limit_time"].(string)
				fdMax := floatOrDefault(sm["fd_max"])
				fdClose := floatOrDefault(sm["fd_close"])
				amount := floatOrDefault(sm["amount"])
				marketC := floatOrDefault(sm["market_c"])
				platesStr := getPlatesStr(stockInfo, sCode)
				sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s | %s | %.2f%% | %.2f%% | %.2f亿 | %.2f亿 | %s |\n",
					sCode, sName, int(keepTimes), upType, upDesc, upTime, fdMax, fdClose, amount, marketC, platesStr))
			}
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	return result
}

func fetchUplimitData(date string) (map[string]any, error) {
	if date == "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		date = time.Now().In(loc).Format("2006-01-02")
	}
	result := data.NewMarketNewsApi().GetUplimitHot(date, 20)
	if result == nil || result["code"] == nil {
		return nil, fmt.Errorf("获取涨停梯队数据失败")
	}
	code, _ := result["code"].(float64)
	if int(code) != 20000 {
		msg, _ := result["message"].(string)
		return nil, fmt.Errorf("获取涨停梯队数据失败: %s", msg)
	}
	dataMap, ok := result["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("涨停梯队数据格式异常")
	}
	return dataMap, nil
}

func floatOrDefault(val any) float64 {
	if f, ok := val.(float64); ok {
		return f
	}
	return 0
}

func getPlatesStr(stockInfo map[string]any, code string) string {
	if si, ok := stockInfo[code].(map[string]any); ok {
		if pa, ok := si["plates"].([]any); ok {
			var ps []string
			for _, p := range pa {
				ps = append(ps, fmt.Sprintf("%v", p))
			}
			return strings.Join(ps, ",")
		}
	}
	return ""
}

func getStockNameFromPlateStocks(plateStocks map[string]any, code string) string {
	for _, pStocks := range plateStocks {
		if arr, ok := pStocks.([]any); ok {
			for _, s := range arr {
				if sm, ok := s.(map[string]any); ok {
					if sm["stock_code"] == code {
						name, _ := sm["stock_name"].(string)
						return name
					}
				}
			}
		}
	}
	return ""
}
