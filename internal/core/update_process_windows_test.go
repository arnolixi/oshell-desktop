//go:build windows

package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestPortableReplacementWaitsForWindowsFileHandle(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "incoming.exe"), filepath.Join(dir, "installed.exe")
	for name, contents := range map[string]string{source: "new executable", target: "old executable"} {
		if err := os.WriteFile(name, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	name, err := windows.UTF16PtrFromString(target)
	if err != nil {
		t.Fatal(err)
	}
	// Allow reads/writes, but deny renaming while this handle is open.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if handle != windows.InvalidHandle {
			windows.CloseHandle(handle)
		}
	}()
	done := make(chan error, 1)
	go func() { done <- replaceUpdateFile(source, target) }()
	select {
	case err := <-done:
		t.Fatalf("replacement returned while target was locked: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "old executable" {
		t.Errorf("locked executable changed: %q, %v", b, err)
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	handle = windows.InvalidHandle
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "new executable" {
		t.Fatalf("replacement after unlock: %q, %v", b, err)
	}
}
