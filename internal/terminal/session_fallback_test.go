package terminal

import (
	"context"
	"github.com/egoist/mygo/ui"
	"net"
	"oshell-desktop/internal/zmodem"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDisconnectedConsoleKeepsHistoryAndRejectsStaleInput(t *testing.T) {
	loadLib(t)
	conn := newPipe()
	var transitions atomic.Int32
	term, e := New(Options{Conn: conn, LocalToolsOnExit: true, OnSessionEnd: func() { transitions.Add(1) }, NoBlink: true})
	if e != nil {
		t.Fatal(e)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill().AutoFocus() }, 600, 320)
	term.Feed([]byte("\x1b[32mremote history retained\x1b[0m\r\n"))
	conn.Close()
	waitFor(t, tt, "local tool prompt", func() bool { return term.InLocalToolsMode() && strings.Contains(term.Text(), "oshell>") })
	if transitions.Load() != 1 || !strings.Contains(term.Text(), "remote history retained") {
		t.Fatal("lost history or duplicate transition")
	}
	if term.HasActiveProcess() {
		t.Fatal("idle tools prompt counted as an active process")
	}
	if !term.ManagedInputAvailable() {
		t.Fatal("local tool input unavailable")
	}
	fallback := term.fallback.Load()
	if _, e = fallback.WriteGeneration([]byte("exit\r"), 0); e != nil {
		t.Fatal(e)
	}
	select {
	case <-term.Done():
		t.Fatal("stale remote input executed in local console")
	default:
	}
	term.SendManagedText("help", true)
	waitFor(t, tt, "local help", func() bool { return strings.Contains(term.Text(), "不解析管道") })
	term.SendManagedText("exit", true)
	select {
	case <-term.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("exit did not close console")
	}
	if !strings.Contains(term.Text(), "remote history retained") {
		t.Fatal("exit dropped retained history before pane disposal")
	}
}
func TestClosingTerminalDoesNotCreateFallback(t *testing.T) {
	loadLib(t)
	conn := newPipe()
	var transitions atomic.Int32
	term, e := New(Options{Conn: conn, LocalToolsOnExit: true, OnSessionEnd: func() { transitions.Add(1) }})
	if e != nil {
		t.Fatal(e)
	}
	term.Close()
	select {
	case <-term.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("close stalled")
	}
	if transitions.Load() != 0 {
		t.Fatal("explicit close launched tools")
	}
}

func TestRealProcessFinalOutputSurvivesFallback(t *testing.T) {
	loadLib(t)
	term, e := New(Options{Command: []string{"/bin/sh", "-c", "printf 'FIRST_LINE\\nFINAL_SENTINEL\\n'"}, LocalToolsOnExit: true, NoBlink: true})
	if e != nil {
		t.Fatal(e)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill() }, 600, 320)
	waitFor(t, tt, "process exit console", func() bool { return term.InLocalToolsMode() && strings.Contains(term.Text(), "oshell>") })
	for _, line := range []string{"FIRST_LINE", "FINAL_SENTINEL"} {
		if !strings.Contains(term.Text(), line) {
			t.Fatalf("lost %s: %q", line, term.Text())
		}
	}
}

func TestDisconnectCancelsTransferBeforeToolsEpoch(t *testing.T) {
	loadLib(t)
	conn := newPipe()
	started := make(chan struct{})
	finished := make(chan error, 1)
	term, e := New(Options{Conn: conn, LocalToolsOnExit: true, AutomaticTransfers: true, NoBlink: true,
		TransferHandler:  func(ctx context.Context, _ bool, _ net.Conn) error { close(started); <-ctx.Done(); return ctx.Err() },
		TransferFinished: func(err error) { finished <- err }})
	if e != nil {
		t.Fatal(e)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill() }, 600, 320)
	term.transfers.feed(zmodem.BuildZhexHeader(zmodem.FrameZRINIT, 0))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("transfer did not start")
	}
	conn.Close()
	waitFor(t, tt, "tools after cancelled transfer", func() bool { return term.InLocalToolsMode() && strings.Contains(term.Text(), "oshell>") })
	select {
	case e := <-finished:
		if e == nil {
			t.Fatal("transfer unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("transfer cancellation stalled")
	}
	term.SendManagedText("help", true)
	waitFor(t, tt, "usable local prompt", func() bool { return strings.Contains(term.Text(), "不解析管道") })
}
