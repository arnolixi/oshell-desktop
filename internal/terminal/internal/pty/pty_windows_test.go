package pty

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func init() {
	if os.Getenv("OSHELL_CONPTY_TEST_CHILD") != "1" {
		return
	}
	var size windows.ConsoleScreenBufferInfo
	stdout, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		os.Exit(71)
	}
	if err = windows.GetConsoleScreenBufferInfo(stdout, &size); err != nil {
		os.Exit(72)
	}
	fmt.Printf("INITIAL_SIZE=%dx%d\n", size.Size.X, size.Size.Y)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		os.Exit(73)
	}
	if err = windows.GetConsoleScreenBufferInfo(stdout, &size); err != nil {
		os.Exit(74)
	}
	fmt.Printf("FINAL_SIZE=%dx%d INPUT=%s\n", size.Size.X, size.Size.Y, strings.TrimSpace(line))
	os.Exit(3)
}

func startConPTYFixture(t *testing.T) *PTY {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p, err := Start(Options{Path: exe, Env: append(os.Environ(), "OSHELL_CONPTY_TEST_CHILD=1"), Cols: 100, Rows: 30})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

func TestConPTYResizeInputAndExit(t *testing.T) {
	p := startConPTYFixture(t)
	if err := p.Resize(120, 40, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Write([]byte("typed-marker\r")); err != nil {
		t.Fatal(err)
	}
	out := readAll(t, p)
	for _, want := range []string{"FINAL_SIZE=120x40", "INPUT=typed-marker"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output %q lacks %q", out, want)
		}
	}
	if code, err := p.Wait(); err != nil || code != 3 {
		t.Fatalf("exit = %d, %v", code, err)
	}
	if err := p.Kill(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("kill after reap = %v", err)
	}
}

func TestConPTYCmdQuotedCommand(t *testing.T) {
	p, err := Start(Options{Path: os.Getenv("COMSPEC"), Args: []string{"cmd", "/d", "/s", "/c", `echo "quoted marker"`}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	out := readAll(t, p)
	if !strings.Contains(out, `"quoted marker"`) || strings.Contains(out, `\"`) {
		t.Fatalf("cmd quoting corrupted: %q", out)
	}
}

func TestConPTYCloseWhileWaitingForInput(t *testing.T) {
	p := startConPTYFixture(t)
	drained := make(chan struct{})
	go func() { io.Copy(io.Discard, p); close(drained) }()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Exited():
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("child survived closing its terminal")
	}
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("terminal output did not close")
	}
	if _, err := p.Write([]byte("closed")); !errors.Is(err, ErrClosed) {
		t.Fatalf("write after close = %v", err)
	}
	if err := p.Resize(80, 24, 0, 0); !errors.Is(err, ErrClosed) {
		t.Fatalf("resize after close = %v", err)
	}
}
