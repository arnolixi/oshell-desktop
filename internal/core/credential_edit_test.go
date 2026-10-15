package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialEditPreservesAndRebindsWithoutSecretRetyping(t *testing.T) {
	v, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := NewProfile()
	p.Host = "fixture.invalid"
	p.Username = "tester"
	if err = v.PutFor(p, "fixture-secret", p.Identity()); err != nil {
		t.Fatal(err)
	}
	renamed := p
	renamed.Name = "renamed"
	changes, err := v.PrepareCredentialEdit(&p, renamed, "", true, p.Identity())
	if err != nil || len(changes) != 0 {
		t.Fatal("rename did not preserve original envelope", err)
	}
	changed := p
	changed.Host = "different.invalid"
	if changes, err = v.PrepareCredentialEdit(&p, changed, "", true, changed.Identity()); err != nil || len(changes) != 1 || changes[0].Password != "fixture-secret" || changes[0].Identity != changed.Identity() {
		t.Fatal("explicit host edit did not rebind saved credential", err)
	}
	file := p
	file.Kind = "sftp"
	changes, err = v.PrepareCredentialEdit(&p, file, "", true, p.Identity())
	if err != nil || len(changes) != 1 || changes[0].Password != "fixture-secret" {
		t.Fatal("SSH/SFTP migration failed", err)
	}
	wrong := p.Identity()
	wrong.Host = "changed-resolution.invalid"
	if _, err = v.PrepareCredentialEdit(&p, file, "", true, wrong); err == nil {
		t.Fatal("resolved identity change reused secret")
	}
	changes, err = v.PrepareCredentialEdit(&p, p, "", false, p.Identity())
	if err != nil || len(changes) != 1 || !changes[0].Delete {
		t.Fatal("unchecking save did not prepare deletion", err)
	}
	if !v.HasStoredCredential(p.ID) {
		t.Fatal("preparation mutated vault")
	}
}
func TestCredentialEditCommitRejectsConcurrentVaultChange(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "fixture.invalid"
	p.Username = "tester"
	cfg.Profiles = []Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = v.PutFor(p, "original", p.Identity()); err != nil {
		t.Fatal(err)
	}
	revision, err := store.Revision()
	if err != nil {
		t.Fatal(err)
	}
	vaultRevision := v.Revision()
	changes, err := v.PrepareCredentialEdit(&p, p, "editor-change", true, p.Identity())
	if err != nil {
		t.Fatal(err)
	}
	if err = v.PutFor(p, "concurrent-change", p.Identity()); err != nil {
		t.Fatal(err)
	}
	if err = store.CommitCredentialEdit(context.Background(), cfg, v, changes, revision, vaultRevision); err == nil {
		t.Fatal("stale editor overwrote new credential")
	}
	got, err := v.GetFor(p, p.Identity())
	if err != nil || got != "concurrent-change" {
		t.Fatal("concurrent secret not retained", err)
	}
}

func TestCancelledCredentialEditDoesNotChangeEitherFile(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "fixture.invalid"
	p.Username = "tester"
	cfg.Profiles = []Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.PutFor(p, "original", p.Identity()); err != nil {
		t.Fatal(err)
	}
	configBefore, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	vaultBefore, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	revision, err := store.Revision()
	if err != nil {
		t.Fatal(err)
	}
	changes, err := vault.PrepareCredentialEdit(&p, p, "replacement", true, p.Identity())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Profiles[0].Name = "cancelled edit"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = store.CommitCredentialEdit(ctx, cfg, vault, changes, revision, vault.Revision()); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation not returned", err)
	}
	configAfter, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	vaultAfter, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if !bytes.Equal(configBefore, configAfter) || !bytes.Equal(vaultBefore, vaultAfter) {
		t.Fatal("cancelled edit changed persisted state")
	}
}
