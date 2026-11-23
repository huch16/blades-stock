package data

import (
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/util"

	"github.com/duke-git/lancet/v2/convertor"
	bladesTools "github.com/go-kratos/blades/tools"
)

func RegisterHotTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetFollowedStocks",
		"获取用户关注/自选的股票列表",
		map[string]*types.ParamDef{
			"groupId": {
				Type:     "integer",
				Desc:     "股票分组ID，不传则返回所有关注/自选的股票",
				Required: false,
			},
		},
		func(args string) (string, error) {
			type followedRow struct {
				StockCode string  `md:"股票代码"`
				Name      string  `md:"股票名称"`
				CostPrice float64 `md:"成本价格"`
				Volume    int64   `md:"持仓数量"`
			}
			groupId, _ := types.ExtractIntParam(args, "groupId", "group_id", "group")
			var rows []followedRow
			if groupId > 0 {
				groupStocks := data.NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
				for _, gs := range groupStocks {
					stockInfo := data.NewStockDataApi().GetFollowedStockByStockCode(gs.StockCode)
					if stockInfo.StockCode != "" {
						rows = append(rows, followedRow{
							StockCode: stockInfo.StockCode,
							Name:      stockInfo.Name,
							CostPrice: stockInfo.CostPrice,
							Volume:    stockInfo.Volume,
						})
					}
				}
			} else {
				list := data.NewStockDataApi().GetFollowList(0)
				if list != nil {
					for _, s := range *list {
						rows = append(rows, followedRow{
							StockCode: s.StockCode,
							Name:      s.Name,
							CostPrice: s.CostPrice,
							Volume:    s.Volume,
						})
					}
				}
			}
			if len(rows) == 0 {
				return "暂无关注/自选的股票", nil
			}
			return util.MarkdownTableWithTitle("关注/自选的股票", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockInfo",
		"获取股票详细信息，包括实时行情、基本信息等",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码(（A股：sh,sz开头;港股hk开头,美股：us开头）)，支持多个股票代码，用逗号分隔",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			codes := types.ExtractStockCodes(args)
			if len(codes) == 0 {
				return "请输入股票代码", nil
			}
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(code)
				if err != nil || stockData == nil || len(*stockData) == 0 {
					results = append(results, code+"：未找到股票信息")
					continue
				}
				for _, s := range *stockData {
					price, _ := convertor.ToFloat(s.Price)
					prePrice, _ := convertor.ToFloat(s.PrePrice)
					change := price - prePrice
					var pChange float64
					if prePrice > 0 {
						pChange = (price - prePrice) / prePrice * 100
					}
					content := fmt.Sprintf("### %s %s\n\n| 项目 | 值 |\n| --- | --- |\n| 股票代码 | %s |\n| 股票名称 | %s |\n| 当前价格 | %.2f |\n| 涨跌额 | %.2f |\n| 涨跌幅 | %.2f%% |\n| 成交量 | %s手 |\n| 成交额 | %s元 |\n| 今开 | %s |\n| 昨收 | %s |\n| 最高 | %s |\n| 最低 | %s |\n| 时间 | %s %s |",
						s.Name, s.Code,
						s.Code, s.Name, price, change, pChange,
						s.Volume, s.Amount,
						s.Open, s.PreClose, s.High, s.Low,
						s.Date, s.Time)
					results = append(results, content)
				}
			}
			return strings.Join(results, "\n\n"), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetHotStockList",
		"获取雪球热门股票排行榜",
		map[string]*types.ParamDef{
			"marketType": {
				Type:     "string",
				Desc:     "市场类型：全球(10)、沪深(12)、港股(13)、美股(11)",
				Required: false,
			},
			"size": {
				Type:     "integer",
				Desc:     "返回条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			marketType := types.ExtractStringParam(args, "marketType", "market_type", "market")
			size, _ := types.ExtractIntParam(args, "size", "count", "limit")
			if size <= 0 {
				size = 20
			}
			if marketType == "" {
				marketType = "10"
			}
			hotItems := data.NewMarketNewsApi().XUEQIUHotStock(size, marketType)
			if hotItems == nil || len(*hotItems) == 0 {
				return "暂无热门股票数据", nil
			}
			type hotStockRow struct {
				Rank    int     `md:"排名"`
				Code    string  `md:"股票代码"`
				Name    string  `md:"股票名称"`
				Price   float64 `md:"当前价"`
				Chg     float64 `md:"股价变化"`
				Percent float64 `md:"涨跌幅(%)"`
				Value   float64 `md:"热度"`
			}
			var rows []hotStockRow
			for i, item := range *hotItems {
				rows = append(rows, hotStockRow{
					Rank:    i + 1,
					Code:    item.Code,
					Name:    item.Name,
					Price:   item.Current,
					Chg:     item.Chg,
					Percent: item.Percent,
					Value:   item.Value,
				})
			}
			marketName := map[string]string{
				"10": "全球",
				"12": "沪深",
				"13": "港股",
				"11": "美股",
			}[marketType]
			return util.MarkdownTableWithTitle(marketName+"热门股票排行榜", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetHotEventList",
		"获取雪球热门话题/事件",
		map[string]*types.ParamDef{
			"size": {
				Type:     "integer",
				Desc:     "返回条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			size, _ := types.ExtractIntParam(args, "size", "count", "limit")
			if size <= 0 {
				size = 20
			}
			hotEvents := data.NewMarketNewsApi().HotEvent(size)
			if hotEvents == nil || len(*hotEvents) == 0 {
				return "暂无热门话题数据", nil
			}
			type hotEventRow struct {
				Rank    int    `md:"排名"`
				Tag     string `md:"话题标签"`
				Content string `md:"话题内容"`
				Hot     int    `md:"热度"`
			}
			var rows []hotEventRow
			for i, event := range *hotEvents {
				rows = append(rows, hotEventRow{
					Rank:    i + 1,
					Tag:     event.Tag,
					Content: event.Content,
					Hot:     event.Hot,
				})
			}
			return util.MarkdownTableWithTitle("雪球热门话题", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetIndustryMoneyRank",
		"获取行业资金流向排名（按行业分类）",
		map[string]*types.ParamDef{
			"fenlei": {
				Type:     "string",
				Desc:     "行业分类：0=所有行业, 1=行业分类, 2=概念板块, 3=地域板块",
				Required: false,
			},
			"sort": {
				Type:     "string",
				Desc:     "排序字段：netamount=净流入, netbuy=主力净流入, change=涨跌幅",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "返回条数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			fenlei := types.ExtractStringParam(args, "fenlei", "category", "type")
			sort := types.ExtractStringParam(args, "sort", "order", "sortBy")
			limit, _ := types.ExtractIntParam(args, "limit", "count")
			if limit <= 0 {
				limit = 20
			}
			if fenlei == "" {
				fenlei = "1"
			}
			if sort == "" {
				sort = "netamount"
			}
			rankData := data.NewMarketNewsApi().GetIndustryMoneyRankSina(fenlei, sort)
			if len(rankData) == 0 {
				return "暂无行业资金流向数据", nil
			}
			type industryRankRow struct {
				Rank       int     `md:"排名"`
				Name       string  `md:"板块名称"`
				NetAmount  float64 `md:"净流入(万)"`
				NetBuy     float64 `md:"主力净流入(万)"`
				ChangeRate float64 `md:"涨跌幅(%)"`
			}
			var rows []industryRankRow
			for i, item := range rankData {
				if i >= limit {
					break
				}
				netAmount, _ := convertor.ToFloat(item["netamount"])
				netBuy, _ := convertor.ToFloat(item["netbuy"])
				changeRate, _ := convertor.ToFloat(item["change_rate"])
				rows = append(rows, industryRankRow{
					Rank:       i + 1,
					Name:       convertor.ToString(item["name"]),
					NetAmount:  netAmount / 10000,
					NetBuy:     netBuy / 10000,
					ChangeRate: changeRate,
				})
			}
			fenleiName := map[string]string{"0": "所有行业", "1": "行业分类", "2": "概念板块", "3": "地域板块"}[fenlei]
			return util.MarkdownTableWithTitle(fenleiName+"资金流向排名", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetLongTigerList",
		"获取龙虎榜数据（营业部排行榜）",
		map[string]*types.ParamDef{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-03-28",
				Required: true,
			},
		},
		func(args string) (string, error) {
			date := types.ExtractStringParam(args, "date", "targetDate", "day", "tradeDate")
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			longTigerData := data.NewMarketNewsApi().LongTiger(date)
			if longTigerData == nil || len(*longTigerData) == 0 {
				return "当日暂无龙虎榜数据", nil
			}
			type longTigerRow struct {
				Rank         int     `md:"排名"`
				Code         string  `md:"股票代码"`
				Name         string  `md:"股票名称"`
				ClosePrice   float64 `md:"收盘价"`
				ChangeRate   float64 `md:"涨跌幅(%)"`
				BizNetAmt    float64 `md:"营业部净买入(万)"`
				TurnoverRate float64 `md:"换手率(%)"`
			}
			var rows []longTigerRow
			for i, item := range *longTigerData {
				if i >= 50 {
					break
				}
				closePrice, _ := convertor.ToFloat(item.CLOSEPRICE)
				changeRate, _ := convertor.ToFloat(item.CHANGERATE)
				bizNetAmt, _ := convertor.ToFloat(item.BILLBOARDNETAMT)
				turnoverRate, _ := convertor.ToFloat(item.TURNOVERRATE)
				rows = append(rows, longTigerRow{
					Rank:         i + 1,
					Code:         item.SECURITYCODE,
					Name:         item.SECURITYNAMEABBR,
					ClosePrice:   closePrice,
					ChangeRate:   changeRate,
					BizNetAmt:    bizNetAmt / 10000,
					TurnoverRate: turnoverRate,
				})
			}
			return util.MarkdownTableWithTitle(date+" 龙虎榜数据", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetEconomicData",
		"获取宏观经济数据，包括GDP、CPI、PPI、PMI等",
		map[string]*types.ParamDef{
			"dataType": {
				Type:     "string",
				Desc:     "数据类型：gdp=国内生产总值, cpi=居民消费价格指数, ppi=工业生产者出厂价格指数, pmi=采购经理指数",
				Required: true,
			},
		},
		func(args string) (string, error) {
			dataType := types.ExtractStringParam(args, "dataType", "data_type", "type")
			if dataType == "" {
				return "请指定数据类型", nil
			}
			api := data.NewMarketNewsApi()
			var result string
			switch dataType {
			case "gdp":
				gdp := api.GetGDP()
				if gdp == nil || len(gdp.GDPResult.Data) == 0 {
					return "暂无GDP数据", nil
				}
				type gdpRow struct {
					Time           string  `md:"时间"`
					Total          float64 `md:"国内生产总值(亿元)"`
					FirstIndustry  float64 `md:"第一产业(亿元)"`
					SecondIndustry float64 `md:"第二产业(亿元)"`
					ThirdIndustry  float64 `md:"第三产业(亿元)"`
				}
				var rows []gdpRow
				for _, d := range gdp.GDPResult.Data {
					rows = append(rows, gdpRow{
						Time:           d.TIME,
						Total:          d.DOMESTICLPRODUCTBASE / 100000000,
						FirstIndustry:  d.FIRSTPRODUCTBASE / 100000000,
						SecondIndustry: d.SECONDPRODUCTBASE / 100000000,
						ThirdIndustry:  d.THIRDPRODUCTBASE / 100000000,
					})
				}
				result = util.MarkdownTableWithTitle("国内生产总值(GDP)", rows)
			case "cpi":
				cpi := api.GetCPI()
				if cpi == nil || len(cpi.CPIResult.Data) == 0 {
					return "暂无CPI数据", nil
				}
				type cpiRow struct {
					Time         string  `md:"时间"`
					NationalBase float64 `md:"全国当月"`
					NationalSame float64 `md:"全国同比增长(%)"`
					CityBase     float64 `md:"城市当月"`
					RuralBase    float64 `md:"农村当月"`
				}
				var rows []cpiRow
				for _, d := range cpi.CPIResult.Data {
					rows = append(rows, cpiRow{
						Time:         d.TIME,
						NationalBase: d.NATIONALBASE,
						NationalSame: d.NATIONALSAME,
						CityBase:     d.CITYBASE,
						RuralBase:    d.RURALBASE,
					})
				}
				result = util.MarkdownTableWithTitle("居民消费价格指数(CPI)", rows)
			case "ppi":
				ppi := api.GetPPI()
				if ppi == nil || len(ppi.PPIResult.Data) == 0 {
					return "暂无PPI数据", nil
				}
				type ppiRow struct {
					Time  string  `md:"时间"`
					Base  float64 `md:"当月指数"`
					Same  float64 `md:"同比增长(%)"`
					Accum float64 `md:"累计指数"`
				}
				var rows []ppiRow
				for _, d := range ppi.PPIResult.Data {
					rows = append(rows, ppiRow{
						Time:  d.TIME,
						Base:  d.BASE,
						Same:  d.BASESAME,
						Accum: d.BASEACCUMULATE,
					})
				}
				result = util.MarkdownTableWithTitle("工业生产者出厂价格指数(PPI)", rows)
			case "pmi":
				pmi := api.GetPMI()
				if pmi == nil || len(pmi.PMIResult.Data) == 0 {
					return "暂无PMI数据", nil
				}
				type pmiRow struct {
					Time       string  `md:"时间"`
					MakeIndex  float64 `md:"制造业PMI"`
					MakeSame   float64 `md:"制造业同比增长(%)"`
					NMakeIndex float64 `md:"非制造业PMI"`
					NMakeSame  float64 `md:"非制造业同比增长(%)"`
				}
				var rows []pmiRow
				for _, d := range pmi.PMIResult.Data {
					rows = append(rows, pmiRow{
						Time:       d.TIME,
						MakeIndex:  d.MAKEINDEX,
						MakeSame:   d.MAKESAME,
						NMakeIndex: d.NMAKEINDEX,
						NMakeSame:  d.NMAKESAME,
					})
				}
				result = util.MarkdownTableWithTitle("采购经理指数(PMI)", rows)
			default:
				return "不支持的数据类型：" + dataType, nil
			}
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetInvestCalendar",
		"获取投资日历，包括财报发布、股东大会、IPO等重要日期事件",
		map[string]*types.ParamDef{
			"yearMonth": {
				Type:     "string",
				Desc:     "年月，格式：2026-03",
				Required: false,
			},
		},
		func(args string) (string, error) {
			yearMonth := types.ExtractStringParam(args, "yearMonth", "year_month", "month")
			if yearMonth == "" {
				yearMonth = time.Now().Format("2006-01")
			}
			calendarData := data.NewMarketNewsApi().InvestCalendar(yearMonth)
			if len(calendarData) == 0 {
				return "当日暂无投资日历数据", nil
			}
			type calendarRow struct {
				Date      string `md:"日期"`
				Type      string `md:"事件类型"`
				StockCode string `md:"股票代码"`
				StockName string `md:"股票名称"`
				Title     string `md:"事件标题"`
			}
			var rows []calendarRow
			for _, item := range calendarData {
				if m, ok := item.(map[string]any); ok {
					date := convertor.ToString(m["date"])
					eventType := convertor.ToString(m["type"])
					title := convertor.ToString(m["title"])
					stockCode := ""
					stockName := ""
					if stock, ok := m["stock"].(map[string]any); ok {
						stockCode = convertor.ToString(stock["code"])
						stockName = convertor.ToString(stock["name"])
					}
					rows = append(rows, calendarRow{
						Date:      date,
						Type:      eventType,
						StockCode: stockCode,
						StockName: stockName,
						Title:     title,
					})
				}
			}
			return util.MarkdownTableWithTitle(yearMonth+" 投资日历", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockNotice",
		"获取个股公告信息",
		map[string]*types.ParamDef{
			"stockCodes": {
				Type:     "string",
				Desc:     "股票代码列表，逗号分隔，如：600519,000001",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCodes := strings.Join(types.ExtractStockCodes(args), ",")
			if stockCodes == "" {
				return "请输入股票代码", nil
			}
			noticeData := data.NewMarketNewsApi().StockNotice(stockCodes)
			if len(noticeData) == 0 {
				return "暂无公告数据", nil
			}
			type noticeRow struct {
				NoticeTime string `md:"公告时间"`
				Title      string `md:"公告标题"`
				Code       string `md:"股票代码"`
			}
			var rows []noticeRow
			for _, item := range noticeData {
				if m, ok := item.(map[string]any); ok {
					noticeTime := convertor.ToString(m["notice_time"])
					title := convertor.ToString(m["title"])
					code := convertor.ToString(m["secu_code"])
					rows = append(rows, noticeRow{
						NoticeTime: noticeTime,
						Title:      title,
						Code:       code,
					})
				}
			}
			return util.MarkdownTableWithTitle("个股公告", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockMinuteData",
		"获取股票分时数据（当日分钟级成交量和价格）",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：600519.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			minuteData, updateTime := data.NewStockDataApi().GetStockMinutePriceData(stockCode)
			if minuteData == nil || len(*minuteData) == 0 {
				return "未获取到分时数据", nil
			}
			type minuteRow struct {
				Time   string  `md:"时间"`
				Price  float64 `md:"价格"`
				Volume float64 `md:"成交量(手)"`
				Amount float64 `md:"成交额(元)"`
			}
			var rows []minuteRow
			for _, m := range *minuteData {
				price, _ := convertor.ToFloat(m.Price)
				volume, _ := convertor.ToFloat(m.Volume)
				amount, _ := convertor.ToFloat(m.Amount)
				rows = append(rows, minuteRow{
					Time:   m.Time,
					Price:  price,
					Volume: volume,
					Amount: amount,
				})
			}
			return fmt.Sprintf("**更新时间**: %s\n\n%s", updateTime, util.MarkdownTableWithTitle(stockCode+" 分时数据", rows)), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetStockConceptInfo",
		"获取股票的概念板块信息，包括概念名称、成分股数量等",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：600519",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			conceptInfo := data.NewStockDataApi().GetStockConceptInfo(stockCode)
			if !conceptInfo.Success || len(conceptInfo.Result.Data) == 0 {
				return "未获取到概念板块信息", nil
			}
			type conceptRow struct {
				BoardName string  `md:"概念名称"`
				Desc      string  `md:"概念描述"`
				Yield     float64 `md:"涨跌幅(%)"`
			}
			var rows []conceptRow
			for _, c := range conceptInfo.Result.Data {
				rows = append(rows, conceptRow{
					BoardName: c.BOARDNAME,
					Desc:      c.SELECTEDBOARDREASON,
					Yield:     c.BOARDYIELD,
				})
			}
			return util.MarkdownTableWithTitle(stockCode+" 概念板块", rows), nil
		},
	))

	return result
}
