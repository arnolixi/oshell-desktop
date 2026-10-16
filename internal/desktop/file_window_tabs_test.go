package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func TestFileWindowCurrentTabIdentification(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			cfg := core.DefaultConfig()
			cfg.Settings.InterfaceTheme = theme
			a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
			for _, name := range []string{"生产文件", "备份文件"} {
				profile := core.NewProfile()
				profile.Name, profile.Kind, profile.Host, profile.Username = name, "sftp", "files.example.test", name
				p := &pane{profile: profile, status: "已连接"}
				p.owner.Store(a)
				a.tabs = append(a.tabs, &tab{id: name, name: name, active: p, root: &node{pane: p}})
			}
			a.current = 0
			tt := ui.NewTester(a.fileWindowView, 760, 480)
			if !tt.HasText("当前会话：" + fileTabDescription(a.tabs[0])) {
				t.Fatal("initial active connection not identified")
			}
			r, ok := tt.Find("文件标签 备份文件")
			if !ok {
				t.Fatal("second file tab missing")
			}
			tt.ClickAt(r.X+40, r.Y+r.H/2)
			if a.activeTab() != a.tabs[1] || !tt.HasText("当前会话："+fileTabDescription(a.tabs[1])) {
				t.Fatal("same-host tab selection did not update current connection")
			}
			snapshot(t, tt, "file-tabs-"+theme+".png")
			// Closing the selected tab must also update the current connection.
			a.closeTab(a.tabs[1])
			tt.Frame()
			if !tt.HasText("当前会话：" + fileTabDescription(a.tabs[0])) {
				t.Fatal("closed tab retained current connection indicator")
			}
		})
	}
}
