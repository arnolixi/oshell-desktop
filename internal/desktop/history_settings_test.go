package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestHistoryAndFontWarningsDoNotMutateSettings(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.config.Settings.ScrollbackLines = 25000
	a.config.Settings.FontFamily = "Fixture Proportional"
	a.prepareSettings()
	a.features.fontCatalog = []core.FontEntry{{Name: "Fixture Proportional", Monospaced: false}}
	tt := ui.NewTester(a.settingsView, 800, 650)
	if !tt.HasText("自定义…") || !tt.HasText("当前超过 20000 行") || !tt.HasText("不是等宽字体") {
		t.Fatal(tt.Texts())
	}
	a.features.settingsLines = "invalid"
	tt.Frame()
	if !tt.HasText("请输入 500～100000 的整数行数") {
		t.Fatal("missing invalid history warning")
	}
	a.applySettings()
	if a.config.Settings.ScrollbackLines != 25000 || a.config.Settings.FontFamily != "Fixture Proportional" {
		t.Fatal("invalid draft changed saved settings")
	}
	a.closeSettings()
	a.prepareSettings()
	if a.features.settingsLines != "25000" || !a.features.historyCustom {
		t.Fatal("custom value not restored")
	}
}
