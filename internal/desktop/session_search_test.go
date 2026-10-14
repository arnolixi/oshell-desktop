package desktop

import (
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func TestSessionSearchSharedByManagerAndSwitcher(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name = "Café ＰＲＯＤ"
	p.Host = "server.invalid"
	p.Group = "生产"
	p.Username = "operator"
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.query = "CAFE prod /生产"
	a.features.switcher.query = a.query
	rows, entries := a.managerRows(), a.switchEntries()
	if len(rows) != 1 || rows[0].profile.ID != p.ID || len(entries) != 1 || entries[0].Profile.ID != p.ID {
		t.Fatal("normalization differs")
	}
	a.query = strings.Repeat("x", 4097)
	a.features.switcher.query = a.query
	if len(a.managerRows()) != 0 || len(a.switchEntries()) != 0 {
		t.Fatal("invalid query returned sessions")
	}
}
