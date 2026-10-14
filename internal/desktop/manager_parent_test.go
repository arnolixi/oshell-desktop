package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"reflect"
	"testing"
)

func TestManagerParentRowNavigationAndMutationGuards(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	p.Group = "outer/child"
	cfg.Profiles = []core.Profile{p}
	store := &core.Store{Dir: t.TempDir()}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.managerGroup = "outer/child"
	rows := a.managerRows()
	if len(rows) < 2 || !rows[0].parent || rows[0].name != "../" || rows[0].group != "outer" {
		t.Fatal("parent row missing")
	}
	parent := rows[0]
	tt := ui.NewTester(a.managerView, 900, 580)
	if err := tt.RightClick("../"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tt.Menu(), []string{"上一级"}) {
		t.Fatal("parent menu exposes mutation", tt.Menu())
	}
	tt.CloseMenu()
	a.managerSelection.Add(p.ID)
	if a.managerSelectionMutable() || a.managerConnectCount() != 1 {
		t.Fatal("mixed parent selection guard/count wrong")
	}
	a.exportManagerSelection()
	if a.features.archive.open {
		t.Fatal("parent exported unintended scope")
	}
	a.openDirectory("移动所选会话", "outer/child")
	a.features.directoryName = ""
	a.applyDirectory()
	if !a.features.directoryOpen || a.config.Profiles[0].Group != p.Group {
		t.Fatal("parent allowed mutation")
	}
	a.features.directoryOpen = false
	a.managerSelection.Clear()
	a.enterRow(parent)
	if a.managerGroup != "outer" || a.managerRow != -1 {
		t.Fatal("parent navigation failed")
	}
}
func TestManagerParentRowAcceptsDropToParent(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	p.Group = "outer/child"
	cfg.Profiles = []core.Profile{p}
	store := &core.Store{Dir: t.TempDir()}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.managerGroup = p.Group
	rows := a.managerRows()
	var source sessionRow
	for _, row := range rows {
		if row.profile.ID == p.ID {
			source = row
		}
	}
	a.moveManagerRows(managerDrag{owner: a, rows: []sessionRow{source}}, rows[0].group)
	if a.config.Profiles[0].Group != "outer" {
		t.Fatal("parent drop failed", a.status)
	}
}
