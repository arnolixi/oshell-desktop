package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestSendTargetPickerSeedsCurrentScopeWithoutApplying(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	first, _ := operatorPane(t, a, "first")
	second, _ := operatorPane(t, a, "second")
	a.current = 0
	a.features.operators.selected = map[string]bool{"removed": true}
	a.quickSendScope = "all"
	a.setSendScope("selected")
	o := &a.features.operators
	if !o.selectionDraft[first.id] || !o.selectionDraft[second.id] || len(o.selectionDraft) != 2 {
		t.Fatal("current scope not seeded")
	}
	if len(o.selected) != 0 || a.sendScope() != "all" {
		t.Fatal("opening picker applied draft")
	}
	o.targetsOpen = false
	o.selected = map[string]bool{second.id: true}
	a.setSendScope("selected")
	if len(o.selectionDraft) != 1 || !o.selectionDraft[second.id] {
		t.Fatal("prior explicit selection not retained")
	}
}
func TestSendGroupPickerSeedsCurrentAndRetainsHiddenGroup(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	operatorPane(t, a, "first")
	a.ensureGroups()
	initial := a.groups.active
	o := &a.features.operators
	o.selectedGroups = map[string]bool{"removed": true}
	a.setSendScope("selectedGroups")
	if len(o.groupSelectionDraft) != 1 || !o.groupSelectionDraft[initial] || len(o.selectedGroups) != 0 {
		t.Fatal("current group not seeded independently")
	}
	o.groupsOpen = false
	hidden, err := a.createGroup("hidden")
	if err != nil {
		t.Fatal(err)
	}
	a.hideGroup(hidden, true)
	o.selectedGroups = map[string]bool{hidden.id: true}
	a.setSendScope("selectedGroups")
	if len(o.groupSelectionDraft) != 1 || !o.groupSelectionDraft[hidden.id] {
		t.Fatal("hidden group selection lost")
	}
}
