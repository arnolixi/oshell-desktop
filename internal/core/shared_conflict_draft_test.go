package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func conflictedSyncFixture(t *testing.T) (*SharedSyncEngine, Config, []byte) {
	t.Helper()
	ctx := context.Background()
	engine, cfg, master := syncEngineFixture(t, t.TempDir())
	plan, err := engine.Prepare(ctx, cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = plan.Commit(ctx, nil, master)
	if err != nil {
		t.Fatal(err)
	}
	object, err := engine.Transport.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := DecodeSharedDocument(object.Data, master)
	if err != nil {
		t.Fatal(err)
	}
	remote.Profiles[0].Definition.Name = "remote-choice"
	data, err := remote.Encode(master)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Transport.Put(ctx, data, object.ETag); err != nil {
		t.Fatal(err)
	}
	cfg.Profiles[0].Name = "private-local-draft"
	if err = engine.Store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return engine, cfg, master
}
func TestSharedConflictDraftRestartAndLatestRemote(t *testing.T) {
	ctx := context.Background()
	engine, cfg, master := conflictedSyncFixture(t)
	plan, err := engine.Prepare(ctx, cfg, master, true)
	if err != nil || len(plan.Conflicts) != 1 {
		t.Fatal(err)
	}
	store := engine.ConflictStore()
	data, err := os.ReadFile(store.path())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("private-local-draft")) {
		t.Fatal("draft leaked connection metadata")
	}
	info, err := os.Stat(store.path())
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("draft not owner-only", err)
	}
	if _, err = store.Load([]byte("wrong-master")); err == nil {
		t.Fatal("draft readable with wrong master")
	}
	object, err := engine.Transport.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := DecodeSharedDocument(object.Data, master)
	if err != nil {
		t.Fatal(err)
	}
	remote.Profiles[0].Definition.Name = "newer-remote-choice"
	next, err := remote.Encode(master)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Transport.Put(ctx, next, object.ETag); err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(ctx, map[string]SharedMergeChoice{plan.Conflicts[0].Key: SharedKeepLocal}, master); err == nil {
		t.Fatal("old choice overwrote changed remote")
	}
	if !store.Exists() {
		t.Fatal("failure discarded draft")
	}
	// Reopen disk state as a new process would, without retaining the old plan.
	vault, err := OpenVault(engine.Store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.Unlock(string(master)); err != nil {
		t.Fatal(err)
	}
	restored := &SharedSyncEngine{Store: engine.Store, Vault: vault, Transport: engine.Transport, Replica: engine.Replica}
	plan, err = restored.Prepare(ctx, cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Local.Profiles[0].Definition.Name != "private-local-draft" || plan.Remote.Profiles[0].Definition.Name != "newer-remote-choice" {
		t.Fatal("restart failed to restore draft against latest remote")
	}
	resolved, err := plan.Commit(ctx, map[string]SharedMergeChoice{plan.Conflicts[0].Key: SharedKeepRemote}, master)
	if err != nil || resolved.Profiles[0].Name != "newer-remote-choice" || store.Exists() {
		t.Fatal("resolution did not commit/clear", err)
	}
}
func TestSharedConflictDraftLocalChangeRequiresExplicitDiscard(t *testing.T) {
	ctx := context.Background()
	engine, cfg, master := conflictedSyncFixture(t)
	if _, err := engine.Prepare(ctx, cfg, master, true); err != nil {
		t.Fatal(err)
	}
	before, err := engine.Transport.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Profiles[0].Username = "edited-after-conflict"
	if err = engine.Store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Prepare(ctx, cfg, master, true); err == nil {
		t.Fatal("silently replaced pending local draft")
	}
	if !engine.ConflictStore().Exists() {
		t.Fatal("stale draft lost")
	}
	if err = engine.ConflictStore().Clear(); err != nil {
		t.Fatal(err)
	}
	after, err := engine.Transport.Fetch(ctx)
	if err != nil || !bytes.Equal(before.Data, after.Data) {
		t.Fatal("discard changed remote", err)
	}
	saved, err := engine.Store.Load()
	if err != nil || saved.Profiles[0].Username != "edited-after-conflict" {
		t.Fatal("discard changed saved local config", err)
	}
	if _, err = engine.Prepare(ctx, saved, master, true); err != nil {
		t.Fatal("explicit discard did not allow fresh plan", err)
	}
}

func TestInitialConflictDraftDoesNotRecreateDeletedRemote(t *testing.T) {
	ctx := context.Background()
	remote := t.TempDir()
	first, cfg, master := syncEngineFixture(t, remote)
	plan, err := first.Prepare(ctx, cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(ctx, nil, master); err != nil {
		t.Fatal(err)
	}
	second, other, _ := syncEngineFixture(t, remote)
	plan, err = second.Prepare(ctx, other, master, true)
	if err != nil || !plan.InitialChoiceRequired {
		t.Fatal("expected first-sync draft", err)
	}
	if err = os.Remove(filepath.Join(remote, "configuration.json")); err != nil {
		t.Fatal(err)
	}
	if _, err = second.Prepare(ctx, other, master, true); err == nil {
		t.Fatal("pending draft allowed deleted remote recreation")
	}
	if !second.ConflictStore().Exists() {
		t.Fatal("missing remote discarded local draft")
	}
}

// Opt-in fixture for the real macOS app. Contains synthetic profiles only.
func TestNativeSharedConflictFixture(t *testing.T) {
	root := os.Getenv("OSHELL_SYNC_NATIVE_FIXTURE")
	if root == "" {
		t.Skip("explicit isolated native fixture directory required")
	}
	if _, e := os.Stat(root); !os.IsNotExist(e) {
		t.Fatal("fixture directory must not already exist")
	}
	local, remote := filepath.Join(root, "local"), filepath.Join(root, "remote")
	if e := os.MkdirAll(local, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(remote, 0700); e != nil {
		t.Fatal(e)
	}
	store := &Store{Dir: local}
	cfg := DefaultConfig()
	for i := 0; i < 3; i++ {
		profile := NewProfile()
		profile.Name = fmt.Sprintf("Sync fixture %d", i+1)
		profile.Host = fmt.Sprintf("sync-%d.fixture.invalid", i+1)
		cfg.Profiles = append(cfg.Profiles, profile)
	}
	if e := store.Save(cfg); e != nil {
		t.Fatal(e)
	}
	vault, e := OpenVault(local)
	if e != nil {
		t.Fatal(e)
	}
	master := []byte("shared-fixture-only")
	if e = vault.SetMaster(string(master)); e != nil {
		t.Fatal(e)
	}
	if e = vault.Put(cfg.Profiles[0], "synthetic-login-fixture"); e != nil {
		t.Fatal(e)
	}
	revision, err := store.Revision()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ProtectConfiguration(cfg, master, revision); err != nil {
		t.Fatal(err)
	}
	options := SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: remote}
	if e = SaveSyncOptions(local, options); e != nil {
		t.Fatal(e)
	}
	engine := &SharedSyncEngine{Store: store, Vault: vault, Transport: DirectoryTransport{remote}, Replica: SharedReplicaStore{Directory: local, Target: options.Target()}}
	ctx := context.Background()
	plan, e := engine.Prepare(ctx, cfg, master, true)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e = plan.Commit(ctx, nil, master)
	if e != nil {
		t.Fatal(e)
	}
	object, e := engine.Transport.Fetch(ctx)
	if e != nil {
		t.Fatal(e)
	}
	document, e := DecodeSharedDocument(object.Data, master)
	if e != nil {
		t.Fatal(e)
	}
	for i := range document.Profiles {
		document.Profiles[i].Definition.Name = fmt.Sprintf("Remote fixture choice %d", i+1)
	}
	data, e := document.Encode(master)
	if e != nil {
		t.Fatal(e)
	}
	if e = engine.Transport.Put(ctx, data, object.ETag); e != nil {
		t.Fatal(e)
	}
	for i := range cfg.Profiles {
		cfg.Profiles[i].Name = fmt.Sprintf("Local fixture draft %d", i+1)
	}
	if e = store.Save(cfg); e != nil {
		t.Fatal(e)
	}
	plan, e = engine.Prepare(ctx, cfg, master, true)
	if e != nil || len(plan.Conflicts) != 3 {
		t.Fatal("fixture did not create conflict", e)
	}
}

func TestNativeSharedConflictResult(t *testing.T) {
	root := os.Getenv("OSHELL_SYNC_NATIVE_VERIFY")
	if root == "" {
		t.Skip("explicit native result directory required")
	}
	store := &Store{Dir: filepath.Join(root, "local")}
	cfg, err := store.UnlockConfiguration([]byte("shared-fixture-only"))
	if err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.Unlock("shared-fixture-only"); err != nil {
		t.Fatal(err)
	}
	secretCount := 0
	for _, profile := range cfg.Profiles {
		if profile.Host == "sync-1.fixture.invalid" {
			secret, err := vault.GetFor(profile, profile.Identity())
			if err != nil || secret != "synthetic-login-fixture" {
				t.Fatal("native resolution lost encrypted credential", err)
			}
			secretCount++
		}
	}
	if secretCount != 1 {
		t.Fatal("credential fixture profile missing")
	}
	options, err := LoadSyncOptions(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if (SharedConflictStore{Directory: store.Dir, Target: options.Target()}).Exists() {
		t.Fatal("native resolution left draft")
	}
}
