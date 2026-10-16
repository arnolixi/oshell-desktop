// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"
)

type SSHIdentity struct {
	Host string `json:"host"`
	User string `json:"user"`
	Port int    `json:"port"`
}

func (p Profile) Identity() SSHIdentity {
	return SSHIdentity{strings.Trim(p.Host, "[]"), p.Username, p.Port}
}

type ResolvedSSH struct {
	Profile      Profile
	ProxyCommand string
}

func ResolveSSH(ctx context.Context, p Profile) (ResolvedSSH, error) {
	result := ResolvedSSH{Profile: p}
	if p.Kind != "ssh" && p.Kind != "sftp" {
		return result, nil
	}
	binary, e := exec.LookPath("ssh")
	if e == nil {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		args := []string{"-G", "-p", strconv.Itoa(p.Port)}
		if p.Username != "" {
			args = append(args, "-l", p.Username)
		}
		if p.IdentityFile != "" {
			args = append(args, "-i", expandHome(p.IdentityFile))
		}
		if os.Getenv("OSHELL_TEST_NO_AGENT") == "1" || os.Getenv("OSHELL_TEST_IGNORE_SSH_CONFIG") == "1" {
			// OpenSSH's sentinel works with both native Windows and MSYS/Git
			// builds; the latter interprets Windows' NUL as a missing filename.
			args = append(args, "-F", "none")
		}
		args = append(args, "--", p.Host)
		command := exec.CommandContext(ctx, binary, args...)
		prepareCommand(command)
		var out limitedOutput
		command.Stdout = &out
		if e = command.Run(); e == nil {
			applyResolvedSSH(&result, out.String())
		} else if ctx.Err() != nil {
			return result, ctx.Err()
		} else {
			return result, fmt.Errorf("无法解析 SSH 配置（ssh -G 失败）")
		}
	}
	if result.Profile.Username == "" {
		if u, e := user.Current(); e == nil {
			result.Profile.Username = u.Username
			if i := strings.LastIndex(result.Profile.Username, `\`); i >= 0 {
				result.Profile.Username = result.Profile.Username[i+1:]
			}
		}
	}
	if !validHost(result.Profile.Host) || result.Profile.Username == "" {
		return result, fmt.Errorf("无法解析 SSH 主机或用户名")
	}
	return result, nil
}

type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, fmt.Errorf("SSH 配置输出过大")
	}
	return b.Buffer.Write(p)
}
func applyResolvedSSH(result *ResolvedSSH, output string) {
	p := &result.Profile
	p.resolvedIdentityFiles = nil
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "hostname":
			p.Host = value
		case "user":
			p.Username = value
		case "port":
			if port, e := strconv.Atoi(value); e == nil {
				p.Port = port
			}
		case "identityfile":
			if value != "none" {
				candidate := expandHome(value)
				if _, e := os.Stat(candidate); e == nil {
					p.resolvedIdentityFiles = append(p.resolvedIdentityFiles, candidate)
				}
			}
		case "proxyjump":
			if p.ProxyID == "" && p.JumpHost == "" && (p.Proxy.Kind == "" || p.Proxy.Kind == "none") && value != "none" {
				p.JumpHost = value
			}
		case "proxycommand":
			if p.ProxyID == "" && p.JumpHost == "" && (p.Proxy.Kind == "" || p.Proxy.Kind == "none") && value != "none" {
				result.ProxyCommand = value
			}
		}
	}
}
