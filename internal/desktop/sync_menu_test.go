package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestSyncMenuUsesSavedOptionsAndShowsSetupError(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	a.settingsOpen = true
	a.features.sharedSync.options = core.SyncOptions{Enabled: true, Backend: "webdav", Address: "https://draft.invalid/"}
	a.runSyncMenuAction(false)
	if a.settingsOpen {
		t.Fatal("settings remained open")
	}
	if a.features.sharedSync.busy {
		t.Fatal("unsaved target started a request")
	}
	if a.status != "请先保存并启用同步设置" {
		t.Fatal(a.status)
	}
	saved, err := core.LoadSyncOptions(a.store.Dir)
	if err != nil || saved.Enabled {
		t.Fatal("draft persisted", err)
	}
}

func TestSyncMenuBusyKeepsSettingsDraft(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.settingsOpen = true
	a.features.sharedSync.busy = true
	a.runSyncMenuAction(false)
	if !a.settingsOpen {
		t.Fatal("busy action dismissed settings")
	}
}

func TestQueuedSyncWaitsForSettingsCloseAndUsesSavedTarget(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	a.features.sharedSync.options = core.SyncOptions{Enabled: true, Backend: "webdav", Address: "https://unsaved.invalid/"}
	a.queueSavedSharedSync()
	if !a.settingsOpen || !a.features.sharedSync.manualQueued || a.features.sharedSync.busy {
		t.Fatal("queue closed settings or started sync")
	}
	a.closeSettings()
	if a.features.sharedSync.manualQueued || a.features.sharedSync.busy {
		t.Fatal("queue not consumed")
	}
	if a.status != "请先保存并启用同步设置" {
		t.Fatal("used unsaved target", a.status)
	}
}

func TestQueuedSyncDoesNotRunWhenWindowStopped(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	a.queueSavedSharedSync()
	a.stopped.Store(true)
	a.closeSettings()
	if a.features.sharedSync.manualQueued || a.features.sharedSync.busy {
		t.Fatal("queue survived window shutdown")
	}
	if a.features.sharedSync.status == "请先保存并启用同步设置" {
		t.Fatal("sync ran during shutdown")
	}
}
