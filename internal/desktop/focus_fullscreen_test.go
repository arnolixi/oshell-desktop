package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestFocusFullscreenChromeKeepsTerminalLayoutAndDraft(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p, _ := operatorPane(t, a, "retained")
	a.ensureGroups()
	a.quick = "keep 中文 draft"
	a.features.operators.composerOpen = true
	a.features.operators.composerText = "second draft"
	term := p.term
	root := a.activeTab().root
	tt := ui.NewTester(a.view, 1000, 700)
	a.focusFullscreen.chrome = true
	tt.Frame()
	snapshot(t, tt, "focus-fullscreen-chrome.png")
	if a.activePane().term != term || a.activeTab().root != root || a.quick != "keep 中文 draft" || a.features.operators.composerText != "second draft" {
		t.Fatal("fullscreen rebuilt terminal or lost draft")
	}
	a.focusFullscreen.chrome = false
	tt.Frame()
	if a.activePane().term != term || a.activeTab().root != root {
		t.Fatal("exit rebuilt workspace")
	}
}
func TestFocusFullscreenQueuedIntentSurvivesCompletion(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.focusFullscreen = focusFullscreenState{requested: false, chrome: true, transitioning: true, target: true}
	a.fullscreenSettled(true)
	if a.focusFullscreen.requested || a.focusFullscreen.transitioning {
		t.Fatal("quick cancellation was lost on entry completion")
	}
	a.focusFullscreen = focusFullscreenState{requested: true, chrome: true, transitioning: true, target: false}
	a.fullscreenSettled(false)
	if !a.focusFullscreen.requested {
		t.Fatal("queued re-entry lost on exit completion")
	}
	a.focusFullscreen = focusFullscreenState{requested: true, chrome: true}
	a.fullscreenSettled(false)
	if a.focusFullscreen.requested {
		t.Fatal("native exit did not restore regular presentation")
	}
}
func TestFocusFullscreenDefaultPreservesExistingOverride(t *testing.T) {
	settings := core.KeyboardShortcuts{Overrides: map[string]core.ShortcutOverride{"newBlank": {Shortcut: &core.KeyboardShortcut{KeyCode: 3, Modifiers: 5}}}}
	if len(settings.Bindings("focusFullscreen", "darwin")) != 0 {
		t.Fatal("new default stole existing shortcut")
	}
	if got := (core.KeyboardShortcuts{}).Bindings("focusFullscreen", "darwin"); len(got) != 1 || got[0].Modifiers != 5 || got[0].KeyCode != 3 {
		t.Fatal("wrong native shortcut", got)
	}
}
