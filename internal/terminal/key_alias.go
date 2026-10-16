package terminal

import "github.com/egoist/mygo/ui"

// KeyAlias maps a press, repeat and release to the same target combination.
// Both combinations pass through the terminal's active keyboard protocol.
type KeyAlias struct {
	Key, TargetKey   ui.Key
	Mods, TargetMods ui.Modifiers
}

// SetKeyAliases updates alternatives on the UI thread, like SetInputPolicy.
func (t *Terminal) SetKeyAliases(aliases []KeyAlias) {
	t.opts.KeyAliases = append([]KeyAlias(nil), aliases...)
}

func (v *view) aliasKeyEvent(ev ui.InputEvent) ui.InputEvent {
	alias, matched := v.pressedAliases[ev.Key]
	if ev.Kind == ui.InputKeyUp {
		delete(v.pressedAliases, ev.Key)
	} else if !matched {
		for _, candidate := range v.t.opts.KeyAliases {
			if ev.Key == candidate.Key && ev.Mods == candidate.Mods {
				alias, matched = candidate, true
				if v.pressedAliases == nil {
					v.pressedAliases = make(map[ui.Key]KeyAlias)
				}
				v.pressedAliases[ev.Key] = alias
				break
			}
		}
	}
	if matched {
		ev.Key, ev.Mods = alias.TargetKey, alias.TargetMods
	}
	return ev
}
