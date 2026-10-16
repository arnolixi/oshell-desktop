package desktop

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal"
)

func TestTerminalAliasesStayWithinTerminalAndUpdateLive(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p, conn := operatorPane(t, a, "alias test")
	a.applyTerminalPreferences(p)
	text := "draft"
	var input ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			terminal.View(c, p.term).Bind(&p.focus).Grow(1).FillWidth().AutoFocus()
			ui.TextInput(c, &text).Label("draft").Bind(&input)
		})
		a.shortcuts(c)
	}, 500, 240)
	tt.Key(ui.Ctrl, ui.KeyBackspace)
	tt.Key(0, ui.KeyHome)
	tt.Key(ui.Ctrl, ui.KeyW)
	tt.Key(ui.Ctrl, ui.KeyA)
	want := "\x17\x01\x17\x01"
	deadline := time.Now().Add(time.Second)
	for conn.text() != want && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if conn.text() != want {
		t.Fatalf("terminal keys: %q", conn.text())
	}
	input.Focus()
	tt.Frame()
	tt.Key(0, ui.KeyHome)
	tt.Type("X")
	if text != "Xdraft" || conn.text() != want {
		t.Fatal("terminal alias intercepted text input")
	}
	a.config.Settings.KeyboardShortcuts.Overrides = map[string]core.ShortcutOverride{"terminalDeleteWord": {Shortcut: &core.KeyboardShortcut{KeyCode: 51, Modifiers: 2}}}
	a.applyTerminalPreferences(p)
	p.focus.Focus()
	tt.Frame()
	tt.Key(ui.Alt, ui.KeyBackspace)
	want += "\x17"
	deadline = time.Now().Add(time.Second)
	for conn.text() != want && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if conn.text() != want {
		t.Fatal("new alias did not apply to existing terminal")
	}
}

func TestTerminalAliasHomeCaptureAndPersistence(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	a := newApp(store, core.DefaultConfig())
	a.prepareSettings()
	a.features.settingsCapture = "terminalLineStart"
	if !a.acceptCapturedShortcut(core.KeyboardShortcut{KeyCode: 115}) {
		t.Fatal(a.features.shortcutError)
	}
	a.applySettings()
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	keys := loaded.Settings.KeyboardShortcuts.Bindings("terminalLineStart", "windows")
	if len(keys) != 1 || keys[0].KeyCode != 115 || keys[0].Modifiers != 0 {
		t.Fatalf("Home was not saved: %+v", keys)
	}
}
