package core

import (
	"slices"
	"testing"
)

func TestSessionDeletionCollapsesSubtreesAndCountsDependents(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Directories = []string{"root/child/empty", "outside", "Links/keep"}
	nested := NewProfile()
	nested.Group = "root/child"
	nested.Host = "nested.invalid"
	outside := NewProfile()
	outside.Group = "outside"
	outside.Host = "outside.invalid"
	cfg.Profiles = []Profile{nested, outside}
	if err := cfg.AddLink(nested, "keep"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddLink(outside, "keep"); err != nil {
		t.Fatal(err)
	}
	next, summary, err := cfg.PlanSessionDeletion([]string{nested.ID}, []string{"root", "root/child"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Profiles != 1 || summary.Directories != 3 || summary.Links != 1 || len(next.Profiles) != 1 || next.Profiles[0].ID != outside.ID {
		t.Fatal("wrong affected scope", summary)
	}
	if !slices.Contains(next.AllDirectories(), "Links/keep") {
		t.Fatal("reference cleanup removed containing folder")
	}
	alias := cfg.Links[1]
	next, summary, err = cfg.PlanSessionDeletion(nil, nil, []string{alias.ID})
	if err != nil || len(next.Profiles) != 2 || summary.Profiles != 0 || summary.Links != 1 {
		t.Fatal("alias deletion removed source", err)
	}
	if _, _, err = cfg.PlanSessionDeletion(nil, []string{"Links"}, nil); err == nil {
		t.Fatal("reserved root accepted")
	}
	if len(cfg.Profiles) != 2 || len(cfg.Links) != 2 {
		t.Fatal("planning changed original")
	}
}
