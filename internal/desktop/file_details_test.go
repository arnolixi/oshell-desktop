package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func TestFileDetailsActionsAndRefresh(t *testing.T) {
	a, profile, _ := savedConnectionFixture(t)
	profile.Kind = "sftp"
	profile.InitialDirectory = "/"
	a.config.Profiles[0] = profile
	if err := a.vault.PutFor(profile, "saved-fixture-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(profile, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.tabs[0].active.files != nil && !a.tabs[0].active.files.busy && a.tabs[0].active.files.client != nil
	})
	p := a.tabs[0].active
	f := p.files
	for _, name := range []string{"/folder10", "/folder2"} {
		if err := f.client.Mkdir(name); err != nil {
			t.Fatal(err)
		}
	}
	a.loadFiles(p, "/")
	waitOperator(t, a, func() bool { return !f.busy && len(f.entries) == 3 })
	if f.entries[1].Name() != "folder2" || f.entries[2].Name() != "folder10" {
		t.Fatal("numeric sorting missing", f.entries)
	}
	f.selectEntry(1, false, false)
	a.loadFiles(p, "/")
	waitOperator(t, a, func() bool { return !f.busy })
	if !f.isSelected("folder2") {
		t.Fatal("refresh lost selected identity")
	}
	a.openFileAction(p, "重命名远程项目")
	a.features.fileName = "folder10"
	a.applyFileAction()
	if !a.features.fileOpen || a.features.fileError != "目标名称已存在" {
		t.Fatal("collision not explained")
	}
	a.features.fileName = "invalid\nname"
	a.applyFileAction()
	if !strings.Contains(a.features.fileError, "控制字符") {
		t.Fatal("control character allowed")
	}
	// The captured source cannot drift to a newly-selected row.
	f.selectEntry(2, false, false)
	a.features.fileName = "folder-renamed"
	tt := ui.NewTester(a.fileWindowView, 1000, 700)
	snapshot(t, tt, "file-rename-details.png")
	if err := tt.Click("文件名称"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	waitOperator(t, a, func() bool { return !f.busy && !a.features.fileOpen })
	if _, err := f.client.Lstat("/folder-renamed"); err != nil {
		t.Fatal("captured source not renamed", err)
	}
	if _, err := f.client.Lstat("/folder10"); err != nil {
		t.Fatal("later selection was renamed", err)
	}
	a.openFileAction(p, "新建远程目录")
	if a.features.fileName != "新目录" {
		t.Fatal("missing default directory name")
	}
	oldDir := f.dir
	f.dir = "/folder10"
	a.applyFileAction()
	if !a.features.fileOpen || !strings.Contains(a.features.fileError, "目录已改变") {
		t.Fatal("stale directory accepted")
	}
	f.dir = oldDir
	a.features.fileOpen = false
	tt.Frame()
	snapshot(t, tt, "file-browser-details.png")
}
func TestFileDetailsLargeSizesAndNoConnection(t *testing.T) {
	for _, tc := range []struct {
		size int64
		want string
	}{{-1, "—"}, {0, "0 B"}, {1024, "1.0 KiB"}, {1 << 30, "1.0 GiB"}, {1 << 40, "1.0 TiB"}} {
		if got := bytesLabel(tc.size); got != tc.want {
			t.Fatalf("%d = %s", tc.size, got)
		}
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	tt := ui.NewTester(func(c *ui.Context) { a.fileBrowser(c, nil, true) }, 800, 600)
	if !tt.HasText("请先连接") {
		t.Fatal("no-connection state missing")
	}
}
