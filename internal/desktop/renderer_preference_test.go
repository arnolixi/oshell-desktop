package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestRendererSettingsApplyKeepsTerminalAndHistory(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	pane, _ := operatorPane(t, a, "renderer-fixture")
	terminal := pane.term
	connection := pane.conn
	terminal.Feed([]byte("renderer-history-fixture"))
	history := terminal.Text()
	for _, gpu := range []bool{false, true} {
		a.prepareSettings()
		a.features.settingsDraft.Metal = gpu
		a.applySettings()
		if a.config.Settings.Metal != gpu || pane.term != terminal || pane.conn != connection || terminal.Text() != history {
			t.Fatal("renderer setting rebuilt terminal or lost history")
		}
	}
	a.prepareSettings()
	a.features.settingsDraft.Metal = false
	a.closeSettings()
	if !a.config.Settings.Metal {
		t.Fatal("cancel changed renderer preference")
	}
}
