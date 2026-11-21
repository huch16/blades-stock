//go:build windows
// +build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func (a *App) RestartAsAdmin() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	shell32 := windows.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")

	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exePath)
	params, _ := windows.UTF16PtrFromString("")
	directory, _ := windows.UTF16PtrFromString("")

	shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(directory)),
		1,
	)

	os.Exit(0)
	return nil
}
