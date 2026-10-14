//go:build darwin

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Run with OSHELL_TEST_LAUNCHER_BINARY pointing to cmd/oshell-launcher built for
// this host. Exercises the real entry point without launching UI or a server login.
func TestCompiledLauncherEntrypoints(t *testing.T) {
	binary := os.Getenv("OSHELL_TEST_LAUNCHER_BINARY")
	if binary == "" {
		t.Skip("compiled helper fixture not supplied")
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(t.TempDir(), "OShell Desktop.app")
	resources := filepath.Join(app, "Contents", "Resources")
	bin := filepath.Join(app, "Contents", "MacOS")
	for _, dir := range []string{resources, bin} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string, data []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, data, mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(app, "Contents", "Info.plist"), []byte(`<plist><dict><key>CFBundleIdentifier</key><string>dev.oshell.desktop</string><key>CFBundleExecutable</key><string>OShell Desktop</string></dict></plist>`), 0600)
	write(filepath.Join(bin, "OShell Desktop"), []byte("#!/bin/sh\nexit 97\n"), 0700)
	mainBinary := os.Getenv("OSHELL_TEST_MAIN_BINARY")
	if mainBinary != "" {
		mainData, err := os.ReadFile(mainBinary)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(bin, "OShell Desktop"), mainData, 0700)
	}
	configuration := t.TempDir()
	received := make(chan ExternalLaunchRequest, 2)
	server, err := StartLaunchServer(configuration, func(_ context.Context, r ExternalLaunchRequest) error { received <- r; return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	cases := []struct {
		name string
		args []string
		file bool
	}{
		{"OShell-ZOC", []string{"/SSH:fixture@terminal.invalid:2222", "/SSHPASSWORD:fixture-token"}, false},
		{"OShell-FileZilla", []string{"sftp://fixture:fixture-token@files.invalid:2222/fixture"}, true},
	}
	for _, direct := range []bool{false, true} {
		if direct && mainBinary == "" {
			continue
		}
		for _, item := range cases {
			name := item.name
			if direct {
				name += "/main-executable"
			}
			t.Run(name, func(t *testing.T) {
				helper := filepath.Join(resources, item.name)
				write(helper, data, 0700)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				entry := helper
				if direct {
					entry = filepath.Join(bin, "OShell Desktop")
				}
				command := exec.CommandContext(ctx, entry, item.args...)
				command.Env = append(os.Environ(), "OSHELL_DATA_DIR="+configuration)
				if out, err := command.CombinedOutput(); err != nil {
					t.Fatalf("helper failed: %v (%d output bytes)", err, len(out))
				}
				select {
				case r := <-received:
					if item.file {
						if r.File == nil || r.Terminal != nil || r.File.Profile.Host != "files.invalid" || r.File.Password == nil || *r.File.Password != "fixture-token" {
							t.Fatal("file request mismatched")
						}
					} else if r.Terminal == nil || r.File != nil || r.Terminal.Host != "terminal.invalid" || r.Terminal.Password == nil || *r.Terminal.Password != "fixture-token" {
						t.Fatal("terminal request mismatched")
					}
				case <-ctx.Done():
					t.Fatal("request not received")
				}
				entries, err := os.ReadDir(configuration)
				if err != nil || len(entries) != 0 {
					t.Fatal("launcher persisted session data", err)
				}
			})
		}
	}
}
