package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestSessionDirectoryTreeChoosesParentsAndRootWithoutSaving(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{"production/database", "production/cache", "testing"}
	profile := core.NewProfile()
	profile.Group = "production/database"
	cfg.Profiles = []core.Profile{profile}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.edit(profile)
	tt := ui.NewTester(a.sessionDirectoryField, 600, 400)
	if err := tt.Click("选择会话目录"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("会话目录 /production/cache"); err != nil {
		t.Fatal(err)
	}
	if a.draft.Group != "production/cache" || a.config.Profiles[0].Group != "production/database" {
		t.Fatal("selection did not stay in draft")
	}
	if err := tt.Click("选择会话目录"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("会话目录 /production"); err != nil {
		t.Fatal(err)
	}
	if a.draft.Group != "production" {
		t.Fatal("parent selection failed")
	}
	if err := tt.Click("选择会话目录"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("会话目录 /"); err != nil {
		t.Fatal(err)
	}
	if a.draft.Group != "" {
		t.Fatal("root selection failed")
	}
}

func TestSessionDirectoryPopoverKeepsFormGeometry(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.config.Directories = []string{"team/servers"}
	a.draft.Group = "team/servers"
	var host string
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Gap(8).Children(func() {
			a.sessionDirectoryField(c)
			ui.TextInput(c, &host).Label("下方主机字段")
		})
	}, 600, 400)
	before, ok := tt.Find("下方主机字段")
	if !ok {
		t.Fatal("missing host field")
	}
	if err := tt.Click("选择会话目录"); err != nil {
		t.Fatal(err)
	}
	after, _ := tt.Find("下方主机字段")
	if before != after {
		t.Fatalf("dropdown moved form: before=%+v after=%+v", before, after)
	}
	snapshot(t, tt, "directory-overlay.png")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.features.directoryPicker.shown {
		t.Fatal("Escape did not close popover")
	}
	if err := tt.Click("选择会话目录"); err != nil {
		t.Fatal(err)
	}
	tt.ClickAt(590, 390)
	tt.Frame()
	if a.features.directoryPicker.shown {
		t.Fatal("outside click did not close popover")
	}
	if err := tt.Click("选择会话目录"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("会话目录 /team"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.features.directoryPicker.shown || a.draft.Group != "team" {
		t.Fatal("selection did not apply and dismiss")
	}
	after, _ = tt.Find("下方主机字段")
	if before != after {
		t.Fatal("selection moved form")
	}
}
