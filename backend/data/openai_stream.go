package data

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/logger"
	"blades-stock/backend/util"

	"github.com/tidwall/gjson"
)

func (o *OpenAi) NewSummaryStockNewsStreamWithTools(userQuestion string, sysPromptId *int, tools []Tool, thinking bool, history []map[string]interface{}) <-chan map[string]any {
	ch := make(chan map[string]any, 512)
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Error("NewSummaryStockNewsStream panic", err)
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine panic: %s", err)
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine panic config: %s", o.String())
			}
		}()
		defer close(ch)

		// sysPromptId 尚未连接到数据库技能表，当前始终使用配置中的 Prompt。
		sysPrompt := o.Prompt

		sysPrompt += `

【强制规则】你必须通过工具调用获取实时数据，严禁凭记忆编造或使用过时数据。以下场景必须调用工具：
1. 股票/指数行情数据（价格、涨跌幅、成交量等）——必须调用工具获取最新实时数据
2. 财务数据（营收、利润、市盈率等）——必须调用工具获取最新财报数据
3. 新闻资讯——必须调用工具获取最新新闻
4. 宏观经济数据——必须调用工具获取最新数据
任何涉及具体数字的回答，都必须先通过工具查询确认，不得使用训练数据中的过时信息。如果你没有获取到最新数据，必须明确告知用户"当前未能获取到最新数据"，绝不能编造数据。`

		sysPrompt += "最后必须调用CreateAiRecommendStocks工具函数保存ai股票推荐记录。"

		sysPrompt += `

## 股票筛选方法论（核心！避免仅推荐大公司）
进行市场资讯分析和股票推荐时，必须按以下步骤筛选，避免只看大盘龙头：

第1步 - 锁定方向：根据新闻热点确定目标板块概念
第2步 - 多维度筛选：使用 SearchStockByIndicators 工具，概念关键字 + 至少 3 个筛选条件叠加，例如：
  示例：SearchStockByIndicators("机器人概念,流通市值20亿~200亿,市盈率>0,营收增长率>10%,股价站上20日均线")
  常用筛选维度：流通市值(优先20~200亿中小盘)、市盈率PE>0、营收增长>10%、技术面站上均线
第3步 - 核查：用 GetStockInfo 确认实时行情和财务数据真实性
第4步 - 聚焦细分龙头：目标板块中市值50~200亿、有技术壁垒、业绩确定的中小盘标的
第5步 - 推荐3~5只覆盖不同子方向，用 BatchCreateAiRecommendStocks 批量保存
【代码-名称一致性硬性要求】每个股票代码必须来自工具返回的真实数据并与名称双向匹配；代码格式必须合法（6位数字：6开头=沪市.688=科创板.0/3开头=深市.300/301=创业板），格式非法或代码-名称不一致（如把000039当"中集车辆"）时禁止推荐；名称用最新官方名称，注意更名与ST标记。`

		msg := []map[string]interface{}{
			{
				"role":    "system",
				"content": sysPrompt,
			},
		}
		var msgMu sync.Mutex
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "当前时间",
		})
		msg = append(msg, map[string]interface{}{
			"role":              "assistant",
			"reasoning_content": "使用工具查询",
			"content":           "当前本地时间是:" + time.Now().Format("2006-01-02 15:04:05"),
		})
		wg := &sync.WaitGroup{}

		//wg.Go(func() {
		//	datas := NewMarketNewsApi().InteractiveAnswer(1, 100, "")
		//	content := util.MarkdownTableWithTitle("当前最新投资者互动数据", datas.Results)
		//	msg = append(msg, map[string]interface{}{
		//		"role":    "user",
		//		"content": "投资者互动数据",
		//	})
		//	msg = append(msg, map[string]interface{}{
		//		"role":              "assistant",
		//		"reasoning_content": "使用工具查询",
		//		"content":           content,
		//	})
		//})

		wg.Go(func() {
			var market strings.Builder
			res := NewMarketNewsApi().GetGDP()
			md := util.MarkdownTableWithTitle("国内生产总值(GDP)", res.GDPResult.Data)
			market.WriteString(md)
			res2 := NewMarketNewsApi().GetCPI()
			md2 := util.MarkdownTableWithTitle("居民消费价格指数(CPI)", res2.CPIResult.Data)
			market.WriteString(md2)
			res3 := NewMarketNewsApi().GetPPI()
			md3 := util.MarkdownTableWithTitle("工业品出厂价格指数(PPI)", res3.PPIResult.Data)
			market.WriteString(md3)
			res4 := NewMarketNewsApi().GetPMI()
			md4 := util.MarkdownTableWithTitle("采购经理人指数(PMI)", res4.PMIResult.Data)
			market.WriteString(md4)

			msgMu.Lock()
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "国内宏观经济数据",
			})
			msg = append(msg, map[string]interface{}{
				"role":              "assistant",
				"reasoning_content": "使用工具查询",
				"content":           "\n# 国内宏观经济数据：\n" + market.String(),
			})
			msgMu.Unlock()
		})

		wg.Go(func() {
			md := strings.Builder{}
			res := NewMarketNewsApi().ClsCalendar()
			for _, a := range res {
				bytes, err := json.Marshal(a)
				if err != nil {
					continue
				}
				date := gjson.Get(string(bytes), "calendar_day")
				md.WriteString("\n### 事件/会议日期：" + date.String())
				list := gjson.Get(string(bytes), "items")
				list.ForEach(func(key, value gjson.Result) bool {
					//logger.SugaredLogger.Debugf("key: %+v,value: %+v", key.String(), gjson.Get(value.String(), "title"))
					md.WriteString("\n- " + gjson.Get(value.String(), "title").String())
					return true
				})
			}
			msgMu.Lock()
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "近期重大事件/会议",
			})
			msg = append(msg, map[string]interface{}{
				"role":              "assistant",
				"reasoning_content": "使用工具查询",
				"content":           "近期重大事件/会议如下：\n" + md.String(),
			})
			msgMu.Unlock()
		})

		wg.Wait()

		//for _, m := range TrimAiAssistantHistoryForAPI(history) {
		//	msg = append(msg, m)
		//}
		if userQuestion == "" {
			userQuestion = "请根据当前时间，总结和分析股票市场新闻中的投资机会"
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": userQuestion,
		})
		AskAiWithTools(o, errors.New(""), msg, ch, userQuestion, tools, thinking)
	}()
	return ch
}
