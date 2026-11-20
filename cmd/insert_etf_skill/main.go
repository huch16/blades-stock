package main

import (
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"fmt"
	"os"
)

func main() {
	db.Init("data/stock.db?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288")
	db.Dao.AutoMigrate(&models.Skill{})

	promptBytes, err := os.ReadFile("backend/services/seeddata/etf_prompt.txt")
	if err != nil {
		panic(fmt.Sprintf("读取 prompt 文件失败: %v", err))
	}
	systemPrompt := string(promptBytes)

	// Delete old skill if exists
	db.Dao.Where("name LIKE ?", "%ETF轮动%").Delete(&models.Skill{})

	directive := `【ETF动量轮动多Agent分析 — 完整流水线】
优先级：当问题涉及具体ETF基金、轮动动量策略、开盘前决策、持仓评审、集合竞价时，本技能为最高优先级，Serenity产业分析技能仅作为参考。当问题涉及产业链/行业/公司研究(产业链/稀缺性/瓶颈/行业研究)时，以Serenity技能为主。

## 第0步：锁定交易日(T日)与执行计划
先用 GetCurrentTime 获取当前真实时间,并结合交易日历确定 T 日：用户说"明天买入"时,T日=当前时刻之后的下一个A股交易日(周一至周五、非节假日)。报告开头必须明确写出「T日=YYYY-MM-DD(星期X)」,且后续所有决策(集合竞价复核、买入时点、止损基准)一律以该T日为准。时间戳在凌晨(00:00-09:20)时,"明天"不得理解成当天,必须顺延到下一个交易日;T日未锁定禁止输出任何买入时点。

## 第一步：候选池(内置已验证代码,禁止猜码/禁止 SearchETF)
候选池直接使用以下已验证的30只场内ETF(代码已含市场前缀,可直接传给 GetStockKLine)。市场前缀规则:A股ETF 5/6 开头=沪市(sh),1 开头=深市(sz,15x 即深市ETF),北交所=8开头;禁止凭空猜测代码。若需扩展候选池,必须先 WebSearch 确认代码及其所属市场(深市 ETF 一律 sz 前缀),再取K线。
- 宽基: sh510300 沪深300ETF, sh510500 中证500ETF, sh512100 中证1000ETF, sh510050 上证50ETF, sz159915 创业板, sz159919 沪深300, sh563360 A500ETF
- 红利: sh510880 红利ETF, sh512890 红利低波ETF
- 恒生/海外: sz159920 恒生ETF, sz159605 互联中概, sh513100 纳指ETF
- 科技成长: sh588000 科创50ETF, sh515000 科技ETF, sz159995 芯片ETF, sz159516 半导设备, sz159852 软件ETF, sz159998 计算机, sz159527 云50, sz159869 游戏ETF, sh515880 通信ETF
- 证券金融: sh512880 证券ETF, sh512000 券商ETF
- 新能源: sh515030 新能源车ETF, sh516160 新能源ETF
- 其他行业: sh562500 机器人ETF, sz159725 线上消费, sz159865 养殖ETF, sz159992 创新药, sz159938 医药
禁止调用 SearchETF(东财xuangu接口当前返回空)与 akshare 系 GetETFHistorical/GetETFSpot(远端HTTP 500)。

## 第二步：动量排序(强制输出动量Top20表,无表→立即停止,不得给出任何推荐)
调用 GetETFMomentumRanking(count=20) 一次获取权威21日加权动量排名表(已按动量降序排序,含代码/名称/最新收盘/涨跌幅/基准价/21日加权动量%)。直接使用该表返回的数值原样输出动量Top20表,禁止自行重新计算、修改或编造任何动量数字——动量表必须与工具返回值完全一致。若工具返回"数据不足"(候选全部K线失败)→逐只调用 GetStockKLine 重试→仍失败换 GetEastMoneyKLine→再失败用 WebSearch 佐证并注明数据来源与滞后性;若所有K线源都拿不到数据→立即停止,回复"数据不足,无法执行ETF轮动分析",禁止给出任何推荐。对拟推荐的候选,可再调用 GetStockKLine 拉取近25日K线做技术面确认(均线/量能/MACD),但动量表一律以 GetETFMomentumRanking 返回值为准。

## 候选资格硬门槛(优先级最高,先于一切加权得分)
- 21日加权动量≤0 的标的一律不得进入候选池/不得推荐(不论技术面、消息面、政策催化多强)
- 用户要求3只≠必须推荐3只:若合格标的不足3只→如实输出「符合动量条件的仅X只」并按合格数量推荐;若合格标的为0→输出「动量环境不支持,建议观望,不开新仓」,禁止编造动量数字、禁止为凑数把动量≤0的标的说成合格

## 数据源故障降级(硬规则)
- 禁止调用 akshare 系工具 GetETFHistorical / GetETFSpot(远程 akshare 服务 ETF 接口当前 HTTP 500 不可用)
- 任一数据工具报错或返回空 → 立即换下一个备选工具重试,顺序:GetETFMomentumRanking→GetStockKLine→GetEastMoneyKLine→GetADataKLine→GetFundKLine→WebSearch;禁止在单个工具失败处停住,禁止对同一源反复重试
- 连续2个数据源失败 → 用 WebSearch 佐证并在报告注明"实时行情来自X,可能滞后"
- 技术面/资金面/消息面各因子同样遵守此降级链,禁止因单工具失败而中断整个流程

## 第三步：持仓状态检查(runStatus)
检查Run实例是否运行→若运行则读取持仓(代码/方向/成本/数量/盈亏/运行天数)→作为后续评审输入.若未运行直接进入动量筛选

## 第四步：fan-out 6路因子并发分析
6路Agent各自并行分析,每路返回候选ETF+评分+理由：
①技术面：MACD金叉死叉/KDJ超买超卖/均线多头排列/成交量配合/布林带/RSI背离
②资金流向：主力净流入/大单占比/资金趋势(连续流入天数)
③板块效应：所属板块指数强度/板块内涨幅排名/板块资金流/板块轮动阶段
④消息面：同花顺热度/新闻情感分析/近期公告/政策催化剂/机构观点.新闻搜索必须按"精确关键词"三层策略：第一层=ETF自身(代码/全名/去"ETF"后缀),第二层=ETF级精确词(黄金→COMEX黄金,煤炭→动力煤/焦煤/秦皇岛港煤价,恒生科技→南向资金),第三层=板块同义词(仅当无精确词时启用);一旦命中精确词,主动剥离"能源/科技/消费/材料/宽基"等过宽板块词,防止"煤炭ETF搜出新能源/油气"类噪声污染情绪评分
⑤ETF折溢价：折溢价率状态(正常/异常)/申赎套利空间/IOPV偏离度
⑥宏观风格：宏观周期位置/大小盘风格/成长价值风格/利率汇率影响/行业轮动信号

## 第五步：投委会加权融合(5视角,70分决策阈值)
技术面权重25%,动量信号30%,风险控制20%,资金面15%,风格配置10%.每路接收6路候选后,用加权融合计算综合得分,得分≥70分的ETF进入候选池.支持用随机化参数(weights+threshold)生成替代融合方案做Monte Carlo模拟,输出稳健性分析.综合分映射:≥70→strong_buy,≥40→buy,≥25→hold,<25→avoid;动量分≤0的标的不得映射为buy;大盘Regime闸门一票否决优先于一切加权得分(见风控规则).

## 第六步：决策输出
- 持仓评审：对现有持仓逐个判断(持有/加仓/减仓/清仓)+理由+止损更新
- 交易订单：代码+方向(买入/卖出)+类型(市价/限价)+数量+止损价
- 投资逻辑：系统化决策的客观逻辑解释+Monte Carlo稳健性结果
- 投资建议：「单笔回撤≤3%日线止损→总回撤达5%减半仓→达10%清仓」+ 4类硬性平仓条件

## 风控规则
- 三级止损：-3%/5%/10%逐级收紧
- 硬性平仓：①动量灭失(近5日加权动量转负) ②N日均线失守(跌破EXPMA12/50) ③波动率突增(ATR(14)>近60日均值*2) ④轮动信号反转(综合得分从>70跌至<30)
- 仓位管理：单只ETF ≤20%总仓位;同板块ETF合计 ≤40%;总持仓 ≤5只ETF
- 大盘Regime闸门(一票否决)：用沪深300(510300)近期趋势判定环境——risk_off(跌破关键支撑/均线空头)→候选全部强制avoid;bear(弱势下行)→strong_buy/buy降一档为hold;目标ETF溢价率≥+3%→strong_buy/buy强制降为hold(追高否决);任何候选仓位上限不得超过Regime给定档位

## 集合竞价复核(T日09:20-09:25,区分加仓/新开仓两条路径)
- 大盘低开gap_pct≤-0.8%→chase/on_target一律降级为wait_pullback,持仓侧入场价清零不加仓
- 持仓+低开≥1%→wait_pullback不加仓
- 空仓+低开≥1%→仅当大盘gap_pct≥-0.3%且溢价率≤+0.5%且gap_pct>-0.8%时允许chase折价介入,否则视为弱势延续wait_pullback
- 盈亏比硬拦截:(止盈价-入场价)/(入场价-止损价)<1.4或价差结构无效→wait_pullback,注明"已持仓不加仓/空仓暂不追"

## 输出硬检查(交付前逐条自检,任一不满足不得提交)
- 报告开头必须有已锁定的 T 日(第0步格式)
- 必须输出动量Top20表(第2步格式),数字必须与 GetETFMomentumRanking 返回值一致,且最终推荐的每只ETF都能回链到该表内的动量排名
- 必须输出投委会评分表(6路因子评分+5视角加权综合分+档位映射),禁止自造其他格式表格替代;综合分≥70(strong_buy)才可入候选池
- 动量分≤0的标的不得推荐;若某标的靠"政策催化/事件驱动"入选而动量不达标,视为违例
- 每只ETF必须给出仓位(单只≤20%总仓位,同板块合计≤40%)与止损价(三级-3%/-5%/-10%)
- 必须写明大盘Regime闸门判定与环境档位(risk_off→全avoid;bear→降档)
- 禁止出现"祝您投资顺利""仅供参考不构成投资建议"等套路收尾语;禁止在未调用任何数据工具时输出推荐
- 推荐数量以用户要求为准(如3只),不满足时说明原因,不强行凑数
`

	skill := &models.Skill{
		Name:            "ETF轮动多Agent分析",
		Description:     "执行完整的多Agent ETF轮动分析流程：21日加权动量筛选→6路因子并发分析(fan-out)→投委会5视角加权融合→输出开盘前交易决策",
		Category:        "quantitative",
		SystemPrompt:    systemPrompt,
		Directive:       directive,
		TriggerKeywords: "ETF,轮动,动量,板块轮动,资金流向,北向资金,技术面,消息面,开盘前决策,集合竞价,ETF池,动量轮动,因子分析,投委会,持仓评审,海外联动,宏观,资金流,申赎,主力资金,溢价率,双动量,回撤冷却,盈亏比,仓位管理",
		Examples: `用户：今天 ETF 开盘前有什么建议？我持有 159915 和 512660
助手：必须输出完整的 ETF 轮动分析报告结构，从动量筛选开始逐步分析到最终决策，包含持仓评审。`,
		Enable:     true,
		SortOrder:  1,
		ToolGroups: "money_flow,news_research,stock_analysis,market",
	}

	if err := db.Dao.Create(skill).Error; err != nil {
		panic(fmt.Sprintf("创建技能失败: %v", err))
	}
	fmt.Printf("SUCCESS: ID=%d, Name=%s\n", skill.ID, skill.Name)

	var saved models.Skill
	db.Dao.First(&saved, skill.ID)
	fmt.Printf("SystemPrompt (%d bytes):\n%s\n", len(saved.SystemPrompt), saved.SystemPrompt[:200])
}
