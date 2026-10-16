//go:build windows

package core

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

func openImportFile(path string) (*os.File, error) {
	name, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	// Readers keep their snapshot while another process atomically publishes
	// the next version. Without SHARE_DELETE, even a short read blocks rename.
	handle, e := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_SEQUENTIAL_SCAN, 0)
	if e != nil {
		// Windows also reports PATH_NOT_FOUND when a parent is a regular file.
		// Do not let callers treat an obstructed path as absent/empty data.
		if errors.Is(e, windows.ERROR_PATH_NOT_FOUND) {
			for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
				info, statErr := os.Stat(parent)
				if statErr == nil {
					if !info.IsDir() {
						e = windows.ERROR_DIRECTORY
					}
					break
				}
				if !os.IsNotExist(statErr) {
					e = statErr
					break
				}
				if filepath.Dir(parent) == parent {
					break
				}
			}
		}
		return nil, &os.PathError{Op: "open", Path: path, Err: e}
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
