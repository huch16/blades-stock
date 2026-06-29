package agent

import (
	"encoding/json"
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"strings"
)

type SkillGenerator struct {
	embeddingService *EmbeddingService
}

func NewSkillGenerator(es *EmbeddingService) *SkillGenerator {
	return &SkillGenerator{embeddingService: es}
}

type SkillGenerationRequest struct {
	SourceURL    string
	Name         string
	Description  string
	TechStack    string
	Readme       string
	Structure    string
	Summary      string
	EntryPoints  []string
	APISummary   string
	ModelSummary string
}

func (sg *SkillGenerator) GenerateSkill(projectID uint) (*models.Skill, error) {
	var analysis models.ProjectAnalysis
	if err := db.Dao.First(&analysis, projectID).Error; err != nil {
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	if analysis.Status != "indexed" {
		return nil, fmt.Errorf("项目尚未完成分析，当前状态: %s", analysis.Status)
	}

	var entryPoints []string
	if analysis.EntryPoints != "" {
		json.Unmarshal([]byte(analysis.EntryPoints), &entryPoints)
	}

	req := &SkillGenerationRequest{
		SourceURL:    analysis.SourceURL,
		Name:         analysis.Name,
		Description:  analysis.Name + " 项目专家",
		TechStack:    analysis.TechStack,
		Readme:       analysis.Readme,
		Structure:    analysis.Structure,
		Summary:      analysis.Summary,
		EntryPoints:  entryPoints,
		APISummary:   analysis.APISummary,
		ModelSummary: analysis.ModelSummary,
	}

	skill := sg.buildSkillRecord(req)

	skill.Enable = true
	skill.SortOrder = 0

	if err := db.Dao.Create(skill).Error; err != nil {
		return nil, fmt.Errorf("保存 Skill 失败: %w", err)
	}

	analysis.SkillID = skill.ID
	analysis.Status = "skill_ready"
	if err := db.Dao.Save(&analysis).Error; err != nil {
		logger.SugaredLogger.Warnf("更新项目分析状态失败: %v", err)
	}

	logger.SugaredLogger.Infof("Skill 生成完成: %s (ID=%d, 关联项目=%s)", skill.Name, skill.ID, analysis.Name)
	return skill, nil
}

func (sg *SkillGenerator) buildSkillRecord(req *SkillGenerationRequest) *models.Skill {
	name := req.Name
	if name == "" {
		name = "未知项目"
	}
	skillName := fmt.Sprintf("%s 专家助手", name)

	description := fmt.Sprintf("你精通 %s 项目的架构设计、代码实现和技术细节", name)
	if req.Description != "" {
		description = req.Description
	}

	systemPrompt := sg.buildSystemPrompt(req)

	var keywords []string
	keywords = append(keywords, strings.ToLower(name))
	if req.TechStack != "" {
		for _, tech := range strings.FieldsFunc(req.TechStack, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
			tech = strings.TrimSpace(tech)
			if tech != "" && !strings.HasPrefix(tech, "Go") && !strings.HasPrefix(tech, "Node") {
				keywords = append(keywords, strings.ToLower(tech))
			}
		}
	}
	keywords = append(keywords, name)
	keywordStr := strings.Join(uniqueStrings(keywords), ",")

	examples := sg.buildExamples(req)

	mcpServerIDs := "[]"

	skill := &models.Skill{
		Name:            skillName,
		Description:     description,
		Category:        "project",
		SystemPrompt:    systemPrompt,
		Examples:        examples,
		TriggerKeywords: keywordStr,
		MCPServerIDs:    mcpServerIDs,
		Enable:          true,
		SortOrder:       0,
	}

	return skill
}

func (sg *SkillGenerator) buildSystemPrompt(req *SkillGenerationRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("你是 %s 项目的专家，精通该项目的架构设计、代码实现和技术细节。\n\n", req.Name))
	sb.WriteString("请利用你掌握的项目知识回答用户问题。如果需要查阅最新代码或文件，请使用 ReadProjectFile、ListProjectDir、SearchProjectFiles 等工具获取实时信息。\n\n")

	if req.Summary != "" {
		sb.WriteString("## 项目概览\n")
		sb.WriteString(req.Summary + "\n\n")
	}

	if req.TechStack != "" {
		sb.WriteString("## 技术栈\n")
		sb.WriteString(req.TechStack + "\n\n")
	}

	if req.Structure != "" {
		sb.WriteString("## 目录结构\n")
		sb.WriteString("```\n" + req.Structure + "```\n\n")
	}

	if len(req.EntryPoints) > 0 {
		sb.WriteString("## 入口文件\n")
		for _, ep := range req.EntryPoints {
			sb.WriteString("- " + ep + "\n")
		}
		sb.WriteString("\n")
	}

	if req.APISummary != "" {
		sb.WriteString("## API 接口\n")
		sb.WriteString(req.APISummary + "\n\n")
	}

	if req.ModelSummary != "" {
		sb.WriteString("## 数据模型\n")
		sb.WriteString(req.ModelSummary + "\n\n")
	}

	if req.Readme != "" {
		readmePreview := req.Readme
		if len(readmePreview) > 2000 {
			readmePreview = readmePreview[:2000] + "\n...（README 过长，已截断）"
		}
		sb.WriteString("## README\n")
		sb.WriteString(readmePreview + "\n\n")
	}

	sb.WriteString("## 行为准则\n")
	sb.WriteString("1. 优先用自己的知识回答，遇到不确定的代码细节使用 ReadProjectFile 工具查阅\n")
	sb.WriteString("2. 回答问题时始终引用具体的文件路径和代码行号\n")
	sb.WriteString("3. 如需修改代码，先阅读相关文件内容理解上下文\n")
	sb.WriteString("4. 无法回答的问题，明确告知用户并提供替代建议\n")

	return sb.String()
}

func (sg *SkillGenerator) buildExamples(req *SkillGenerationRequest) string {
	var sb strings.Builder

	if req.Name != "" {
		sb.WriteString(fmt.Sprintf("用户: 请介绍一下 %s 项目的整体架构\n", req.Name))
		sb.WriteString(fmt.Sprintf("助手: 好的，%s 项目是一个基于 %s 的项目，主要目录结构如下：\n", req.Name, req.TechStack))
		if req.Structure != "" {
			lines := strings.SplitN(req.Structure, "\n", 10)
			for _, line := range lines {
				sb.WriteString(line + "\n")
			}
		}
		sb.WriteString("\n")
	}

	if len(req.EntryPoints) > 0 {
		sb.WriteString(fmt.Sprintf("用户: %s 的入口文件在哪里？\n", req.Name))
		sb.WriteString("助手: 入口文件位于以下路径：\n")
		for _, ep := range req.EntryPoints {
			sb.WriteString("- " + ep + "\n")
		}
		sb.WriteString("你可以使用 ReadProjectFile 工具查看具体内容。\n\n")
	}

	sb.WriteString(fmt.Sprintf("用户: 帮我看看 %s 项目的代码\n", req.Name))
	sb.WriteString("助手: 好的，我来读取相关文件。请告诉我你想看哪个文件的具体内容？\n")
	sb.WriteString("也可以使用 SearchProjectFiles 工具搜索感兴趣的文件。\n")

	return sb.String()
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(input))
	for _, s := range input {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func (sg *SkillGenerator) GetProjectSkills(projectID uint) (*models.Skill, error) {
	var skills []models.Skill
	if err := db.Dao.Where("category = ?", "project").Find(&skills).Error; err != nil {
		return nil, err
	}

	var analysis models.ProjectAnalysis
	if err := db.Dao.First(&analysis, projectID).Error; err != nil {
		return nil, err
	}

	if analysis.SkillID == 0 {
		return nil, nil
	}

	var skill models.Skill
	if err := db.Dao.First(&skill, analysis.SkillID).Error; err != nil {
		return nil, err
	}

	return &skill, nil
}

func (sg *SkillGenerator) DeleteSkillForProject(projectID uint) error {
	var analysis models.ProjectAnalysis
	if err := db.Dao.First(&analysis, projectID).Error; err != nil {
		return err
	}

	if analysis.SkillID > 0 {
		if err := db.Dao.Delete(&models.Skill{}, analysis.SkillID).Error; err != nil {
			return err
		}
	}

	analysis.SkillID = 0
	analysis.Status = "indexed"
	return db.Dao.Save(&analysis).Error
}

func (sg *SkillGenerator) ImportProject(sourceURL string, name string) (*models.ProjectAnalysis, error) {
	var analysis models.ProjectAnalysis
	if sourceURL != "" {
		db.Dao.Where("source_url = ?", sourceURL).First(&analysis)
	}

	if analysis.ID == 0 {
		analysis = models.ProjectAnalysis{
			SourceURL: sourceURL,
			Name:      name,
			Status:    "pending",
		}
		if err := db.Dao.Create(&analysis).Error; err != nil {
			return nil, err
		}
	}

	return &analysis, nil
}

func (sg *SkillGenerator) FullPipeline(sourceURL, localPath, name string) (*models.Skill, error) {
	analysis, err := sg.ImportProject(sourceURL, name)
	if err != nil {
		return nil, fmt.Errorf("导入项目失败: %w", err)
	}

	pa := NewProjectAnalyzer(sg.embeddingService)
	if err := pa.AnalyzeProject(analysis.ID, localPath); err != nil {
		analysis.Status = "failed"
		db.Dao.Save(analysis)
		return nil, fmt.Errorf("分析项目失败: %w", err)
	}

	skill, err := sg.GenerateSkill(analysis.ID)
	if err != nil {
		return nil, fmt.Errorf("生成 Skill 失败: %w", err)
	}

	return skill, nil
}
