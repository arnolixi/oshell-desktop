//go:build darwin

package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMacToolLookupPrioritiesAndExecutableChecks(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	write := func(root string, mode os.FileMode) string {
		file := filepath.Join(root, "mtr")
		if err := os.WriteFile(file, []byte("#!/bin/sh\nexit 0\n"), mode); err != nil {
			t.Fatal(err)
		}
		return file
	}
	blocked := write(first, 0600)
	available := write(second, 0700)
	if got, err := findToolInRoots("mtr", []string{".", first, second}); err != nil || got != available {
		t.Fatal("non-executable or relative root selected", got, err)
	}
	if err := os.Chmod(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if got, err := findToolInRoots("mtr", []string{first, second}); err != nil || got != blocked {
		t.Fatal("root precedence changed", got, err)
	}
	if got, err := findToolInRoots(available, []string{first}); err != nil || got != available {
		t.Fatal("explicit path ignored", got, err)
	}
	directory := filepath.Join(first, "nmap")
	os.Mkdir(directory, 0700)
	if _, err := findToolInRoots("nmap", []string{first}); err == nil {
		t.Fatal("directory accepted as executable")
	}
}
func TestMacToolNamesResolveCanonicalCaseWithoutShellPATH(t *testing.T) {
	t.Setenv("PATH", "")
	command, err := ParseToolCommand("PING -c 1 127.0.0.1")
	if err != nil || command.Name != "ping" || !filepath.IsAbs(command.Path) {
		t.Fatal("canonical system tool lookup failed", err)
	}
}
