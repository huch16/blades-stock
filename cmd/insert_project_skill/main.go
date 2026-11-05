package main

import (
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"fmt"
	"os"
)

func main() {
	db.Init("data/stock.db?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288")

	directiveBytes, err := os.ReadFile("cmd/insert_project_skill/project_directive.txt")
	if err != nil {
		panic(fmt.Sprintf("读取 directive 文件失败: %v", err))
	}
	directive := string(directiveBytes)

	// Delete old skill if exists
	db.Dao.Where("name LIKE ?", "%项目交付%").Delete(&models.Skill{})

	skill := &models.Skill{
		Name:            "项目交付工作流",
		Description:     "项目难度评估、代码理解、任务拆解、分步实现、编译验证、知识沉淀。覆盖系统工程全生命周期。",
		Category:        "system-engineering",
		Directive:       directive,
		TriggerKeywords: "项目,代码,功能,Bug,实现,重构,架构,开发,编译,测试,构建,写文件,改代码,读代码,提PR,提交,GitOperation,WriteProjectFile,ReadProjectFile,RunCommand,LearnProject,git,commit,build,deploy,修Bug,加功能,代码审查,项目复杂度,任务拆解,工程,系统设计,技术方案,模块,接口,依赖",
		Examples: `用户：帮我分析这个项目的复杂度，估算需要多久实现
助手：开始 P1 项目理解 → ListProjectDir → 读关键文件 → GitOperation log → 输出项目地图和复杂度评估

用户：给 backend/agent/agent.go 加一个日志，每次工具调用时记录耗时
助手：P3 规划 → P4 执行 ReadProjectFile → WriteProjectFile → RunCommand("go build ./..." 验证) → GitOperation(diff) → 交付

用户：这个项目的测试覆盖情况怎么样？
助手：SearchProjectFiles("**_test.go") → GitOperation(log -30) → 输出测试覆盖分析和建议`,
		Enable:    true,
		SortOrder: 1,
	}

	if err := db.Dao.Create(skill).Error; err != nil {
		panic(fmt.Sprintf("创建技能失败: %v", err))
	}
	fmt.Printf("SUCCESS: ID=%d, Name=%s\n", skill.ID, skill.Name)
	fmt.Printf("Directive: %d bytes\n", len(skill.Directive))
}
