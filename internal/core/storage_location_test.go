package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageLocationMigratesLatestConfigAndCredentials(t *testing.T) {
	base := filepath.Join(t.TempDir(), "data")
	t.Setenv("OSHELL_DATA_DIR", base)
	store, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "fixture.invalid"
	p.Username = "tester"
	cfg.Profiles = []Profile{p}
	if err = store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.Put(p, "fixture-secret"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "custom")
	if err = store.ScheduleStorageDirectory(target); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("Apply moved data before restart")
	}
	cfg.Profiles[0].Name = "last write before restart"
	if err = store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	realTarget, _ := filepath.EvalSymlinks(target)
	if reopened.Dir != realTarget || reopened.StorageWarning != "" {
		t.Fatalf("migration failed: %q %s", reopened.Dir, reopened.StorageWarning)
	}
	loaded, err := reopened.Load()
	if err != nil || loaded.Profiles[0].Name != cfg.Profiles[0].Name {
		t.Fatal("latest config missing", err)
	}
	for _, name := range []string{"config.json", "credentials.json", "credential-key"} {
		before, e := os.ReadFile(filepath.Join(base, name))
		if e != nil {
			t.Fatal(e)
		}
		after, e := os.ReadFile(filepath.Join(target, name))
		if e != nil || !bytes.Equal(before, after) {
			t.Fatal("migration changed bytes", name, e)
		}
	}
	movedVault, err := OpenVault(target)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := movedVault.Get(p)
	if err != nil || secret != "fixture-secret" {
		t.Fatal("moved credential cannot decrypt", err)
	}
	again, err := OpenStore()
	if err != nil || again.Dir != reopened.Dir {
		t.Fatal("custom directory not retained", err)
	}
}

func TestStorageLocationFailureKeepsOriginalAndCancel(t *testing.T) {
	base := filepath.Join(t.TempDir(), "data")
	t.Setenv("OSHELL_DATA_DIR", base)
	store, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "custom")
	if err = store.ScheduleStorageDirectory(target); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(target, "existing"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Dir != store.Dir || reopened.StorageWarning == "" {
		t.Fatal("failed migration did not fall back")
	}
	if err = reopened.ScheduleStorageDirectory(reopened.Dir); err != nil {
		t.Fatal(err)
	}
	location, err := reopened.StorageLocation()
	if err != nil || location.Pending != "" {
		t.Fatal("cancel failed", err)
	}
	if err = store.ValidateStorageDirectory(filepath.Join(base, "child")); err == nil {
		t.Fatal("nested target allowed")
	}
	if err = store.ValidateStorageDirectory("relative"); err == nil {
		t.Fatal("relative target allowed")
	}
}

func TestStorageLocationEmptyDestinationAndSecondMove(t *testing.T) {
	base := t.TempDir()
	t.Setenv("OSHELL_DATA_DIR", base)
	store, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		target := t.TempDir()
		if err = store.ScheduleStorageDirectory(target); err != nil {
			t.Fatal(err)
		}
		store, err = OpenStore()
		if err != nil || store.StorageWarning != "" {
			t.Fatalf("move failed %v %s", err, store.StorageWarning)
		}
		if _, err = store.Load(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStorageLocationRejectsSymlinkWithoutSwitching(t *testing.T) {
	base := t.TempDir()
	t.Setenv("OSHELL_DATA_DIR", base)
	store, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external")
	if err = os.WriteFile(external, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(external, filepath.Join(base, "linked-file")); err != nil {
		t.Skip(err)
	}
	target := filepath.Join(t.TempDir(), "new-data")
	if err = store.ScheduleStorageDirectory(target); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore()
	if err != nil || reopened.Dir != store.Dir || reopened.StorageWarning == "" {
		t.Fatal("symlink migration did not retain original", err)
	}
	if _, err = os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("partial target published")
	}
}

func TestStorageLocationKeepsMasterProtectedDataLocked(t *testing.T) {
	t.Setenv("OSHELL_DATA_DIR", t.TempDir())
	store, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "protected.invalid"
	cfg.Profiles = []Profile{p}
	if err = store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	revision, err := store.Revision()
	if err != nil {
		t.Fatal(err)
	}
	master := []byte("fixture-storage-master")
	if err = store.ProtectConfiguration(cfg, master, revision); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster(string(master)); err != nil {
		t.Fatal(err)
	}
	if err = vault.Put(p, "fixture-protected-secret"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "protected")
	if err = store.ScheduleStorageDirectory(target); err != nil {
		t.Fatal(err)
	}
	migrated, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = migrated.Load(); !errors.Is(err, ErrConfigurationLocked) {
		t.Fatal("migration bypassed master lock", err)
	}
	if _, err = migrated.UnlockConfiguration(master); err != nil {
		t.Fatal(err)
	}
	moved, err := OpenVault(migrated.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = moved.Unlock(string(master)); err != nil {
		t.Fatal(err)
	}
	password, err := moved.Get(p)
	if err != nil || password != "fixture-protected-secret" {
		t.Fatal("protected credential did not survive", err)
	}
}
