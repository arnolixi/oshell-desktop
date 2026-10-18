package terminal

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func wordClickFixture(t *testing.T, text string, width int) (*Terminal, *ui.Tester) {
	t.Helper()
	loadLib(t)
	term, err := New(Options{Conn: newPipe(), CustomWordSelection: true, WordSeparators: "/.", ShiftWhitespace: true, CopyOnSelect: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill().AutoFocus() }, width, 200)
	term.Feed([]byte(text))
	tt.Frame()
	return term, tt
}

func clickWord(term *Terminal, tt *ui.Tester, col, row int) {
	x, y := cellCenter(term, col, row)
	tt.Press(x, y)
	tt.Release(x, y)
}

func TestProgressiveDoubleClickAndTripleLine(t *testing.T) {
	term, tt := wordClickFixture(t, "/home/file.txt /etc/name.conf", 500)
	for i, want := range []string{"", "file", "/home/file.txt /etc/name.conf", "/home/file.txt"} {
		clickWord(term, tt, 8, 0)
		if i > 0 && tt.Clipboard() != want {
			t.Fatalf("click %d selected %q, want %q", i+1, tt.Clipboard(), want)
		}
	}
	// Word dragging keeps the broader whitespace policy, including punctuation.
	x, y := cellCenter(term, 8, 0)
	tt.Press(x, y)
	x, y = cellCenter(term, 21, 0)
	tt.Move(x, y)
	tt.Release(x, y)
	if got := tt.Clipboard(); got != "/home/file.txt /etc/name.conf" {
		t.Fatalf("expanded drag selected %q", got)
	}
	for range 2 {
		clickWord(term, tt, 8, 0)
	}
	if tt.Clipboard() != "file" {
		t.Fatal("drag did not reset progressive selection", tt.Clipboard())
	}
}

func TestProgressiveDoubleClickAfterPauseAndReset(t *testing.T) {
	term, tt := wordClickFixture(t, "/home/file.txt /etc/name.conf", 500)
	// Make room in the monotonic clock to simulate a pause without sleeping.
	clickWord(term, tt, 8, 0)
	term.v.gestureStart = time.Now().Add(-2 * time.Second)
	term.v.wordClicks = wordClickState{}
	for range 2 {
		clickWord(term, tt, 8, 0)
	}
	if tt.Clipboard() != "file" {
		t.Fatal(tt.Clipboard())
	}
	term.v.wordClicks.last -= uint64(600 * time.Millisecond)
	for range 2 {
		clickWord(term, tt, 8, 0)
	}
	if tt.Clipboard() != "/home/file.txt" {
		t.Fatal("second double click after pause did not expand", tt.Clipboard())
	}
	for range 2 {
		clickWord(term, tt, 21, 0)
	}
	if tt.Clipboard() != "name" {
		t.Fatal("another position inherited whitespace selection", tt.Clipboard())
	}
	term.Feed([]byte(" changed"))
	for range 2 {
		clickWord(term, tt, 21, 0)
	}
	if tt.Clipboard() != "name" {
		t.Fatal("new output did not reset selection progression", tt.Clipboard())
	}
	tt.Key(0, ui.KeyEscape)
	for range 2 {
		clickWord(term, tt, 21, 0)
	}
	if tt.Clipboard() != "name" {
		t.Fatal("keyboard input did not reset selection progression", tt.Clipboard())
	}
}

func TestProgressiveDoubleClickWideWrappedText(t *testing.T) {
	term, tt := wordClickFixture(t, "目录/file.txt end", 80)
	for range 2 {
		clickWord(term, tt, 0, 0)
	}
	if got := tt.Clipboard(); got != "目录" {
		t.Fatalf("first double click selected %q", got)
	}
	for range 2 {
		clickWord(term, tt, 0, 0)
	}
	if got := tt.Clipboard(); got != "目录/file.txt" {
		t.Fatalf("wrapped whitespace selection = %q", got)
	}
	save(t, tt, "progressive-word-selection")
}

func TestProgressiveShiftDoubleClickStillSelectsWhitespace(t *testing.T) {
	term, _ := wordClickFixture(t, "/home/file.txt end", 500)
	x, y := cellCenter(term, 8, 0)
	for range 2 {
		term.v.pointerEvent(ui.InputEvent{Kind: ui.InputPointerDown, Mods: ui.Shift, X: x, Y: y})
		term.v.pointerEvent(ui.InputEvent{Kind: ui.InputPointerUp, Mods: ui.Shift, X: x, Y: y})
	}
	term.mu.Lock()
	text, _ := term.term.SelectionText()
	term.mu.Unlock()
	if text != "/home/file.txt" {
		t.Fatal("Shift double click regressed", text)
	}
}
