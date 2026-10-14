package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
	"time"
)

func TestUnlockReschedulesSharedSyncAndFilesOnlyWindowKeepsTimer(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: t.TempDir()}
	if err := core.SaveSyncOptions(store.Dir, options); err != nil {
		t.Fatal(err)
	}
	hub := &applicationHub{store: store, config: cfg, vault: a.vault, fileWindow: a}
	a.hub = hub
	t.Cleanup(func() {
		hub.quitting = true
		if hub.syncTimer != nil {
			hub.syncTimer.Stop()
		}
	})
	hub.scheduleSharedSync(time.Hour)
	old := hub.syncTimer
	if old == nil {
		t.Fatal("file-only workspace has no sync timer")
	}
	a.afterVaultUnlock()
	if hub.syncTimer == nil || hub.syncTimer == old {
		t.Fatal("unlock did not replace delayed timer")
	}
	hub.fileWindow = nil
	hub.scheduleSharedSync(time.Minute)
	if hub.syncTimer != nil {
		t.Fatal("last workspace closed but sync timer retained")
	}
}
func TestAutomaticSyncDoesNotStartDuringSessionEditor(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.editing = true
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: t.TempDir()}
	if err := core.SaveSyncOptions(store.Dir, options); err != nil {
		t.Fatal(err)
	}
	hub := &applicationHub{store: store, config: cfg, vault: a.vault, windows: []*App{a}}
	a.hub = hub
	t.Cleanup(func() {
		hub.quitting = true
		if hub.syncTimer != nil {
			hub.syncTimer.Stop()
		}
	})
	hub.scheduleSharedSync(time.Millisecond)
	initial := hub.syncTimer
	waitOperator(t, a, func() bool { return hub.syncTimer != initial })
	if a.features.sharedSync.busy || a.features.sharedSync.plan != nil {
		t.Fatal("automatic sync started while editing")
	}
}

func TestSyncApplicationChecksOtherWindowEditorsAndFileWindow(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	a := newApp(store, cfg)
	other := newApp(store, cfg)
	files := newApp(store, cfg)
	hub := &applicationHub{windows: []*App{a, other}, fileWindow: files}
	a.hub = hub
	other.editing = true
	if !a.syncEditingBlocked() {
		t.Fatal("other-window editor ignored")
	}
	other.editing = false
	files.features.uploadDraft = &uploadConfirmation{open: true}
	if !a.syncEditingBlocked() {
		t.Fatal("file-window modal ignored")
	}
	files.features.uploadDraft = nil
	if a.syncEditingBlocked() {
		t.Fatal("closed editors kept sync blocked")
	}
}
