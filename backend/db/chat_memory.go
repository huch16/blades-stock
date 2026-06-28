package db

import (
	"go-stock/backend/models"
	"time"
)

var AutoMigrateList []interface{}

type ChatMemory struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	SessionID string    `gorm:"index;size:64" json:"sessionId"`
	Role      string    `gorm:"size:20" json:"role"`
	Content   string    `gorm:"type:text" json:"content"`
	Embedding []byte    `gorm:"type:blob" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ChatMemory) TableName() string {
	return "chat_memory"
}

func (c *ChatMemory) Save() error {
	return Dao.Create(c).Error
}

func GetChatMemoryList(sessionID string, limit int) ([]ChatMemory, error) {
	var memories []ChatMemory
	err := Dao.Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(limit).
		Find(&memories).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}
	return memories, nil
}

func GetRecentChatMemory(sessionID string, limit int) ([]ChatMemory, error) {
	var memories []ChatMemory
	var err error
	if sessionID == "" {
		err = Dao.Order("created_at DESC").
			Limit(limit).
			Find(&memories).Error
	} else {
		err = Dao.Where("session_id = ?", sessionID).
			Order("created_at DESC").
			Limit(limit).
			Find(&memories).Error
	}
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}
	return memories, err
}

func (c *ChatMemory) Delete() error {
	return Dao.Delete(c).Error
}

func CountChatMemory(sessionID string) (int, error) {
	var count int64
	err := Dao.Model(&ChatMemory{}).Where("session_id = ?", sessionID).Count(&count).Error
	return int(count), err
}

func ClearChatMemory(sessionID string) error {
	return Dao.Where("session_id = ?", sessionID).Delete(&ChatMemory{}).Error
}

type ChatMemorySummary struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	SessionID string    `gorm:"index;size:64" json:"sessionId"`
	Summary   string    `gorm:"type:text" json:"summary"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ChatMemorySummary) TableName() string {
	return "chat_memory_summary"
}

func (s *ChatMemorySummary) Save() error {
	return Dao.Create(s).Error
}

func GetChatMemorySummary(sessionID string) (*ChatMemorySummary, error) {
	var summary ChatMemorySummary
	err := Dao.Where("session_id = ?", sessionID).
		Order("created_at DESC").
		First(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

func DeleteChatMemorySummary(sessionID string) error {
	return Dao.Where("session_id = ?", sessionID).Delete(&ChatMemorySummary{}).Error
}

func AutoMigrate() {
	Dao.AutoMigrate(&ChatMemory{})
	Dao.AutoMigrate(&ChatMemorySummary{})
	Dao.AutoMigrate(&models.StockChangeHistory{})
	Dao.AutoMigrate(&models.MarketStatistic{})
	for _, m := range AutoMigrateList {
		Dao.AutoMigrate(m)
	}
}
