package desktop

import (
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadConfirmationCancelDestinationAndFrozenPaths(t *testing.T) {
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
	source := filepath.Join(t.TempDir(), "confirmed.txt")
	if err := os.WriteFile(source, []byte("confirmed-upload"), 0600); err != nil {
		t.Fatal(err)
	}
	paths := []string{source}
	a.uploadDropped(pane, paths)
	tt := ui.NewTester(a.fileWindowView, 1080, 720)
	if !tt.HasText("上传 1 个项目") || !tt.HasText("confirmed.txt") {
		t.Fatal("missing upload preview")
	}
	if pane.files.busy {
		t.Fatal("transfer began before confirmation")
	}
	if _, err := pane.files.client.Lstat("/confirmed.txt"); !os.IsNotExist(err) {
		t.Fatal("confirmation already wrote remote file", err)
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if a.features.uploadDraft != nil {
		t.Fatal("cancel retained pending files")
	}
	a.uploadDropped(pane, paths)
	paths[0] = "/not-the-selected-file"
	a.features.uploadDraft.directory = "/fixture-folder"
	tt = ui.NewTester(a.fileWindowView, 1080, 720)
	if err := tt.Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool {
		return !pane.files.busy && pane.files.dir == "/fixture-folder" && len(pane.files.entries) == 1
	})
	if pane.files.entries[0].Name() != "confirmed.txt" || a.features.uploadDraft != nil {
		t.Fatal("selected files/destination not preserved")
	}
	a.uploadDropped(pane, []string{source})
	a.features.uploadDraft.directory = ""
	tt = ui.NewTester(a.fileWindowView, 1080, 720)
	if err := tt.Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	if a.features.uploadDraft == nil || a.features.uploadDraft.err == "" || pane.files.busy {
		t.Fatal("empty destination accepted")
	}
	pane.conn.Client.Close()
	waitOperator(t, a, func() bool { return pane.status == "文件连接已断开" })
	a.features.uploadDraft.directory = "/"
	tt = ui.NewTester(a.fileWindowView, 1080, 720)
	if err := tt.Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	if a.features.uploadDraft == nil || a.features.uploadDraft.err != "连接已变化，请重新选择上传内容" {
		t.Fatal("ended connection accepted pending upload")
	}
}

func TestMixedFileAndDirectoryUploadUsesOneConfirmation(t *testing.T) {
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
	root := t.TempDir()
	file := filepath.Join(root, "batch.txt")
	directory := filepath.Join(root, "batch-folder")
	if err := os.MkdirAll(filepath.Join(directory, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{file, filepath.Join(directory, "nested", "data.txt")} {
		if err := os.WriteFile(name, []byte("mixed-fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a.prepareUpload(pane, []string{file, directory})
	tt := ui.NewTester(a.fileWindowView, 1080, 720)
	if !tt.HasText("上传 2 个项目") || !tt.HasText("batch.txt") || !tt.HasText("batch-folder") {
		t.Fatal("mixed selection not shown together")
	}
	if err := tt.Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !pane.files.busy && len(pane.files.entries) == 3 })
	for _, remote := range []string{"/batch.txt", "/batch-folder/nested/data.txt"} {
		info, err := pane.files.client.Lstat(remote)
		if err != nil || info.Size() != 13 {
			t.Fatal("mixed recursive upload incomplete", err)
		}
	}
}

func TestRowDropTargetsFolderAndFreezesPendingConfirmation(t *testing.T) {
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
	source := filepath.Join(t.TempDir(), "dropped.txt")
	if err := os.WriteFile(source, []byte("drop fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	a.uploadDroppedOn(pane, []string{source}, "fixture-folder")
	draft := a.features.uploadDraft
	if draft == nil || draft.directory != "/fixture-folder" || !a.modalActive() {
		t.Fatal("folder drop target not used")
	}
	a.uploadDroppedOn(pane, []string{"/unrelated"}, "")
	if a.features.uploadDraft != draft || draft.paths[0] != source {
		t.Fatal("second drop replaced pending consent")
	}
	if _, err := pane.files.client.Lstat("/fixture-folder/dropped.txt"); !os.IsNotExist(err) {
		t.Fatal("drop uploaded before confirmation", err)
	}
	tt := ui.NewTester(a.fileWindowView, 1080, 720)
	if err := tt.Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool {
		return !pane.files.busy && pane.files.dir == "/fixture-folder" && len(pane.files.entries) == 1
	})
	a.uploadDroppedOn(pane, []string{source}, "dropped.txt")
	if a.features.uploadDraft == nil || a.features.uploadDraft.directory != "/fixture-folder" {
		t.Fatal("file-row drop did not use current directory")
	}
	tt = ui.NewTester(a.fileWindowView, 1080, 720)
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	pane.files.filter = "not-visible"
	a.uploadDroppedOn(pane, []string{source}, "dropped.txt")
	if a.features.uploadDraft != nil {
		t.Fatal("stale/hidden row accepted a drop")
	}
	pane.files.filter = ""
	pane.files.busy = true
	a.uploadDropped(pane, []string{source})
	if a.features.uploadDraft != nil {
		t.Fatal("busy pane accepted a drop")
	}
	pane.files.busy = false
	pane.conn.Client.Close()
	waitOperator(t, a, func() bool { return pane.status == "文件连接已断开" })
	a.uploadDropped(pane, []string{source})
	if a.features.uploadDraft != nil {
		t.Fatal("disconnected pane accepted a drop")
	}
}
