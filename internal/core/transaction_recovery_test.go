package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// On Windows this also checks that recovery closes its journal before deleting it.
func TestPortableStartupRecoversJournalAndAllowsRepeatedSaves(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "recovery.invalid"
	p.Username = "fixture"
	cfg.Profiles = []Profile{p}
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = v.Put(p, "fixture-password"); err != nil {
		t.Fatal(err)
	}
	config, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	vault, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := json.Marshal(stateTransaction{1, config, vault})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, transactionFile), journal, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := openStoreDirectory(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load()
	if err != nil || len(loaded.Profiles) != 1 || loaded.Profiles[0].ID != p.ID {
		t.Fatalf("recovery lost config: %v", err)
	}
	v, err = OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	password, err := v.Get(p)
	if err != nil || password != "fixture-password" {
		t.Fatalf("recovery lost credential: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err = os.Stat(filepath.Join(dir, transactionFile)); !os.IsNotExist(err) {
			t.Fatalf("journal not removed: %v", err)
		}
		revision, err := s.Revision()
		if err != nil {
			t.Fatal(err)
		}
		if err = s.CommitCredentials(context.Background(), loaded, v, nil, revision); err != nil {
			t.Fatalf("save %d after recovery: %v", i, err)
		}
	}
	if _, err = os.Stat(filepath.Join(dir, transactionFile)); !os.IsNotExist(err) {
		t.Fatalf("last journal not removed: %v", err)
	}
}
