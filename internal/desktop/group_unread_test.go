package desktop

import (
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func TestHiddenGroupUnreadPersistsUntilTabIsShown(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	operatorPane(t, a, "visible")
	hiddenPane, _ := operatorPane(t, a, "hidden")
	a.ensureGroups()
	group, err := a.createGroup("background")
	if err != nil {
		t.Fatal(err)
	}
	hiddenTab := a.tabs[1]
	a.moveToGroup(hiddenTab, group)
	a.hideGroup(group, true)
	hiddenPane.pendingUnread.Store(true)
	a.refreshPaneVisibility()
	if a.hiddenUnreadGroups() != 1 || !strings.Contains(a.groupMenuTitle(group), "●") || !strings.Contains(a.groupMenuTitle(group), "隐藏") {
		t.Fatal("hidden output not discoverable")
	}
	_ = a.groupMenuTitle(group)
	if !hiddenTab.unread {
		t.Fatal("reading label cleared unread")
	}
	a.selectTab(hiddenTab)
	a.refreshPaneVisibility()
	if group.hidden || a.hiddenUnreadGroups() != 0 || hiddenTab.unread || !hiddenPane.term.Available() {
		t.Fatal("showing group failed to acknowledge without disconnect")
	}
}
