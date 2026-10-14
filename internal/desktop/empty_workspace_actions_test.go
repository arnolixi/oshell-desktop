package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestSplitInEmptyWorkspaceCreatesLocalTerminal(t *testing.T) {
	for _, action := range []string{"splitVertical", "splitHorizontal"} {
		t.Run(action, func(t *testing.T) {
			a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
			a.performAction(action)
			if len(a.tabs) != 1 || a.tabs[0].active.profile.Kind != "local" || a.tabs[0].root.pane == nil {
				t.Fatal("empty split did not create local terminal")
			}
			p := a.tabs[0].active
			// New terminal launch is deferred until it is laid out; no shell needs to run.
			waitOperator(t, a, func() bool { return p.term != nil })
			a.closeTab(a.tabs[0])
			p.disposal.Wait()
		})
	}
}
func TestInvalidSplitDoesNotCreateTerminal(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.splitActivePane(0)
	if len(a.tabs) != 0 {
		t.Fatal("invalid split opened terminal")
	}
}
