package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestManagerProtocolFilterAndConnectionCount(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{"empty"}
	for _, kind := range []string{"ssh", "sftp", "ftp", "local"} {
		p := core.NewProfile()
		p.Kind = kind
		p.Group = ""
		p.Name = kind
		p.Host = "fixture.invalid"
		p.Username = "tester"
		cfg.Profiles = append(cfg.Profiles, p)
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	for _, p := range cfg.Profiles {
		a.managerSelection.Add(p.ID)
	}
	a.managerSelection.Add("folder:empty")
	if a.managerConnectCount() != 4 {
		t.Fatal("directory counted as a connection")
	}
	a.managerProtocol = "sftp"
	tt := ui.NewTester(a.managerView, 900, 580)
	if a.managerConnectCount() != 1 || a.managerSelection.Len() != 2 || !a.managerSelection.Has(cfg.Profiles[1].ID) || !a.managerSelection.Has("folder:empty") {
		t.Fatal("filter failed to retain only visible selection")
	}
	rows := a.managerRows()
	for _, row := range rows {
		if !row.folder && row.profile.Kind != "sftp" {
			t.Fatal("protocol filter failed")
		}
	}
	if err := tt.Click("sftp"); err != nil {
		t.Fatal(err)
	}
	if a.managerConnectCount() != 1 {
		t.Fatal("filtered profile cannot connect")
	}
	a.query = "missing"
	if a.managerConnectCount() != 0 {
		t.Fatal("search retained hidden connection target")
	}
}
