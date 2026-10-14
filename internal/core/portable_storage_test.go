package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPortableDirectoryUsesExecutableLocation(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "OShell Desktop.exe")
	if err := os.WriteFile(filepath.Join(root, PortableMarker), nil, 0600); err != nil {
		t.Fatal(err)
	}
	directory, portable, err := portableStorageDirectory("windows", exe)
	if err != nil || !portable || directory != root {
		t.Fatal("portable root not executable directory", directory, err)
	}
	if _, portable, err = portableStorageDirectory("darwin", exe); err != nil || portable {
		t.Fatal("portable marker changed macOS behavior")
	}
	if _, portable, err = portableStorageDirectory("windows", filepath.Join(t.TempDir(), "OShell Desktop.exe")); err != nil || portable {
		t.Fatal("ordinary build unexpectedly portable")
	}
}
func TestPortableDataMovesWithFolderAndIgnoresOldLocator(t *testing.T) {
	parent := t.TempDir()
	before := filepath.Join(parent, "before")
	after := filepath.Join(parent, "after")
	if err := os.Mkdir(before, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(before, PortableMarker), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeStorageLocation(before, StorageLocation{Active: filepath.Join(parent, "missing-old-location")}); err != nil {
		t.Fatal(err)
	}
	store, err := openStoreDirectory(before, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Name = "portable fixture"
	p.Host = "fixture.invalid"
	cfg.Profiles = []Profile{p}
	if err = store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(before)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.Put(p, "fixture-secret"); err != nil {
		t.Fatal(err)
	}
	vault.Lock()
	if err = store.ScheduleStorageDirectory(filepath.Join(parent, "elsewhere")); err == nil {
		t.Fatal("portable store allowed relocation away from executable")
	}
	if err = os.Rename(before, after); err != nil {
		t.Fatal(err)
	}
	moved, err := openStoreDirectory(after, true)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := moved.Load()
	if err != nil || loaded.Profiles[0].Name != "portable fixture" {
		t.Fatal("moved configuration lost", err)
	}
	reopened, err := OpenVault(after)
	if err != nil {
		t.Fatal(err)
	}
	password, err := reopened.Get(p)
	if err != nil || password != "fixture-secret" {
		t.Fatal("moved credentials lost", err)
	}
	location, err := moved.StorageLocation()
	if err != nil || location.Active != after || location.Pending != "" {
		t.Fatal("old absolute locator affected portable store", location, err)
	}
	for _, name := range []string{"config.json", "credentials.json", "credential-key"} {
		if _, err = os.Stat(filepath.Join(after, name)); err != nil {
			t.Fatal("data not next to executable", name, err)
		}
	}
}
func TestCredentialKeyPublicationNeverOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	source, dest := filepath.Join(dir, "incoming"), filepath.Join(dir, "credential-key")
	if err := os.WriteFile(source, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishLocalCredentialKey(source, dest); err == nil {
		t.Fatal("existing key overwritten")
	}
	content, err := os.ReadFile(dest)
	if err != nil || string(content) != "existing" {
		t.Fatal("key was changed", err)
	}
}
