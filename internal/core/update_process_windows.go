//go:build windows

package core

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Windows can briefly retain image/scanner handles after process exit. Retry
// only sharing/access errors; persistent failures still preserve the journal.
func renameUpdateFile(source, destination string) error {
	deadline := time.Now().Add(3 * time.Second)
	for {
		err := os.Rename(source, destination)
		if err == nil || (!errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION)) || !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func prepareUpdateCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
func waitUpdateParent(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("更新父进程无效")
	}
	h, e := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if e == windows.ERROR_INVALID_PARAMETER {
		return nil
	}
	if e != nil {
		return e
	}
	defer windows.CloseHandle(h)
	result, e := windows.WaitForSingleObject(h, uint32(timeout/time.Millisecond))
	if e != nil {
		return e
	}
	if result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("旧程序尚未退出，未替换程序文件")
	}
	return nil
}
