package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMasterTransactionKeepsConfigAndVaultInSameGeneration(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "private-master.invalid"
	cfg.Profiles = []Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.PutFor(p, "login-secret", p.Identity()); err != nil {
		t.Fatal(err)
	}
	change := func(current, next string) error {
		revision, err := store.Revision()
		if err != nil {
			return err
		}
		return store.ChangeProtection(context.Background(), cfg, vault, current, next, revision, vault.Revision())
	}
	if err = change("", "first-master-password"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !IsSharedVault(raw) || bytes.Contains(raw, []byte(p.Host)) {
		t.Fatal("setup left plaintext config")
	}
	revision, _ := store.Revision()
	vaultRevision := vault.Revision()
	if err = change("wrong", "second-master-password"); err == nil {
		t.Fatal("wrong current password accepted")
	}
	after, _ := store.Revision()
	if after != revision || vault.Revision() != vaultRevision {
		t.Fatal("failed rotation changed state")
	}
	if err = change("first-master-password", "second-master-password"); err != nil {
		t.Fatal(err)
	}
	reopened := &Store{Dir: dir}
	if _, err = reopened.UnlockConfiguration([]byte("first-master-password")); err == nil {
		t.Fatal("old password decrypts config")
	}
	if _, err = reopened.UnlockConfiguration([]byte("second-master-password")); err != nil {
		t.Fatal(err)
	}
	diskVault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = diskVault.Unlock("second-master-password"); err != nil {
		t.Fatal(err)
	}
	if got, err := diskVault.GetFor(p, p.Identity()); err != nil || got != "login-secret" {
		t.Fatal("credential rotation lost secret", err)
	}
	if err = change("second-master-password", ""); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(dir, "config.json"))
	if IsSharedVault(raw) {
		t.Fatal("clear left config protected")
	}
	if protected, locked := store.ConfigurationProtection(); protected || locked {
		t.Fatal("clear left store locked")
	}
	if got, err := vault.GetFor(p, p.Identity()); err != nil || got != "login-secret" {
		t.Fatal("clear lost local credential", err)
	}
}
func TestMasterTransactionFailureBeforeIntentKeepsOldState(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	cfg := DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	revision, _ := store.Revision()
	vaultRevision := vault.Revision()
	if err = os.Mkdir(filepath.Join(store.Dir, transactionFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err = store.ChangeProtection(context.Background(), cfg, vault, "", "fixture-master-password", revision, vaultRevision); err == nil {
		t.Fatal("failed intent write reported success")
	}
	after, _ := store.Revision()
	if after != revision || vault.Revision() != vaultRevision {
		t.Fatal("failed intent changed config/vault")
	}
}
