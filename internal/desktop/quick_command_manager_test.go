package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestQuickCommandManagerEditsConfirmsDeletionAndLoadsComposer(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.openCommands()
	a.editQuickCommand(nil)
	q := a.features.commandEdit
	q.Name = "inspect"
	q.Group = "tools"
	q.Text = "df -h"
	q.AppendReturn = false
	a.commitQuickCommand(false)
	if len(a.config.QuickCommands) != 1 || a.features.commandEdit != nil {
		t.Fatal(a.features.commandError)
	}
	tt := ui.NewTester(a.quickCommandDialog, 900, 600)
	if err := tt.Click("inspect"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("删除命令…"); err != nil {
		t.Fatal(err)
	}
	if len(a.config.QuickCommands) != 1 {
		t.Fatal("deleted before confirmation")
	}
	if err := tt.Click("取消删除"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("填入撰写窗"); err != nil {
		t.Fatal(err)
	}
	if a.features.commandsOpen {
		t.Fatal("manager did not hand off to composer")
	}
	saved := a.config.QuickCommands[0]
	a.openCommands()
	a.editQuickCommand(&saved)
	a.features.commandEdit.Text = "new"
	a.config.QuickCommands[0].Text = "changed elsewhere"
	a.commitQuickCommand(false)
	if a.features.commandError == "" || a.config.QuickCommands[0].Text != "changed elsewhere" {
		t.Fatal("stale edit overwrote command")
	}
	current := a.config.QuickCommands[0]
	a.features.commandDelete = &current
	a.commitQuickCommand(true)
	if len(a.config.QuickCommands) != 0 {
		t.Fatal("confirmed delete failed", a.features.commandError)
	}
}
