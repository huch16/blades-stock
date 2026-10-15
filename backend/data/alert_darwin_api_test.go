//go:build darwin
// +build darwin

package data

import (
	"testing"
)

func TestAlertDarwin(t *testing.T) {
	alert := NewAlertWindowsApi("blades-stock", "Hello, World!", "This is a notification.", "")
	if alert == nil {
		t.Fatal("expected non-nil alert")
	}
	// SendNotification delegates to osascript in macOS desktop environment
	_ = alert.SendNotification()
}
