//go:build windows

package core

import (
	"context"
	"fmt"
	"github.com/Microsoft/go-winio"
	"net"
	"os"
	"time"
)

func dialSSHAgent(ctx context.Context) (net.Conn, error) {
	if os.Getenv("OSHELL_TEST_NO_AGENT") == "1" {
		return nil, fmt.Errorf("测试禁用 Agent")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return winio.DialPipeContext(ctx, `\\.\pipe\openssh-ssh-agent`)
}
