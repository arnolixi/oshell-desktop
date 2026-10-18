package terminal

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestPromptUnicodeEditingAndKill(t *testing.T) {
	e := newPromptEditor(io.Discard)
	e.insert("中e\u0301👩‍💻")
	if len(e.line) != 3 {
		t.Fatalf("graphemes: %#v", e.line)
	}
	e.key("\x7f")
	if e.text() != "中e\u0301" {
		t.Fatal(e.text())
	}
	e.key("\x14")
	if e.text() != "e\u0301中" {
		t.Fatal(e.text())
	}
	e.key("\x01")
	e.key("\x06")
	e.key("\x0b")
	e.key("\x19")
	if e.text() != "e\u0301中" {
		t.Fatal(e.text())
	}
	e.set("first 中文 👩‍💻")
	e.key("\x17")
	if e.text() != "first 中文 " {
		t.Fatal(e.text())
	}
	for _, s := range []string{"中", "👩‍💻", "❤️"} {
		if clusterWidth(s) != 2 {
			t.Fatal(s)
		}
	}
}
func TestPromptHistorySearchNeverExecutesStaleMatch(t *testing.T) {
	e := newPromptEditor(io.Discard)
	for i := 0; i < 105; i++ {
		e.set(fmt.Sprint("command ", i))
		e.key("\r")
	}
	if len(e.history) != 100 || e.history[0] != "command 5" {
		t.Fatal(e.history)
	}
	e.set("draft")
	e.pos = 2
	e.key("\x12")
	e.key("no match")
	if _, ready, _ := e.key("\r"); ready {
		t.Fatal("executed stale search result")
	}
	e.key("\x07")
	if e.text() != "draft" || e.pos != 2 {
		t.Fatal("draft not restored")
	}
	e.key("\x12")
	e.key("command 9")
	if e.text() != "command 99" {
		t.Fatal(e.text())
	}
	e.key("\x12")
	if e.text() != "command 98" {
		t.Fatal(e.text())
	}
	e.key("\x13")
	if e.text() != "command 99" {
		t.Fatal(e.text())
	}
	e.key("\x1b")
	if e.search || e.text() != "command 99" {
		t.Fatal("escape did not recover editable match")
	}
	other := newPromptEditor(io.Discard)
	if len(other.history) != 0 {
		t.Fatal("shared history")
	}
}
func TestPromptFragmentedInputAndPaste(t *testing.T) {
	e := newPromptEditor(io.Discard)
	defer e.Close()
	done := make(chan string, 1)
	go func() { line, _ := e.ReadLine(); done <- line }()
	for _, s := range []string{"中", "e\u0301", "\x1b[", "D", "X", "\x1b[200~", "one\r\ntwo", "\x1b[201~", "\r"} {
		if _, err := e.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case line := <-done:
		if line != "中Xone  twoe\u0301" {
			t.Fatal(line)
		}
	case <-time.After(time.Second):
		t.Fatal("parser blocked")
	}
	e.pending = []byte("\x1b[Hok\r")
	line, err := e.ReadLine()
	if err != nil || line != "ok" {
		t.Fatal(line, err)
	}
}
func TestPromptSuspensionDiscardsInteractiveInput(t *testing.T) {
	e := newPromptEditor(io.Discard)
	defer e.Close()
	pending := make(chan struct{})
	go func() { e.Write([]byte("synthetic password\r")); close(pending) }()
	e.suspend()
	select {
	case <-pending:
	case <-time.After(time.Second):
		t.Fatal("writer stranded at tool boundary")
	}
	e.Write([]byte("another synthetic password\r"))
	done := make(chan string, 1)
	go func() { line, _ := e.ReadLine(); done <- line }()
	for {
		e.gate.Lock()
		ready := e.reading != nil
		e.gate.Unlock()
		if ready {
			break
		}
		time.Sleep(time.Millisecond)
	}
	e.Write([]byte("help\r"))
	if line := <-done; line != "help" {
		t.Fatal(line)
	}
	if strings.Contains(strings.Join(e.history, " "), "password") {
		t.Fatal("interactive input leaked into history")
	}
}
