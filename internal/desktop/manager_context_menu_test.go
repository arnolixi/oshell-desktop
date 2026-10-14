package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"slices"
	"testing"
)

func TestManagerContextSelectionAndEdit(t *testing.T) {
	cfg := core.DefaultConfig()
	for _, name := range []string{"first", "second"} {
		p := core.NewProfile()
		p.Group = ""
		p.Name = name
		p.Host = name + ".invalid"
		cfg.Profiles = append(cfg.Profiles, p)
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	tt := ui.NewTester(a.managerView, 900, 580)
	rows := a.managerRows()
	for _, r := range rows {
		if !r.folder {
			a.managerSelection.Add(r.id)
		}
	}
	if err := tt.RightClick("first"); err != nil {
		t.Fatal(err)
	}
	if a.managerSelection.Len() != 2 || !slices.Contains(tt.Menu(), "删除所选（2）…") {
		t.Fatal("context lost batch selection", tt.Menu())
	}
	tt.CloseMenu()
	a.managerSelection.Clear()
	a.managerRow = -1
	if err := tt.RightClick("second"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("属性…"); err != nil {
		t.Fatal(err)
	}
	if !a.editing || a.draft.Name != "second" {
		t.Fatal("wrong context target")
	}
}
func TestApplyDefaultKeepAliveChangesDraftOnly(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.SessionDefaults.KeepAlive.Interval = 47
	cfg.SessionDefaults.KeepAlive.IdleEnabled = true
	cfg.SessionDefaults.KeepAlive.IdleText = "echo ready"
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.edit(p)
	a.applyDefaultKeepAlive()
	if a.features.interval != "47" || !a.draft.KeepAlive.IdleEnabled || a.draft.KeepAlive.IdleText != "echo ready" {
		t.Fatal("default not applied")
	}
	if a.config.Profiles[0].KeepAlive != p.KeepAlive {
		t.Fatal("draft application saved prematurely")
	}
	a.draft.Kind = "sftp"
	a.applyDefaultKeepAlive()
	if a.draft.KeepAlive.IdleEnabled {
		t.Fatal("SFTP enabled shell input")
	}
}

func TestManagerBlankContextDoesNotExportOldSelection(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name = "fixture"
	p.Group = ""
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.managerSelection.Add(p.ID)
	tt := ui.NewTester(a.managerView, 900, 580)
	// A nonexistent row key has exactly the same clearing semantics as blank space.
	if _, valid := a.selectManagerContext(""); valid || a.managerSelection.Len() != 0 || a.managerRow != -1 {
		t.Fatal("blank space kept old selection")
	}
	if err := tt.RightClick("fixture"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tt.Menu(), "导出此会话…") || !slices.Contains(tt.Menu(), "导出全部会话…") {
		t.Fatal("selection and whole-catalog export must be separate", tt.Menu())
	}
	if slices.Contains(tt.Menu(), "链接属性…") || slices.Contains(tt.Menu(), "打开目录") {
		t.Fatal("session menu exposed unrelated operations", tt.Menu())
	}
}
