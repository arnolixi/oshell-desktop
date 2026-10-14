package desktop

import (
	"oshell-desktop/internal/core"
	"slices"
	"testing"
)

func TestMenuMoveCapturesMixedSelectionAndMovesDirectoryTree(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{"source/child", "destination"}
	nested := core.NewProfile()
	nested.Group = "source/child"
	nested.Host = "nested.invalid"
	loose := core.NewProfile()
	loose.Group = ""
	loose.Host = "loose.invalid"
	cfg.Profiles = []core.Profile{nested, loose}
	store := &core.Store{Dir: t.TempDir()}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.managerSelection.Add("folder:source")
	a.managerSelection.Add(loose.ID)
	a.openDirectory("移动所选会话", "")
	a.managerSelection.Clear()
	a.features.directoryName = "destination"
	a.applyDirectory()
	if a.features.directoryOpen {
		t.Fatal(a.features.directoryError)
	}
	if a.config.Profiles[0].Group != "destination/source/child" || a.config.Profiles[1].Group != "destination" || !slices.Contains(a.config.AllDirectories(), "destination/source/child") {
		t.Fatal("mixed move incomplete")
	}
}
func TestMenuMoveDistinguishesReferenceFromActualSession(t *testing.T) {
	for _, actual := range []bool{false, true} {
		cfg := core.DefaultConfig()
		p := core.NewProfile()
		p.Group = ""
		p.Host = "fixture.invalid"
		cfg.Profiles = []core.Profile{p}
		store := &core.Store{Dir: t.TempDir()}
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		a := newApp(store, cfg)
		a.managerSelection.Add(p.ID)
		action := "移动所选会话"
		if actual {
			action = "移动实际会话"
		}
		a.openDirectory(action, "")
		a.features.directoryName = "Links"
		a.applyDirectory()
		if a.features.directoryOpen {
			t.Fatal(a.features.directoryError)
		}
		if actual {
			if a.config.Profiles[0].Group != "Links" || len(a.config.Links) != 0 {
				t.Fatal("actual move created reference")
			}
		} else if a.config.Profiles[0].Group != "" || len(a.config.Links) != 1 {
			t.Fatal("reference move changed source")
		}
	}
}
