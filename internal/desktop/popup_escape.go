package desktop

import "github.com/egoist/mygo/ui"

// MyGo reserves Escape for an open menu/modal first. Only an unhandled bare
// Escape reaches the auxiliary window, even when its search field has focus.
func auxiliaryEscape(c *ui.Context, close func()) bool {
	if !c.Shortcut(0, ui.KeyEscape) {
		return false
	}
	close()
	return true
}
