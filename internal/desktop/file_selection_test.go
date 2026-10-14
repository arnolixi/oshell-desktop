package desktop

import (
	"context"
	"errors"
	"fmt"
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRemoteFileSelectionToggleRangeAndSelectAll(t *testing.T) {
	f := &fileState{}
	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
		file := filepath.Join(root, name)
		if err := os.WriteFile(file, nil, 0600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		f.entries = append(f.entries, info)
	}
	check := func(want ...string) {
		t.Helper()
		if got := f.selectedNames(); !reflect.DeepEqual(got, want) {
			t.Fatalf("selected=%v want=%v", got, want)
		}
	}
	f.selectEntry(1, false, false)
	check("b.txt")
	f.selectEntry(3, true, false)
	check("b.txt", "c.txt", "d.txt")
	f.selectEntry(2, false, true)
	check("b.txt", "d.txt")
	f.selectEntry(0, false, true)
	check("a.txt", "b.txt", "d.txt")
	f.selectEntry(3, false, false)
	check("d.txt")
	f.selectEntry(3, false, true)
	check()
	f.selectAll()
	check("a.txt", "b.txt", "c.txt", "d.txt")
	f.selectEntry(-1, false, false)
	check("a.txt", "b.txt", "c.txt", "d.txt")
}

func TestFileListHandlesNativeSelectAllCommand(t *testing.T) {
	f := &fileState{dir: "/"}
	root := t.TempDir()
	for _, name := range []string{"one", "two"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		f.entries = append(f.entries, info)
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := &pane{files: f, conn: &core.Connection{}}
	tt := ui.NewTester(func(c *ui.Context) { a.fileBrowser(c, p, true) }, 1080, 720)
	if err := tt.Click("one"); err != nil {
		t.Fatal(err)
	}
	if len(f.selectedNames()) != 1 {
		t.Fatal("row not selected")
	}
	tt.Command("selectAll")
	if len(f.selectedNames()) != 2 {
		t.Fatal("native edit command did not select all")
	}
}

func TestBatchRemoveOnlySelectedEntriesAndHonorsCancellation(t *testing.T) {
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
	client := a.tabs[0].active.files.client
	local := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(local, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"remove.txt", "keep.txt"} {
		if err := client.Upload(context.Background(), local, "/"+name, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := removeRemoteSelection(context.Background(), client, "/", []string{"fixture-folder", "remove.txt"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fixture-folder", "remove.txt"} {
		if _, err := client.Lstat("/" + name); !os.IsNotExist(err) {
			t.Fatal("selected item remains", err)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := removeRemoteSelection(cancelled, client, "/", []string{"keep.txt"}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancel not honored", err)
	}
	if _, err := client.Lstat("/keep.txt"); err != nil {
		t.Fatal("unselected item removed", err)
	}
}

func TestRemoteFileKeyboardRangeAndOffscreenFocus(t *testing.T) {
	f := &fileState{dir: "/"}
	root := t.TempDir()
	for i := 0; i < 120; i++ {
		name := fmt.Sprintf("file-%03d", i)
		file := filepath.Join(root, name)
		if err := os.WriteFile(file, nil, 0600); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(file)
		f.entries = append(f.entries, info)
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := &pane{files: f, conn: &core.Connection{}}
	tt := ui.NewTester(func(c *ui.Context) { a.fileBrowser(c, p, true) }, 800, 450)
	if err := tt.Click("file-000"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyDown)
	if got := f.selectedNames(); !reflect.DeepEqual(got, []string{"file-001"}) {
		t.Fatal("down did not choose next row", got)
	}
	tt.Key(ui.Shift, ui.KeyDown)
	if got := f.selectedNames(); !reflect.DeepEqual(got, []string{"file-001", "file-002"}) {
		t.Fatal("shift-down did not extend range", got)
	}
	tt.Key(ui.Shift, ui.KeyUp)
	if got := f.selectedNames(); !reflect.DeepEqual(got, []string{"file-001"}) {
		t.Fatal("shift-up did not contract range", got)
	}
	tt.Key(0, ui.KeyEnd)
	tt.Frame()
	if f.selected != "file-119" || !tt.HasText("file-119") {
		t.Fatal("end did not reveal last row", f.selected)
	}
	tt.Key(0, ui.KeyUp)
	if f.selected != "file-118" {
		t.Fatal("focus did not follow offscreen selection", f.selected)
	}
	tt.Key(0, ui.KeyHome)
	tt.Frame()
	if f.selected != "file-000" || !tt.HasText("file-000") {
		t.Fatal("home did not reveal first row")
	}
	step := f.pageRows
	if step < 2 {
		t.Fatal("viewport page size not measured", step)
	}
	tt.Key(0, ui.KeyPageDown)
	tt.Frame()
	if f.selected != fmt.Sprintf("file-%03d", step) {
		t.Fatal("page down did not move by visible page", f.selected, step)
	}
	tt.Key(ui.Shift, ui.KeyPageDown)
	tt.Frame()
	if len(f.selectedNames()) != step+1 {
		t.Fatal("shift page down did not extend selection", len(f.selectedNames()), step)
	}
	tt.Key(ui.Shift, ui.KeyPageUp)
	tt.Frame()
	if len(f.selectedNames()) != 1 {
		t.Fatal("shift page up did not contract selection")
	}
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyPageDown)
	tt.Frame()
	if f.selected != "file-119" {
		t.Fatal("page down crossed final row")
	}

}

func TestDirectoryFilterScopesSelectionAndKeyboardMovement(t *testing.T) {
	f := &fileState{dir: "/"}
	root := t.TempDir()
	for _, name := range []string{"Alpha.txt", "Beta.txt", "alpha.log", "目录报告.txt", "Cafe\u0301.txt"} {
		file := filepath.Join(root, name)
		if err := os.WriteFile(file, nil, 0600); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(file)
		f.entries = append(f.entries, info)
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := &pane{files: f, conn: &core.Connection{}}
	tt := ui.NewTester(func(c *ui.Context) { a.fileBrowser(c, p, true) }, 1000, 600)
	if err := tt.Click("Beta.txt"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("筛选当前目录"); err != nil {
		t.Fatal(err)
	}
	tt.Type("ALPHA")
	if len(f.visibleEntries()) != 2 || len(f.selectedNames()) != 0 || tt.HasText("Beta.txt") {
		t.Fatal("filter did not hide unmatched files or clear old selection")
	}
	if err := tt.Click("Alpha.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Shift, ui.KeyDown)
	if got := f.selectedNames(); !reflect.DeepEqual(got, []string{"Alpha.txt", "alpha.log"}) {
		t.Fatal("range crossed hidden row", got)
	}
	tt.Command("selectAll")
	if len(f.selectedNames()) != 2 {
		t.Fatal("all selected hidden rows")
	}
	if err := tt.Click("筛选当前目录"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("目录")
	if len(f.visibleEntries()) != 1 || len(f.selectedNames()) != 0 || !tt.HasText("目录报告.txt") {
		t.Fatal("unicode filter failed")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("no-match")
	if len(f.visibleEntries()) != 0 || len(f.selectedNames()) != 0 {
		t.Fatal("empty result retained selection")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	if len(f.visibleEntries()) != 5 || len(f.selectedNames()) != 0 {
		t.Fatal("clearing filter did not restore unselected list")
	}
	tt.Type("CAFÉ")
	if len(f.visibleEntries()) != 1 {
		t.Fatal("canonical Unicode forms did not match")
	}
	if err := tt.Click("清除筛选"); err != nil {
		t.Fatal(err)
	}
	if f.filter != "" || len(f.visibleEntries()) != 5 {
		t.Fatal("clear button failed")
	}
	tt.Type("ALPHA")
	tt.Key(0, ui.KeyEscape)
	if f.filter != "" || len(f.visibleEntries()) != 5 {
		t.Fatal("Escape did not clear focused filter")
	}

}
