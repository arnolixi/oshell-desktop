package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestArrangementsPreserveNamedHiddenAndEmptyGroups(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	for _, name := range []string{"one", "two", "three"} {
		operatorPane(t, a, name)
	}
	a.ensureGroups()
	named, err := a.createGroup("production")
	if err != nil {
		t.Fatal(err)
	}
	a.moveToGroup(a.tabs[1], named)
	a.hideGroup(named, true)
	empty, err := a.createGroup("empty")
	if err != nil {
		t.Fatal(err)
	}
	a.hideGroup(empty, true)
	owners := map[*tab]string{}
	for _, tab := range a.tabs {
		owners[tab] = tab.group
	}
	for _, mode := range []string{"horizontal", "vertical", "tile"} {
		a.arrangeTabs(mode)
		if a.groupByID(named.id) != named || !named.hidden || a.groupByID(empty.id) != empty || !empty.hidden {
			t.Fatal("lost named/hidden/empty group", mode)
		}
		for tab, group := range owners {
			if tab.group != group || tab.active.closed || !tab.active.term.Available() {
				t.Fatal("changed membership or connection", mode)
			}
		}
	}
	a.arrangeTabs("tabs")
	if len(a.groups.root.leaves()) != 1 || a.groups.root.group.name != "" {
		t.Fatal("explicit merge retained groups")
	}
}
func TestContextSplitMovesExistingTabWithoutReconnect(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	operatorPane(t, a, "one")
	operatorPane(t, a, "two")
	a.ensureGroups()
	a.selectTab(a.tabs[0])
	moving := a.tabs[1]
	term := moving.active.term
	target := a.tabSplitTarget(moving)
	if target == nil {
		t.Fatal("split unavailable")
	}
	a.dropTab(moving, target, "right")
	if len(a.groups.root.leaves()) != 2 || moving.active.term != term || moving.active.closed {
		t.Fatal("split replaced session")
	}
	if a.tabSplitTarget(moving) != nil {
		t.Fatal("single current tab offered empty split")
	}
}
