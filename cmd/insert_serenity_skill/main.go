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

	promptBytes, err := os.ReadFile("cmd/insert_serenity_skill/serenity_prompt.txt")
	if err != nil {
		panic(fmt.Sprintf("读取 prompt 文件失败: %v", err))
	}
	systemPrompt := string(promptBytes)

	// Delete old skill if exists
	db.Dao.Where("name LIKE ?", "%Serenity%").Delete(&models.Skill{})

	directive := `【Serenity产业链瓶颈研究 — 完整框架】
优先级：当问题涉及产业链/行业/公司层面研究(产业链/稀缺性/瓶颈/行业研究/赛道分析/竞争格局/上下游/供需/国产替代)时，本技能为最高优先级，ETF轮动技能仅作为参考。当问题明确涉及具体ETF/轮动/开盘前决策时，以ETF轮动技能为主。

## 请求路由
- 主题扫描：市场+方向(如A股AI半导体),执行完整流程,输出优先排序候选
- 单公司挑战：判断产业链真实位置、证据强度、市场高估了什么、什么情况应降级
- 候选比较：按产业链位置、证据强度、稀缺性、估值压力、风险排序
- 研究伙伴：每轮只追一个关键问题,把想法从故事推向证据
- 学习模式：每次一问,从趋势→系统变化→卡点→证据→证伪逐步走

## 研究流程(必须完整执行)
Step1-设定范围：市场(A/港/美/台/日/韩/欧),主题(AI/半导体/机器人/CPO/电力设备/材料/创新药),时间窗口默认3-12月
Step2-系统变化：回答什么技术/经济变化驱动需求→哪个旧设计变紧张→哪个物理约束(功耗/延迟/带宽/散热/良率/纯度)最重要
Step3-产业链8层：终端需求→系统集成→模块/子系统→芯片/器件→工艺/封装/测试→设备/量测→材料/耗材→基础设施
Step4-稀缺层识别：信号叠加时关注(客户绕不开/供应商≤3/验证周期12-24月+ /扩产需特种设备+许可证 /客户预付款/长协/涨价/旧分类定价)
Step5-公司池：至少20家,分类🟢控制稀缺层/🔵供应/🟡间接受益/🟠定价权弱/🔴仅有故事;标注覆盖度+市值+再融资风险
Step6-证据：🟢Strong=公告/申报/批文/电话会;🟡Medium=权威媒体/交叉验证;🔴Weak=社媒仅作线索.深度扫描25+来源:10份申报+5份IR+5份行业+额外交叉
Step7-排序：按需求压力/稀缺层紧张度/供应商集中/扩产难度/证据强度/估值偏差/重估事件/风险.保持层级排序和公司排序分离

## 证据处理
- 每个主张注明来源类型,区分事实与解读。弱证据标"线索",不得仅凭弱证据下高置信度结论
- 每个优先候选至少1条中/强证据支撑业务定位+1条支撑需求/产能/财务+标明缺失证据
- 红旗信号(降低置信度)：依赖单一客户传闻;股价靠社媒炒作;应收/存货增速超营收;毛利率未随稀缺性改善;管理层用热点标签但数据不变;再融资压力且现金紧张

## 输出风格
- 像研究伙伴在讲判断,自然中文,先结论后证据,句子短,判断直接
- 每轮必须给出证伪条件+下一步核验动作
- 跨市场适配：A股查年报/问询函/互动易/环评/招投标;港股查HKEX/配售/流动性;美股查SEC/电话会/ATM/客户集中度;台日韩欧查当地申报/月营收/行业刊物
- 语感替换："稀缺层"代替chokepoint,"市场没看清的"代替mispricing,"什么情况错了"代替failure

## 研究边界
Research support only. 不保证收益,不下买卖指令,不炒作低流动性标的,不基于传闻推荐,不使用内幕信息,不捏造数据.最终决策由用户决定.`

	skill := &models.Skill{
		Name:            "Serenity式产业分析",
		Description:     "强制使用Serenity产业分析框架：产业链拆解→稀缺层识别→优先级排序→证据验证→证伪条件",
		Category:        "research",
		SystemPrompt:    systemPrompt,
		Directive:       directive,
		TriggerKeywords: "产业分析,产业链,稀缺性,瓶颈,行业研究,赛道分析,上下游,供需格局,渗透率,国产替代,市场份额,竞争格局,华阳集团,投资机会,细分龙头",
		Examples: `用户：分析华阳集团在汽车电子产业链中的位置
助手：必须输出Serenity报告结构：产业链全景图→稀缺层识别→优先级排序→证据链→证伪条件`,
		Enable:     true,
		SortOrder:  1,
		ToolGroups: "stock_analysis,market,news_research,screening",
	}

	if err := db.Dao.Create(skill).Error; err != nil {
		panic(fmt.Sprintf("创建技能失败: %v", err))
	}
	fmt.Printf("SUCCESS: ID=%d, Name=%s\n", skill.ID, skill.Name)

	// Verify by reading back
	var saved models.Skill
	db.Dao.First(&saved, skill.ID)
	fmt.Printf("SystemPrompt (%d bytes):\n%s\n", len(saved.SystemPrompt), saved.SystemPrompt)
}
