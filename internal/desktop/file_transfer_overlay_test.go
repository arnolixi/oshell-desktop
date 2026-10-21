package desktop

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func TestFileTransferNoticeLifecycle(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := &pane{files: &fileState{}}
	p.owner.Store(a)
	for _, protocol := range []string{"SFTP", "FTP", "SCP"} {
		for _, upload := range []bool{false, true} {
			task := &fileTransfer{pane: p, files: p.files}
			p.files.activeTransfer = task
			task.beginNotice("/folder/file.bin", protocol, "record", upload, 100)
			a.drain()
			ctx := context.WithValue(context.Background(), fileTransferContextKey{}, task)
			a.transferProgress(p, ctx)(core.TransferProgress{Path: "/folder/file.bin", Bytes: 60})
			a.drain()
			if n := p.files.notice; n == nil || n.protocol != protocol || n.upload != upload || n.fraction() != .6 {
				t.Fatalf("wrong progress: %+v", n)
			}
			a.transferDone(p, task, nil)
			a.drain()
			n := p.files.notice
			if n.active || !n.visible || n.dismissAt.IsZero() || n.fraction() != 1 {
				t.Fatal("missing completion", n)
			}
			n.remaining(n.dismissAt)
			if n.visible {
				t.Fatal("completion not dismissed")
			}
			// Old callbacks must not resurrect a completed notice or replace the next transfer.
			next := &fileTransfer{pane: p, files: p.files}
			p.files.activeTransfer = next
			next.beginNotice("/folder/new.bin", protocol, "next", upload, -1)
			task.beginNotice("/stale", protocol, "stale", upload, 1)
			a.transferDone(p, task, nil)
			a.drain()
			if p.files.notice.recordID != "next" || !p.files.notice.active || p.files.notice.fraction() != -1 {
				t.Fatal("stale task changed new notice")
			}
			next.pause.Store(true)
			a.transferDone(p, next, context.Canceled)
			a.drain()
			p.files.notice.remaining(time.Now().Add(time.Hour))
			if !p.files.notice.visible || !p.files.notice.dismissAt.IsZero() || p.files.notice.status != "传输已暂停" {
				t.Fatal("pause hidden", p.files.notice)
			}
		}
	}
}

func TestFileTransferOverlayActionsAndThemes(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
			a.config.Settings.InterfaceTheme = theme
			f := &fileState{records: &transferLog{records: []*transferRecord{{ID: "record", Resume: true, Status: "传输中"}}}}
			p := &pane{id: "upload", profile: core.Profile{Name: "开发服务器"}, files: f}
			p.owner.Store(a)
			task := &fileTransfer{pane: p, files: f}
			f.activeTransfer = task
			task.beginNotice("/files/release.tar.gz", "SFTP", "record", true, 10<<20)
			a.drain()
			f.notice.progress.Bytes, f.notice.speed = 6<<20, 2<<20
			cancelled := false
			f.cancel = func() { cancelled = true }
			q := &pane{id: "download", profile: core.Profile{Name: "测试服务器"}, files: &fileState{notice: &fileTransferNotice{
				transferNoticeState: transferNoticeState{visible: true, hasProgress: true, status: "传输完成", dismissAt: time.Now().Add(15 * time.Second), progress: transferNoticeProgress{Name: "服务日志.txt", Bytes: 4096, Total: 4096}}, protocol: "FTP",
			}}}
			a.tabs = []*tab{{id: "one", root: &node{pane: p}, active: p}, {id: "two", root: &node{pane: q}, active: q}}
			tt := ui.NewTester(func(c *ui.Context) {
				a.theme(c)
				ui.Column(c).Fill().Background(c.Theme().Surface)
				a.transferOverlays(c)
			}, 900, 500)
			for _, label := range []string{"SFTP 上传", "FTP 下载", "暂停", "15 秒后自动关闭"} {
				if !tt.HasText(label) {
					t.Fatal("missing", label)
				}
			}
			snapshot(t, tt, "file-transfer-overlay-"+theme+".png")
			if err := tt.Click("关闭传输进度"); err != nil {
				t.Fatal(err)
			}
			if q.files.notice.visible || !f.notice.visible {
				t.Fatal("closed wrong notice")
			}
			if err := tt.Click("暂停"); err != nil || !cancelled || !task.pause.Load() {
				t.Fatal("pause not wired", err)
			}
			cancelled = false
			if err := tt.Click("取消传输"); err != nil || !cancelled {
				t.Fatal("cancel not wired", err)
			}
		})
	}
}
