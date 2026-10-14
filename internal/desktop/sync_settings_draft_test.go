package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestWebDAVDraftCancelDoesNotPersist(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	a.loadSharedSyncSettings()
	a.features.sharedSync.options.Address = "https://draft.invalid/"
	a.features.sharedSync.password = "synthetic-secret"
	a.closeSettings()
	saved, err := core.LoadSyncOptions(a.store.Dir)
	if err != nil || saved.Address != "" || a.features.sharedSync.password != "" {
		t.Fatal("cancel retained draft", err)
	}
}

func TestMainApplySavesWebDAVDraftAndCredential(t *testing.T) {
	s := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(s, cfg)
	if err := a.vault.SetMaster("synthetic-master"); err != nil {
		t.Fatal(err)
	}
	a.prepareSettings()
	a.loadSharedSyncSettings()
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "https://fixture.invalid/dav", Username: "fixture-user"}
	a.features.sharedSync.options = options
	a.features.sharedSync.password = "fixture-password"
	a.applySettings()
	drainSharedSync(t, a)
	saved, err := core.LoadSyncOptions(s.Dir, []byte("synthetic-master"))
	if err != nil || saved != options {
		t.Fatal("main Apply did not save sync options", err, a.features.settingsError)
	}
	p := options.CredentialProfile()
	password, err := a.vault.GetFor(p, p.Identity())
	if err != nil || password != "fixture-password" {
		t.Fatal("credential missing", err)
	}
	if a.settingsOpen {
		t.Fatal("Apply did not close settings")
	}
}

func TestWebDAVDraftRejectsPendingDirectoryAndStaleOptions(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	if err := a.vault.SetMaster("synthetic-master"); err != nil {
		t.Fatal(err)
	}
	a.prepareSettings()
	a.loadSharedSyncSettings()
	f := &a.features.sharedSync
	f.options = core.SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "https://draft.invalid/"}
	f.pendingDirectory = &core.PendingSyncDirectory{Version: 1, Mode: "create", Path: t.TempDir()}
	if err := a.validateSyncDraft(); err == nil {
		t.Fatal("accepted both sync modes")
	}
	f.pendingDirectory = nil
	other := core.SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "https://other.invalid/"}
	if err := core.SaveSyncOptions(a.store.Dir, other, []byte("synthetic-master")); err != nil {
		t.Fatal(err)
	}
	if err := a.validateSyncDraft(); err == nil {
		t.Fatal("overwrote externally changed options")
	}
}
