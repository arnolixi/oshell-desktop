//go:build darwin

package core

import "testing"

func TestMacUpdateTargetUsesHardwareForRosetta(t *testing.T) {
	if macUpdateTarget(true, "amd64") != "darwin-arm64" || macUpdateTarget(false, "amd64") != "darwin-amd64" {
		t.Fatal("update target followed emulation instead of hardware")
	}
}
