package terminal

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestKeyAliasesUseTerminalProtocolAndSynchronize(t *testing.T) {
	loadLib(t)
	for _, protocol := range []string{"", "\x1b[>3u"} {
		left, right := newPipe(), newPipe()
		aliases := []KeyAlias{{Key: ui.KeyBackspace, Mods: ui.Ctrl, TargetKey: ui.KeyW, TargetMods: ui.Ctrl}, {Key: ui.KeyHome, TargetKey: ui.KeyA, TargetMods: ui.Ctrl}}
		receiver, err := New(Options{Conn: right, KeyAliases: aliases})
		if err != nil {
			t.Fatal(err)
		}
		defer receiver.Close()
		source, err := New(Options{Conn: left, KeyAliases: aliases, OnUserInput: receiver.ReplayUserInput})
		if err != nil {
			t.Fatal(err)
		}
		defer source.Close()
		source.Feed([]byte(protocol))
		receiver.Feed([]byte(protocol))
		tt := ui.NewTester(func(c *ui.Context) { View(c, source).Fill().AutoFocus() }, 400, 200)
		for _, alias := range aliases {
			want := "\x17"
			if alias.TargetKey == ui.KeyA {
				want = "\x01"
			}
			if protocol != "" {
				key := int('a') + int(alias.TargetKey-ui.KeyA)
				want = fmt.Sprintf("\x1b[%d;5u\x1b[%d;5:3u", key, key)
			}
			tt.Key(alias.TargetMods, alias.TargetKey)
			if got := left.take(len(want)); got != want {
				t.Fatalf("original key: %q != %q", got, want)
			}
			if got := right.take(len(want)); got != want {
				t.Fatalf("original sync: %q != %q", got, want)
			}
			tt.Key(alias.Mods, alias.Key)
			if got := left.take(len(want)); got != want {
				t.Fatalf("alias with protocol %q: %q != %q", protocol, got, want)
			}
			if got := right.take(len(want)); got != want {
				t.Fatalf("alias sync: %q != %q", got, want)
			}
		}
	}
}

func TestKeyAliasReleaseTracksPressAcrossModifierAndSettingsChanges(t *testing.T) {
	alias := KeyAlias{Key: ui.KeyBackspace, Mods: ui.Ctrl, TargetKey: ui.KeyW, TargetMods: ui.Ctrl}
	v := &view{t: &Terminal{opts: Options{KeyAliases: []KeyAlias{alias}}}}
	press := v.aliasKeyEvent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyBackspace, Mods: ui.Ctrl})
	v.t.SetKeyAliases(nil)
	repeat := v.aliasKeyEvent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyBackspace, Repeat: true})
	release := v.aliasKeyEvent(ui.InputEvent{Kind: ui.InputKeyUp, Key: ui.KeyBackspace})
	for _, event := range []ui.InputEvent{press, repeat, release} {
		if event.Key != ui.KeyW || event.Mods != ui.Ctrl {
			t.Fatalf("mismatched mapped event: %+v", event)
		}
	}
	if !repeat.Repeat || len(v.pressedAliases) != 0 {
		t.Fatal("repeat/release state was lost")
	}
	plain := v.aliasKeyEvent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyBackspace})
	if plain.Key != ui.KeyBackspace || plain.Mods != 0 {
		t.Fatal("alias leaked into next key press")
	}
}
