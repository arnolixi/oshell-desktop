package terminal

import (
	"bytes"
	"os"
	"oshell-desktop/internal/terminal/internal/pty"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestToolProcessDrainsAfterExit(t *testing.T) {
	path, args := "/bin/sh", []string{"sh", "-c", "printf 'final-output'; exit 3"}
	if runtime.GOOS == "windows" {
		path, args = os.Getenv("COMSPEC"), []string{"cmd", "/d", "/c", "echo final-output& exit /b 3"}
	}
	proc, err := pty.Start(pty.Options{Path: path, Args: args})
	if err != nil {
		t.Fatal(err)
	}
	defer proc.Close()
	var output bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- drainToolProcess(&output, proc) }()
	select {
	case code := <-done:
		if code != 3 || !strings.Contains(output.String(), "final-output") {
			t.Fatalf("exit=%d output=%q", code, output.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("tool exited but output drain did not finish")
	}
}
