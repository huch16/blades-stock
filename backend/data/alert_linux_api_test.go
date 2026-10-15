//go:build linux
// +build linux

package data

import (
	"testing"
)

func TestAlertLinux(t *testing.T) {
	alert := NewAlertWindowsApi("blades-stock", "Hello, Linux!", "This is a notification.", "")
	if alert == nil {
		t.Fatal("expected non-nil alert")
	}
	// SendNotification delegates to notify-send in Linux desktop environment
	_ = alert.SendNotification()
}
