package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func syncEngineFixture(t *testing.T, remote string) (*SharedSyncEngine, Config, []byte) {
	t.Helper()
	dir := t.TempDir()
	store := &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "example.invalid"
	p.Username = "tester"
	cfg.Profiles = []Profile{p}
	if e := store.Save(cfg); e != nil {
		t.Fatal(e)
	}
	vault, e := OpenVault(dir)
	if e != nil {
		t.Fatal(e)
	}
	master := []byte("shared-test-password")
	if e = vault.SetMaster(string(master)); e != nil {
		t.Fatal(e)
	}
	return &SharedSyncEngine{Store: store, Vault: vault, Transport: DirectoryTransport{remote}, Replica: SharedReplicaStore{Directory: dir, Target: remote}}, cfg, master
}
func TestSharedSyncCreateDownloadAndConflict(t *testing.T) {
	ctx := context.Background()
	remote := t.TempDir()
	a, cfg, master := syncEngineFixture(t, remote)
	if e := a.Vault.Put(cfg.Profiles[0], "shared-login-secret"); e != nil {
		t.Fatal(e)
	}
	plan, e := a.Prepare(ctx, cfg, master, true)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e = plan.Commit(ctx, nil, master)
	if e != nil {
		t.Fatal(e)
	}
	b, other, _ := syncEngineFixture(t, remote)
	plan, e = b.Prepare(ctx, other, master, true)
	if e != nil {
		t.Fatal(e)
	}
	if !plan.InitialChoiceRequired {
		t.Fatal("existing remote silently accepted")
	}
	if e = plan.ChooseInitial(false); e != nil {
		t.Fatal(e)
	}
	other, e = plan.Commit(ctx, nil, master)
	if e != nil {
		t.Fatal(e)
	}
	secret, e := b.Vault.GetFor(other.Profiles[0], other.Profiles[0].Identity())
	if e != nil || secret != "shared-login-secret" {
		t.Fatal("password not preserved", e)
	}
	backups, e := os.ReadDir(filepath.Join(b.Store.Dir, "storage-backups"))
	if e != nil || len(backups) != 1 {
		t.Fatal("missing first-sync backup", e)
	}
	cfg.Profiles[0].Name = "A edit"
	if e = a.Store.Save(cfg); e != nil {
		t.Fatal(e)
	}
	plan, e = a.Prepare(ctx, cfg, master, true)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e = plan.Commit(ctx, nil, master)
	if e != nil {
		t.Fatal(e)
	}
	other.Profiles[0].Name = "B edit"
	if e = b.Store.Save(other); e != nil {
		t.Fatal(e)
	}
	plan, e = b.Prepare(ctx, other, master, true)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan.Conflicts) != 1 {
		t.Fatalf("expected one whole-session conflict, got %+v", plan.Conflicts)
	}
	if _, e = plan.Commit(ctx, nil, master); e == nil {
		t.Fatal("unresolved conflict committed")
	}
	other, e = plan.Commit(ctx, map[string]SharedMergeChoice{plan.Conflicts[0].Key: SharedKeepRemote}, master)
	if e != nil {
		t.Fatal(e)
	}
	if other.Profiles[0].Name != "A edit" {
		t.Fatal("remote choice not applied")
	}
}
func TestSharedSyncRejectsStaleLocalAndMissingRemote(t *testing.T) {
	ctx := context.Background()
	remote := t.TempDir()
	engine, cfg, master := syncEngineFixture(t, remote)
	plan, e := engine.Prepare(ctx, cfg, master, true)
	if e != nil {
		t.Fatal(e)
	}
	cfg.Profiles[0].Name = "new local edit"
	if e = engine.Store.Save(cfg); e != nil {
		t.Fatal(e)
	}
	if _, e = plan.Commit(ctx, nil, master); e == nil {
		t.Fatal("stale local snapshot uploaded")
	}
	if _, e = os.Stat(filepath.Join(remote, "configuration.json")); !os.IsNotExist(e) {
		t.Fatal("remote created despite stale local snapshot")
	}
	plan, e = engine.Prepare(ctx, cfg, master, true)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e = plan.Commit(ctx, nil, master)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(remote, "configuration.json")); e != nil {
		t.Fatal(e)
	}
	if _, e = engine.Prepare(ctx, cfg, master, true); e == nil {
		t.Fatal("deleted remote would be recreated")
	}
}

func TestSharedSyncNoChangeDoesNotRewrite(t *testing.T) {
	ctx := context.Background()
	remote := t.TempDir()
	engine, cfg, master := syncEngineFixture(t, remote)
	if e := engine.Vault.Put(cfg.Profiles[0], "stable-secret"); e != nil {
		t.Fatal(e)
	}
	plan, e := engine.Prepare(ctx, cfg, master, true)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e = plan.Commit(ctx, nil, master)
	if e != nil {
		t.Fatal(e)
	}
	revision, _ := engine.Store.Revision()
	vaultRevision := engine.Vault.Revision()
	before, e := engine.Transport.Fetch(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		plan, e = engine.Prepare(ctx, cfg, master, true)
		if e != nil {
			t.Fatal(e)
		}
		cfg, e = plan.Commit(ctx, nil, master)
		if e != nil {
			t.Fatal(e)
		}
	}
	after, e := engine.Transport.Fetch(ctx)
	if e != nil {
		t.Fatal(e)
	}
	next, _ := engine.Store.Revision()
	if before.ETag != after.ETag || revision != next || vaultRevision != engine.Vault.Revision() {
		t.Fatal("unchanged sync rewrote local or remote data")
	}
}
