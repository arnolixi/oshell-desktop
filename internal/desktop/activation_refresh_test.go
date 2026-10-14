package desktop

import (
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
)

func TestActivationRefreshAdoptsChangesAndSkipsUnchangedState(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	h := &applicationHub{store: store, config: cfg, vault: a.vault, windows: []*App{a}}
	a.hub = h
	old := a.vault
	h.refreshOnActivation()
	if a.configurationBusy() || a.vault != old {
		t.Fatal("unchanged activation reloaded state")
	}
	disk := &core.Store{Dir: store.Dir}
	next, err := disk.Load()
	if err != nil {
		t.Fatal(err)
	}
	next.Settings.FontSize = 18
	if err := disk.Save(next); err != nil {
		t.Fatal(err)
	}
	h.refreshOnActivation()
	waitOperator(t, a, func() bool { return !a.configurationBusy() })
	if a.config.Settings.FontSize != 18 || a.features.vaultOpen || a.features.localConflict.open {
		t.Fatal("activation failed or prompted", a.status)
	}
	replacement := a.vault
	h.refreshOnActivation()
	if a.configurationBusy() || a.vault != replacement {
		t.Fatal("repeated activation replaced unchanged vault")
	}
}
func TestActivationRefreshDoesNotPromptOrDiscardEdits(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	other := newApp(store, cfg)
	h := &applicationHub{store: store, config: cfg, vault: a.vault, windows: []*App{a, other}}
	a.hub = h
	other.hub = h
	other.vault = a.vault
	if err := a.vault.SetMaster("fixture-master"); err != nil {
		t.Fatal(err)
	}
	a.vault.Lock()
	h.refreshOnActivation()
	if a.features.vaultOpen || a.configurationBusy() {
		t.Fatal("activation prompted locked user")
	}
	if err := a.vault.Unlock("fixture-master"); err != nil {
		t.Fatal(err)
	}
	other.prepareSettings()
	other.features.settingsDraft.FontSize = 23
	h.settingsOwner = other
	h.refreshOnActivation()
	if a.configurationBusy() || other.features.settingsDraft.FontSize != 23 {
		t.Fatal("activation discarded other window draft")
	}
	other.closeSettings()
	if err := os.WriteFile(filepath.Join(store.Dir, "local-conflict-draft.json"), []byte("preserve corrupt draft"), 0600); err != nil {
		t.Fatal(err)
	}
	h.refreshOnActivation()
	if a.features.localConflict.open || a.configurationBusy() {
		t.Fatal("activation opened pending conflict")
	}
}
