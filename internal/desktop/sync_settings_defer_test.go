package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestSyncReadResultDefersAcrossWindows(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	owner := newApp(store, core.DefaultConfig())
	editor := newApp(store, core.DefaultConfig())
	hub := &applicationHub{windows: []*App{owner, editor}, settingsOwner: editor}
	owner.hub = hub
	editor.hub = hub
	editor.prepareSettings()
	editor.features.settingsDraft.FontSize = 21
	owner.features.sharedSync.plan = &core.SharedSyncPlan{}
	owner.features.sharedSync.master = []byte("synthetic-master")
	if !owner.deferSyncForSettings(true) {
		t.Fatal("other-window settings not detected")
	}
	if owner.features.sharedSync.plan != nil || len(owner.features.sharedSync.master) != 0 || owner.features.sharedSync.conflictOpen {
		t.Fatal("obsolete plan retained")
	}
	if editor.features.settingsDraft.FontSize != 21 {
		t.Fatal("draft overwritten")
	}
	editor.closeSettings()
	if owner.features.sharedSync.deferred {
		t.Fatal("deferred read not resumed")
	}
	if owner.features.sharedSync.status != "请先保存并启用同步设置" {
		t.Fatal("did not re-read saved options")
	}
}

func TestSyncStartWhileSettingsOpenDoesNotRead(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	a.startSharedSync()
	if !a.features.sharedSync.deferred || a.features.sharedSync.busy {
		t.Fatal("read started during editing")
	}
	a.features.sharedSync.deferred = false // Cancel This Operation.
	a.closeSettings()
	if a.features.sharedSync.status == "请先保存并启用同步设置" {
		t.Fatal("cancelled read restarted")
	}
}
