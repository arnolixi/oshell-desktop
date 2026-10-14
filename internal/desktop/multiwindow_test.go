package desktop

import (
	"context"
	"github.com/egoist/mygo"
	"testing"

	"oshell-desktop/internal/core"
)

func TestWindowMovePreservesConnectionOwnerAndSharedConfig(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	first, second := newApp(store, core.DefaultConfig()), newApp(store, core.DefaultConfig())
	hub := &applicationHub{store: store, config: first.config, windows: []*App{first, second}}
	first.hub = hub
	second.hub = hub
	_, cancel := context.WithCancel(context.Background())
	p := &pane{id: "pane", cancel: cancel}
	p.owner.Store(first)
	workspace := &tab{id: "tab", active: p, root: &node{pane: p}}
	first.tabs = []*tab{workspace}
	first.transferTab(workspace, second)
	if len(first.tabs) != 0 || len(second.tabs) != 1 || p.owner.Load() != second || p.closed {
		t.Fatal("window move replaced or closed its terminal")
	}
	profile := core.NewProfile()
	profile.Host = "example.invalid"
	profile.Username = "tester"
	if !first.changeConfig(func(c *core.Config) error { c.Profiles = append(c.Profiles, profile); return nil }) {
		t.Fatal(first.status)
	}
	if len(second.config.Profiles) != 1 {
		t.Fatal("configuration did not propagate")
	}
	second.dispose(p)
}

func TestWindowMoveDropsOldInputSelectionsAndPendingDispatch(t *testing.T) {
	first := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	second := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p, connection := operatorPane(t, first, "moving")
	tab := first.tabs[0]
	terminal := p.term
	o := &first.features.operators
	first.features.syncTargets = map[string]bool{p.id: true}
	o.selected = map[string]bool{p.id: true}
	o.selectionDraft = map[string]bool{p.id: true}
	o.composerTargets = map[string]bool{p.id: true}
	o.pendingTargets = []*pane{p}
	o.pendingText = "should-not-send"
	first.confirmSend = true
	first.features.pasteTargets = []pasteDestination{{pane: p, term: p.term}}
	first.features.pasteText = "should-not-paste"
	first.features.pasteOpen = true
	first.transferTab(tab, second)
	if o.selected[p.id] || o.selectionDraft[p.id] || o.composerTargets[p.id] || first.features.syncTargets[p.id] {
		t.Fatal("old selections retained moved target")
	}
	if first.confirmSend || len(o.pendingTargets) > 0 || o.pendingText != "" || first.features.pasteOpen || len(first.features.pasteTargets) > 0 {
		t.Fatal("pending input retained moved target")
	}
	if p.term != terminal || p.closed {
		t.Fatal("move rebuilt terminal")
	}
	second.transferTab(tab, first)
	first.quickSendScope = "selected"
	if len(first.sendCandidates()) != 0 {
		t.Fatal("moving back revived selection")
	}
	first.send()
	first.confirmPaste()
	p.term.Send([]byte("barrier"))
	waitOperator(t, first, func() bool { return connection.text() != "" })
	if connection.text() != "barrier" {
		t.Fatal("stale pending input dispatched")
	}
}

func TestWindowMoveWaitsForNativeSheetBeforeCreatingWindow(t *testing.T) {
	first := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	second := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	first.hub = &applicationHub{windows: []*App{first}}
	p, _ := operatorPane(t, first, "sheet-fixture")
	tab := first.tabs[0]
	old := tabMoveDialogOpen
	tabMoveDialogOpen = func(*mygo.Window) bool { return true }
	t.Cleanup(func() { tabMoveDialogOpen = old })
	first.moveTabToNewWindow()
	first.transferTab(tab, second)
	if len(first.hub.windows) != 1 || len(first.tabs) != 1 || len(second.tabs) != 0 || p.owner.Load() != first {
		t.Fatal("move happened under sheet")
	}
	tabMoveDialogOpen = func(*mygo.Window) bool { return false }
	first.transferTab(tab, second)
	if len(first.tabs) != 0 || len(second.tabs) != 1 || p.owner.Load() != second {
		t.Fatal("sheet dismissal did not allow move")
	}
}

func TestWindowMoveRejectsLockedDestination(t *testing.T) {
	first := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	second := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p, _ := operatorPane(t, first, "lock-fixture")
	tab := first.tabs[0]
	if err := second.vault.SetMaster("fixture-master"); err != nil {
		t.Fatal(err)
	}
	second.vault.Lock()
	first.transferTab(tab, second)
	if len(first.tabs) != 1 || len(second.tabs) != 0 || p.owner.Load() != first {
		t.Fatal("moved terminal into locked destination")
	}
}
