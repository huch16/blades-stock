package models

import (
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type VersionInfo struct {
	gorm.Model
	Version           string                `json:"version"`
	Content           string                `json:"content"`
	Icon              string                `json:"icon"`
	Alipay            string                `json:"alipay"`
	Wxpay             string                `json:"wxpay"`
	BuildTimeStamp    int64                 `json:"buildTimeStamp"`
	OfficialStatement string                `json:"officialStatement"`
	IsDel             soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

func (receiver VersionInfo) TableName() string {
	return "version_info"
}
