package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestExistingLinkPickerPreservesSourceAndRejectsDuplicate(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name = "Production SSH"
	p.Host = "fixture.invalid"
	p.Username = "tester"
	cfg.Profiles = []core.Profile{p}
	cfg.Directories = []string{"Links/Operations"}
	store := &core.Store{Dir: t.TempDir()}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.addExistingLink("Operations")
	a.features.linkEditor.query = "production tester"
	tt := ui.NewTester(a.linkEditorDialog, 700, 600)
	if err := tt.Click("Production SSH"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("添加链接"); err != nil {
		t.Fatal(err)
	}
	if a.features.linkEditor.open || len(a.config.Links) != 1 || a.config.Links[0].Folder != "Operations" || a.config.Profiles[0].Group != p.Group {
		t.Fatal("link not created independently", a.status)
	}
	a.addExistingLink("Operations")
	a.features.linkEditor.id = p.ID
	a.savePickedLink()
	if !a.features.linkEditor.open || a.features.linkEditor.error == "" || len(a.config.Links) != 1 {
		t.Fatal("duplicate not rejected")
	}
	a.addExistingLink("")
	a.features.linkEditor.id = p.ID
	a.config.Profiles = nil
	a.savePickedLink()
	if !a.features.linkEditor.open || a.features.linkEditor.error == "" {
		t.Fatal("deleted source restored")
	}
}
