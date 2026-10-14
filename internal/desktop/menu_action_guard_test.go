package desktop

import (
	"github.com/egoist/mygo"
	"oshell-desktop/internal/core"
	"testing"
)

func TestNativeMenuCannotReplaceActiveEditor(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := core.NewProfile()
	p.Host = "draft.invalid"
	a.editing = true
	a.draft = p
	a.performAction("newSession")
	if a.draft.ID != p.ID || a.draft.Host != "draft.invalid" {
		t.Fatal("menu overwrote editor")
	}
	a.features.vaultOpen = true
	a.features.master = "typed-fixture-value"
	a.performAction("sessionManager")
	if a.features.master != "typed-fixture-value" {
		t.Fatal("menu reset credential input")
	}
}
func TestWorkspaceMenuCallbackHonorsModalAndNativeSheet(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	h := &applicationHub{windows: []*App{a}}
	calls := 0
	item := h.workspaceMenuItem("test", func(*App) { calls++ })
	a.features.uploadDraft = &uploadConfirmation{open: true}
	item.Click(item, nil)
	if calls != 0 {
		t.Fatal("custom native menu bypassed upload modal")
	}
	a.features.uploadDraft = nil
	original := tabMoveDialogOpen
	tabMoveDialogOpen = func(*mygo.Window) bool { return true }
	t.Cleanup(func() { tabMoveDialogOpen = original })
	item.Click(item, nil)
	if calls != 0 {
		t.Fatal("custom menu bypassed native sheet")
	}
	tabMoveDialogOpen = func(*mygo.Window) bool { return false }
	item.Click(item, nil)
	if calls != 1 {
		t.Fatal("dismissed dialog left menu blocked")
	}
}

func TestNativeMenuGuardIncludesAuxiliaryWindowSheets(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	sheetParent := &mygo.Window{}
	old := tabMoveDialogOpen
	tabMoveDialogOpen = func(w *mygo.Window) bool { return w == sheetParent }
	t.Cleanup(func() { tabMoveDialogOpen = old })
	a.preferencesWindow = sheetParent
	if !a.workspaceMenuBlocked() {
		t.Fatal("preferences sheet ignored")
	}
	a.preferencesWindow = nil
	a.manager = sheetParent
	if !a.workspaceMenuBlocked() {
		t.Fatal("manager sheet ignored")
	}
	a.manager = nil
	if a.workspaceMenuBlocked() {
		t.Fatal("closed sheet kept guard active")
	}
}
