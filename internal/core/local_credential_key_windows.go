//go:build windows

package core

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func validateLocalKeyFile(file *os.File, limit int64, repair bool) error {
	info, e := file.Stat()
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return fmt.Errorf("本机密钥文件无效")
	}
	return nil
}
func lockLocalKey(dir string) (func(), error) { return lockSyncDirectory(context.Background(), dir) }
func syncLocalKeyDirectory(string) error      { return nil }

// Same-directory atomic move without REPLACE_EXISTING also works on portable
// FAT/exFAT volumes, which do not support the hard links used on Unix.
func publishLocalCredentialKey(source, destination string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
}
