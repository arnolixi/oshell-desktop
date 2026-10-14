package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedReplicaPrivateRoundtrip(t *testing.T) {
	document, master := mergeFixture(t)
	store := SharedReplicaStore{Directory: t.TempDir(), Target: "synthetic-target"}
	replica := SharedReplica{Version: 1, Target: store.Target, Baseline: &document, CachedLocal: &document}
	if e := store.Save(replica, master); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(store.path())
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(data, []byte("example.invalid")) || bytes.Contains(data, []byte(store.Target)) {
		t.Fatal("private replica is plaintext")
	}
	loaded, e := store.Load(master)
	if e != nil || loaded.Baseline.Profiles[0].Definition.Host != "example.invalid" {
		t.Fatal("replica roundtrip", e)
	}
	if _, e = store.Load([]byte("wrong-password")); e == nil {
		t.Fatal("wrong master accepted")
	}
	if e = store.Save(SharedReplica{Version: 1, Target: "different-target"}, master); e == nil {
		t.Fatal("accepted wrong target")
	}
}
func TestSharedReplacementRemovesMissingCredential(t *testing.T) {
	document, master := mergeFixture(t)
	previous, e := document.Configuration()
	if e != nil {
		t.Fatal(e)
	}
	vault, e := OpenVault(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	profile := previous.Profiles[0]
	if e = vault.Put(profile, "old-local-secret"); e != nil {
		t.Fatal(e)
	}
	cfg, updates, e := document.PrepareLocalReplacement(context.Background(), previous, master)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, u := range updates {
		if u.Profile.ID == cfg.Profiles[0].ID {
			found = u.Delete
		}
	}
	if !found {
		t.Fatal("missing remote credential would retain old local secret")
	}
}

func TestDisableSyncArchivesBaselineForMasterRotation(t *testing.T) {
	document, master := mergeFixture(t)
	dir := t.TempDir()
	options := SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: t.TempDir()}
	if e := SaveSyncOptions(dir, options); e != nil {
		t.Fatal(e)
	}
	replica := SharedReplicaStore{Directory: dir, Target: options.Target()}
	if e := replica.Save(SharedReplica{Version: 1, Target: options.Target(), Baseline: &document}, master); e != nil {
		t.Fatal(e)
	}
	options.Enabled = false
	if e := SaveSyncOptions(dir, options); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(replica.path()); !os.IsNotExist(e) {
		t.Fatal("disabled target retains active baseline")
	}
	matches, e := filepath.Glob(replica.path() + ".archived-*")
	if e != nil || len(matches) != 1 {
		t.Fatal("old baseline not retained", e)
	}
	options.Enabled = true
	if e = SaveSyncOptions(dir, options); e != nil {
		t.Fatal(e)
	}
	if restored, e := replica.Load([]byte("rotated-master")); e != nil || restored != nil {
		t.Fatal("re-enable depends on old master", e)
	}
}
