package desktop

import (
	"github.com/egoist/mygo/ui"
	"reflect"
	"testing"
)

func TestFileContextMenuPreservesGroupSwitchesRowAndClearsBlank(t *testing.T) {
	a, p, _ := savedConnectionFixture(t)
	p.Kind = "sftp"
	p.InitialDirectory = "/"
	a.config.Profiles[0] = p
	if err := a.vault.PutFor(p, "saved-fixture-password", p.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(p, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.tabs[0].active.files != nil && !a.tabs[0].active.files.busy && a.tabs[0].active.files.client != nil
	})
	pane := a.tabs[0].active
	for _, dir := range []string{"/another", "/third"} {
		if err := pane.files.client.Mkdir(dir); err != nil {
			t.Fatal(err)
		}
	}
	a.loadFiles(pane, "/")
	waitOperator(t, a, func() bool { return !pane.files.busy && len(pane.files.entries) == 3 })
	tt := ui.NewTester(a.fileWindowView, 1000, 700)
	if err := tt.Click("another"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ClickWith(ui.Cmd, "fixture-folder"); err != nil {
		t.Fatal(err)
	}
	if err := tt.RightClick("another"); err != nil {
		t.Fatal(err)
	}
	if len(pane.files.selectedNames()) != 2 {
		t.Fatal("context menu discarded group")
	}
	if want := []string{"下载…", "重命名…", "删除…", "-", "上传…", "新建目录…", "刷新"}; !reflect.DeepEqual(tt.Menu(), want) {
		t.Fatal(tt.Menu())
	}
	if err := tt.ChooseMenuItem("重命名…"); err == nil {
		t.Fatal("group rename was enabled")
	}
	tt.CloseMenu()
	if err := tt.RightClick("third"); err != nil {
		t.Fatal(err)
	}
	if got := pane.files.selectedNames(); !reflect.DeepEqual(got, []string{"third"}) {
		t.Fatal("right click did not choose target", got)
	}
	if err := tt.ChooseMenuItem("重命名…"); err != nil {
		t.Fatal(err)
	}
	if !a.features.fileOpen || a.features.fileName != "third" {
		t.Fatal("rename opened for wrong item")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	rect, ok := tt.Find("远程文件列表")
	if !ok {
		t.Fatal("file list missing")
	}
	tt.RightClickAt(rect.X+rect.W/2, rect.Y+rect.H-10)
	if len(pane.files.selectedNames()) != 0 {
		t.Fatal("blank menu retained selection")
	}
	if err := tt.ChooseMenuItem("下载…"); err == nil {
		t.Fatal("blank menu enabled download")
	}
	tt.CloseMenu()
	pane.conn.Client.Close()
	waitOperator(t, a, func() bool { return pane.status == "文件连接已断开" })
	if err := tt.RightClick("another"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("新建目录…"); err == nil {
		t.Fatal("disconnected context menu allowed mutation")
	}
}
