package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"encoding/json"
	"gorm.io/gorm"
)

// Cache for skills to reduce DB hits and support hot reloading.
var (
	enabledSkillsCache []models.Skill
	allSkillsCache     []models.Skill
	cacheMu            sync.RWMutex
)

// loadEnabledSkillsCache loads enabled skills from DB.
// Returns nil on error.
func loadEnabledSkillsCache() []models.Skill {
	var skills []models.Skill
	if err := db.Dao.Where("enable = ?", true).Find(&skills).Error; err != nil {
		logger.SugaredLogger.Errorf("Failed to load enabled skills cache: %v", err)
		return nil
	}
	return skills
}

// loadAllSkillsCache loads all skills from DB.
// Returns nil on error.
func loadAllSkillsCache() []models.Skill {
	var skills []models.Skill
	if err := db.Dao.Find(&skills).Error; err != nil {
		logger.SugaredLogger.Errorf("Failed to load all skills cache: %v", err)
		return nil
	}
	return skills
}

// invalidateSkillsCache invalidates the skill caches.
// Next get will reload from DB.
func invalidateSkillsCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	enabledSkillsCache = nil
	allSkillsCache = nil
}

// SkillAPI provides operations for managing skills.
type SkillAPI struct {
	dao *gorm.DB
}

// NewSkillApi returns a new SkillAPI instance.
func NewSkillApi() *SkillAPI {
	return &SkillAPI{dao: db.Dao}
}

// Create saves a new skill to the database.
func (s *SkillAPI) Create(skill *models.Skill) error {
	if skill == nil {
		return errors.New("skill cannot be nil")
	}
	err := s.dao.Create(skill).Error
	if err == nil {
		invalidateSkillsCache()
	}
	return err
}

// List returns skills matching the given query.
func (s *SkillAPI) List(query *models.SkillQuery) *models.SkillPageResp {
	if query == nil {
		query = &models.SkillQuery{}
	}
	var result models.SkillPageResp
	db := s.dao.Model(&models.Skill{})
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Category != "" {
		db = db.Where("category = ?", query.Category)
	}
	if query.Enable != nil {
		db = db.Where("enable = ?", *query.Enable)
	}
	// Count total
	if err := db.Count(&result.Total).Error; err != nil {
		logger.SugaredLogger.Errorf("GetEnabledSkills query failed: %s", err.Error())
		return &result
	}
	// Pagination
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	// Get data
	var skills []models.Skill
	if err := db.Offset(offset).Limit(pageSize).Find(&skills).Error; err != nil {
		logger.SugaredLogger.Errorf("GetEnabledSkills query failed: %s", err.Error())
		return &result
	}
	result.Data = skills
	return &result
}

// GetByID returns a skill by its ID.
func (s *SkillAPI) GetByID(id uint) (*models.Skill, error) {
	var skill models.Skill
	if err := s.dao.First(&skill, id).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

// Update updates an existing skill in the database.
func (s *SkillAPI) Update(skill *models.Skill) error {
	if skill == nil || skill.ID == 0 {
		return errors.New("skill or skill ID cannot be zero")
	}
	err := s.dao.Save(skill).Error
	if err == nil {
		invalidateSkillsCache()
	}
	return err
}

// Delete deletes a skill by its ID.
func (s *SkillAPI) Delete(id uint) error {
	err := s.dao.Delete(&models.Skill{}, id).Error
	if err == nil {
		invalidateSkillsCache()
	}
	return err
}

// EnableSkill enables or disables a skill by setting its enable flag.
func (s *SkillAPI) EnableSkill(id uint, enable bool) error {
	err := s.dao.Model(&models.Skill{}).Where("id = ?", id).Update("enable", enable).Error
	if err == nil {
		invalidateSkillsCache()
	}
	return err
}

// GetEnabledSkills returns all skills that are enabled (active = true) from the database.
// Uses cache to reduce DB hits.
func (s *SkillAPI) GetEnabledSkills() []models.Skill {
	cacheMu.RLock()
	if enabledSkillsCache != nil {
		defer cacheMu.RUnlock()
		return enabledSkillsCache
	}
	cacheMu.RUnlock()

	// Load from DB
	skills := loadEnabledSkillsCache()
	if skills == nil {
		return []models.Skill{}
	}
	cacheMu.Lock()
	enabledSkillsCache = skills
	cacheMu.Unlock()
	return skills
}

// ReloadSkills reloads skills from the database and returns any error.
// This function can be called to hot-reload skill definitions without restarting the application.
// It invalidates the skill caches so that the next GetEnabledSkills or GetAll will reload from DB.
func (s *SkillAPI) ReloadSkills() error {
	invalidateSkillsCache()
	return nil
}

// GetAll returns all skills from the database.
// Uses cache to reduce DB hits.
func (s *SkillAPI) GetAll() []models.Skill {
	cacheMu.RLock()
	if allSkillsCache != nil {
		defer cacheMu.RUnlock()
		return allSkillsCache
	}
	cacheMu.RUnlock()

	// Load from DB
	skills := loadAllSkillsCache()
	if skills == nil {
		return []models.Skill{}
	}
	cacheMu.Lock()
	allSkillsCache = skills
	cacheMu.Unlock()
	return skills
}

// GetMCPServerIDs parses the MCPServerIDs field of a skill and returns a slice of uint.
func (s *SkillAPI) GetMCPServerIDs(skill *models.Skill) []uint {
	if skill == nil || skill.MCPServerIDs == "" {
		return []uint{}
	}
	parts := strings.Split(skill.MCPServerIDs, ",")
	ids := make([]uint, 0, len(parts))
	for _, part := range parts {
		if id, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			ids = append(ids, uint(id))
		}
	}
	return ids
}

// CreateSkillVersion creates a new version for a skill.
func (s *SkillAPI) CreateSkillVersion(skillID uint) (*models.SkillVersion, error) {
	skill, err := s.GetByID(skillID)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, fmt.Errorf("skill %d not found", skillID)
	}
	// Determine next version number
	var lastVersion models.SkillVersion
	err = s.dao.Model(&models.SkillVersion{}).
		Where("skill_id = ?", skillID).
		Order("version_number desc").
		First(&lastVersion).Error
	var versionNumber int
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		versionNumber = 1
	} else {
		versionNumber = lastVersion.VersionNumber + 1
	}
	// Serialize skill to JSON
	skillBytes, err := json.Marshal(skill)
	if err != nil {
		return nil, err
	}
	// Deactivate any existing active version for this skill (optional, we'll keep IsActive false by default)
	// Create version
	ver := &models.SkillVersion{
		SkillID:      skillID,
		VersionNumber: versionNumber,
		Content:      string(skillBytes),
		CreatedAt:    time.Now(),
		IsActive:     false, // new versions are not active by default; activation via ActivateSkillVersion
	}
	err = s.dao.Create(ver).Error
	if err != nil {
		return nil, err
	}
	return ver, nil
}

// GetSkillVersions returns paginated list of versions for a skill.
func (s *SkillAPI) GetSkillVersions(query *models.SkillVersionQuery) (*models.SkillVersionPageResp, error) {
	if query == nil {
		query = &models.SkillVersionQuery{}
	}
	var result models.SkillVersionPageResp
	db := s.dao.Model(&models.SkillVersion{})
	if query.SkillID != 0 {
		db = db.Where("skill_id = ?", query.SkillID)
	}
	if query.Active != nil {
		db = db.Where("is_active = ?", *query.Active)
	}
	// Count total
	if err := db.Count(&result.Total).Error; err != nil {
		logger.SugaredLogger.Errorf("GetSkillVersions count failed: %v", err)
		return &result
	}
	// Pagination
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	// Get data
	var versions []models.SkillVersion
	if err := db.Offset(offset).Limit(pageSize).Find(&versions).Error; err != nil {
		logger.SugaredLogger.Errorf("GetSkillVersions find failed: %v", err)
		return &result
	}
	result.Data = versions
	return &result, nil
}

// GetActiveSkillVersion returns the active version for a skill.
func (s *SkillAPI) GetActiveSkillVersion(skillID uint) (*models.SkillVersion, error) {
	var ver models.SkillVersion
	err := s.dao.Where("skill_id = ? AND is_active = true", skillID).First(&ver).Error
	if err != nil {
		return nil, err
	}
	return &ver, nil
}

// ActivateSkillVersion activates a specific version, deactivating all other versions of the same skill.
func (s *SkillAPI) ActivateSkillVersion(versionID uint) error {
	var ver models.SkillVersion
	err := s.dao.First(&ver, versionID).Error
	if err != nil {
		return err
	}
	if ver == nil {
		return fmt.Errorf("skill version %d not found", versionID)
	}
	// Deactivate all versions of this skill
	if err := s.dao.Model(&models.SkillVersion{}).Where("skill_id = ?", ver.SkillID).Update("is_active", false).Error; err != nil {
		return err
	}
	// Activate the requested version
	if err := s.dao.Model(&models.SkillVersion{}).Where("id = ?", versionID).Update("is_active", true).Error; err != nil {
		return err
	}
	return nil
}
