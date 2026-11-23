package data

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/util"

	bladesTools "github.com/go-kratos/blades/tools"
)

func RegisterWallstreetcnTdxTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetWallstreetcnLives",
		"获取华尔街见闻实时快讯。支持全球7x24、A股、美股、港股、外汇、商品、黄金、原油、债券、加密货币等频道。数据来源：华尔街见闻(wallstreetcn.com)。",
		map[string]*types.ParamDef{
			"channel": {
				Type:     "string",
				Desc:     "频道：global-channel=全球7x24, a-stock-channel=A股, us-stock-channel=美股, hk-stock-channel=港股, forex-channel=外汇, commodity-channel=商品, goldc-channel=黄金, oil-channel=原油, bond-channel=债券, crypto-channel=加密货币",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "条数，默认20，最大50",
				Required: false,
			},
		},
		func(args string) (string, error) {
			channel := types.ExtractStringParam(args, "channel", "name")
			limit, _ := types.ExtractIntParam(args, "limit", "count")
			if channel == "" {
				channel = "global-channel"
			}
			if limit <= 0 {
				limit = 20
			}
			return data.NewWallstreetcnApi().GetLivesReadable(channel, limit), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetWallstreetcnMarketReal",
		"获取华尔街见闻全球实时行情报价。包含美元指数、欧元/美元、美元/日元、离岸人民币、现货黄金、WTI原油等品种。数据来源：华尔街见闻(wallstreetcn.com)。",
		map[string]*types.ParamDef{
			"prodCodes": {
				Type:     "string",
				Desc:     "品种代码(逗号分隔)，可选：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油。留空返回全部。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			prodCodes := types.ExtractStringListParam(args, "prodCodes", "prod_codes", "codes")
			return data.NewWallstreetcnApi().GetMarketRealReadable(prodCodes), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetWallstreetcnKline",
		"获取华尔街见闻K线数据。支持美元指数、外汇、黄金、原油等品种。数据来源：华尔街见闻(wallstreetcn.com)。",
		map[string]*types.ParamDef{
			"prodCode": {
				Type:     "string",
				Desc:     "品种代码：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油",
				Required: true,
			},
			"periodType": {
				Type:     "integer",
				Desc:     "K线周期(秒)：60=1分钟, 300=5分钟, 900=15分钟, 1800=30分钟, 3600=1小时, 14400=4小时, 86400=日线",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "K线条数，默认50",
				Required: false,
			},
		},
		func(args string) (string, error) {
			prodCode := types.ExtractStringParam(args, "prodCode", "prod_code", "code", "symbol")
			periodType, _ := types.ExtractIntParam(args, "periodType", "period_type", "period")
			limit, _ := types.ExtractIntParam(args, "limit", "count")
			if prodCode == "" {
				prodCode = "XAUUSD.OTC"
			}
			if periodType <= 0 {
				periodType = 300
			}
			if limit <= 0 {
				limit = 50
			}
			return data.NewWallstreetcnApi().GetKlineReadable(prodCode, periodType, limit), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetWallstreetcnCalendar",
		"获取华尔街见闻财经日历。包含全球重要经济数据公布时间、预期值、前值等。数据来源：华尔街见闻(wallstreetcn.com)。",
		map[string]*types.ParamDef{
			"days": {
				Type:     "integer",
				Desc:     "查看未来几天内的财经日历，默认3天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days, _ := types.ExtractIntParam(args, "days", "dayCount")
			if days <= 0 {
				days = 3
			}
			return data.NewWallstreetcnApi().GetCalendarReadable(days), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetTdxCompanyInfo",
		"通过通达信协议获取股票F10公司资料，包括公司简介、股本结构、财务摘要、除权除息等完整信息。当东方财富F10接口不可用或需要补充数据时可使用此工具。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			bundle := api.GetF10Data(stockCode)
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s F10公司资料（通达信）\n\n", stockCode))
			for _, s := range bundle.Sections {
				sb.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", s.Name, s.Content))
			}
			if bundle.Finance != nil {
				f := bundle.Finance
				sb.WriteString("## 财务摘要\n\n")
				sb.WriteString("| 指标 | 值 |\n|---|---|\n")
				sb.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", f.Code))
				if f.IPODate != "" {
					sb.WriteString(fmt.Sprintf("| 上市日期 | %s |\n", f.IPODate))
				}
				sb.WriteString(fmt.Sprintf("| 每股收益 | %.4f |\n", f.EPS))
				sb.WriteString(fmt.Sprintf("| 每股净资产 | %.4f |\n", f.NetAssetsPerShare))
				sb.WriteString(fmt.Sprintf("| 流通股本(万股) | %.2f |\n", f.FloatShares))
				sb.WriteString(fmt.Sprintf("| 总股本(万股) | %.2f |\n", f.TotalShares))
				sb.WriteString(fmt.Sprintf("| 总资产(万元) | %.2f |\n", f.TotalAssets))
				sb.WriteString(fmt.Sprintf("| 净资产(万元) | %.2f |\n", f.TotalEquity))
				sb.WriteString(fmt.Sprintf("| 营业收入(万元) | %.2f |\n", f.OperatingRevenue))
				sb.WriteString(fmt.Sprintf("| 净利润(万元) | %.2f |\n", f.NetProfit))
				sb.WriteString(fmt.Sprintf("| 股东人数 | %.0f |\n", f.ShareholderCount))
				sb.WriteString("\n")
			}
			if len(bundle.XDXR) > 0 {
				sb.WriteString("## 除权除息\n\n")
				sb.WriteString("| 日期 | 类别 | 分红(每股) | 送转股 | 配股价 | 配股 |\n|---|---|---|---|---|---|\n")
				for _, x := range bundle.XDXR {
					fh := "-"
					if x.Fenhong != nil {
						fh = fmt.Sprintf("%.4f", *x.Fenhong)
					}
					szg := "-"
					if x.Songzhuangu != nil {
						szg = fmt.Sprintf("%.4f", *x.Songzhuangu)
					}
					pgj := "-"
					if x.Peigujia != nil {
						pgj = fmt.Sprintf("%.2f", *x.Peigujia)
					}
					pg := "-"
					if x.Peigu != nil {
						pg = fmt.Sprintf("%.4f", *x.Peigu)
					}
					sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n", x.Date, x.Name, fh, szg, pgj, pg))
				}
				sb.WriteString("\n")
			}
			return sb.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetTdxFinanceInfo",
		"通过通达信协议获取股票财务信息，包括每股收益、总资产、净资产、营业收入、净利润、股东人数等核心财务指标。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			f := api.GetFinanceInfo(stockCode)
			if f == nil {
				return fmt.Sprintf("%s：获取财务信息失败", stockCode), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 财务信息（通达信）\n\n", stockCode))
			sb.WriteString("| 指标 | 值 |\n|---|---|\n")
			sb.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", f.Code))
			if f.IPODate != "" {
				sb.WriteString(fmt.Sprintf("| 上市日期 | %s |\n", f.IPODate))
			}
			if f.UpdatedDate != "" {
				sb.WriteString(fmt.Sprintf("| 更新日期 | %s |\n", f.UpdatedDate))
			}
			sb.WriteString(fmt.Sprintf("| 每股收益 | %.4f |\n", f.EPS))
			sb.WriteString(fmt.Sprintf("| 每股净资产 | %.4f |\n", f.NetAssetsPerShare))
			sb.WriteString(fmt.Sprintf("| 流通股本(万股) | %.2f |\n", f.FloatShares))
			sb.WriteString(fmt.Sprintf("| 总股本(万股) | %.2f |\n", f.TotalShares))
			sb.WriteString(fmt.Sprintf("| 总资产(万元) | %.2f |\n", f.TotalAssets))
			sb.WriteString(fmt.Sprintf("| 净资产(万元) | %.2f |\n", f.TotalEquity))
			sb.WriteString(fmt.Sprintf("| 营业收入(万元) | %.2f |\n", f.OperatingRevenue))
			sb.WriteString(fmt.Sprintf("| 营业成本(万元) | %.2f |\n", f.OperatingCost))
			sb.WriteString(fmt.Sprintf("| 营业利润(万元) | %.2f |\n", f.OperatingProfit))
			sb.WriteString(fmt.Sprintf("| 净利润(万元) | %.2f |\n", f.NetProfit))
			sb.WriteString(fmt.Sprintf("| 股东人数 | %.0f |\n", f.ShareholderCount))
			sb.WriteString(fmt.Sprintf("| 资本公积金(万元) | %.2f |\n", f.CapitalReserve))
			sb.WriteString(fmt.Sprintf("| 未分配利润(万元) | %.2f |\n", f.UndistributedProfit))
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetTdxXDXRInfo",
		"通过通达信协议获取股票除权除息信息，包括分红、配股、送转股等历史记录及股本变动情况。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			items := api.GetXDXRInfo(stockCode)
			if items == nil || len(*items) == 0 {
				return fmt.Sprintf("%s：暂无除权除息数据", stockCode), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 除权除息信息（通达信）\n\n", stockCode))
			sb.WriteString("| 日期 | 类别 | 分红(每股) | 送转股 | 配股价 | 配股 |\n|---|---|---|---|---|---|\n")
			for _, x := range *items {
				fh := "-"
				if x.Fenhong != nil {
					fh = fmt.Sprintf("%.4f", *x.Fenhong)
				}
				szg := "-"
				if x.Songzhuangu != nil {
					szg = fmt.Sprintf("%.4f", *x.Songzhuangu)
				}
				pgj := "-"
				if x.Peigujia != nil {
					pgj = fmt.Sprintf("%.2f", *x.Peigujia)
				}
				pg := "-"
				if x.Peigu != nil {
					pg = fmt.Sprintf("%.4f", *x.Peigu)
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n", x.Date, x.Name, fh, szg, pgj, pg))
			}
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetTdxCompanyCategory",
		"获取股票F10分类信息（通达信协议）。不传category返回所有分类列表；传category返回该分类详情。分类：最新提示/公司概况/财务分析/股本结构/股东研究/机构持股/分红融资/高管治理/资金动向/资本运作/热点题材/公司公告/公司报道/经营分析/行业分析/研报评级。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
			"category": {
				Type:     "string",
				Desc:     "F10分类名称，如：公司概况、财务分析、股本结构、股东研究、机构持股、分红融资、高管治理、资金动向、资本运作、热点题材、公司公告、公司报道、经营分析、行业分析、研报评级、最新提示。不传或为空时返回所有可用分类列表。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			category := types.ExtractStringParam(args, "category", "type")
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			if category == "" {
				cats := api.GetF10CategoryList(stockCode)
				if cats == nil || len(*cats) == 0 {
					return fmt.Sprintf("%s：获取分类列表失败", stockCode), nil
				}
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("# %s F10可用分类列表（通达信）\n\n", stockCode))
				sb.WriteString("| 序号 | 分类名称 |\n|---|---|\n")
				for i, c := range *cats {
					sb.WriteString(fmt.Sprintf("| %d | %s |\n", i+1, c.Name))
				}
				sb.WriteString("\n> 提示：传入 category 参数可获取对应分类的详细内容。\n")
				return sb.String(), nil
			}
			section := api.GetF10CategoryContent(stockCode, category)
			if section == nil || section.Content == "" {
				return fmt.Sprintf("%s：分类 '%s' 获取失败或内容为空", stockCode, category), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s - %s（通达信）\n\n", stockCode, section.Name))
			sb.WriteString(section.Content + "\n")
			return sb.String(), nil
		},
	))

	// GetTdxSymbolBelongBoard - 通过通达信MAC接口获取股票所属板块信息
	result = append(result, NewDataToolWrapper(
		"GetTdxSymbolBelongBoard",
		"通过通达信MAC接口获取股票所属板块信息，包括行业板块、概念板块、地域板块、风格板块等，以及板块指数、涨停/跌停家数等数据。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾，港股以.HK结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			items := api.GetMACSymbolBelongBoard(stockCode)
			if items == nil || len(*items) == 0 {
				return fmt.Sprintf("%s：获取所属板块信息失败或无数据", stockCode), nil
			}
			return util.MarkdownTableWithTitle(stockCode+" 所属板块（通达信MAC）", *items), nil
		},
	))

	return result
}
