package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestSharedSaveUpdatesFileWindowWithoutOverwritingDraftOrScope(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	cfg.Profiles = []core.Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	source := newApp(store, cfg)
	other := newApp(store, cfg)
	files := newApp(store, cfg)
	hub := &applicationHub{store: store, config: cfg, vault: source.vault, windows: []*App{source, other}, fileWindow: files}
	for _, a := range []*App{source, other, files} {
		a.hub = hub
		a.vault = source.vault
	}
	files.quickSendScope = "selected"
	files.prepareSettings()
	files.features.settingsDraft.FontSize = 23
	if !source.changeConfig(func(next *core.Config) error {
		next.Settings.FontSize = 18
		next.Profiles[0].Name = "changed"
		return nil
	}) {
		t.Fatal(source.status)
	}
	if files.config.Settings.FontSize != 18 || files.config.Profiles[0].Name != "changed" || other.config.Settings.FontSize != 18 {
		t.Fatal("shared save skipped file workspace")
	}
	if files.features.settingsDraft.FontSize != 23 || files.quickSendScope != "selected" {
		t.Fatal("publication replaced per-window state")
	}
	files.config.Profiles[0].Name = "uncommitted"
	if hub.config.Profiles[0].Name != "changed" || source.config.Profiles[0].Name != "changed" {
		t.Fatal("workspace config shares mutable storage")
	}
}
func TestConfigurationWindowSnapshotIncludesFilesOnlyOnce(t *testing.T) {
	first, files := &App{}, &App{}
	hub := &applicationHub{windows: []*App{first, files}, fileWindow: files}
	snapshot := hub.configurationWindows()
	if len(snapshot) != 2 {
		t.Fatal("duplicate file workspace")
	}
	hub.windows[0] = files
	if snapshot[0] != first {
		t.Fatal("returned live list instead of snapshot")
	}
}
