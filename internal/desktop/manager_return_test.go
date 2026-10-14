package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestManagerSourceEditReturnsToReferenceSelection(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{"Links/moved"}
	p := core.NewProfile()
	p.Name = "source"
	p.Host = "fixture.invalid"
	cfg.Profiles = []core.Profile{p}
	if err := cfg.AddLink(p, "team"); err != nil {
		t.Fatal(err)
	}
	store := &core.Store{Dir: t.TempDir()}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.managerGroup = "Links/team"
	id := "link:" + cfg.Links[0].ID
	a.managerSelection.Add(id)
	a.editManagerProfile(p)
	a.draft.Name = "changed"
	a.saveDraft(false)
	if a.editing || a.managerGroup != "Links/team" || !a.managerSelection.Has(id) || a.features.managerReturn != nil {
		t.Fatal("reference location/selection lost", a.formError)
	}
	if a.config.Links[0].Name != "source" || a.config.Profiles[0].Group != p.Group {
		t.Fatal("source edit mutated reference or moved profile")
	}
	a.editManagerProfile(a.config.Profiles[0])
	a.draft.Host = "unsaved.invalid"
	a.editing = false
	a.restoreManagerReturn(nil)
	if !a.managerSelection.Has(id) || a.config.Profiles[0].Host != "fixture.invalid" {
		t.Fatal("cancel did not restore selection without saving")
	}
	a.editManagerLink(cfg.Links[0].ID)
	a.features.linkEditor.folder = "moved"
	a.applyLinkEditor()
	if a.features.linkEditor.open {
		t.Fatal(a.features.linkEditor.error)
	}
	a.restoreManagerReturn(nil)
	if a.managerGroup != "Links/moved" || !a.managerSelection.Has(id) {
		t.Fatal("moved link not revealed")
	}

}
func TestManagerNewProfileSaveRevealsMovedDirectory(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{"destination"}
	store := &core.Store{Dir: t.TempDir()}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	p := a.newProfile("ssh")
	a.editManagerProfile(p)
	a.draft.Host = "fixture.invalid"
	a.draft.Group = "destination"
	a.saveDraft(false)
	if a.editing || a.managerGroup != "destination" || !a.managerSelection.Has(p.ID) {
		t.Fatal("new profile not revealed", a.formError)
	}
}
