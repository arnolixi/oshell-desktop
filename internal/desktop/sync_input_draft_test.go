package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestSyncInputSelectionAppliesOnlyOnConfirmation(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	first, _ := operatorPane(t, a, "first")
	second, _ := operatorPane(t, a, "second")
	a.current = 0
	a.showSyncInput()
	a.applySyncInputDraft()
	if !a.features.syncOpen || a.features.syncEnabled {
		t.Fatal("single target applied")
	}
	a.features.syncSelectionDraft[second.id] = true
	if a.features.syncTargets[second.id] {
		t.Fatal("draft applied before confirmation")
	}
	a.applySyncInputDraft()
	if !a.features.syncEnabled || !a.features.syncTargets[first.id] || !a.features.syncTargets[second.id] {
		t.Fatal("two-target selection not applied")
	}
	a.showSyncInput()
	a.features.syncDraftEnabled = false
	a.features.syncOpen = false
	if !a.features.syncEnabled {
		t.Fatal("cancel disabled live synchronization")
	}
	a.showSyncInput()
	a.features.syncDraftEnabled = false
	a.applySyncInputDraft()
	if a.features.syncEnabled || len(a.features.syncTargets) != 0 {
		t.Fatal("disable did not clear targets")
	}
}
func TestUnavailableSyncTargetStopsWholeBroadcast(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	source, _ := operatorPane(t, a, "source")
	unavailable, _ := operatorPane(t, a, "unavailable")
	remaining, receiver := operatorPane(t, a, "remaining")
	a.features.syncEnabled = true
	a.features.syncTargets = map[string]bool{source.id: true, unavailable.id: true, remaining.id: true}
	unavailable.term.Close()
	a.routeInput(source, ui.InputEvent{Kind: ui.InputText, Text: "must-not-broadcast"})
	remaining.term.Send([]byte("barrier"))
	waitOperator(t, a, func() bool { return receiver.text() != "" })
	if receiver.text() != "barrier" || a.features.syncEnabled || len(a.features.syncTargets) != 0 {
		t.Fatal("partial synchronization continued")
	}
}
