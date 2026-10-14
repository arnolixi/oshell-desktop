package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestNewGroupMovesCapturedTabAndKeepsConnection(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	operatorPane(t, a, "one")
	operatorPane(t, a, "two")
	a.ensureGroups()
	moving := a.tabs[0]
	term := moving.active.term
	a.promptGroupWithTab(moving)
	a.selectTab(a.tabs[1])
	a.groups.name = "production"
	tt := ui.NewTester(a.groupsDialog, 700, 400)
	if err := tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	group := a.groupByID(moving.group)
	if a.groups.dialog || group == nil || group.name != "production" || moving.active.term != term || moving.active.closed || a.tabs[1].group == group.id {
		t.Fatal("wrong tab or connection changed")
	}
	a.promptGroupWithTab(moving)
	a.groups.name = "should-not-exist"
	a.tabs = a.tabs[1:]
	count := len(a.groups.root.leaves())
	a.confirmGroupDialog()
	if !a.groups.dialog || a.groups.error == "" || len(a.groups.root.leaves()) != count {
		t.Fatal("missing captured tab created group")
	}
}
