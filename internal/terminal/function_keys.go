package terminal

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/terminal/internal/vt"
)

// SendFunctionKey bridges native keys absent from MyGo's key enumeration.
// Call on the UI thread; the focused view and Ghostty protocol state own encoding.
func (t *Terminal) SendFunctionKey(number int, mods ui.Modifiers, release, repeat bool) bool {
	if number < 13 || number > 20 || t.v == nil || !t.v.focused || !t.ManagedInputAvailable() {
		return false
	}
	action := vt.KeyPress
	if release {
		action = vt.KeyRelease
	} else if repeat {
		action = vt.KeyRepeat
	}
	handled := t.v.encodeKey(vt.KeyEvent{Action: action, Key: vt.KeyF1 + vt.Key(number-1), Mods: vtMods(mods)}, !release)
	if handled {
		if t.opts.OnUserInput != nil {
			kind := ui.InputKeyDown
			if release {
				kind = ui.InputKeyUp
			}
			t.opts.OnUserInput(ui.InputEvent{Kind: kind, Mods: mods})
		}
		t.redraw()
	}
	// Legacy protocols omit releases; still consume the matching native event.
	return handled || release
}
