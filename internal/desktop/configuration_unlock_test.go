package desktop

import (
	"bytes"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
)

func TestEncryptedStartupUnlockLoadsConfigurationAndRelocks(t *testing.T) {
	dir := t.TempDir()
	store := &core.Store{Dir: dir}
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "startup-private.invalid"
	cfg.Profiles = []core.Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	master := "fixture-master-password"
	vault, err := core.OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster(master); err != nil {
		t.Fatal(err)
	}
	revision, _ := store.Revision()
	if err = store.ProtectConfiguration(cfg, []byte(master), revision); err != nil {
		t.Fatal(err)
	}
	protected, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	reopened := &core.Store{Dir: dir}
	initial, err := startupConfiguration(reopened)
	if err != nil || len(initial.Profiles) != 0 {
		t.Fatal("startup exposed config", err)
	}
	a := newApp(reopened, initial)
	a.features.startupUnlock = true
	a.features.vaultAction = "unlock"
	a.features.master = "wrong"
	a.runVaultAction()
	waitOperator(t, a, func() bool { return !a.features.vaultBusy })
	if !a.passwordLocked() || a.features.vaultError == "" || len(a.config.Profiles) != 0 {
		t.Fatal("wrong password exposed configuration")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !bytes.Equal(after, protected) {
		t.Fatal("failed unlock wrote config")
	}
	a.features.master = master
	a.runVaultAction()
	waitOperator(t, a, func() bool { return !a.features.vaultBusy })
	if a.passwordLocked() || len(a.config.Profiles) != 1 || a.config.Profiles[0].Host != p.Host || a.features.startupUnlock {
		t.Fatal("configuration not loaded after unlock", a.features.vaultError)
	}
	a.lockWorkspaceSecrets()
	if !a.passwordLocked() {
		t.Fatal("workspace did not relock")
	}
	if _, err = reopened.Load(); err == nil {
		t.Fatal("store retained unlocked key")
	}
}

func TestLegacyMasterUnlockProtectsExistingPlainConfig(t *testing.T) {
	dir := t.TempDir()
	store := &core.Store{Dir: dir}
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "legacy-private.invalid"
	cfg.Profiles = []core.Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := core.OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster("legacy-master-password"); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.features.vaultAction = "unlock"
	a.features.master = "legacy-master-password"
	a.runVaultAction()
	waitOperator(t, a, func() bool { return !a.features.vaultBusy })
	if a.features.vaultError != "" {
		t.Fatal(a.features.vaultError)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !core.IsSharedVault(raw) || bytes.Contains(raw, []byte(p.Host)) {
		t.Fatal("legacy config not protected after unlock")
	}
	if loaded, err := store.Load(); err != nil || loaded.Profiles[0].ID != p.ID {
		t.Fatal("legacy upgrade changed config", err)
	}
}
