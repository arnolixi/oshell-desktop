package desktop

import (
	"github.com/egoist/mygo/ui"
	"runtime"
	"unicode/utf8"
)

func passwordInput(c *ui.Context, value *string) ui.Element {
	return passwordSelection(ui.TextInput(c, value), value)
}
func passwordSelection(input ui.Element, value *string) ui.Element {
	input.Password()
	if runtime.GOOS == "darwin" {
		pending := ui.Local(input, "password-select-all", func() bool { return false })
		if *pending {
			input.SetTextSelection(0, utf8.RuneCountInString(*value))
			*pending = false
		}
		input.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputKeyDown || ev.Key != ui.KeyA || ev.Mods != ui.Ctrl {
				return false
			}
			// The UI editor takes rune offsets, not UTF-8 bytes or AppKit UTF-16 units.
			// Selection never reads or writes the clipboard.
			*pending = true
			return true
		})
	}
	return input
}
