package terminal

import (
	"strings"
	"testing"
	"time"
)

func TestToolsConsoleEditingHelpAndInterrupt(t *testing.T) {
	loadLib(t)
	console := NewToolsConsole()
	terminal, e := New(Options{Conn: console, NoBlink: true, ManagedInputAllowed: console.ManagedInputAllowed})
	if e != nil {
		t.Fatal(e)
	}
	defer terminal.Close()
	wait := func(text string) {
		t.Helper()
		until := time.Now().Add(3 * time.Second)
		for time.Now().Before(until) {
			if strings.Contains(terminal.Text(), text) {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("missing %q in %q", text, terminal.Text())
	}
	wait("oshell>")
	terminal.Send([]byte("help\r"))
	wait("不解析管道")
	terminal.Send([]byte("cancel this\x03help\r"))
	wait("Ctrl+C 中断工具")
	if !terminal.ManagedInputAvailable() {
		t.Fatal("Ctrl+C closed blank console")
	}
	terminal.Send([]byte("exit\r"))
	select {
	case <-terminal.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("exit didn't close")
	}
}

func TestToolsConsoleReadlineControls(t *testing.T) {
	loadLib(t)
	console := NewToolsConsole()
	term, err := New(Options{Conn: console, NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	wait := func(text string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for !strings.Contains(term.Text(), text) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !strings.Contains(term.Text(), text) {
			t.Fatalf("missing %q: %q", text, term.Text())
		}
	}
	wait("oshell>")
	// Ctrl+A inserts the missing h at the beginning; Ctrl+E returns to the
	// end; Ctrl+W removes the extra word. The resulting command is "help ".
	term.Send([]byte("elp\x01h\x05 extra\x17\r"))
	wait("不解析管道")
	term.Send([]byte("exit\r"))
	select {
	case <-term.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("console no longer accepts commands after editing")
	}
}
