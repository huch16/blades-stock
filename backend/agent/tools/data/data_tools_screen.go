package data

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/models"
	"blades-stock/backend/util"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/random"
	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func RegisterScreenTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"FilterStocks",
		"根据技术指标或者关注排名或者连涨/连跌跌天数筛选股票。支持多种K线形态和技术指标条件筛选，如MACD金叉、KDJ金叉、均线排列、K线形态，人气，关注排名，连涨/连跌跌天数等。",
		map[string]*types.ParamDef{
			"keyword": {
				Type:     "string",
				Desc:     "股票名称或代码关键词搜索（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: false,
			},
			"macdGoldenFork": {
				Type:     "boolean",
				Desc:     "MACD金叉",
				Required: false,
			},
			"kdjGoldenFork": {
				Type:     "boolean",
				Desc:     "KDJ金叉",
				Required: false,
			},
			"breakThrough": {
				Type:     "boolean",
				Desc:     "放量突破",
				Required: false,
			},
			"lowFundsInflow": {
				Type:     "boolean",
				Desc:     "低位资金净流入",
				Required: false,
			},
			"highFundsOutflow": {
				Type:     "boolean",
				Desc:     "高位资金净流出",
				Required: false,
			},
			"breakUpMa5Days": {
				Type:     "boolean",
				Desc:     "向上突破5日均线",
				Required: false,
			},
			"longAvgArray": {
				Type:     "boolean",
				Desc:     "均线多头排列",
				Required: false,
			},
			"shortAvgArray": {
				Type:     "boolean",
				Desc:     "均线空头排列",
				Required: false,
			},
			"upperLargeVolume": {
				Type:     "boolean",
				Desc:     "连涨放量",
				Required: false,
			},
			"downNarrowVolume": {
				Type:     "boolean",
				Desc:     "下跌无量",
				Required: false,
			},
			"oneDayangLine": {
				Type:     "boolean",
				Desc:     "一根大阳线",
				Required: false,
			},
			"twoDayangLines": {
				Type:     "boolean",
				Desc:     "两根大阳线",
				Required: false,
			},
			"riseSun": {
				Type:     "boolean",
				Desc:     "旭日东升",
				Required: false,
			},
			"powerFulgun": {
				Type:     "boolean",
				Desc:     "强势多方炮",
				Required: false,
			},
			"restoreJustice": {
				Type:     "boolean",
				Desc:     "拨云见日",
				Required: false,
			},
			"down7Days": {
				Type:     "boolean",
				Desc:     "七仙女下凡(七连阴)",
				Required: false,
			},
			"upper8Days": {
				Type:     "boolean",
				Desc:     "八仙过海(八连阳)",
				Required: false,
			},
			"upper9Days": {
				Type:     "boolean",
				Desc:     "九阳神功(九连阳)",
				Required: false,
			},
			"upper4Days": {
				Type:     "boolean",
				Desc:     "四串阳",
				Required: false,
			},
			"heavenRule": {
				Type:     "boolean",
				Desc:     "天量法则",
				Required: false,
			},
			"upsideVolume": {
				Type:     "boolean",
				Desc:     "放量上攻",
				Required: false,
			},
			"bearishEngulfing": {
				Type:     "boolean",
				Desc:     "穿头破脚",
				Required: false,
			},
			"reversingHammer": {
				Type:     "boolean",
				Desc:     "倒转锤头",
				Required: false,
			},
			"shootingStar": {
				Type:     "boolean",
				Desc:     "射击之星",
				Required: false,
			},
			"eveningStar": {
				Type:     "boolean",
				Desc:     "黄昏之星",
				Required: false,
			},
			"firstDawn": {
				Type:     "boolean",
				Desc:     "曙光初现",
				Required: false,
			},
			"pregnant": {
				Type:     "boolean",
				Desc:     "身怀六甲",
				Required: false,
			},
			"blackCloudTops": {
				Type:     "boolean",
				Desc:     "乌云盖顶",
				Required: false,
			},
			"morningStar": {
				Type:     "boolean",
				Desc:     "早晨之星",
				Required: false,
			},
			"narrowFinish": {
				Type:     "boolean",
				Desc:     "窄幅整理",
				Required: false,
			},
			"uppDays": {
				Type:     "integer",
				Desc:     "人气排名连涨天数：3/5/7天及以上",
				Required: false,
			},
			"concernRank7Days": {
				Type:     "integer",
				Desc:     "7日关注排名：10/50/100名以内",
				Required: false,
			},
			"upNday": {
				Type:     "integer",
				Desc:     "连涨天数：3/5/8天及以上",
				Required: false,
			},
			"downNday": {
				Type:     "integer",
				Desc:     "连跌天数：3/5/8/10/14天及以上",
				Required: false,
			},
		},
		func(args string) (string, error) {
			keyword := gjson.Get(args, "keyword").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			indicators := models.TechnicalIndicators{
				MACDGOLDENFORK:     gjson.Get(args, "macdGoldenFork").Bool(),
				KDJGOLDENFORK:      gjson.Get(args, "kdjGoldenFork").Bool(),
				BREAKTHROUGH:       gjson.Get(args, "breakThrough").Bool(),
				LOWFUNDSINFLOW:     gjson.Get(args, "lowFundsInflow").Bool(),
				HIGHFUNDSOUTFLOW:   gjson.Get(args, "highFundsOutflow").Bool(),
				BREAKUPMA5DAYS:     gjson.Get(args, "breakUpMa5Days").Bool(),
				LONGAVGARRAY:       gjson.Get(args, "longAvgArray").Bool(),
				SHORTAVGARRAY:      gjson.Get(args, "shortAvgArray").Bool(),
				UPPERLARGEVOLUME:   gjson.Get(args, "upperLargeVolume").Bool(),
				DOWNNARROWVOLUME:   gjson.Get(args, "downNarrowVolume").Bool(),
				ONEDAYANGLINE:      gjson.Get(args, "oneDayangLine").Bool(),
				TWODAYANGLINES:     gjson.Get(args, "twoDayangLines").Bool(),
				RISESUN:            gjson.Get(args, "riseSun").Bool(),
				POWERFULGUN:        gjson.Get(args, "powerFulgun").Bool(),
				RESTOREJUSTICE:     gjson.Get(args, "restoreJustice").Bool(),
				DOWN7DAYS:          gjson.Get(args, "down7Days").Bool(),
				UPPER8DAYS:         gjson.Get(args, "upper8Days").Bool(),
				UPPER9DAYS:         gjson.Get(args, "upper9Days").Bool(),
				UPPER4DAYS:         gjson.Get(args, "upper4Days").Bool(),
				HEAVENRULE:         gjson.Get(args, "heavenRule").Bool(),
				UPSIDEVOLUME:       gjson.Get(args, "upsideVolume").Bool(),
				BEARISHENGULFING:   gjson.Get(args, "bearishEngulfing").Bool(),
				REVERSINGHAMMER:    gjson.Get(args, "reversingHammer").Bool(),
				SHOOTINGSTAR:       gjson.Get(args, "shootingStar").Bool(),
				EVENINGSTAR:        gjson.Get(args, "eveningStar").Bool(),
				FIRSTDAWN:          gjson.Get(args, "firstDawn").Bool(),
				PREGNANT:           gjson.Get(args, "pregnant").Bool(),
				BLACKCLOUDTOPS:     gjson.Get(args, "blackCloudTops").Bool(),
				MORNINGSTAR:        gjson.Get(args, "morningStar").Bool(),
				NARROWFINISH:       gjson.Get(args, "narrowFinish").Bool(),
				UPP_DAYS:           int(gjson.Get(args, "uppDays").Int()),
				CONCERN_RANK_7DAYS: int(gjson.Get(args, "concernRank7Days").Int()),
				UPNDAY:             int(gjson.Get(args, "upNday").Int()),
				DOWNNDAY:           int(gjson.Get(args, "downNday").Int()),
			}

			result := data.NewStockDataApi().GetAllStocks(page, pageSize, keyword, indicators)
			if result == nil || len(result.Result.Data) == 0 {
				return "未找到符合条件的股票", nil
			}

			type stockRow struct {
				SECUCODE         string  `md:"股票代码"`
				SECURITYNAMEABBR string  `md:"股票名称"`
				NEWPRICE         float64 `md:"最新价"`
				CHANGERATE       float64 `md:"涨跌幅(%)"`
				HIGHPRICE        float64 `md:"最高价"`
				LOWPRICE         float64 `md:"最低价"`
				VOLUME           string  `md:"成交量"`
				DEALAMOUNT       float64 `md:"成交额"`
				TURNOVERRATE     float64 `md:"换手率(%)"`
				VOLUMERATIO      float64 `md:"量比"`
				INDUSTRY         string  `md:"所属行业"`
			}

			var rows []stockRow
			for _, s := range result.Result.Data {
				newPrice, _ := convertor.ToFloat(s.NEWPRICE)
				changeRate, _ := convertor.ToFloat(s.CHANGERATE)
				highPrice, _ := convertor.ToFloat(s.HIGHPRICE)
				lowPrice, _ := convertor.ToFloat(s.LOWPRICE)
				dealAmount, _ := convertor.ToFloat(s.DEALAMOUNT)
				turnoverRate, _ := convertor.ToFloat(s.TURNOVERRATE)
				volumeRatio, _ := convertor.ToFloat(s.VOLUMERATIO)

				rows = append(rows, stockRow{
					SECUCODE:         s.SECUCODE,
					SECURITYNAMEABBR: s.SECURITYNAMEABBR,
					NEWPRICE:         newPrice,
					CHANGERATE:       changeRate,
					HIGHPRICE:        highPrice,
					LOWPRICE:         lowPrice,
					VOLUME:           convertor.ToString(s.VOLUME),
					DEALAMOUNT:       dealAmount,
					TURNOVERRATE:     turnoverRate,
					VOLUMERATIO:      volumeRatio,
					INDUSTRY:         s.INDUSTRY,
				})
			}

			summary := fmt.Sprintf("共找到 %d 只符合条件的股票，当前显示第 %d 页（每页 %d 条）", result.Result.Count, page, pageSize)
			return summary + "\n\n" + util.MarkdownTableWithTitle("股票筛选结果", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"SearchStockByIndicators",
		"根据自然语言筛选股票。可以使用K线形态、技术指标、财务指标等条件选股。可以查询股票常用的指标，如均线，kdj,rsi,boll，macd等。",
		map[string]*types.ParamDef{
			"words": {
				Type:     "string",
				Desc:     "选股条件描述，支持K线形态、技术指标、财务指标、均线、kdj、rsi、boll、macd等。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchStock(random.RandInt(50, 120))
			content := data.ThsResultToMarkdown(res, "工具筛选出的相关股票数据")
			if strings.Contains(content, "无符合条件") || strings.Contains(content, "请先获取东财用户标识") {
				if fb, ok := data.FallbackScreenByIndicators(words); ok && fb != "" {
					return fb, nil
				}
			}
			return content, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"SearchBk",
		"根据自然语言查询板块/概念/指数整体数据。",
		map[string]*types.ParamDef{
			"words": {
				Type:     "string",
				Desc:     "板块/概念/指数查询条件描述。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchBk(random.RandInt(50, 120))
			content := data.ThsResultToMarkdown(res, "工具筛选出的相关板块/概念数据")
			return content, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"SearchETF",
		"根据自然语言查询ETF数据。",
		map[string]*types.ParamDef{
			"words": {
				Type:     "string",
				Desc:     "ETF查询条件描述。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchETF(random.RandInt(50, 120))
			content := data.ThsResultToMarkdown(res, "工具筛选出的相关ETF数据")
			return content, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"InteractiveAnswer",
		"获取投资者与上市公司互动问答的数据",
		map[string]*types.ParamDef{
			"page": {
				Type:     "string",
				Desc:     "分页号",
				Required: true,
			},
			"pageSize": {
				Type:     "string",
				Desc:     "分页大小",
				Required: true,
			},
			"keyWord": {
				Type:     "string",
				Desc:     "搜索关键词",
				Required: false,
			},
		},
		func(args string) (string, error) {
			page := gjson.Get(args, "page").String()
			pageSize := gjson.Get(args, "pageSize").String()
			keyWord := gjson.Get(args, "keyWord").String()
			pageNo, _ := convertor.ToInt(page)
			if pageNo == 0 {
				pageNo = 1
			}
			pageSizeNum, _ := convertor.ToInt(pageSize)
			if pageSizeNum == 0 {
				pageSizeNum = 50
			}
			datas := data.NewMarketNewsApi().InteractiveAnswer(int(pageNo), int(pageSizeNum), keyWord)
			content := util.MarkdownTableWithTitle("投资互动数据", datas.Results)
			return content, nil
		},
	))

	return result
}
