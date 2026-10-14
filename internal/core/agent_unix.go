//go:build !windows

package core

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"
)

func dialSSHAgent(ctx context.Context) (net.Conn, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" || os.Getenv("OSHELL_TEST_NO_AGENT") == "1" {
		return nil, fmt.Errorf("SSH Agent 未配置")
	}
	return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", socket)
}
