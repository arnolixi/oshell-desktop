package desktop

import (
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
)

func TestDirectoryDraftCancelPreservesSavedPending(t *testing.T) {
	dir, target := t.TempDir(), t.TempDir()
	target, _ = filepath.EvalSymlinks(target)
	if err := core.ScheduleSyncDirectory(dir, target, "create", []byte("fixture-master")); err != nil {
		t.Fatal(err)
	}
	a := newApp(&core.Store{Dir: dir}, core.DefaultConfig())
	a.prepareSettings()
	a.loadSharedSyncSettings()
	f := &a.features.sharedSync
	f.directoryChanged = true
	f.directoryDisabled = true
	f.pendingDirectory = nil
	a.closeSettings()
	pending, err := core.LoadPendingSyncDirectory(dir)
	if err != nil || pending == nil || pending.Path != target {
		t.Fatal("cancel mutated saved pending", err)
	}
	a.prepareSettings()
	a.loadSharedSyncSettings()
	if f.directoryChanged || f.directoryDisabled || f.pendingDirectory == nil {
		t.Fatal("draft survived reopen")
	}
}

func TestDirectoryDraftOnlyPublishesOnApply(t *testing.T) {
	dir, target := t.TempDir(), t.TempDir()
	target, _ = filepath.EvalSymlinks(target)
	a := newApp(&core.Store{Dir: dir}, core.DefaultConfig())
	if err := a.vault.SetMaster("fixture-master"); err != nil {
		t.Fatal(err)
	}
	pending, err := core.ValidateSyncDirectorySelection(dir, target, "create", []byte("fixture-master"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := core.LoadPendingSyncDirectory(dir)
	if err != nil || saved != nil {
		t.Fatal("validation wrote pending", err)
	}
	a.features.sharedSync.pendingDirectory = pending
	a.features.sharedSync.directoryChanged = true
	if err := a.applyDirectoryDraft(); err != nil {
		t.Fatal(err)
	}
	saved, err = core.LoadPendingSyncDirectory(dir)
	if err != nil || saved == nil || saved.Path != target {
		t.Fatal("apply did not save", err)
	}
	a.features.sharedSync.pendingDirectory = nil
	a.features.sharedSync.directoryChanged = true
	if err := a.applyDirectoryDraft(); err != nil {
		t.Fatal(err)
	}
	saved, err = core.LoadPendingSyncDirectory(dir)
	if err != nil || saved != nil {
		t.Fatal("apply did not cancel pending", err)
	}
}
