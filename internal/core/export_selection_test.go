package core

import (
	"context"
	"slices"
	"testing"
)

func TestExportSelectionRetainsReferencesSourcesAndEmptyFolders(t *testing.T) {
	cfg := DefaultConfig()
	for _, name := range []string{"selected", "source", "unrelated"} {
		p := NewProfile()
		p.Name = name
		p.Host = name + ".invalid"
		p.Group = name
		cfg.Profiles = append(cfg.Profiles, p)
	}
	cfg.Directories = []string{"selected/empty", "Links/team/empty"}
	if err := cfg.AddLink(cfg.Profiles[1], "team"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddLink(cfg.Profiles[2], "other"); err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ExportSelection(nil, []string{"selected", "Links/team"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Profiles) != 2 || len(selected.Links) != 1 || selected.Links[0].ProfileID != cfg.Profiles[1].ID {
		t.Fatal("scope or source dependency incorrect")
	}
	if !slices.Contains(selected.Directories, "selected/empty") || !slices.Contains(selected.Directories, "Links/team/empty") || slices.Contains(selected.Directories, "unrelated") {
		t.Fatal("empty directories or scope lost")
	}
	archive, err := (&Vault{}).PortableArchive(context.Background(), selected, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = archive.Encode(); err != nil {
		t.Fatal(err)
	}
	single, err := cfg.ExportSelection(nil, nil, []string{cfg.Links[0].ID})
	if err != nil || len(single.Profiles) != 1 || len(single.Links) != 1 {
		t.Fatal("single reference missing source", err)
	}
	plain, err := cfg.ExportSelection([]string{cfg.Profiles[1].ID}, nil, nil)
	if err != nil || len(plain.Profiles) != 1 || len(plain.Links) != 0 {
		t.Fatal("session export included unrelated references", err)
	}
	if _, err = cfg.ExportSelection([]string{NewID()}, nil, nil); err == nil {
		t.Fatal("missing source silently skipped")
	}
}
