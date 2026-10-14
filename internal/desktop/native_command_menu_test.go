package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestNativeCommandMenuGroupsAndResolvesCurrentCommand(t *testing.T) {
	cfg := core.DefaultConfig()
	q := core.NewQuickCommand()
	q.Name = "inspect"
	q.Group = "ops"
	q.Text = "old"
	q.AppendReturn = false
	cfg.QuickCommands = []core.QuickCommand{q}
	h := &applicationHub{config: cfg}
	menu := h.nativeCommandMenu()
	if menu.Label != "快速命令" || len(menu.Submenu) != 3 || menu.Submenu[2].Label != "ops" || menu.Submenu[2].Submenu[0].Label != "inspect" {
		t.Fatal("menu grouping missing")
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	_, conn := operatorPane(t, a, "target")
	a.config.QuickCommands[0].Text = "updated"
	a.loadSavedCommandID(q.ID)
	if !a.features.operators.composerOpen || a.features.operators.composerText != "updated" || a.features.operators.composerReturn || conn.text() != "" {
		t.Fatal("menu executed command or used stale contents")
	}
	a.features.operators.composerOpen = false
	a.config.QuickCommands = nil
	a.loadSavedCommandID(q.ID)
	if a.features.operators.composerOpen || conn.text() != "" {
		t.Fatal("deleted command opened")
	}
}
func TestNativeCommandAndTitleMenusRespectLock(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	if err := a.vault.SetMaster("fixture-master"); err != nil {
		t.Fatal(err)
	}
	a.vault.Lock()
	a.loadSavedCommandID("unused")
	if a.features.vaultAction != "unlock" || a.features.operators.composerOpen {
		t.Fatal("command bypassed lock")
	}
	a.features.vaultOpen = false
	a.refreshActiveHostFromMenu()
	if !a.features.vaultOpen {
		t.Fatal("title refresh bypassed lock")
	}
}
