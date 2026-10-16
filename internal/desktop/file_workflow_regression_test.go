package desktop

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func connectedFileWorkflow(t *testing.T) (*App, *pane) {
	t.Helper()
	a, profile, _ := savedConnectionFixture(t)
	profile.Kind, profile.InitialDirectory = "sftp", "/"
	a.config.Profiles[0] = profile
	if err := a.vault.PutFor(profile, "saved-fixture-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(profile, 0)
	waitOperator(t, a, func() bool {
		p := a.activePane()
		return p != nil && p.files != nil && !p.files.busy && p.files.client != nil
	})
	return a, a.activePane()
}

func TestFileNavigationResolvesRelativeToCurrentDirectory(t *testing.T) {
	a, p := connectedFileWorkflow(t)
	for _, directory := range []string{"/工作区", "/工作区/子目录"} {
		if err := p.files.client.Mkdir(directory); err != nil {
			t.Fatal(err)
		}
	}
	a.loadFiles(p, "/工作区")
	waitOperator(t, a, func() bool { return !p.files.busy })
	p.files.input = "子目录"
	tt := ui.NewTester(a.fileWindowView, 1080, 720)
	if err := tt.Click("转到"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !p.files.busy })
	if p.files.dir != "/工作区/子目录" {
		t.Fatal("relative path used server default directory", p.files.dir, p.files.status)
	}
	a.loadFiles(p, "..")
	waitOperator(t, a, func() bool { return !p.files.busy })
	if p.files.dir != "/工作区" {
		t.Fatal("relative parent escaped current directory", p.files.dir)
	}
}

func TestFileUploadResolvesRelativeDestination(t *testing.T) {
	a, p := connectedFileWorkflow(t)
	for _, directory := range []string{"/工作区", "/工作区/目标", "/目标"} {
		if err := p.files.client.Mkdir(directory); err != nil {
			t.Fatal(err)
		}
	}
	a.loadFiles(p, "/工作区")
	waitOperator(t, a, func() bool { return !p.files.busy })
	source := filepath.Join(t.TempDir(), "中文 空格.txt")
	if err := os.WriteFile(source, []byte("correct destination"), 0600); err != nil {
		t.Fatal(err)
	}
	a.prepareUpload(p, []string{source})
	a.features.uploadDraft.directory = "目标"
	if err := ui.NewTester(a.fileWindowView, 1080, 720).Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !p.files.busy })
	if _, err := p.files.client.Lstat("/工作区/目标/中文 空格.txt"); err != nil {
		t.Fatal("upload missed current directory", err, p.files.status)
	}
	if _, err := p.files.client.Lstat("/目标/中文 空格.txt"); !os.IsNotExist(err) {
		t.Fatal("upload went to server default directory", err)
	}
}

func TestUnsupportedDownloadNameReportsError(t *testing.T) {
	a, p := connectedFileWorkflow(t)
	if err := p.files.client.Mkdir("/report:2026"); err != nil {
		t.Fatal(err)
	}
	a.loadFiles(p, "/")
	waitOperator(t, a, func() bool { return !p.files.busy })
	p.files.selected = "report:2026"
	a.download(p)
	if !p.files.statusError || !strings.Contains(p.files.status, "report:2026") {
		t.Fatal("unsupported download silently ignored", p.files.status)
	}
}

func TestFileTransfersRemainIsolatedAcrossSessions(t *testing.T) {
	a, first := connectedFileWorkflow(t)
	other, second := connectedFileWorkflow(t)
	secondTab := other.activeTab()
	other.tabs = nil
	second.owner.Store(a)
	a.tabs = append(a.tabs, secondTab)
	a.selectTab(a.tabs[0])
	root := t.TempDir()
	tree := filepath.Join(root, "目录 空格")
	if err := os.MkdirAll(filepath.Join(tree, "空目录"), 0700); err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("first-session\x00\xff"), 16384)
	if err := os.WriteFile(filepath.Join(tree, "数据.bin"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "零字节.txt"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	a.prepareUpload(first, []string{tree})
	a.selectTab(secondTab) // An external SSH toolbar action can change the active tab.
	if err := ui.NewTester(a.fileWindowView, 1080, 720).Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !first.files.busy })
	if a.activePane() != second {
		t.Fatal("background upload stole active session")
	}
	if _, err := second.files.client.Lstat("/目录 空格"); !os.IsNotExist(err) {
		t.Fatal("upload leaked to other session", err)
	}
	if _, err := first.files.client.Lstat("/目录 空格/数据.bin"); err != nil {
		t.Fatal(err, first.files.status)
	}

	started := make(chan struct{})
	a.withTransfer(first, func(ctx context.Context, client core.RemoteFileSystem) error {
		return client.Download(ctx, "/目录 空格/数据.bin", filepath.Join(root, "cancelled.bin"), false, func(core.TransferProgress) {
			select {
			case <-started:
			default:
				close(started)
			}
			<-ctx.Done()
		})
	})
	waitOperator(t, a, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	secondFile := filepath.Join(root, "second.txt")
	if err := os.WriteFile(secondFile, []byte("second-session"), 0600); err != nil {
		t.Fatal(err)
	}
	a.prepareUpload(second, []string{secondFile})
	if err := ui.NewTester(a.fileWindowView, 1080, 720).Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	first.files.cancel()
	waitOperator(t, a, func() bool { return !first.files.busy && !second.files.busy })
	if first.files.status != "传输已取消" || !first.conn.Alive() || !second.conn.Alive() {
		t.Fatal("cancel affected another session or shared transport")
	}
	if _, err := os.Stat(filepath.Join(root, "cancelled.bin")); !os.IsNotExist(err) {
		t.Fatal("cancel published incomplete download", err)
	}
	if _, err := second.files.client.Lstat("/second.txt"); err != nil {
		t.Fatal("parallel upload failed", err)
	}
	if _, err := first.files.client.Lstat("/second.txt"); !os.IsNotExist(err) {
		t.Fatal("parallel upload used wrong session", err)
	}

	destination := t.TempDir()
	if err := os.Mkdir(filepath.Join(destination, "目录 空格"), 0700); err != nil {
		t.Fatal(err)
	}
	a.withTransfer(first, func(ctx context.Context, client core.RemoteFileSystem) error {
		return downloadUnique(ctx, client, "/目录 空格", destination, a.transferProgress(first, ctx))
	})
	waitOperator(t, a, func() bool { return !first.files.busy })
	got, err := os.ReadFile(filepath.Join(destination, "目录 空格.1", "数据.bin"))
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatal("recursive download lost content", err, first.files.status)
	}
	for _, name := range []string{"空目录", "零字节.txt"} {
		if _, err := os.Stat(filepath.Join(destination, "目录 空格.1", name)); err != nil {
			t.Fatal(err)
		}
	}
	if a.activePane() != second {
		t.Fatal("background download changed active session")
	}
	// Interrupt an upload while the other session stays active. The refreshed
	// list must agree with the server even if cancellation leaves a partial file.
	uploadStarted := make(chan struct{})
	a.withTransfer(first, func(ctx context.Context, client core.RemoteFileSystem) error {
		return client.Upload(ctx, filepath.Join(tree, "数据.bin"), "/cancel-upload.bin", func(core.TransferProgress) {
			select {
			case <-uploadStarted:
			default:
				close(uploadStarted)
			}
			<-ctx.Done()
		})
	})
	waitOperator(t, a, func() bool {
		select {
		case <-uploadStarted:
			return true
		default:
			return false
		}
	})
	first.files.cancel()
	waitOperator(t, a, func() bool { return !first.files.busy })
	if first.files.status != "传输已取消" || a.activePane() != second {
		t.Fatal("upload cancellation lost status or active session", first.files.status)
	}
	remoteInfo, remoteErr := first.files.client.Lstat("/cancel-upload.bin")
	listed := false
	for _, entry := range first.files.entries {
		if entry.Name() == "cancel-upload.bin" {
			listed = true
			if remoteErr != nil || entry.Size() != remoteInfo.Size() {
				t.Fatal("partial upload size is stale")
			}
		}
	}
	if listed != (remoteErr == nil) {
		t.Fatal("cancelled upload left directory view stale")
	}

	// Closing a transferring tab cancels only that tab's operation.
	closedTransfer := make(chan struct{})
	startedClose := make(chan struct{})
	a.withTransfer(first, func(ctx context.Context, client core.RemoteFileSystem) error {
		defer close(closedTransfer)
		return client.Download(ctx, "/目录 空格/数据.bin", filepath.Join(root, "closed.bin"), false, func(core.TransferProgress) {
			select {
			case <-startedClose:
			default:
				close(startedClose)
			}
			<-ctx.Done()
		})
	})
	waitOperator(t, a, func() bool {
		select {
		case <-startedClose:
			return true
		default:
			return false
		}
	})
	a.closeTab(a.ownerTab(first))
	waitOperator(t, a, func() bool {
		select {
		case <-closedTransfer:
			return true
		default:
			return false
		}
	})
	if a.activePane() != second || !second.conn.Alive() {
		t.Fatal("closing background transfer affected active session")
	}
	if _, err := second.files.client.ReadDir("/"); err != nil {
		t.Fatal("remaining file session unusable", err)
	}
	if _, err := os.Stat(filepath.Join(root, "closed.bin")); !os.IsNotExist(err) {
		t.Fatal("closed transfer published incomplete download", err)
	}
}

func TestRemoteDirectoryActionsKeepTheirOriginalSession(t *testing.T) {
	a, first := connectedFileWorkflow(t)
	other, second := connectedFileWorkflow(t)
	secondTab := other.activeTab()
	other.tabs = nil
	second.owner.Store(a)
	a.tabs = append(a.tabs, secondTab)
	a.openFileAction(first, "新建远程目录")
	a.features.fileName = "report:2026"
	a.selectTab(secondTab)
	if err := ui.NewTester(a.fileWindowView, 1080, 720).Click("应用"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !first.files.busy })
	if _, err := second.files.client.Lstat("/report:2026"); !os.IsNotExist(err) {
		t.Fatal("directory created on wrong session", err)
	}
	first.files.selected = "report:2026"
	a.openFileAction(first, "重命名远程项目")
	a.features.fileName = "NUL"
	a.applyFileAction()
	waitOperator(t, a, func() bool { return !first.files.busy })
	if _, err := first.files.client.Lstat("/NUL"); err != nil {
		t.Fatal("remote-only rename was rejected by Windows naming rules", err)
	}
	// A legitimate server name must remain removable, but a whole batch is
	// rejected before deleting anything if it contains a path traversal entry.
	if err := removeRemoteSelection(context.Background(), first.files.client, "/", []string{"NUL", "../fixture-folder"}); err == nil {
		t.Fatal("delete accepted a path traversal entry")
	}
	if _, err := first.files.client.Lstat("/NUL"); err != nil {
		t.Fatal("invalid batch partially deleted content", err)
	}
	if err := removeRemoteSelection(context.Background(), first.files.client, "/", []string{"NUL"}); err != nil {
		t.Fatal(err)
	}
	if _, err := first.files.client.Lstat("/NUL"); !os.IsNotExist(err) {
		t.Fatal("remote-only deletion failed", err)
	}
	a.loadFiles(first, "/")
	waitOperator(t, a, func() bool { return !first.files.busy })
	a.selectTab(a.tabs[0])
	tt := ui.NewTester(a.fileWindowView, 1080, 720)
	for range 2 {
		if err := tt.Click("fixture-folder"); err != nil {
			t.Fatal(err)
		}
	}
	waitOperator(t, a, func() bool { return !first.files.busy })
	if first.files.dir != "/fixture-folder" {
		t.Fatal("directory double click failed", first.files.dir)
	}
	tt.Frame()
	if err := tt.Click("上级"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !first.files.busy })
	if first.files.dir != "/" || second.files.dir != "/" {
		t.Fatal("directory navigation crossed sessions")
	}
}

func TestFailedUploadRefreshesCompletedBatchItems(t *testing.T) {
	a, p := connectedFileWorkflow(t)
	root := t.TempDir()
	first := filepath.Join(root, "completed.txt")
	if err := os.WriteFile(first, []byte("completed before failure"), 0600); err != nil {
		t.Fatal(err)
	}
	a.prepareUpload(p, []string{first, filepath.Join(root, "missing.txt")})
	if err := ui.NewTester(a.fileWindowView, 1080, 720).Click("确认上传"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !p.files.busy })
	if !p.files.statusError || !strings.HasPrefix(p.files.status, "操作失败") {
		t.Fatal("upload failure was hidden", p.files.status)
	}
	found := false
	for _, entry := range p.files.entries {
		found = found || entry.Name() == "completed.txt"
	}
	if !found {
		t.Fatal("failed batch left completed files invisible until manual refresh")
	}
}
