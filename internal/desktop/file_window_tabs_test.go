package desktop

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func TestFileTabDragKeepsPaneAndRecords(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	for i := 0; i < 3; i++ {
		name := fmt.Sprint("server-", i)
		p := &pane{profile: core.Profile{Name: name, Host: name, Kind: "sftp"}, files: &fileState{dir: "/", records: &transferLog{}}}
		p.owner.Store(a)
		a.tabs = append(a.tabs, &tab{id: name, name: name, active: p, root: &node{pane: p}})
	}
	original := a.tabs[0]
	a.current = 0
	tt := ui.NewTester(a.fileWindowView, 1000, 650)
	tt.Frame()
	from, ok := tt.Find("server-0")
	if !ok {
		t.Fatal("first title missing")
	}
	to, ok := tt.Find("文件标签 server-2")
	if !ok {
		t.Fatal("target missing")
	}
	tt.Press(from.X+8, from.Y+from.H/2)
	tt.Move(from.X+20, from.Y+from.H/2)
	tt.Move(to.X+to.W-4, to.Y+to.H/2)
	tt.Release(to.X+to.W-4, to.Y+to.H/2)
	tt.Frame()
	if a.tabs[2] != original || a.activeTab() != original || original.active.files.records == nil {
		t.Fatal("drag lost tab, selection or records")
	}
}

func TestFileWindowWithNoVisibleTerminalGroup(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.ensureGroups()
	a.groups.root.group.hidden = true
	a.groups.active = ""
	view := ui.NewTester(a.fileWindowView, 900, 600)
	view.Frame()
	if _, ok := view.Find("设置"); !ok {
		t.Fatal("file window unavailable without visible terminal group")
	}
}

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
