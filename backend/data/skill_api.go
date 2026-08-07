package data

import (
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SkillApi struct{}

func NewSkillApi() *SkillApi {
	return &SkillApi{}
}

// enabledSkillsCache caches GetEnabledSkills() for a short TTL. Skills are
// loaded on every chat request (buildSkillPrompt + skill matching), which
// turns a trivial query into avoidable per-request DB round-trips. The cache
// is invalidated by any write path (Create/Update/Delete/EnableSkill) so
// runtime edits stay consistent.
var (
	enabledSkillsCache      []models.Skill
	enabledSkillsCacheTime  time.Time
	enabledSkillsCacheMu    sync.Mutex
	enabledSkillsCacheTTL   = 10 * time.Second
)

func invalidateEnabledSkillsCache() {
	enabledSkillsCacheMu.Lock()
	enabledSkillsCache = nil
	enabledSkillsCacheTime = time.Time{}
	enabledSkillsCacheMu.Unlock()
}

func (a *SkillApi) Create(skill *models.Skill) error {
	if err := db.Dao.Create(skill).Error; err != nil {
		return err
	}
	invalidateEnabledSkillsCache()
	return nil
}

func (a *SkillApi) Update(skill *models.Skill) error {
	if skill == nil || skill.ID == 0 {
		return fmt.Errorf("无效的技能ID")
	}

	updates := map[string]any{
		"name":             skill.Name,
		"description":      skill.Description,
		"category":         skill.Category,
		"system_prompt":    skill.SystemPrompt,
		"directive":        skill.Directive,
		"examples":         skill.Examples,
		"trigger_keywords": skill.TriggerKeywords,
		"tool_groups":      skill.ToolGroups,
		"mcp_server_ids":   skill.MCPServerIDs,
		"enable":           skill.Enable,
		"sort_order":       skill.SortOrder,
	}

	if err := db.Dao.Model(&models.Skill{}).Where("id = ?", skill.ID).Updates(updates).Error; err != nil {
		return err
	}
	invalidateEnabledSkillsCache()
	return nil
}

func (a *SkillApi) Delete(id uint) error {
	err := db.Dao.Delete(&models.Skill{}, id).Error
	invalidateEnabledSkillsCache()
	return err
}

func (a *SkillApi) GetByID(id uint) (*models.Skill, error) {
	var skill models.Skill
	err := db.Dao.First(&skill, id).Error
	if err != nil {
		return nil, err
	}
	return &skill, nil
}

func (a *SkillApi) List(query *models.SkillQuery) *models.SkillPageResp {
	var skills []models.Skill
	var total int64

	q := db.Dao.Model(&models.Skill{})

	if query.Name != "" {
		q = q.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Category != "" {
		q = q.Where("category = ?", query.Category)
	}
	if query.Enable != nil {
		q = q.Where("enable = ?", *query.Enable)
	}

	q.Count(&total)

	offset := (query.Page - 1) * query.PageSize
	q.Order("sort_order ASC, created_at DESC").Offset(offset).Limit(query.PageSize).Find(&skills)

	return &models.SkillPageResp{
		Total: int(total),
		Data:  skills,
	}
}

func (a *SkillApi) GetAll() []models.Skill {
	var skills []models.Skill
	db.Dao.Where("enable = ?", true).Order("sort_order ASC, created_at DESC").Find(&skills)
	return skills
}

func (a *SkillApi) EnableSkill(id uint, enable bool) error {
	err := db.Dao.Model(&models.Skill{}).Where("id = ?", id).Update("enable", enable).Error
	invalidateEnabledSkillsCache()
	return err
}

func (a *SkillApi) GetEnabledSkills() []models.Skill {
	enabledSkillsCacheMu.Lock()
	defer enabledSkillsCacheMu.Unlock()
	if enabledSkillsCache != nil && time.Since(enabledSkillsCacheTime) < enabledSkillsCacheTTL {
		return enabledSkillsCache
	}
	var skills []models.Skill
	if err := db.Dao.Where("enable = ?", true).Order("sort_order ASC, created_at DESC").Find(&skills).Error; err != nil {
		logger.SugaredLogger.Warnf("GetEnabledSkills query failed: %v", err)
		if enabledSkillsCache != nil {
			return enabledSkillsCache
		}
		return nil
	}
	enabledSkillsCache = skills
	enabledSkillsCacheTime = time.Now()
	return skills
}

func (a *SkillApi) GetMCPServerIDs(skill *models.Skill) []uint {
	if skill.MCPServerIDs == "" {
		return nil
	}
	parts := strings.Split(skill.MCPServerIDs, ",")
	var ids []uint
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, uint(id))
	}
	return ids
}
