package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestManagerExportCapturesSelectedScope(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Group = ""
	p.Host = "chosen.invalid"
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.managerSelection.Add(p.ID)
	a.exportManagerSelection()
	selected := a.features.archive.exportConfig
	if selected == nil || len(selected.Profiles) != 1 || !a.features.archive.open {
		t.Fatal("scope not captured")
	}
	a.config.Profiles[0].Host = "changed.invalid"
	if selected.Profiles[0].Host != "chosen.invalid" {
		t.Fatal("export snapshot changed")
	}
	a.exportProfiles()
	if a.features.archive.exportConfig != nil {
		t.Fatal("export all retained old selection")
	}
}
