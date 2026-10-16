package terminal

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCursorAutomaticContrastAndCustomColor(t *testing.T) {
	loadLib(t)
	for _, tc := range []struct {
		name, background, output, want string
		auto                           bool
	}{
		{"dark", "#000000", "cursor ", "#FFFFFF", true},
		{"light", "#FFFFFF", "cursor ", "#000000", true},
		{"gray", "#808080", "cursor ", "#000000", true},
		{"remote-background", "#000000", "\x1b]11;#FFFFFF\acursor ", "#000000", true},
		{"colored-cell", "#000000", "\x1b[48;2;255;255;255m \r", "#000000", true},
		{"remote-cursor", "#000000", "\x1b]12;#000000\acursor ", "#FFFFFF", true},
		{"custom", "#000000", "cursor ", "#00FF00", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			theme := DarkTheme()
			theme.Background, theme.Cursor, theme.CursorText = ui.Hex(tc.background), ui.Hex("#00FF00"), ui.Hex("#FF00FF")
			theme.AutoCursor = tc.auto
			term, err := New(Options{Conn: newPipe(), Theme: theme, NoBlink: true})
			if err != nil {
				t.Fatal(err)
			}
			defer term.Close()
			term.Feed([]byte(tc.output))
			tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill().AutoFocus() }, 400, 180)
			tt.Frame()
			r, cursor, text, block := term.v.cursorRect(0, 0, 10, 20)
			if r.W == 0 || !block || cursor != ui.Hex(tc.want) {
				t.Fatalf("cursor: rect=%+v color=%+v block=%v", r, cursor, block)
			}
			if tc.auto && text != CursorContrast(cursor) {
				t.Fatal("block text has insufficient contrast")
			}
			if !tc.auto && text != theme.CursorText {
				t.Fatal("custom cursor text ignored")
			}
			if tc.name == "dark" || tc.name == "light" {
				save(t, tt, "cursor-high-contrast-"+tc.name)
			}
		})
	}
}
