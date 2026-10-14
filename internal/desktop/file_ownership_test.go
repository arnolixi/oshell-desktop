package desktop

import (
	"github.com/egoist/mygo/ui"
	"github.com/pkg/sftp"
	"os"
	"oshell-desktop/internal/core"
	"testing"
)

type ownedFileInfo struct{ iconFileInfo }

func (i ownedFileInfo) Sys() any { return &sftp.FileStat{Mode: 0100644, UID: 1001, GID: 100} }
func TestFileOwnershipColumnsAndTooltip(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	info := ownedFileInfo{iconFileInfo{"config.yaml", 0640}}
	f := &fileState{dir: "/", input: "/", entries: []os.FileInfo{info}, status: "1 个项目"}
	p := &pane{profile: core.Profile{Kind: "sftp"}, ftp: &core.FTPSession{}, files: f}
	tt := ui.NewTester(func(c *ui.Context) { a.theme(c); a.fileBrowser(c, p, true) }, 1000, 500)
	for _, text := range []string{"权限", "用户 (UID)", "组 (GID)", "-rw-r-----", "1001", "100"} {
		if !tt.HasText(text) {
			t.Fatal("column/value missing", text)
		}
	}
	if err := tt.Click("config.yaml"); err != nil {
		t.Fatal(err)
	}
	if !f.isSelected("config.yaml") {
		t.Fatal("new columns broke selection")
	}
	snapshot(t, tt, "file-permissions-ownership.png")
}
