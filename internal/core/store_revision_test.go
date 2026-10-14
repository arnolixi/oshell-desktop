package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRejectsExternalChangesUntilExplicitReload(t *testing.T) {
	dir := t.TempDir()
	first, second := &Store{Dir: dir}, &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "fixture.invalid"
	cfg.Profiles = []Profile{p}
	if err := first.Save(cfg); err != nil {
		t.Fatal(err)
	}
	stale, err := second.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Profiles[0].Name = "external update"
	if err = first.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err = second.Save(stale); !errors.Is(err, ErrConfigurationChanged) {
		t.Fatal("stale save overwrote external config", err)
	}
	vault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	latestRevision, _ := second.Revision()
	if err = second.CommitCredentials(context.Background(), stale, vault, []CredentialUpdate{{Profile: p, Password: "secret", Identity: p.Identity()}}, latestRevision); !errors.Is(err, ErrConfigurationChanged) {
		t.Fatal("fresh disk revision bypassed observed config guard", err)
	}
	if vault.HasStoredCredential(p.ID) {
		t.Fatal("rejected transaction saved credential")
	}
	loaded, err := second.Load()
	if err != nil || loaded.Profiles[0].Name != "external update" {
		t.Fatal("external data lost", err)
	}
	loaded.Profiles[0].Name = "after reload"
	if err = second.Save(loaded); err != nil {
		t.Fatal("explicit reload did not permit save", err)
	}
	if err = (&Store{Dir: dir}).Save(stale); !errors.Is(err, ErrConfigurationChanged) {
		t.Fatal("unobserved store overwrote existing config", err)
	}
	if err = os.Remove(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal(err)
	}
	if _, err = second.Load(); err == nil {
		t.Fatal("missing known config became defaults")
	}
	if err = second.Save(loaded); !errors.Is(err, ErrConfigurationChanged) {
		t.Fatal("deleted config silently recreated", err)
	}
}
