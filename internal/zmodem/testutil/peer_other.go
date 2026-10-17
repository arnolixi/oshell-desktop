//go:build !windows

package testutil

import "os/exec"

func hidePeerWindow(cmd *exec.Cmd) {}
