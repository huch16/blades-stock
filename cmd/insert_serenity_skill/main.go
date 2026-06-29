package main

import (
	"fmt"
	"os"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func main() {
	db.Init("data/stock.db?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288")

	promptBytes, err := os.ReadFile("cmd/insert_serenity_skill/serenity_prompt.txt")
	if err != nil {
		panic(fmt.Sprintf("读取 prompt 文件失败: %v", err))
	}
	systemPrompt := string(promptBytes)

	// Delete old skill if exists
	db.Dao.Where("name LIKE ?", "%Serenity%").Delete(&models.Skill{})

	skill := &models.Skill{
		Name:            "Serenity式产业分析",
		Description:     "强制使用Serenity产业分析框架：产业链拆解→稀缺层识别→优先级排序→证据验证→证伪条件",
		Category:        "research",
		SystemPrompt:    systemPrompt,
		TriggerKeywords: "产业分析,产业链,稀缺性,瓶颈,行业研究,赛道分析,上下游,供需格局,渗透率,国产替代,市场份额,竞争格局,华阳集团,投资机会,细分龙头",
		Examples: `用户：分析华阳集团在汽车电子产业链中的位置
助手：必须输出Serenity报告结构：产业链全景图→稀缺层识别→优先级排序→证据链→证伪条件`,
		Enable:    true,
		SortOrder: 1,
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
