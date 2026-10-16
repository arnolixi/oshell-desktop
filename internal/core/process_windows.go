//go:build windows

package core

import (
	"context"
	"golang.org/x/sys/windows"
	"os/exec"
	"strconv"
	"syscall"
)

func proxyShellCommand(ctx context.Context, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "cmd.exe")
	// cmd /s /c strips the outer pair of quotes. Go's default argument quoting
	// targets CommandLineToArgvW and escapes inner quotes incompatibly with cmd.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: syscall.EscapeArg(cmd.Path) + ` /d /s /c "` + command + `"`}
	return cmd
}

// These commands communicate over pipes, not an interactive console. Hiding a
// window alone still lets Windows allocate a console when the GUI starts a
// console executable, which can flash while connecting or opening settings.
func prepareCommand(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.HideWindow = true
	c.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
func stopCommand(c *exec.Cmd) {
	if c.Process != nil {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(c.Process.Pid), "/T", "/F")
		prepareCommand(cmd)
		cmd.Run()
		c.Process.Kill()
	}
}
