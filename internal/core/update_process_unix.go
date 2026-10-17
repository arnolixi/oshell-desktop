//go:build !windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func renameUpdateFile(source, destination string) error { return os.Rename(source, destination) }

func prepareUpdateCommand(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func waitUpdateParent(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("更新父进程无效")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if e := syscall.Kill(pid, 0); e == syscall.ESRCH {
			return nil
		} else if e != nil {
			return e
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("旧程序尚未退出，未替换程序文件")
}
