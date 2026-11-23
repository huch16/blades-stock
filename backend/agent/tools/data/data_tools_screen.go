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
			keyword := types.ExtractStringParam(args, "keyword", "query", "searchWords", "keyWord")
			page, _ := types.ExtractIntParam(args, "page", "page_number")
			pageSize, _ := types.ExtractIntParam(args, "pageSize", "page_size")
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			// 结构体字面量里不能直接用返回 (bool, bool) / (int, bool) 的
			// helper，用两个小闭包适配；键名同时接受 snake_case 别名。
			flag := func(keys ...string) bool {
				v, _ := types.ExtractBoolParam(args, keys...)
				return v
			}
			num := func(keys ...string) int {
				v, _ := types.ExtractIntParam(args, keys...)
				return v
			}

			indicators := models.TechnicalIndicators{
				MACDGOLDENFORK:     flag("macdGoldenFork", "macd_golden_fork"),
				KDJGOLDENFORK:      flag("kdjGoldenFork", "kdj_golden_fork"),
				BREAKTHROUGH:       flag("breakThrough", "break_through"),
				LOWFUNDSINFLOW:     flag("lowFundsInflow", "low_funds_inflow"),
				HIGHFUNDSOUTFLOW:   flag("highFundsOutflow", "high_funds_outflow"),
				BREAKUPMA5DAYS:     flag("breakUpMa5Days", "break_up_ma5_days"),
				LONGAVGARRAY:       flag("longAvgArray", "long_avg_array"),
				SHORTAVGARRAY:      flag("shortAvgArray", "short_avg_array"),
				UPPERLARGEVOLUME:   flag("upperLargeVolume", "upper_large_volume"),
				DOWNNARROWVOLUME:   flag("downNarrowVolume", "down_narrow_volume"),
				ONEDAYANGLINE:      flag("oneDayangLine", "one_dayang_line"),
				TWODAYANGLINES:     flag("twoDayangLines", "two_dayang_lines"),
				RISESUN:            flag("riseSun", "rise_sun"),
				POWERFULGUN:        flag("powerFulgun", "power_fulgun"),
				RESTOREJUSTICE:     flag("restoreJustice", "restore_justice"),
				DOWN7DAYS:          flag("down7Days", "down7_days"),
				UPPER8DAYS:         flag("upper8Days", "upper8_days"),
				UPPER9DAYS:         flag("upper9Days", "upper9_days"),
				UPPER4DAYS:         flag("upper4Days", "upper4_days"),
				HEAVENRULE:         flag("heavenRule", "heaven_rule"),
				UPSIDEVOLUME:       flag("upsideVolume", "upside_volume"),
				BEARISHENGULFING:   flag("bearishEngulfing", "bearish_engulfing"),
				REVERSINGHAMMER:    flag("reversingHammer", "reversing_hammer"),
				SHOOTINGSTAR:       flag("shootingStar", "shooting_star"),
				EVENINGSTAR:        flag("eveningStar", "evening_star"),
				FIRSTDAWN:          flag("firstDawn", "first_dawn"),
				PREGNANT:           flag("pregnant"),
				BLACKCLOUDTOPS:     flag("blackCloudTops", "black_cloud_tops"),
				MORNINGSTAR:        flag("morningStar", "morning_star"),
				NARROWFINISH:       flag("narrowFinish", "narrow_finish"),
				UPP_DAYS:           num("uppDays", "upp_days"),
				CONCERN_RANK_7DAYS: num("concernRank7Days", "concern_rank7_days"),
				UPNDAY:             num("upNday", "up_nday"),
				DOWNNDAY:           num("downNday", "down_nday"),
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
			words := types.ExtractStringParam(args, "words", "word", "query", "keyword")
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
			words := types.ExtractStringParam(args, "words", "word", "query", "keyword")
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
			words := types.ExtractStringParam(args, "words", "word", "query", "keyword")
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
			page := types.ExtractStringParam(args, "page", "page_number")
			pageSize := types.ExtractStringParam(args, "pageSize", "page_size")
			keyWord := types.ExtractStringParam(args, "keyWord", "keyword", "query")
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
