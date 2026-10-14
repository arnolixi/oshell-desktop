//go:build !darwin

package core

import "os/exec"

func resolveLocalTool(requested, canonical string) (string, error) { return exec.LookPath(requested) }
