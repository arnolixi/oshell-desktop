package core

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeAdoptionBacksUpAndCommitsCredentialsTogether(t *testing.T) {
	source := t.TempDir()
	engine, sourceConfig, master := syncEngineFixture(t, source)
	plan, err := engine.Prepare(context.Background(), sourceConfig, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(context.Background(), nil, master); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadNativeStorage(source, master)
	if err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: t.TempDir()}
	old := DefaultConfig()
	p := NewProfile()
	p.Name = "old"
	p.Host = "old.invalid"
	old.Profiles = []Profile{p}
	if err = store.Save(old); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster("destination-master-password"); err != nil {
		t.Fatal(err)
	}
	if err = vault.Put(p, "old-secret"); err != nil {
		t.Fatal(err)
	}
	revision, _ := store.Revision()
	vaultRevision := vault.Revision()
	result, backup, err := store.AdoptNativeStorage(context.Background(), snapshot, master, old, vault, revision, vaultRevision)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	var saved stateTransaction
	if err = json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	var savedConfig Config
	if err = json.Unmarshal(saved.Config, &savedConfig); err != nil || savedConfig.Profiles[0].ID != p.ID {
		t.Fatal("backup lost previous config", err)
	}
	if vault.HasStoredCredential(p.ID) {
		t.Fatal("old credential retained")
	}
	if len(result.Profiles) != len(sourceConfig.Profiles) {
		t.Fatal("source config not adopted")
	}
	disk, err := store.Load()
	if err != nil || len(disk.Profiles) != len(result.Profiles) {
		t.Fatal("configuration not persisted", err)
	}
	for _, entry := range snapshot.Document.Profiles {
		if entry.Password == nil {
			continue
		}
		profile := entry.Definition
		got, err := vault.GetFor(profile, entry.Password.Identity)
		want, decryptErr := DecryptMacPassword(*entry.Password, master, profile)
		if err != nil || decryptErr != nil || got != want {
			t.Fatal("credential not migrated", err, decryptErr)
		}
	}
	if _, _, err = store.AdoptNativeStorage(context.Background(), snapshot, master, old, vault, revision, vaultRevision); err == nil {
		t.Fatal("stale preview overwrote new state")
	}
}
