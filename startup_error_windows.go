//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"runtime"
	"unsafe"
)

// GUI builds have no console: surface storage/initialization failures instead
// of silently exiting when a portable folder is not writable.
func showStartupError(err error) {
	message, convErr := windows.UTF16PtrFromString(err.Error())
	if convErr != nil {
		return
	}
	title, _ := windows.UTF16PtrFromString("OShell Desktop 启动失败")
	dialog := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	if dialog.Find() != nil {
		return
	}
	dialog.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
	runtime.KeepAlive(message)
	runtime.KeepAlive(title)
}
