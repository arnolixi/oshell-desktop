package core

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestMixedCredentialProtectionLifecycle(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir}
	cfg := DefaultConfig()
	local, master := NewProfile(), NewProfile()
	local.Host, master.Host = "local.example", "master.example"
	cfg.Profiles = []Profile{local, master}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster("original-master-password"); err != nil {
		t.Fatal(err)
	}
	commit := func(updates []CredentialUpdate) {
		t.Helper()
		revision, err := store.Revision()
		if err != nil {
			t.Fatal(err)
		}
		if err = store.CommitCredentials(context.Background(), cfg, vault, updates, revision); err != nil {
			t.Fatal(err)
		}
	}
	commit([]CredentialUpdate{{Profile: local, Identity: local.Identity(), Password: "local-secret", Protection: "local"}, {Profile: master, Identity: master.Identity(), Password: "master-secret", Protection: "master"}})
	ciphertext := append([]byte(nil), vault.data.Entries[local.ID].Data...)
	if err = vault.SetMaster("replacement-master-password"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ciphertext, vault.data.Entries[local.ID].Data) {
		t.Fatal("master rotation rewrote local credential")
	}
	vault.Lock()
	if secret, err := vault.GetFor(local, local.Identity()); err != nil || secret != "local-secret" {
		t.Fatal("local credential depends on master", err)
	}
	if _, err := vault.GetFor(master, master.Identity()); !errors.Is(err, ErrVaultLocked) {
		t.Fatal("master credential readable while locked", err)
	}
	vault, err = OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if secret, err := vault.GetFor(local, local.Identity()); err != nil || secret != "local-secret" {
		t.Fatal("reopen lost local credential", err)
	}
	if err = vault.Unlock("replacement-master-password"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := vault.snapshotCredentials()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(snapshot.key)
	defer clear(snapshot.localKey)
	for _, item := range []struct {
		p    Profile
		want string
	}{{local, "local-secret"}, {master, "master-secret"}} {
		secret, _, _, err := snapshot.credential(item.p)
		if err != nil || secret != item.want {
			t.Fatal("sync snapshot failed", err)
		}
		secret, _, err = vault.exportCredential(item.p)
		if err != nil || secret != item.want {
			t.Fatal("archive export failed", err)
		}
	}
	copy := local
	copy.ID = NewID()
	if err = vault.CopyCredential(local, copy); err != nil {
		t.Fatal(err)
	}
	if vault.CredentialProtection(copy.ID) != "local" {
		t.Fatal("copy lost protection")
	}
	changes, err := vault.PrepareCredentialEdit(&local, local, "", true, local.Identity(), "master")
	if err != nil {
		t.Fatal(err)
	}
	commit(changes)
	vault.Lock()
	if _, err = vault.GetFor(local, local.Identity()); !errors.Is(err, ErrVaultLocked) {
		t.Fatal("protection conversion failed", err)
	}
	if secret, err := vault.GetFor(copy, copy.Identity()); err != nil || secret != "local-secret" {
		t.Fatal("copy changed after source conversion", err)
	}
}

func TestCredentialProtectionRejectsUnavailableMaster(t *testing.T) {
	vault, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := NewProfile()
	if _, err = vault.PrepareCredentialEdit(nil, p, "secret", true, p.Identity(), "master"); !errors.Is(err, ErrVaultLocked) {
		t.Fatal("accepted missing master", err)
	}
	if _, err = vault.PrepareCredentialEdit(nil, p, "secret", true, p.Identity(), "invalid"); err == nil {
		t.Fatal("accepted invalid mode")
	}
}

func TestMasterEnableClearAndReenableConvertsLocalCredentials(t *testing.T) {
	vault, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := NewProfile()
	p.Host = "fixture.invalid"
	if err = vault.PutForRevision(p, "saved-secret", p.Identity(), vault.Revision(), "local"); err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster("first-master-password"); err != nil {
		t.Fatal(err)
	}
	if vault.CredentialProtection(p.ID) != "master" {
		t.Fatal("enable did not convert local password")
	}
	before := vault.Revision()
	for _, current := range []string{"", "wrong-password"} {
		if err = vault.ChangeMaster(current, ""); err == nil {
			t.Fatal("cleared without verifying current password")
		}
		if vault.Revision() != before {
			t.Fatal("failed verification modified vault")
		}
	}
	if err = vault.ChangeMaster("first-master-password", "first-master-password"); err == nil {
		t.Fatal("same password accepted")
	}
	if err = vault.ChangeMaster("first-master-password", ""); err != nil {
		t.Fatal(err)
	}
	if !vault.data.Entries[p.ID].Local {
		t.Fatal("clear did not mark local encryption")
	}
	vault, err = OpenVault(vault.dir)
	if err != nil {
		t.Fatal(err)
	}
	if secret, err := vault.GetFor(p, p.Identity()); err != nil || secret != "saved-secret" {
		t.Fatal("clear lost credential", err)
	}
	if err = vault.SetMaster("second-master-password"); err != nil {
		t.Fatal(err)
	}
	vault.Lock()
	if _, err = vault.GetFor(p, p.Identity()); !errors.Is(err, ErrVaultLocked) {
		t.Fatal("reenable left credential local", err)
	}
}
