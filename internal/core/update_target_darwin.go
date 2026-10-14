//go:build darwin

package core

import (
	"golang.org/x/sys/unix"
	"runtime"
)

// Prefer the native Apple Silicon package even when this instance runs through
// Rosetta, matching OShell's hardware-based update flavor selection.
func CurrentMacUpdateTarget() string {
	silicon, e := unix.SysctlUint32("hw.optional.arm64")
	return macUpdateTarget(e == nil && silicon != 0, runtime.GOARCH)
}
func macUpdateTarget(appleSilicon bool, architecture string) string {
	if appleSilicon {
		return "darwin-arm64"
	}
	return "darwin-" + architecture
}
