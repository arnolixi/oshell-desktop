package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeWebDAVRestoresEncryptedCredentialsWithoutNetwork(t *testing.T) {
	source := t.TempDir()
	engine, cfg, master := syncEngineFixture(t, source)
	plan, err := engine.Prepare(context.Background(), cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(context.Background(), nil, master); err != nil {
		t.Fatal(err)
	}
	dav, err := SealShared([]byte(`{"address":"https://dav.example.invalid/oshell/","username":"fixture","password":"webdav-login-secret"}`), master)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(source, "webdav-settings.json"), dav, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadNativeStorage(source, master)
	if err != nil || snapshot.WebDAV == nil {
		t.Fatal("missing native WebDAV settings", err)
	}
	store := &Store{Dir: t.TempDir()}
	vault, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster("different-master-password"); err != nil {
		t.Fatal(err)
	}
	if err = store.RestoreNativeWebDAV(snapshot, master, vault); err == nil {
		t.Fatal("different destination master accepted")
	}
	options, _ := LoadSyncOptions(store.Dir)
	if options.Enabled {
		t.Fatal("failed restore enabled sync")
	}
	if err = vault.ChangeMaster("different-master-password", string(master)); err != nil {
		t.Fatal(err)
	}
	if err = store.RestoreNativeWebDAV(snapshot, master, vault); err != nil {
		t.Fatal(err)
	}
	options, err = LoadSyncOptions(store.Dir, master)
	if err != nil || !options.Enabled || options.Backend != "webdav" {
		t.Fatal("settings not enabled", err)
	}
	p := options.CredentialProfile()
	secret, err := vault.GetFor(p, p.Identity())
	if err != nil || secret != "webdav-login-secret" {
		t.Fatal("credential lost", err)
	}
	for _, name := range []string{"sync-settings.json", "credentials.json"} {
		data, err := os.ReadFile(filepath.Join(store.Dir, name))
		if err != nil || bytes.Contains(data, []byte("webdav-login-secret")) || bytes.Contains(data, []byte("dav.example.invalid")) {
			t.Fatal("plaintext WebDAV password persisted", name, err)
		}
	}
	after, _ := os.ReadFile(filepath.Join(source, "webdav-settings.json"))
	if !bytes.Equal(after, dav) {
		t.Fatal("source WebDAV settings changed")
	}
}
