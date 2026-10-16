package core

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Copies of the test binary impersonate console helpers. Handle their normal
// command-line arguments before the testing package parses flags such as -G.
func init() {
	if os.Getenv("OSHELL_TEST_BACKGROUND_HELPER") != "no-console" {
		return
	}
	window, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	if window != 0 {
		fmt.Fprintln(os.Stderr, "background helper has a console")
		os.Exit(70)
	}
	switch strings.ToLower(filepath.Base(os.Args[0])) {
	case "ssh.exe":
		fmt.Print("hostname fixture.invalid\nuser fixture-user\nport 2222\n")
	case "powershell.exe":
		fmt.Print("Consolas\t100\nArial\t0\n")
	case "pipe-helper.exe":
		if len(os.Args) > 1 {
			fmt.Fprintln(os.Stdout, strings.Join(os.Args[1:], "|"))
		}
		io.Copy(os.Stdout, os.Stdin)
	default:
		os.Exit(71)
	}
	os.Exit(0)
}

func backgroundHelper(t *testing.T, directory, name string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	path := filepath.Join(directory, name)
	target, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return path
}

func TestWindowsBackgroundHelpersHaveNoConsole(t *testing.T) {
	directory := t.TempDir()
	backgroundHelper(t, directory, "ssh.exe")
	backgroundHelper(t, directory, "powershell.exe")
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OSHELL_TEST_BACKGROUND_HELPER", "no-console")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	profile := NewProfile()
	profile.Host = "fixture-alias"
	resolved, err := ResolveSSH(ctx, profile)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Profile.Host != "fixture.invalid" || resolved.Profile.Username != "fixture-user" || resolved.Profile.Port != 2222 {
		t.Fatalf("SSH configuration output lost: %+v", resolved.Profile)
	}
	fonts, err := FontCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 2 || fonts[0].Name != "Consolas" || !fonts[0].Monospaced {
		t.Fatalf("font enumeration output lost: %+v", fonts)
	}
}

func TestWindowsBackgroundProxyPreservesPipesAndCloses(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "代理 helper files")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	executable := backgroundHelper(t, directory, "pipe-helper.exe")
	for _, command := range []string{`pipe-helper.exe`, `"` + executable + `" "user name" "host.example"`} {
		t.Run(command, func(t *testing.T) { testWindowsProxyRoundtrip(t, executable, command) })
	}
}

func testWindowsProxyRoundtrip(t *testing.T, executable, command string) {
	t.Setenv("OSHELL_TEST_BACKGROUND_HELPER", "no-console")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t.Chdir(filepath.Dir(executable))
	conn, err := dialProxyCommand(ctx, command, NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	const payload = "proxy roundtrip\r\n"
	if _, err := io.WriteString(conn, payload); err != nil {
		t.Fatal(err)
	}
	want := payload
	if strings.HasPrefix(command, `"`) {
		want = "user name|host.example\n" + payload
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("pipe output = %q", got)
	}
	conn.Close()
	select {
	case <-conn.(*commandConn).done:
	case <-time.After(3 * time.Second):
		t.Fatal("proxy command was not reaped")
	}
}

func TestWindowsBackgroundCommandPreservesProcessAttributes(t *testing.T) {
	command := exec.Command("unused")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_UNICODE_ENVIRONMENT, NoInheritHandles: true}
	prepareCommand(command)
	if !command.SysProcAttr.HideWindow || command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 || command.SysProcAttr.CreationFlags&windows.CREATE_UNICODE_ENVIRONMENT == 0 || !command.SysProcAttr.NoInheritHandles {
		t.Fatalf("incorrect background process attributes: %+v", command.SysProcAttr)
	}
}

func TestWindowsProxyCancellationClosesPendingRead(t *testing.T) {
	executable := backgroundHelper(t, t.TempDir(), "pipe-helper.exe")
	t.Setenv("OSHELL_TEST_BACKGROUND_HELPER", "no-console")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := dialProxyCommand(ctx, `"`+executable+`"`, NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Ensure the child has started and owns both pipe ends before cancellation.
	if _, err = conn.Write([]byte("ready")); err != nil {
		t.Fatal(err)
	}
	if _, err = io.ReadFull(conn, make([]byte, 5)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := conn.Read(make([]byte, 1)); done <- err }()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled read unexpectedly succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled proxy left a read blocked")
	}
}
