//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var singleInstanceMutex uintptr

// acquireSingleInstanceLock ensures only one instance of the app runs at a
// time. Returns false if another instance is already running.
func acquireSingleInstanceLock() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	createMutex := k32.NewProc("CreateMutexW")
	name, _ := syscall.UTF16PtrFromString("BladesStockSingleInstance")
	h, _, lastErr := createMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return true
	}
	if lastErr == syscall.ERROR_ALREADY_EXISTS {
		syscall.CloseHandle(syscall.Handle(h))
		return false
	}
	singleInstanceMutex = h
	return true
}

func releaseSingleInstanceLock() {
	if singleInstanceMutex != 0 {
		syscall.CloseHandle(syscall.Handle(singleInstanceMutex))
		singleInstanceMutex = 0
	}
}

func showAlreadyRunningMessage() {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	title, _ := syscall.UTF16PtrFromString("blades-stock")
	text, _ := syscall.UTF16PtrFromString("\u7a0b\u5e8f\u5df2\u5728\u8fd0\u884c\uff0c\u8bf7\u52ff\u91cd\u590d\u542f\u52a8\u3002")
	messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0)
}
