//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

func prepareCommand(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func stopCommand(c *exec.Cmd) {
	if c.Process != nil {
		syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
}
