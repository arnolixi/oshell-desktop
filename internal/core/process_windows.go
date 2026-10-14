//go:build windows

package core

import (
	"os/exec"
	"strconv"
	"syscall"
)

func prepareCommand(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func stopCommand(c *exec.Cmd) {
	if c.Process != nil {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(c.Process.Pid), "/T", "/F")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Run()
		c.Process.Kill()
	}
}
