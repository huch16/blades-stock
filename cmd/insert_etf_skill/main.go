package main

import (
	"fmt"
	"os"
	"go-stock/backend/db"
	"go-stock/backend/models"
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

	skill := &models.Skill{
		Name:            "ETF轮动多Agent分析",
		Description:     "执行完整的多Agent ETF轮动分析流程：21日加权动量筛选→6路因子并发分析(fan-out)→投委会5视角加权融合→输出开盘前交易决策",
		Category:        "quantitative",
		SystemPrompt:    systemPrompt,
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
