package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func TestComposerSaveOpensIndividualCommandEditor(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	text := "echo one\n  echo two"
	a.openComposer(text, false)
	tt := ui.NewTester(a.composerView, 1000, 400)
	if err := tt.Click("保存为快速命令"); err != nil {
		t.Fatal(err)
	}
	q := a.features.commandEdit
	if !a.features.commandsOpen || q == nil || q.Text != text || q.AppendReturn || a.features.commandOriginal != nil || len(a.config.QuickCommands) != 0 {
		t.Fatal("composer did not seed unsaved new command")
	}
	q.Name = "from composer"
	a.commitQuickCommand(false)
	if len(a.config.QuickCommands) != 1 || a.config.QuickCommands[0].Text != text || a.config.QuickCommands[0].AppendReturn {
		t.Fatal("saved command lost composer content", a.features.commandError)
	}
}
func TestQuickSendAndComposerUseSeparateByteLimits(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	large := strings.Repeat("x", 65536) + "\n"
	a.quick = large
	a.prepareSend()
	if a.features.operators.composerOpen || a.status == "" {
		t.Fatal("oversized quick command bypassed limit through multiline")
	}
	a.openComposer(large, false)
	if !a.features.operators.composerOpen || a.features.operators.composerText != large {
		t.Fatal("valid larger composer input rejected")
	}
	a.features.operators.composerOpen = false
	a.openComposer(strings.Repeat("x", 1<<20+1), true)
	if a.features.operators.composerOpen {
		t.Fatal("oversized composer opened")
	}
	a.openComposer("bad\x00input", true)
	if a.features.operators.composerOpen {
		t.Fatal("NUL composer opened")
	}
}
