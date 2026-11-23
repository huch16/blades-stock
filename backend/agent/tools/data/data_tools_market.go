package data

import (
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/util"

	"github.com/duke-git/lancet/v2/convertor"
	bladesTools "github.com/go-kratos/blades/tools"
	fakeUserAgent "github.com/lib4u/fake-useragent"
)

func RegisterMarketTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetNewsListData",
		"获取新闻资讯",
		map[string]*types.ParamDef{
			"startTime": {
				Type:     "string",
				Desc:     "开始时间（如：2026-02-23 00:00:00）",
				Required: true,
			},
			"keyWord": {
				Type:     "string",
				Desc:     "搜索关键词",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数",
				Required: false,
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
			startTime := types.ExtractStringParam(args, "startTime", "start_time")
			keyWord := types.ExtractStringParam(args, "keyWord", "keyword", "query")
			limit, _ := types.ExtractIntParam(args, "limit", "count")
			page, _ := types.ExtractIntParam(args, "page", "page_number")
			pageSize, _ := types.ExtractIntParam(args, "pageSize", "page_size")
			if pageSize <= 0 {
				pageSize = limit
			}
			if pageSize <= 0 {
				pageSize = 20
			}
			if page < 1 {
				page = 1
			}

			parseTime, err := time.Parse(time.DateTime, startTime)
			if err != nil {
				parseTime = time.Now().Add(-time.Hour * 24)
			}
			list, total := data.NewMarketNewsApi().GetNewsListData(keyWord, parseTime, page, pageSize)

			var md strings.Builder
			md.WriteString("### 最近新闻资讯（共 " + convertor.ToString(total) + " 条，本页 " + convertor.ToString(len(*list)) + " 条）\r\n")
			for _, d := range *list {
				md.WriteString(d.DataTime.Format(time.DateTime) + " " + d.Content + "\r\n")
			}
			return md.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GlobalStockIndexesReadable",
		"获取全球主要指数概览",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			result := data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
			if result == "" {
				return "暂无全球指数数据。", nil
			}
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"StockNotice",
		"获取上市公司公告列表",
		map[string]*types.ParamDef{
			"stock_list": {
				Type:     "string",
				Desc:     "股票代码，多只用英文逗号分隔",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockList := strings.Join(types.ExtractStockCodes(args), ",")
			res := data.NewMarketNewsApi().StockNotice(stockList)
			if len(res) == 0 {
				return "未查询到相关上市公司公告。", nil
			}

			type row struct {
				Title      string `md:"公告标题"`
				NoticeDate string `md:"公告日期"`
				ColumnName string `md:"公告类型"`
			}
			var rows []row
			for _, a := range res {
				m, ok := a.(map[string]any)
				if !ok {
					continue
				}
				if len(m["columns"].([]any)) > 0 {
					cols, ok := m["columns"].([]any)
					if !ok || len(cols) == 0 {
						continue
					}
					first, ok := cols[0].(map[string]any)
					if !ok {
						continue
					}
					columns := first
					rows = append(rows, row{
						Title:      convertor.ToString(m["title"]),
						NoticeDate: convertor.ToString(m["notice_date"]),
						ColumnName: convertor.ToString(columns["column_name"]),
					})
				} else {
					rows = append(rows, row{
						Title:      convertor.ToString(m["title"]),
						NoticeDate: convertor.ToString(m["notice_date"]),
					})
				}
			}
			md := util.MarkdownTableWithTitle("上市公司公告", rows)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetSecuritiesCompanyOpinion",
		"获取券商/机构的市场分析观点",
		map[string]*types.ParamDef{
			"startDate": {
				Type:     "string",
				Desc:     "开始时间（如：2026-02-23）",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束时间（如：2026-02-26）",
				Required: true,
			},
		},
		func(args string) (string, error) {
			startDate := types.ExtractStringParam(args, "startDate", "start_date")
			endDate := types.ExtractStringParam(args, "endDate", "end_date")
			res := data.NewMarketNewsApi().GetSecuritiesCompanyOpinion(startDate, endDate)
			var md strings.Builder
			for _, d := range res.Data {
				md.WriteString(d.OpinionData + "\r\n")
			}
			return md.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetCurrentTime",
		"获取当前本地时间（含星期几）及全球市场开盘状态",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			now := time.Now()
			weekday := data.WeekdayCN(now.Weekday())
			marketStatus := data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
			return "当前本地时间是：" + now.Format("2006-01-02 15:04:05") + " " + weekday + "\n\n" + marketStatus, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetMarketData",
		"获取市场行情数据，包括指数行情、上涨/下跌/涨停/跌停家数、涨跌分布和今日申购信息",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			return getMarketDataContent()
		},
	))

	result = append(result, NewDataToolWrapper(
		"AiRecommendStocks",
		"获取近期AI分析/推荐股票明细列表",
		map[string]*types.ParamDef{
			"startDate": {
				Type:     "string",
				Desc:     "开始时间",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束时间",
				Required: true,
			},
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
			startDate := types.ExtractStringParam(args, "startDate", "start_date")
			endDate := types.ExtractStringParam(args, "endDate", "end_date")
			page := types.ExtractStringParam(args, "page", "page_number")
			pageSize := types.ExtractStringParam(args, "pageSize", "page_size")
			keyWord := types.ExtractStringParam(args, "keyWord", "keyword", "query")

			pageNo := int64(1)
			if page != "" {
				if d, err := parseInt(page); err == nil {
					pageNo = int64(d)
				}
			}
			pageSizeNum := int64(50)
			if pageSize != "" {
				if d, err := parseInt(pageSize); err == nil {
					pageSizeNum = int64(d)
				}
			}

			pageData, svcErr := data.NewAiRecommendStocksService().GetAiRecommendStocksList(&models.AiRecommendStocksQuery{
				StartDate: startDate,
				EndDate:   endDate,
				Page:      int(pageNo),
				PageSize:  int(pageSizeNum),
				StockCode: keyWord,
				StockName: keyWord,
				BkName:    keyWord,
			})
			if svcErr != nil {
				return "", svcErr
			}

			var dataExport []models.AiRecommendStocksMdExport
			for _, v := range pageData.List {
				dataExport = append(dataExport, v.ToMdExportStruct())
			}
			content := util.MarkdownTableWithTitle("近期AI分析/推荐股票明细列表", dataExport)
			return content, nil
		},
	))

	return result
}

type APIResponse struct {
	Code int     `json:"code"`
	Msg  string  `json:"msg"`
	Data APIData `json:"data"`
}

type APIData struct {
	IndexQuote    []APIIndexQuote `json:"index_quote"`
	UpDownDis     APIUpDownDis    `json:"up_down_dis"`
	PurchaseToday []APIPurchase   `json:"purchase_today"`
}

type APIIndexQuote struct {
	SecuCode string  `json:"secu_code" md:"指数代码"`
	SecuName string  `json:"secu_name" md:"指数名称"`
	LastPx   float64 `json:"last_px" md:"最新价格"`
	Change   float64 `json:"change" md:"涨跌"`
	ChangePx float64 `json:"change_px" md:"涨跌点数"`
	UpNum    int     `json:"up_num" md:"上涨家数"`
	DownNum  int     `json:"down_num" md:"下跌家数"`
	FlatNum  int     `json:"flat_num" md:"平盘家数"`
}

type APIUpDownDis struct {
	UpNum       int     `json:"up_num" md:"涨停家数"`
	DownNum     int     `json:"down_num" md:"跌停家数"`
	AverageRise float64 `json:"average_rise" md:"平均涨幅"`
	RiseNum     int     `json:"rise_num" md:"上涨家数总计"`
	FallNum     int     `json:"fall_num" md:"下跌家数总计"`
	Down10      int     `json:"down_10" md:"跌幅8%~10%家数"`
	Down8       int     `json:"down_8" md:"跌幅6%~8%家数"`
	Down6       int     `json:"down_6" md:"跌幅4%~6%家数"`
	Down4       int     `json:"down_4" md:"跌幅2%~4%家数"`
	Down2       int     `json:"down_2" md:"跌幅0%~2%家数"`
	FlatNum     int     `json:"flat_num" md:"平盘家数"`
	Up2         int     `json:"up_2" md:"涨幅0%~2%家数"`
	Up4         int     `json:"up_4" md:"涨幅2%~4%家数"`
	Up6         int     `json:"up_6" md:"涨幅4%~6%家数"`
	Up8         int     `json:"up_8" md:"涨幅6%~8%家数"`
	Up10        int     `json:"up_10" md:"涨幅8%~10%家数"`
	SuspendNum  int     `json:"suspend_num" md:"停牌家数"`
	Status      bool    `json:"status" md:"状态"`
}

type APIPurchase struct {
	SecuCode     string   `json:"secu_code"`
	SecuName     string   `json:"secu_name"`
	SecuCodeFull string   `json:"SecuCode"`
	IPOPrice     float64  `json:"ipo_price"`
	IPOPE        float64  `json:"ipo_pe"`
	AllotMax     int      `json:"allot_max"`
	LotRate      *float64 `json:"lot_rate"`
}

func getMarketDataContent() (string, error) {
	client := data.SharedHTTPClient
	apiURL := "https://x-quote.cls.cn/quote/index/home?app=CailianpressWeb&os=web&sv=8.4.6"

	uaGen, err := fakeUserAgent.New()
	if err != nil {
		uaGen, _ = fakeUserAgent.New()
	}
	ua := uaGen.GetRandom()

	var apiResp APIResponse
	resp, err := client.R().
		SetHeader("User-Agent", ua).
		SetResult(&apiResp).
		Get(apiURL)

	if err != nil {
		logger.SugaredLogger.Warnf("GetMarketData 工具调用失败: %v", err)
		return "# 市场行情数据\r\n\r\n" +
			"⚠️ GetMarketData 工具调用失败（数据源暂不可用）: " + err.Error() + "\r\n\r\n" +
			"【回退】大盘指数/涨跌家数数据暂不可用。可改用 GlobalStockIndexesReadable / GetStockChanges 等其他已注入工具获取替代数据，并基于已成功获取的数据继续分析。\r\n", nil
	}

	if resp.StatusCode() != 200 || apiResp.Code != 200 {
		msg := fmt.Sprintf("API返回错误: 状态码=%d, 错误信息=%s", resp.StatusCode(), apiResp.Msg)
		logger.SugaredLogger.Warnf("GetMarketData 工具调用失败: %s", msg)
		return "# 市场行情数据\r\n\r\n" +
			"⚠️ GetMarketData 工具调用失败（数据源暂不可用）: " + msg + "\r\n\r\n" +
			"【回退】大盘指数/涨跌家数数据暂不可用。可改用 GlobalStockIndexesReadable / GetStockChanges 等其他已注入工具获取替代数据，并基于已成功获取的数据继续分析。\r\n", nil
	}

	content := strings.Builder{}
	content.WriteString("# 市场行情数据\r\n\r\n")

	content.WriteString("## 指数行情\r\n\r\n")
	content.WriteString("| 指数代码 | 指数名称 | 最新价格 | 涨跌(%) | 涨跌点数 | 上涨家数 | 下跌家数 | 平盘家数 |\r\n")
	content.WriteString("|----------|----------|----------|---------|----------|----------|----------|----------|\r\n")
	for _, index := range apiResp.Data.IndexQuote {
		content.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %.2f | %d | %d | %d |\r\n",
			index.SecuCode, index.SecuName, index.LastPx, index.Change*100, index.ChangePx,
			index.UpNum, index.DownNum, index.FlatNum))
	}

	content.WriteString("\r\n## 涨跌分布\r\n\r\n")
	content.WriteString("| 涨停家数 | 跌停家数 | 平均涨幅(%) | 上涨家数总计 | 下跌家数总计 |\r\n")
	content.WriteString("|----------|----------|-------------|--------------|--------------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %.2f | %d | %d |\r\n",
		apiResp.Data.UpDownDis.UpNum, apiResp.Data.UpDownDis.DownNum, apiResp.Data.UpDownDis.AverageRise*100,
		apiResp.Data.UpDownDis.RiseNum, apiResp.Data.UpDownDis.FallNum))

	content.WriteString("\r\n### 跌幅分布\r\n\r\n")
	content.WriteString("| 跌幅8%~10% | 跌幅6%~8% | 跌幅4%~6% | 跌幅2%~4% | 跌幅0%~2% |\r\n")
	content.WriteString("|-----------|-----------|-----------|-----------|-----------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\r\n",
		apiResp.Data.UpDownDis.Down10, apiResp.Data.UpDownDis.Down8, apiResp.Data.UpDownDis.Down6,
		apiResp.Data.UpDownDis.Down4, apiResp.Data.UpDownDis.Down2))

	content.WriteString("\r\n### 涨幅分布\r\n\r\n")
	content.WriteString("| 涨幅0%~2% | 涨幅2%~4% | 涨幅4%~6% | 涨幅6%~8% | 涨幅8%~10% |\r\n")
	content.WriteString("|-----------|-----------|-----------|-----------|------------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\r\n",
		apiResp.Data.UpDownDis.Up2, apiResp.Data.UpDownDis.Up4, apiResp.Data.UpDownDis.Up6,
		apiResp.Data.UpDownDis.Up8, apiResp.Data.UpDownDis.Up10))

	content.WriteString("\r\n### 其他统计\r\n\r\n")
	content.WriteString(fmt.Sprintf("- 平盘家数: %d\r\n", apiResp.Data.UpDownDis.FlatNum))
	content.WriteString(fmt.Sprintf("- 停牌家数: %d\r\n", apiResp.Data.UpDownDis.SuspendNum))

	content.WriteString("\r\n## 今日申购\r\n\r\n")
	if len(apiResp.Data.PurchaseToday) > 0 {
		content.WriteString("| 股票代码 | 股票名称 | 申购价格 | 申购PE | 最大申购数量 | 中签率(%) |\r\n")
		content.WriteString("|----------|----------|----------|--------|--------------|-----------|\r\n")
		for _, purchase := range apiResp.Data.PurchaseToday {
			lotRate := "-"
			if purchase.LotRate != nil {
				lotRate = fmt.Sprintf("%.2f", *purchase.LotRate)
			}
			content.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %d | %s |\r\n",
				purchase.SecuCode, purchase.SecuName, purchase.IPOPrice, purchase.IPOPE,
				purchase.AllotMax, lotRate))
		}
	} else {
		content.WriteString("今日无新股申购\r\n")
	}

	logger.SugaredLogger.Debug("%s", content.String())
	return content.String(), nil
}
