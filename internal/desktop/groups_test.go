package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"runtime"
	"testing"
)

func TestNamedGroupsVisibilityOwnershipNumberingAndScopes(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	for _, name := range []string{"one", "two", "three", "four"} {
		operatorPane(t, a, name)
	}
	a.ensureGroups()
	initial := a.groups.root.leaves()[0]
	second, e := a.createGroup("生产")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.createGroup("生产"); e == nil {
		t.Fatal("duplicate name accepted")
	}
	moving := a.tabs[1]
	oldTerm := moving.active.term
	a.moveToGroup(moving, second)
	if a.activeTab() != moving || moving.active.term != oldTerm {
		t.Fatal("move replaced terminal")
	}
	a.hideGroup(second, true)
	if a.activeTab() == moving || !oldTerm.Available() {
		t.Fatal("hide disconnected or kept hidden active")
	}
	a.quickSendScope = "visible"
	if len(a.sendCandidates()) != 1 {
		t.Fatal("hidden group included")
	}
	a.quickSendScope = "selectedGroups"
	a.features.operators.selectedGroups = map[string]bool{second.id: true}
	if targets := a.sendCandidates(); len(targets) != 1 || targets[0] != moving.active {
		t.Fatal("explicit hidden group not selected")
	}
	a.activateGroup(initial)
	if !a.selectTabNumber(2) || a.activeTab() != a.tabs[2] {
		t.Fatal("numbering used global order")
	}
	a.closeTab(a.activeTab())
	if a.activeTab() != a.groupTab(initial) {
		t.Fatal("closed tab selection diverged from visible tab")
	}
	a.dissolveGroup(second)
	if len(a.groups.root.leaves()) != 1 || moving.active.term != oldTerm || moving.active.closed {
		t.Fatal("dissolving changed connection")
	}
	tt := ui.NewTester(a.view, 1200, 800)
	a.arrangeTabs("tile")
	tt.Frame()
	snapshot(t, tt, "tab-groups-tiled.png")
	a.arrangeTabs("tabs")
	tt.Frame()
	if len(a.groups.root.leaves()) != 1 {
		t.Fatal("merge incomplete")
	}
}
func TestGroupDropEdgesAndHiddenEmptyGroups(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	operatorPane(t, a, "one")
	operatorPane(t, a, "two")
	a.ensureGroups()
	target := a.groups.root.group
	a.dropTab(a.tabs[1], target, "right")
	if len(a.groups.root.leaves()) != 2 {
		t.Fatal("edge didn't split")
	}
	a.dropTab(a.tabs[1], target, "center")
	if len(a.groups.root.leaves()) != 1 || a.tabs[1].group != target.id {
		t.Fatal("center didn't merge")
	}
	empty, e := a.createGroup("空组")
	if e != nil {
		t.Fatal(e)
	}
	a.hideGroup(empty, true)
	a.hideGroup(target, true)
	a.ensureGroups()
	if a.activeTab() != nil || a.groups.root.visible() {
		t.Fatal("all-hidden group lost its state")
	}
	next := a.groupForNewTab()
	if next.hidden {
		t.Fatal("new tab would be hidden")
	}
	if dropPosition(10, 50, 100, 100) != "left" || dropPosition(50, 50, 100, 100) != "center" {
		t.Fatal("drop hit testing")
	}
}
func TestShortcutDraftConflictDisableAndRestore(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	keys := &a.features.settingsDraft.KeyboardShortcuts
	key := keys.Bindings("find", runtime.GOOS)[0]
	if keys.Conflict(key, "newWindow", runtime.GOOS) == "" {
		t.Fatal("missing conflict")
	}
	keys.Overrides["find"] = core.ShortcutOverride{}
	if len(keys.Bindings("find", runtime.GOOS)) != 0 {
		t.Fatal("disabled binding used default")
	}
	a.closeSettings()
	if len(a.config.Settings.KeyboardShortcuts.Bindings("find", runtime.GOOS)) == 0 {
		t.Fatal("cancel changed settings")
	}
	a.prepareSettings()
	a.features.settingsDraft.KeyboardShortcuts.Overrides["find"] = core.ShortcutOverride{Shortcut: &core.KeyboardShortcut{KeyCode: 120, Modifiers: 0}}
	a.applySettings()
	if got := a.config.Settings.KeyboardShortcuts.Bindings("find", runtime.GOOS); len(got) != 1 || got[0].KeyCode != 120 {
		t.Fatal("binding not applied")
	}
	a.prepareSettings()
	a.features.settingsTab = 2
	tt := ui.NewTester(a.settingsView, 900, 700)
	snapshot(t, tt, "keyboard-shortcuts.png")
}
