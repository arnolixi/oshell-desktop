//go:build darwin

package desktop

import (
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func TestExtendedMacFunctionKeysPreserveNativeCodesAndModifiers(t *testing.T) {
	codes := []uint16{105, 107, 113, 106, 64, 79, 80, 90}
	for i, code := range codes {
		key, ok := extendedMacShortcut(code, (1<<20)|(1<<19)|(1<<18)|(1<<17)|(1<<16)|(1<<23))
		if !ok || key.KeyCode != code || key.Modifiers != 15 {
			t.Fatalf("F%d conversion: %+v %v", i+13, key, ok)
		}
	}
	for _, code := range []uint16{0, 36, 53, 122, 111} {
		if _, ok := extendedMacShortcut(code, 1<<20); ok {
			t.Fatal("native extension stole a regular key", code)
		}
	}
}
func TestExtendedShortcutCaptureUsesDraftAndConflictRules(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	a.features.settingsCapture = "newBlank"
	shortcut, ok := extendedMacShortcut(105, (1<<20)|(1<<19))
	if !ok {
		t.Fatal("F13 rejected")
	}
	if !a.acceptCapturedShortcut(shortcut) || a.features.settingsCapture != "" {
		t.Fatal("capture not accepted")
	}
	if len(a.config.Settings.KeyboardShortcuts.Bindings("newBlank", "darwin")) != 0 {
		t.Fatal("capture wrote live preferences before Apply")
	}
	a.features.settingsCapture = "local"
	if a.acceptCapturedShortcut(shortcut) || !strings.Contains(a.features.shortcutError, "冲突") {
		t.Fatal("duplicate capture accepted")
	}
	a.features.settingsCapture = ""
	a.applySettings()
	bindings := a.config.Settings.KeyboardShortcuts.Bindings("newBlank", "darwin")
	if len(bindings) != 1 || bindings[0].KeyCode != 105 || !strings.Contains(bindings[0].Display("darwin"), "F13") {
		t.Fatal("native menu accelerator lost extended key", bindings)
	}
	saved, e := a.store.Load()
	if e != nil || len(saved.Settings.KeyboardShortcuts.Bindings("newBlank", "darwin")) != 1 {
		t.Fatal("shortcut not persisted", e)
	}
}
