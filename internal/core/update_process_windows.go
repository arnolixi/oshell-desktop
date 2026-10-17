//go:build windows

package core

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"time"
)

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
