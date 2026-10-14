package desktop

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func conflictViewFixture(t *testing.T, count int) (*App, *ui.Tester) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	a.features.settingsTab = 3
	f := &a.features.sharedSync
	f.plan = &core.SharedSyncPlan{}
	f.conflictOpen = true
	f.choices = map[string]core.SharedMergeChoice{}
	for i := 0; i < count; i++ {
		f.plan.Conflicts = append(f.plan.Conflicts, core.SharedMergeConflict{Key: fmt.Sprint(i), Title: fmt.Sprintf("冲突 %04d", i), LocalDescription: fmt.Sprintf("LOCAL-%d\n主机：local.example.invalid\n密码：已隐藏", i), RemoteDescription: fmt.Sprintf("REMOTE-%d\n主机：remote.example.invalid\n密码：已隐藏", i)})
	}
	return a, ui.NewTester(a.settingsView, 1000, 700)
}
func TestSharedConflictComparisonSelectionAndCompletion(t *testing.T) {
	a, tt := conflictViewFixture(t, 3)
	f := &a.features.sharedSync
	if err := tt.Click("保留本机版本"); err != nil {
		t.Fatal(err)
	}
	if f.choices["0"] != core.SharedKeepLocal || f.conflictIndex != 1 || a.sharedConflictChoiceCount() != 1 {
		t.Fatal("choice did not advance to next unresolved item")
	}
	if err := tt.Click("保留共享版本"); err != nil {
		t.Fatal(err)
	}
	if f.choices["1"] != core.SharedKeepRemote || f.conflictIndex != 2 {
		t.Fatal("remote choice applied to wrong item")
	}
	if err := tt.Click("保留本机版本"); err != nil {
		t.Fatal(err)
	}
	if a.sharedConflictChoiceCount() != 3 || !tt.HasText("已选择 3 / 3 项") {
		t.Fatal("progress did not complete")
	}
	a.chooseSharedConflict("missing", core.SharedKeepLocal)
	a.chooseSharedConflict("1", core.SharedMergeChoice("invalid"))
	if len(f.choices) != 3 || f.choices["1"] != core.SharedKeepRemote {
		t.Fatal("accepted stale/invalid choice")
	}
	snapshot(t, tt, "shared-conflict-comparison.png")
}
func TestSharedConflictComparisonVirtualizesLargeListsAndKeyboardSelection(t *testing.T) {
	a, tt := conflictViewFixture(t, 1000)
	if len(tt.Texts()) > 200 {
		t.Fatalf("rendered too many conflict labels: %d", len(tt.Texts()))
	}
	if err := tt.Click("[待选] 冲突 0000"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyDown)
	if a.features.sharedSync.conflictIndex != 1 {
		t.Fatal("list keyboard selection failed")
	}
	a.features.sharedSync.conflictIndex = 999
	a.features.sharedSync.conflictList.ScrollTo(999, ui.Center)
	tt.Frame()
	if !tt.HasText("[待选] 冲突 0999") {
		t.Fatal("last virtualized row not reachable")
	}
	if len(tt.Texts()) > 200 {
		t.Fatal("scrolling expanded entire list")
	}
	a.features.sharedSync.choices["unrelated"] = core.SharedKeepLocal
	if a.sharedConflictChoiceCount() != 0 {
		t.Fatal("unrelated choice counted toward confirmation")
	}
	snapshot(t, tt, "shared-conflict-large-list.png")
}

func TestSharedConflictLongDetailsStartAtFirstLine(t *testing.T) {
	a, tt := conflictViewFixture(t, 1)
	a.features.sharedSync.plan.Conflicts[0].LocalDescription = "FIRST LINE LOCAL\n" + strings.Repeat("middle detail\n", 100) + "LAST LINE LOCAL"
	a.features.sharedSync.plan.Conflicts[0].RemoteDescription = "FIRST LINE REMOTE\n" + strings.Repeat("middle detail\n", 100) + "LAST LINE REMOTE"
	a.features.sharedSync.conflictDetailKey = ""
	tt.Frame()
	snapshot(t, tt, "shared-conflict-first-line.png")
}
