//go:build darwin

package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func LauncherConfigurationDirectory() (string, error) {
	if dir := os.Getenv("OSHELL_DATA_DIR"); dir != "" {
		return filepath.Abs(dir)
	}
	dir, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "oshell-desktop"), nil
}
func LaunchApplicationExecutable(helper string) (string, error) {
	absolute, e := filepath.Abs(helper)
	if e != nil {
		return "", e
	}
	absolute, e = filepath.EvalSymlinks(absolute)
	if e != nil {
		return "", e
	}
	for dir := filepath.Dir(absolute); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if filepath.Ext(dir) != ".app" {
			continue
		}
		bundleInfo, e := ReadAppBundleInfo(dir)
		if e != nil {
			return "", e
		}
		if id := bundleInfo.Identifier; id != "dev.oshell.desktop" && id != "dev.oshell.desktop.fullpreview" {
			return "", fmt.Errorf("请使用 OShell Desktop 应用内的启动助手")
		}
		name := bundleInfo.Executable
		executable := filepath.Join(dir, "Contents/MacOS", name)
		info, e := os.Stat(executable)
		if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return "", fmt.Errorf("应用执行文件不可用")
		}
		return executable, nil
	}
	return "", fmt.Errorf("请使用完整 OShell Desktop.app 内的启动助手")
}
func LaunchApplicationAndSend(ctx context.Context, configuration, executable string, request ExternalLaunchRequest) error {
	if e := request.Validate(); e != nil {
		return e
	}
	dir, e := launchDirectory(configuration)
	if e != nil {
		return e
	}
	var lock *os.File
	for {
		lock, e = launchLock(dir, "launch.lock")
		if e == nil {
			break
		}
		if !errors.Is(e, ErrLaunchBusy) {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer lock.Close()
	// Probe only the endpoint before deciding to start the application. A rejected
	// request must never cause an additional app instance to be spawned.
	available := func() bool {
		conn, e := net.DialTimeout("unix", filepath.Join(dir, "s"), time.Second)
		if e != nil {
			return false
		}
		defer conn.Close()
		return verifyLaunchPeer(conn.(*net.UnixConn)) == nil
	}
	if !available() {
		null, e := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if e != nil {
			return e
		}
		defer null.Close()
		command := exec.Command(executable, "--external-launch-service")
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		command.Stdin = null
		command.Stdout = null
		command.Stderr = null
		if e = command.Start(); e != nil {
			return fmt.Errorf("无法启动 OShell Desktop")
		}
		ended := make(chan struct{})
		go func() { _ = command.Wait(); close(ended) }()
		for !available() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ended:
				return fmt.Errorf("OShell 启动服务未就绪，应用已退出")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	return SendExternalLaunch(ctx, configuration, request)
}
func ParseExternalLauncher(name string, args []string) (ExternalLaunchRequest, error) {
	if strings.Contains(strings.ToLower(filepath.Base(name)), "filezilla") {
		request, e := ParseFileLaunch(args, ReadFileZillaSites)
		return ExternalLaunchRequest{File: &request}, e
	}
	request, e := ParseZOCLaunch(args)
	return ExternalLaunchRequest{Terminal: &request}, e
}
