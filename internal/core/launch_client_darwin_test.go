//go:build darwin

package core

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestLaunchClientUsesRunningServer(t *testing.T) {
	configuration := t.TempDir()
	var seen atomic.Bool
	server, e := StartLaunchServer(configuration, func(context.Context, ExternalLaunchRequest) error { seen.Store(true); return nil })
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	request, e := ParseZOCLaunch([]string{"/SSH:tester@example.invalid"})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = LaunchApplicationAndSend(ctx, configuration, "/does-not-exist", ExternalLaunchRequest{Terminal: &request}); e != nil {
		t.Fatal(e)
	}
	if !seen.Load() {
		t.Fatal("existing app did not receive request")
	}
}
func TestLauncherFindsOwnApplicationOnly(t *testing.T) {
	app := filepath.Join(t.TempDir(), "OShell Desktop.app")
	resources := filepath.Join(app, "Contents/Resources")
	bin := filepath.Join(app, "Contents/MacOS")
	os.MkdirAll(resources, 0700)
	os.MkdirAll(bin, 0700)
	helper := filepath.Join(resources, "OShell-ZOC")
	os.WriteFile(helper, []byte("fixture"), 0700)
	executable := filepath.Join(bin, "OShell Desktop")
	os.WriteFile(executable, []byte("fixture"), 0700)
	plist := filepath.Join(app, "Contents/Info.plist")
	os.WriteFile(plist, []byte(`<plist><dict><key>CFBundleIdentifier</key><string>dev.oshell.desktop</string><key>CFBundleExecutable</key><string>OShell Desktop</string></dict></plist>`), 0600)
	executable, _ = filepath.EvalSymlinks(executable)
	actual, e := LaunchApplicationExecutable(helper)
	if e != nil || actual != executable {
		t.Fatal(actual, e)
	}
	os.WriteFile(plist, []byte(`<plist><dict><key>CFBundleIdentifier</key><string>app.oshell.mac</string><key>CFBundleExecutable</key><string>OShell Desktop</string></dict></plist>`), 0600)
	if _, e = LaunchApplicationExecutable(helper); e == nil {
		t.Fatal("launcher targeted original OShell")
	}
}
