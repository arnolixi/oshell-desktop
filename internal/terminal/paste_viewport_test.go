package terminal

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestPasteViewportOnlyMovesForAcceptedContent(t *testing.T) {
	loadLib(t)
	conn := newPipe()
	preview := ""
	term, err := New(Options{Conn: conn, NoBlink: true, BeforePaste: func(s string) bool { preview = s; return false }})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill() }, 640, 300)
	term.Feed([]byte(strings.Repeat("history\r\n", 240)))
	history := func() { term.mu.Lock(); term.term.ScrollToTop(); term.mu.Unlock(); tt.Frame() }
	atBottom := func() bool { term.mu.Lock(); defer term.mu.Unlock(); return term.term.AtBottom() }
	history()
	if atBottom() {
		t.Fatal("fixture has no history")
	}
	term.RequestPaste("first\nsecond")
	if preview == "" || atBottom() {
		t.Fatal("preview moved viewport before acceptance")
	}
	for _, text := range []string{"", "bad\x00input", strings.Repeat("x", (1<<20)+1)} {
		term.ConfirmedPaste(text)
		if atBottom() {
			t.Fatal("rejected content moved viewport")
		}
	}
	term.ConfirmedPaste(preview)
	if !atBottom() {
		t.Fatal("accepted paste did not reveal input")
	}
	if got := conn.take(len("first\rsecond")); got != "first\rsecond" {
		t.Fatal(got)
	}
	history()
	term.in.mu.Lock()
	term.in.paused = true
	term.in.mu.Unlock()
	term.Paste("blocked")
	if atBottom() {
		t.Fatal("busy terminal moved viewport")
	}
	term.in.mu.Lock()
	term.in.paused = false
	term.in.mu.Unlock()
	term.Feed([]byte("\x1b[?1049h\x1b[?2004h"))
	term.Paste("alternate\ntext")
	if got := conn.take(len("\x1b[200~alternate\ntext\x1b[201~")); got != "\x1b[200~alternate\ntext\x1b[201~" {
		t.Fatal(got)
	}
	term.mu.Lock()
	alternate := term.term.AltScreen()
	term.mu.Unlock()
	if !alternate {
		t.Fatal("paste exited alternate screen")
	}
}
