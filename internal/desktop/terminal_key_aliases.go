package desktop

import (
	"runtime"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal"
)

func terminalKeyAliases(bindings core.KeyboardShortcuts) []terminal.KeyAlias {
	var out []terminal.KeyAlias
	for _, action := range []struct {
		id     string
		target ui.Key
	}{{"terminalDeleteWord", ui.KeyW}, {"terminalLineStart", ui.KeyA}} {
		for _, key := range bindings.Bindings(action.id, runtime.GOOS) {
			if source := shortcutKey(key.KeyCode); source != ui.KeyUnknown && key.ValidFor(action.id, runtime.GOOS) {
				out = append(out, terminal.KeyAlias{Key: source, Mods: shortcutMods(key), TargetKey: action.target, TargetMods: ui.Ctrl})
			}
		}
	}
	return out
}
