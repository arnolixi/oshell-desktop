package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedStoreSaveTransactionAndRecovery(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "private.example.invalid"
	p.Name = "private-metadata"
	cfg.Profiles = []Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	master := []byte("fixture-master-password")
	revision, _ := store.Revision()
	if err := store.ProtectConfiguration(cfg, master, revision); err != nil {
		t.Fatal(err)
	}
	cfg.Profiles[0].Name = "updated-private-metadata"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster(string(master)); err != nil {
		t.Fatal(err)
	}
	revision, _ = store.Revision()
	if err = store.CommitCredentials(context.Background(), cfg, vault, []CredentialUpdate{{Profile: p, Identity: p.Identity(), Password: "login-secret"}}, revision); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil || !IsSharedVault(raw) || bytes.Contains(raw, []byte(p.Host)) {
		t.Fatal("transaction downgraded configuration", err)
	}
	vaultBytes, _ := json.Marshal(vault.data)
	tx, _ := json.Marshal(stateTransaction{1, raw, vaultBytes})
	if err = os.WriteFile(filepath.Join(dir, transactionFile), tx, 0600); err != nil {
		t.Fatal(err)
	}
	if err = recoverTransaction(dir); err != nil {
		t.Fatal("encrypted recovery failed", err)
	}
	reopened := &Store{Dir: dir}
	if _, err = reopened.Load(); !errors.Is(err, ErrConfigurationLocked) {
		t.Fatal("restart bypassed lock", err)
	}
	if _, err = reopened.UnlockConfiguration([]byte("wrong")); err == nil {
		t.Fatal("wrong key accepted")
	}
	loaded, err := reopened.UnlockConfiguration(master)
	if err != nil || loaded.Profiles[0].Name != cfg.Profiles[0].Name {
		t.Fatal("unlock/load failed", err)
	}
	reopened.LockConfiguration()
	if err = reopened.Save(cfg); !errors.Is(err, ErrConfigurationLocked) {
		t.Fatal("locked save accepted", err)
	}
	revision, _ = reopened.Revision()
	if err = reopened.CommitCredentials(context.Background(), cfg, vault, nil, revision); !errors.Is(err, ErrConfigurationLocked) {
		t.Fatal("locked transaction accepted", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !bytes.Equal(after, raw) {
		t.Fatal("locked operation changed config")
	}
}
