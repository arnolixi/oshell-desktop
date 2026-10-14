//go:build windows

package core

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func openImportFile(path string) (*os.File, error) {
	name, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	handle, e := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_SEQUENTIAL_SCAN, 0)
	if e != nil {
		return nil, e
	}
	var info windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(handle, &info); e != nil {
		windows.CloseHandle(handle)
		return nil, e
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("不读取重解析点或符号链接")
	}
	return os.NewFile(uintptr(handle), path), nil
}
