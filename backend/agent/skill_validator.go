// Package agent 的 Skill 校验模块（P3.2）。
//
// 设计目标：
//   - 在 AutoMigrate 之后、Agent 启动之前，扫描所有 Skill.ToolGroups
//     字段，凡包含非法 ToolGroup 名称 → 记 Warn（不要 log.Fatal：
//     一个技能的配置 typo 不应让整个应用闪退；该函数目前尚未接线，
//     见 ValidateSkillToolGroups 的注释，属 dormant 的校验工具）。
//   - 在 matchedSkills 注入 Directive 之前，校验每个 Skill 的 ToolGroups
//     实际能在当前 agent 上下文中满足，缺失的工具加入警告标记
package agent

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/tools"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

// validToolGroupNames 是 ToolGroup 枚举的合法字符串集合。
// 单一事实源：与 backend/agent/tools/tool_groups.go 的 Group 常量对应。
//
// 重要：保持与 tools.ParseToolGroups 的可解析值一致。
var validToolGroupNames = []string{
	"base",           // tools.GroupBase
	"stock_analysis", // tools.GroupStockAnalysis
	"market",         // tools.GroupMarket
	"screening",      // tools.GroupScreening
	"money_flow",     // tools.GroupMoneyFlow
	"news_research",  // tools.GroupNewsResearch
	"ai_analysis",    // tools.GroupAIAnalysis
	"operations",     // tools.GroupOperations
	"project",        // tools.GroupProject
}

// validToolGroupSet 是 Set 版本，O(1) 查找
var validToolGroupSet = buildToolGroupSet()

func buildToolGroupSet() map[string]bool {
	s := make(map[string]bool, len(validToolGroupNames))
	for _, n := range validToolGroupNames {
		s[n] = true
	}
	return s
}

// IsValidToolGroup 判断 name 是否为合法 ToolGroup。
func IsValidToolGroup(name string) bool {
	return validToolGroupSet[name]
}

// ValidateSkillToolGroups 扫描 DB 中所有 Skill.ToolGroups，凡包含非法名 → 记录警告日志（避免因配置错误导致应用闪退）。
// 设计：在 AutoMigrate 后、SeedDefaultSkills 后调用一次。
//
// 注意：本函数目前全仓零生产调用者（dormant）——启动路径从未接线，
// 校验只存在于测试中。不要误以为技能配置已在启动时被校验过；
// 要恢复校验应从启动处调用一次，而不是删掉本函数。
//
// 失败模式：
//   - 非法 ToolGroup → logger.Warnf（平滑容错，避免意外崩溃）
//   - 空 ToolGroups 字段 → 仅 Debug（合法但可能没注入工具）
func ValidateSkillToolGroups() {
	var skills []models.Skill
	if err := db.Dao.Find(&skills).Error; err != nil {
		logger.SugaredLogger.Errorf("ValidateSkillToolGroups: query failed: %v", err)
		return
	}
	for _, s := range skills {
		if err := ValidateOneSkillToolGroups(s); err != nil {
			logger.SugaredLogger.Warnf("Skill '%s' (ID=%d) has invalid ToolGroups: %v",
				s.Name, s.ID, err)
		}
	}
	logger.SugaredLogger.Infof("ValidateSkillToolGroups: %d skills validated", len(skills))
}

// ValidateOneSkillToolGroups 校验单个 Skill 的 ToolGroups 字段。
// 暴露为公共函数以便测试。
func ValidateOneSkillToolGroups(s models.Skill) error {
	if s.ToolGroups == "" {
		return nil // 空字段合法（无强制工具）
	}
	for _, raw := range strings.Split(s.ToolGroups, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !validToolGroupSet[name] {
			return fmt.Errorf("invalid ToolGroup %q (must be one of %v)",
				name, validToolGroupNames)
		}
	}
	return nil
}

// CheckSkillDependencies 检查 matched Skill 的 ToolGroups 实际能否在 available 工具集中满足。
// 返回每个 Skill 缺失的工具组（用于在 Directive 顶部加 ⚠️ 警告）。
//
// 参数：
//   - skill: 待检查的 Skill
//   - availableGroups: 当前 agent 已开启的 ToolGroup 集合（来自 ClassifyQuestion + mergeSkillToolGroups）
//
// 返回：
//   - missingGroups: skill 需要的、但 available 中没有的 ToolGroup 列表
//   - nil: 完全满足（无缺失）
func CheckSkillDependencies(skill models.Skill, availableGroups map[tools.ToolGroup]bool) []string {
	if skill.ToolGroups == "" {
		return nil
	}
	var missing []string
	for _, raw := range strings.Split(skill.ToolGroups, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !availableGroups[tools.ToolGroup(name)] {
			missing = append(missing, name)
		}
	}
	return missing
}

// WarnMissingSkillDependencies 在 matchedSkills 注入前调用，给缺失依赖的 Skill 加警告标记。
// 行为：在 Skill.Directive 顶部注入 ⚠️ 警告块（仅在原 Directive 不含此警告时追加）。
//
// 返回值：修改后的 Skill 列表（不修改原对象，深拷贝）。
func WarnMissingSkillDependencies(matched []models.Skill, availableGroups map[tools.ToolGroup]bool) []models.Skill {
	out := make([]models.Skill, 0, len(matched))
	for _, s := range matched {
		missing := CheckSkillDependencies(s, availableGroups)
		if len(missing) == 0 {
			out = append(out, s)
			continue
		}
		// 深拷贝避免修改原对象
		copy := s
		warning := fmt.Sprintf("\n⚠️ 警告：本 Skill 声明依赖 %v，但当前会话已注入的工具组中没有这些；"+
			"部分指令可能无法执行。\n", missing)
		// 仅追加一次（避免重复注入）：Directive 里只要出现过 "⚠️"
		// 就不再追加警告块。
		if !strings.Contains(copy.Directive, "⚠️") {
			copy.Directive = warning + copy.Directive
		}
		out = append(out, copy)
	}
	return out
}
