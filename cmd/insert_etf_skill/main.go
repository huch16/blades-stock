package main

import (
	"fmt"
	"os"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func main() {
	db.Init("data/stock.db?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288")

	promptBytes, err := os.ReadFile("cmd/insert_etf_skill/etf_prompt.txt")
	if err != nil {
		panic(fmt.Sprintf("读取 prompt 文件失败: %v", err))
	}
	systemPrompt := string(promptBytes)

	// Delete old skill if exists
	db.Dao.Where("name LIKE ?", "%ETF轮动%").Delete(&models.Skill{})

	directive := `【ETF动量轮动多Agent分析 — 完整流水线】
优先级：当问题涉及具体ETF基金、轮动动量策略、开盘前决策、持仓评审、集合竞价时，本技能为最高优先级，Serenity产业分析技能仅作为参考。当问题涉及产业链/行业/公司研究(产业链/稀缺性/瓶颈/行业研究)时，以Serenity技能为主。

## 第一步：数据采集
SearchETF爬取全市场ETF当日数据(代码/名称/最新价/涨跌幅/成交额/规模/折溢价率)

## 第二步：动量排序
计算21日加权收益率(半衰期7日,w_i=0.5^(i/7),i=0..20从新到旧),按加权收益率降序取Top20动量ETF

## 第三步：持仓状态检查(runStatus)
检查Run实例是否运行→若运行则读取持仓(代码/方向/成本/数量/盈亏/运行天数)→作为后续评审输入.若未运行直接进入动量筛选

## 第四步：fan-out 6路因子并发分析
6路Agent各自并行分析,每路返回候选ETF+评分+理由：
①技术面：MACD金叉死叉/KDJ超买超卖/均线多头排列/成交量配合/布林带/RSI背离
②资金流向：主力净流入/大单占比/资金趋势(连续流入天数)
③板块效应：所属板块指数强度/板块内涨幅排名/板块资金流/板块轮动阶段
④消息面：同花顺热度/新闻情感分析/近期公告/政策催化剂/机构观点
⑤ETF折溢价：折溢价率状态(正常/异常)/申赎套利空间/IOPV偏离度
⑥宏观风格：宏观周期位置/大小盘风格/成长价值风格/利率汇率影响/行业轮动信号

## 第五步：投委会加权融合(5视角,70分决策阈值)
技术面权重25%,动量信号30%,风险控制20%,资金面15%,风格配置10%.每路接收6路候选后,用加权融合计算综合得分,得分≥70分的ETF进入候选池.支持用随机化参数(weights+threshold)生成替代融合方案做Monte Carlo模拟,输出稳健性分析.

## 第六步：决策输出
- 持仓评审：对现有持仓逐个判断(持有/加仓/减仓/清仓)+理由+止损更新
- 交易订单：代码+方向(买入/卖出)+类型(市价/限价)+数量+止损价
- 投资逻辑：系统化决策的客观逻辑解释+Monte Carlo稳健性结果
- 投资建议：「单笔回撤≤3%日线止损→总回撤达5%减半仓→达10%清仓」+ 4类硬性平仓条件

## 风控规则
- 三级止损：-3%/5%/10%逐级收紧
- 硬性平仓：①动量灭失(近5日加权动量转负) ②N日均线失守(跌破EXPMA12/50) ③波动率突增(ATR(14)>近60日均值*2) ④轮动信号反转(综合得分从>70跌至<30)
- 仓位管理：单只ETF ≤20%总仓位;同板块ETF合计 ≤40%;总持仓 ≤5只ETF`

	skill := &models.Skill{
		Name:            "ETF轮动多Agent分析",
		Description:     "执行完整的多Agent ETF轮动分析流程：21日加权动量筛选→6路因子并发分析(fan-out)→投委会5视角加权融合→输出开盘前交易决策",
		Category:        "quantitative",
		SystemPrompt:    systemPrompt,
		Directive:       directive,
		TriggerKeywords: "ETF,轮动,动量,板块轮动,资金流向,北向资金,技术面,消息面,开盘前决策,集合竞价,ETF池,动量轮动,因子分析,投委会,持仓评审,海外联动,宏观,资金流,申赎,主力资金,溢价率,双动量,回撤冷却,盈亏比,仓位管理",
		Examples: `用户：今天 ETF 开盘前有什么建议？我持有 159915 和 512660
助手：必须输出完整的 ETF 轮动分析报告结构，从动量筛选开始逐步分析到最终决策，包含持仓评审。`,
		Enable:    true,
		SortOrder: 1,
	}

	if err := db.Dao.Create(skill).Error; err != nil {
		panic(fmt.Sprintf("创建技能失败: %v", err))
	}
	fmt.Printf("SUCCESS: ID=%d, Name=%s\n", skill.ID, skill.Name)

	var saved models.Skill
	db.Dao.First(&saved, skill.ID)
	fmt.Printf("SystemPrompt (%d bytes):\n%s\n", len(saved.SystemPrompt), saved.SystemPrompt[:200])
}
