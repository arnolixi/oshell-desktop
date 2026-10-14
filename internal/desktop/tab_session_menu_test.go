package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestTabContextPropertiesTargetsClickedBackgroundTab(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p, _ := operatorPane(t, a, "background")
	operatorPane(t, a, "foreground")
	a.ensureGroups()
	a.selectTab(a.tabs[1])
	clicked := a.tabs[0]
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Label("target-tab").ContextMenu(func(m *ui.Menu) { a.tabSessionMenu(m, clicked) })
	}, 700, 400)
	if err := tt.RightClick("target-tab"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("当前会话属性…"); err == nil {
		t.Fatal("live properties enabled without live SSH connection")
	}
	if err := tt.ChooseMenuItem("会话配置属性…"); err != nil {
		t.Fatal(err)
	}
	if !a.editing || a.draft.Host != p.profile.Host || a.activeTab() != clicked {
		t.Fatal("edited foreground tab instead of clicked tab")
	}
}
