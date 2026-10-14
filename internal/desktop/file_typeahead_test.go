package desktop

import (
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileTypeAheadPrefixCycleUnicodeAndFilter(t *testing.T) {
	f := &fileState{}
	dir := t.TempDir()
	for _, name := range []string{"alpha.txt", "alpine.txt", "beta.txt", "École.txt", "中文.txt"} {
		file := filepath.Join(dir, name)
		os.WriteFile(file, nil, 0600)
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		f.entries = append(f.entries, info)
	}
	now := time.Now()
	if got := f.findTypedFile("a", -1, now); got != 0 {
		t.Fatal(got)
	}
	if got := f.findTypedFile("l", 0, now.Add(10*time.Millisecond)); got != 0 {
		t.Fatal(got)
	}
	f.typeQuery = ""
	if got := f.findTypedFile("a", 0, now); got != 1 {
		t.Fatal("cycle start", got)
	}
	if got := f.findTypedFile("a", 1, now.Add(20*time.Millisecond)); got != 0 {
		t.Fatal("repeat cycle", got)
	}
	if got := f.findTypedFile("e\u0301", 0, now.Add(2*time.Second)); got != 3 {
		t.Fatal("normalized unicode", got)
	}
	f.filter = "beta"
	f.typeQuery = ""
	if got := f.findTypedFile("b", -1, now); got != 0 {
		t.Fatal("filtered row indexing", got)
	}
}
func TestEmptyRemoteDirectoryAcceptsKeyboardParentNavigation(t *testing.T) {
	a, profile, _ := savedConnectionFixture(t)
	profile.Kind = "sftp"
	profile.InitialDirectory = "/fixture-folder"
	a.config.Profiles[0] = profile
	if err := a.vault.PutFor(profile, "saved-fixture-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(profile, 0)
	waitOperator(t, a, func() bool { return a.activePane() != nil && a.activePane().files != nil && !a.activePane().files.busy })
	p := a.activePane()
	if len(p.files.entries) != 0 {
		t.Fatal("fixture not empty")
	}
	tt := ui.NewTester(a.fileWindowView, 1000, 700)
	if err := tt.Click("远程文件列表"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyUp)
	waitOperator(t, a, func() bool { return !p.files.busy && p.files.dir == "/" })
	if err := p.files.client.Mkdir("/中文目录"); err != nil {
		t.Fatal(err)
	}
	a.loadFiles(p, "/")
	waitOperator(t, a, func() bool { return !p.files.busy })
	tt.Frame()
	if err := tt.Click("fixture-folder"); err != nil {
		t.Fatal(err)
	}
	tt.Type("中文")
	if p.files.selected != "中文目录" {
		t.Fatal("typed input did not reach file-list selection", p.files.selected)
	}

}
