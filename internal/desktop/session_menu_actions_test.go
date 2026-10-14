package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestSessionMenuUsesCurrentRowAndRejectsDirectories(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name = "source"
	p.Host = "fixture.invalid"
	p.Group = ""
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.managerGroup = ""
	a.managerAll = false
	rows := a.managerRows()
	for i, r := range rows {
		if r.profile.ID == p.ID {
			a.managerRow = i
		}
	}
	selected, ok := a.sessionMenuProfile()
	if !ok || selected.ID != p.ID {
		t.Fatal("profile selection lost")
	}
	a.config.Profiles[0].Name = "latest"
	a.editSelectedSessionFromMenu()
	if !a.editing || a.draft.Name != "latest" {
		t.Fatal("edit used stale profile")
	}
	a.editing = false
	a.managerGroup = ""
	a.managerAll = false
	for i, r := range a.managerRows() {
		if r.folder {
			a.managerRow = i
			break
		}
	}
	if _, ok := a.sessionMenuProfile(); ok {
		t.Fatal("directory resolved to old selected profile")
	}
	a.managerRow = -1
	if _, ok := a.sessionMenuProfile(); ok {
		t.Fatal("no selection resolved a profile")
	}
}

func TestMenuConnectSelectsOneProfileInsteadOfBatch(t *testing.T) {
	cfg := core.DefaultConfig()
	for _, name := range []string{"first", "second"} {
		p := core.NewProfile()
		p.Name = name
		p.Kind = "local"
		p.Group = ""
		cfg.Profiles = append(cfg.Profiles, p)
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.managerAll = true
	rows := a.managerRows()
	for i, row := range rows {
		if !row.folder {
			a.managerSelection.Add(row.id)
			if row.profile.ID == cfg.Profiles[1].ID {
				a.managerRow = i
			}
		}
	}
	a.performAction("connectSelected")
	if len(a.tabs) != 1 || a.tabs[0].active.profile.ID != cfg.Profiles[1].ID || len(a.authQueue) != 0 {
		t.Fatal("native menu used batch selection")
	}
	pane := a.tabs[0].active
	waitOperator(t, a, func() bool { return pane.term != nil })
	a.closeTab(a.tabs[0])
	pane.disposal.Wait()
}
func TestMenuConnectDoesNotEnterSelectedDirectory(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.managerGroup = ""
	a.managerAll = false
	for i, row := range a.managerRows() {
		if row.folder {
			a.managerRow = i
			break
		}
	}
	a.performAction("connectSelected")
	if a.managerGroup != "" || len(a.tabs) != 0 {
		t.Fatal("native menu entered directory or connected stale profile")
	}
}
