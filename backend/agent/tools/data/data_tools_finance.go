package data

import (
	"strings"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/util"

	bladesTools "github.com/go-kratos/blades/tools"
)

func RegisterFinanceTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetStockResearchReport",
		"获取市场分析师的股票研究报告。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			news := data.NewMarketNewsApi()
			res := news.StockResearchReport(stockCode, 30)
			var md strings.Builder
			for _, a := range res {
				d, ok := a.(map[string]any)
				if !ok {
					continue
				}
				infoCode, _ := d["infoCode"].(string)
				md.WriteString(news.GetIndustryReportInfo(infoCode))
			}
			out := strings.TrimSpace(md.String())
			if out == "" {
				return stockCode + "：未查询到相关研究报告。", nil
			}
			return "### " + stockCode + " 研究报告\r\n" + out, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"HotStrategyTable",
		"获取当前热门选股策略",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			result := data.NewSearchStockApi("").HotStrategyTable()
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"HotStockTable",
		"当前热门股票排名",
		map[string]*types.ParamDef{
			"pageSize": {
				Type:     "string",
				Desc:     "分页大小",
				Required: true,
			},
		},
		func(args string) (string, error) {
			// Check trading day
			if !data.IsTodayTradingDay() {
				return "⚠️ 非交易日（周末或节假日），无热门股票数据", nil
			}
			pageSize := types.ExtractStringParam(args, "pageSize", "page_size")
			pageSizeNum := int64(50)
			if pageSize != "" {
				if d, err := parseInt(pageSize); err == nil {
					pageSizeNum = int64(d)
				}
			}
			res := data.NewMarketNewsApi().XUEQIUHotStock(int(pageSizeNum), "10")
			md := util.MarkdownTableWithTitle("当前热门股票排名", res)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockMoneyData",
		"今日股票资金流入排名",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			// 简单交易日判断：周末无数据
			wd := time.Now().Weekday()
			if wd == time.Saturday || wd == time.Sunday {
				return "⚠️ 非交易日（周末），无资金流向数据", nil
			}
			res := data.NewStockDataApi().GetStockMoneyData()
			md := util.MarkdownTableWithTitle("今日个股资金流向Top50", res.Data.Diff)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetMutualTop10Deal",
		"获取北向资金/南向资金十大成交股数据",
		map[string]*types.ParamDef{
			"mutualType": {
				Type:     "string",
				Desc:     "通道类型：001=沪股通，002=港股通(沪)，003=深股通，004=港股通(深)",
				Required: true,
			},
			"tradeDate": {
				Type:     "string",
				Desc:     "交易日期，格式：YYYY-MM-DD",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			mutualType := types.ExtractStringParam(args, "mutualType", "mutual_type", "type")
			tradeDate := types.ExtractStringParam(args, "tradeDate", "trade_date", "date")
			page, _ := types.ExtractIntParam(args, "page", "page_number")
			pageSize, _ := types.ExtractIntParam(args, "pageSize", "page_size")
			if page == 0 {
				page = 1
			}
			if pageSize == 0 {
				pageSize = 10
			}
			result := data.NewStockDataApi().GetMutualTop10Deal(mutualType, tradeDate, page, pageSize)
			title := data.MutualTypeName(mutualType) + " " + tradeDate
			md := util.MarkdownTableWithTitle(title, result.Result.Data)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockFinancialInfo",
		"获取股票财务报表信息",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			result := data.NewStockDataApi().GetStockFinancialInfo(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"财务报表信息", result.Result.Data)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockHolderNum",
		"获取股票股东人数信息",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			result := data.NewStockDataApi().GetStockHolderNum(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"股东人数信息", result.Result.Data)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockHistoryMoneyData",
		"获取股票历史资金流向数据",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			result := data.NewStockDataApi().GetStockHistoryMoneyData(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"历史资金流向数据", result)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockRZRQInfo",
		"获取股票融资融券信息",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			result := data.NewStockDataApi().GetStockRZRQInfo(stockCode)
			md := util.MarkdownTableWithTitle(stockCode+" 融资融券信息", result.Result.Data)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetIndustryValuation",
		"获取行业/板块平均估值和中值",
		map[string]*types.ParamDef{
			"bkName": {
				Type:     "string",
				Desc:     "行业/板块名称，如：半导体",
				Required: true,
			},
		},
		func(args string) (string, error) {
			bkName := types.ExtractStringParam(args, "bkName", "bk_name", "name")
			result := data.NewStockDataApi().GetIndustryValuation(bkName)
			md := util.MarkdownTableWithTitle(bkName+"行业估值", result.Result.Data)
			return md, nil
		},
	))

	return result
}
