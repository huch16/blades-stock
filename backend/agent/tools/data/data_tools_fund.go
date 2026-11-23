package data

import (
	"fmt"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/util"

	bladesTools "github.com/go-kratos/blades/tools"
)

func RegisterFundTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"SearchFund",
		"搜索基金信息，支持按基金代码或名称模糊搜索",
		map[string]*types.ParamDef{
			"keyword": {
				Type:     "string",
				Desc:     "搜索关键词（基金代码或名称）",
				Required: true,
			},
		},
		func(args string) (string, error) {
			keyword := types.ExtractStringParam(args, "keyword", "query", "searchWords", "keyWord")
			if keyword == "" {
				return "请输入搜索关键词", nil
			}
			funds := data.NewFundApi().GetFundList(keyword)
			if len(funds) == 0 {
				return "未找到相关基金，请检查关键词是否正确", nil
			}
			type fundRow struct {
				Code        string   `md:"基金代码"`
				Name        string   `md:"基金名称"`
				Type        string   `md:"基金类型"`
				Manager     string   `md:"基金经理"`
				NetGrowth1  *float64 `md:"近1月涨幅"`
				NetGrowth3  *float64 `md:"近3月涨幅"`
				NetGrowth6  *float64 `md:"近6月涨幅"`
				NetGrowth12 *float64 `md:"近1年涨幅"`
			}
			var rows []fundRow
			for _, f := range funds {
				rows = append(rows, fundRow{
					Code:        f.Code,
					Name:        f.Name,
					Type:        f.Type,
					Manager:     f.Manager,
					NetGrowth1:  f.NetGrowth1,
					NetGrowth3:  f.NetGrowth3,
					NetGrowth6:  f.NetGrowth6,
					NetGrowth12: f.NetGrowth12,
				})
			}
			return util.MarkdownTableWithTitle("基金搜索结果", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetFundInfo",
		"获取基金详细信息，包括净值、涨跌幅、评级等",
		map[string]*types.ParamDef{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 000001",
				Required: true,
			},
		},
		func(args string) (string, error) {
			fundCode := types.ExtractFundCode(args)
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			fund, err := data.NewFundApi().CrawlFundBasic(fundCode)
			if err != nil || fund.Code == "" {
				return "未找到该基金信息，请检查基金代码是否正确", nil
			}
			type fundDetailRow struct {
				Code           string   `md:"基金代码"`
				Name           string   `md:"基金名称"`
				FullName       string   `md:"基金全称"`
				Type           string   `md:"基金类型"`
				Establishment  string   `md:"成立日期"`
				Scale          string   `md:"最新规模(亿)"`
				Company        string   `md:"基金管理人"`
				Manager        string   `md:"基金经理"`
				Rating         string   `md:"基金评级"`
				TrackingTarget string   `md:"跟踪标的"`
				NetUnitValue   *float64 `md:"单位净值"`
				NetAccumulated *float64 `md:"累计净值"`
				NetGrowth1     *float64 `md:"近1月涨幅(%)"`
				NetGrowth3     *float64 `md:"近3月涨幅(%)"`
				NetGrowth6     *float64 `md:"近6月涨幅(%)"`
				NetGrowth12    *float64 `md:"近1年涨幅(%)"`
				NetGrowthYTD   *float64 `md:"今年来涨幅(%)"`
			}
			row := fundDetailRow{
				Code:           fund.Code,
				Name:           fund.Name,
				FullName:       fund.FullName,
				Type:           fund.Type,
				Establishment:  fund.Establishment,
				Scale:          fund.Scale,
				Company:        fund.Company,
				Manager:        fund.Manager,
				Rating:         fund.Rating,
				TrackingTarget: fund.TrackingTarget,
				NetUnitValue:   fund.NetUnitValue,
				NetAccumulated: fund.NetAccumulated,
				NetGrowth1:     fund.NetGrowth1,
				NetGrowth3:     fund.NetGrowth3,
				NetGrowth6:     fund.NetGrowth6,
				NetGrowth12:    fund.NetGrowth12,
				NetGrowthYTD:   fund.NetGrowthYTD,
			}
			return util.MarkdownTableWithTitle(fund.Name+" ("+fund.Code+") 基金详细信息", row), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetFundKLine",
		"获取基金K线数据，支持多周期(日K/周K/月K/年K等)。场内基金(ETF/LOF)使用4层数据源fallback，场外基金从东方财富历史净值接口获取",
		map[string]*types.ParamDef{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 510050(场内ETF)、000001(场外基金)",
				Required: true,
			},
			"klt": {
				Type:     "string",
				Desc:     "K线周期: 101=日K, 102=周K, 103=月K, 104=年K",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "返回数据条数，默认100",
				Required: false,
			},
		},
		func(args string) (string, error) {
			fundCode := types.ExtractFundCode(args)
			klt := types.ExtractStringParam(args, "klt", "period", "klineType")
			limit, _ := types.ExtractIntParam(args, "limit", "count")
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			if klt == "" {
				klt = "101"
			}
			if limit <= 0 {
				limit = 100
			}
			result := data.NewFundKLineApi().GetFundKLine(fundCode, klt, int(limit))
			if result == nil || result.Data == nil || len(*result.Data) == 0 {
				return "未获取到该基金的K线数据", nil
			}
			type klineRow struct {
				Day           string `md:"日期"`
				Open          string `md:"开盘价"`
				Close         string `md:"收盘价"`
				High          string `md:"最高价"`
				Low           string `md:"最低价"`
				Volume        string `md:"成交量"`
				ChangePercent string `md:"涨跌幅(%)"`
			}
			var rows []klineRow
			klineData := *result.Data
			startIdx := 0
			if len(klineData) > 20 {
				startIdx = len(klineData) - 20
			}
			for i := startIdx; i < len(klineData); i++ {
				item := klineData[i]
				rows = append(rows, klineRow{
					Day:           item.Day,
					Open:          item.Open,
					Close:         item.Close,
					High:          item.High,
					Low:           item.Low,
					Volume:        item.Volume,
					ChangePercent: item.ChangePercent,
				})
			}
			source := result.Source
			if source == "" {
				source = "未知"
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("基金 %s K线数据(最近20条, 来源:%s, 总%d条)", fundCode, source, len(klineData)), rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetFundHistoryNetValue",
		"获取基金历史净值数据。场外基金从东方财富API获取，场内基金(ETF/LOF)从K线收盘价换算",
		map[string]*types.ParamDef{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 000001",
				Required: true,
			},
			"pageIndex": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式 YYYY-MM-DD",
				Required: false,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式 YYYY-MM-DD",
				Required: false,
			},
		},
		func(args string) (string, error) {
			fundCode := types.ExtractFundCode(args)
			pageIndex, _ := types.ExtractIntParam(args, "pageIndex", "page_index", "page")
			pageSize, _ := types.ExtractIntParam(args, "pageSize", "page_size")
			startDate := types.ExtractStringParam(args, "startDate", "start_date")
			endDate := types.ExtractStringParam(args, "endDate", "end_date")
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			if pageIndex <= 0 {
				pageIndex = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}
			values, err := data.NewFundApi().GetFundHistoryNetValue(fundCode, int(pageIndex), int(pageSize), startDate, endDate)
			if err != nil {
				return fmt.Sprintf("获取基金历史净值失败: %v", err), nil
			}
			if len(values) == 0 {
				return "未获取到该基金的历史净值数据", nil
			}
			type netValueRow struct {
				Date        string  `md:"日期"`
				NetValue    float64 `md:"单位净值"`
				AccumValue  float64 `md:"累计净值"`
				DailyGrowth float64 `md:"日增长率(%)"`
			}
			var rows []netValueRow
			for _, v := range values {
				rows = append(rows, netValueRow{
					Date:        v.Date,
					NetValue:    v.NetValue,
					AccumValue:  v.AccumValue,
					DailyGrowth: v.DailyGrowth,
				})
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("基金 %s 历史净值(第%d页, 每页%d条)", fundCode, pageIndex, pageSize), rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetFundTop10Holdings",
		"获取基金前十大重仓持股信息，包括股票代码、名称、持仓占比、实时股价和涨跌幅",
		map[string]*types.ParamDef{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 000001",
				Required: true,
			},
		},
		func(args string) (string, error) {
			fundCode := types.ExtractFundCode(args)
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			holdings, err := data.NewFundApi().GetFundTop10Holdings(fundCode)
			if err != nil {
				return fmt.Sprintf("获取基金十大持仓股失败: %v", err), nil
			}
			if len(holdings) == 0 {
				return "未获取到该基金的持仓数据", nil
			}
			type holdingRow struct {
				Rank       int      `md:"排名"`
				StockCode  string   `md:"股票代码"`
				StockName  string   `md:"股票名称"`
				Market     string   `md:"市场"`
				Ratio      float64  `md:"持仓占比(%)"`
				Price      *float64 `md:"最新价"`
				ChangeRate *float64 `md:"涨跌幅(%)"`
				Quarter    string   `md:"报告期"`
			}
			var rows []holdingRow
			for _, h := range holdings {
				rows = append(rows, holdingRow{
					Rank:       h.Rank,
					StockCode:  h.StockCode,
					StockName:  h.StockName,
					Market:     h.Market,
					Ratio:      h.Ratio,
					Price:      h.Price,
					ChangeRate: h.ChangeRate,
					Quarter:    h.Quarter,
				})
			}
			quarter := holdings[0].Quarter
			if quarter == "" {
				quarter = "最新"
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("基金 %s 十大重仓股(%s)", fundCode, quarter), rows), nil
		},
	))

	return result
}
