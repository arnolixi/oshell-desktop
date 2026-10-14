package core

import (
	"reflect"
	"testing"
)

func moveFixture() Config {
	cfg := DefaultConfig()
	cfg.Directories = []string{"Source", "Source/Child", "Target", "Links/Saved"}
	p := NewProfile()
	p.Name = "first"
	p.Host = "first.invalid"
	p.Group = "Source/Child"
	q := NewProfile()
	q.Name = "second"
	q.Host = "second.invalid"
	q.Group = "Source"
	cfg.Profiles = []Profile{p, q}
	return cfg
}
func TestSessionMoveBatchAndReferenceCopy(t *testing.T) {
	t.Run("move subtree with contained selection", func(t *testing.T) {
		cfg := moveFixture()
		id := cfg.Profiles[0].ID
		if err := cfg.MoveSessionItems([]string{id}, []string{"Source/Child"}, nil, "Target"); err != nil {
			t.Fatal(err)
		}
		if cfg.Profiles[0].ID != id || cfg.Profiles[0].Group != "Target/Child" || cfg.Profiles[1].Group != "Source" {
			t.Fatal("subtree/profile move mismatch")
		}
	})
	t.Run("copy tree into Links", func(t *testing.T) {
		cfg := moveFixture()
		old := cfg.Clone()
		if err := cfg.MoveSessionItems(nil, []string{"Source"}, nil, "Links"); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cfg.Profiles, old.Profiles) || len(cfg.Links) != 2 {
			t.Fatal("reference drop changed source profiles")
		}
		folders := map[string]bool{}
		for _, l := range cfg.Links {
			folders[l.Folder] = true
		}
		if !folders["Source"] || !folders["Source/Child"] {
			t.Fatal("reference hierarchy lost")
		}
	})
	t.Run("single profile reference and reference move", func(t *testing.T) {
		cfg := moveFixture()
		id := cfg.Profiles[0].ID
		if err := cfg.MoveSessionItems([]string{id}, nil, nil, "Links"); err != nil {
			t.Fatal(err)
		}
		if cfg.Profiles[0].Group != "Source/Child" || len(cfg.Links) != 1 {
			t.Fatal("profile moved instead of referenced")
		}
		if err := cfg.MoveSessionItems(nil, nil, []string{cfg.Links[0].ID}, "Links/Saved"); err != nil {
			t.Fatal(err)
		}
		if cfg.Links[0].Folder != "Saved" {
			t.Fatal("reference not moved")
		}
	})
}
func TestSessionMoveRejectsInvalidBatchWithoutMutation(t *testing.T) {
	for _, mode := range []string{"descendant", "collision", "missing", "links-root", "reference-outside"} {
		t.Run(mode, func(t *testing.T) {
			cfg := moveFixture()
			var ids, dirs, links []string
			dest := "Target"
			switch mode {
			case "descendant":
				dirs = []string{"Source"}
				dest = "Source/Child"
			case "collision":
				cfg.Directories = append(cfg.Directories, "Target/Child")
				ids = []string{cfg.Profiles[1].ID}
				dirs = []string{"Source/Child"}
			case "missing":
				ids = []string{"missing"}
			case "links-root":
				dirs = []string{"Links"}
			case "reference-outside":
				if err := cfg.AddLink(cfg.Profiles[0], ""); err != nil {
					t.Fatal(err)
				}
				links = []string{cfg.Links[0].ID}
			}
			before := cfg.Clone()
			if err := cfg.MoveSessionItems(ids, dirs, links, dest); err == nil {
				t.Fatal("invalid batch accepted")
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("rejected batch partially changed config")
			}
		})
	}
}
