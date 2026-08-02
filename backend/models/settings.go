package models

import "gorm.io/gorm"

type OldSettings struct {
	gorm.Model
	TushareToken           string `json:"tushareToken"`
	LocalPushEnable        bool   `json:"localPushEnable"`
	DingPushEnable         bool   `json:"dingPushEnable"`
	DingRobot              string `json:"dingRobot"`
	UpdateBasicInfoOnStart bool   `json:"updateBasicInfoOnStart"`
	RefreshInterval        int64  `json:"refreshInterval"`

	OpenAiEnable      bool    `json:"openAiEnable"`
	OpenAiBaseUrl     string  `json:"openAiBaseUrl"`
	OpenAiApiKey      string  `json:"openAiApiKey"`
	OpenAiModelName   string  `json:"openAiModelName"`
	OpenAiMaxTokens   int     `json:"openAiMaxTokens"`
	OpenAiTemperature float64 `json:"openAiTemperature"`
	OpenAiApiTimeOut  int     `json:"openAiApiTimeOut"`
	Prompt            string  `json:"prompt"`
	CheckUpdate       bool    `json:"checkUpdate"`
	QuestionTemplate  string  `json:"questionTemplate"`
	CrawlTimeOut      int64   `json:"crawlTimeOut"`
	KDays             int64   `json:"kDays"`
	EnableDanmu       bool    `json:"enableDanmu"`
	BrowserPath       string  `json:"browserPath"`
	EnableNews        bool    `json:"enableNews"`
	DarkTheme         bool    `json:"darkTheme"`
	EnableFund        bool    `json:"enableFund"`
	EnablePushNews    bool    `json:"enablePushNews"`
}

func (receiver OldSettings) TableName() string {
	return "settings"
}
