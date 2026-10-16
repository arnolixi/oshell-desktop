package terminal

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestWindowsInteractiveShellTypingPasteEnvironmentAndExit(t *testing.T) {
	loadLib(t)
	t.Setenv("OSHELL_INPUT_ENV", "old")
	term, err := New(Options{Command: []string{os.Getenv("COMSPEC"), "/d", "/q", "/k", `chcp 65001>nul & prompt OSHELL_TEST$G`}, Env: []string{"oshell_input_env=最新值"}})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill().AutoFocus() }, 800, 400)
	waitFor(t, tt, "cmd prompt", func() bool { return strings.Contains(term.Text(), "OSHELL_TEST>") })
	tt.Type("echo typed_中文_marker")
	tt.Key(0, ui.KeyEnter)
	waitFor(t, tt, "typed Unicode output", func() bool { return strings.Count(term.Text(), "typed_中文_marker") >= 2 })
	tt.SetClipboard("echo pasted_%OSHELL_INPUT_ENV%_marker")
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyV)
	tt.Key(0, ui.KeyEnter)
	waitFor(t, tt, "paste and overridden environment", func() bool { return strings.Contains(term.Text(), "pasted_最新值_marker") })
	tt.Type("exit /b 7")
	tt.Key(0, ui.KeyEnter)
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("interactive cmd did not exit")
	}
	if term.ExitCode() != 7 {
		t.Fatalf("exit code = %d", term.ExitCode())
	}
}

func TestWindowsTerminalCloseWhileShellWaits(t *testing.T) {
	loadLib(t)
	term, err := New(Options{Command: []string{os.Getenv("COMSPEC"), "/d", "/q", "/k", "prompt WAITING$G"}})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill() }, 640, 300)
	waitFor(t, tt, "waiting shell", func() bool { return strings.Contains(term.Text(), "WAITING>") })
	term.Close()
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("terminal close did not finish")
	}
}
