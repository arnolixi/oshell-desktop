package desktop

import (
	"encoding/json"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
)

func TestLegacyWebDAVOptionsUpgradeOnUnlockedRead(t *testing.T) {
	dir := t.TempDir()
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "https://fixture.invalid/", Username: "fixture"}
	legacy, _ := json.Marshal(options)
	if err := os.WriteFile(filepath.Join(dir, "sync-settings.json"), legacy, 0600); err != nil {
		t.Fatal(err)
	}
	a := newApp(&core.Store{Dir: dir}, core.DefaultConfig())
	if err := a.vault.SetMaster("fixture-master"); err != nil {
		t.Fatal(err)
	}
	got, err := a.loadSyncOptions()
	if err != nil || got != options {
		t.Fatal("upgrade changed options", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "sync-settings.json"))
	if err != nil || !core.IsSharedVault(raw) {
		t.Fatal("not upgraded", err)
	}
	a.vault.Lock()
	if _, err := a.loadSyncOptions(); err == nil {
		t.Fatal("locked endpoint readable")
	}
	if err := a.vault.Unlock("fixture-master"); err != nil {
		t.Fatal(err)
	}
	got, err = a.loadSyncOptions()
	if err != nil || got != options {
		t.Fatal("reopen failed", err)
	}
}

func TestDisableWebDAVRemovesSavedServicePassword(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	if err := a.vault.SetMaster("fixture-master"); err != nil {
		t.Fatal(err)
	}
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "https://fixture.invalid/", Username: "fixture"}
	p := options.CredentialProfile()
	if err := a.vault.PutFor(p, "fixture-secret", p.Identity()); err != nil {
		t.Fatal(err)
	}
	if err := a.saveSyncOptions(options); err != nil {
		t.Fatal(err)
	}
	options.Enabled = false
	if err := a.saveSyncOptions(options); err != nil {
		t.Fatal(err)
	}
	if a.vault.HasStoredCredential(p.ID) {
		t.Fatal("disabled WebDAV kept service password")
	}
}
