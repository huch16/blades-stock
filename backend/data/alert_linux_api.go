//go:build linux
// +build linux

package data

import (
	"os/exec"

	"blades-stock/backend/logger"
)

// AlertWindowsApi provides desktop notification capability on Linux via notify-send.
type AlertWindowsApi struct {
	AppID   string
	Title   string
	Content string
	Icon    string
}

func NewAlertWindowsApi(AppID string, Title string, Content string, Icon string) *AlertWindowsApi {
	return &AlertWindowsApi{
		AppID:   AppID,
		Title:   Title,
		Content: Content,
		Icon:    Icon,
	}
}

func (a AlertWindowsApi) SendNotification() bool {
	if !GetSettingConfig().LocalPushEnable {
		return false
	}

	cmd := exec.Command("notify-send", a.Title, a.Content)
	if err := cmd.Run(); err != nil {
		logger.SugaredLogger.Warnf("Linux notify-send error: %v", err)
		return false
	}
	return true
}
