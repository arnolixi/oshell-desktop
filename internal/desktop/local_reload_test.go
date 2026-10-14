package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestLocalRefreshUpdatesWindowsWithoutReplacingTerminal(t *testing.T) {
	s := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(s, cfg)
	other := newApp(s, cfg)
	files := newApp(s, cfg)
	a.hub = &applicationHub{store: s, config: cfg, vault: a.vault, windows: []*App{a, other}, fileWindow: files}
	other.hub = a.hub
	other.vault = a.vault
	pane, _ := operatorPane(t, a, "live")
	term := pane.term
	disk := &core.Store{Dir: s.Dir}
	remote, err := disk.Load()
	if err != nil {
		t.Fatal(err)
	}
	remote.Settings.FontSize = 18
	if err := disk.Save(remote); err != nil {
		t.Fatal(err)
	}
	a.refreshLocalSharedData()
	waitOperator(t, a, func() bool { return !a.configurationBusy() })
	if a.config.Settings.FontSize != 18 || other.config.Settings.FontSize != 18 || a.vault != other.vault || files.config.Settings.FontSize != 18 || files.vault != a.vault {
		t.Fatal("refresh not shared", a.status)
	}
	if pane.closed || pane.term != term {
		t.Fatal("refresh replaced terminal")
	}
}
func TestLocalRefreshDoesNotDiscardOpenSettingsDraft(t *testing.T) {
	s := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(s, cfg)
	a.prepareSettings()
	a.features.settingsDraft.FontSize = 22
	a.refreshLocalSharedData()
	if a.configurationBusy() || !a.settingsOpen || a.features.settingsDraft.FontSize != 22 {
		t.Fatal("refresh disturbed editing")
	}
}
