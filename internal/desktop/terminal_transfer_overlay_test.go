package desktop

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal"
)

func TestTerminalTransferCompletionCountdown(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := &pane{}
	p.owner.Store(a)
	var opts terminal.Options
	a.configureTerminalTransfer(p, &opts, true)
	for _, upload := range []bool{false, true} {
		p.transfer = transferNoticeState{active: true, visible: true, upload: upload}
		opts.TransferFinished(nil)
		a.drain()
		start := p.transfer.dismissAt.Add(-15 * time.Second)
		if p.transfer.active || p.transfer.remaining(start) != 15 || p.transfer.fraction() != 1 {
			t.Fatal(p.transfer)
		}
		if p.transfer.remaining(start.Add(14900*time.Millisecond)) != 1 || !p.transfer.visible {
			t.Fatal("dismissed early")
		}
		p.transfer.remaining(start.Add(15 * time.Second))
		if p.transfer.visible {
			t.Fatal("completion did not expire")
		}
		// A later transfer in this pane must survive the previous deadline.
		p.transfer = transferNoticeState{active: true, visible: true, upload: upload}
		p.transfer.remaining(start.Add(time.Minute))
		if !p.transfer.visible {
			t.Fatal("old deadline dismissed new transfer")
		}
		opts.TransferFinished(errors.New("disk full"))
		a.drain()
		p.transfer.remaining(start.Add(time.Hour))
		if !p.transfer.visible || !p.transfer.dismissAt.IsZero() {
			t.Fatal("error was automatically hidden")
		}
	}
}

func TestTerminalTransferOverlaysAcrossTabs(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
			a.config.Settings.InterfaceTheme = theme
			upload := &pane{id: "upload", profile: core.Profile{Name: "开发服务器"}, transfer: transferNoticeState{
				active: true, visible: true, upload: true, hasProgress: true, speed: 2 << 20,
				progress: transferNoticeProgress{Name: "release.tar.gz", Bytes: 6 << 20, Total: 10 << 20},
			}}
			download := &pane{id: "download", profile: core.Profile{Name: "测试服务器"}, transfer: transferNoticeState{
				visible: true, status: "ZMODEM · 传输完成", dismissAt: time.Now().Add(15 * time.Second), hasProgress: true,
				progress: transferNoticeProgress{Name: "服务日志.txt", Bytes: 4096, Total: 4096},
			}}
			a.tabs = []*tab{{id: "one", root: &node{pane: upload}, active: upload}, {id: "two", root: &node{pane: download}, active: download}}
			clicked := false
			tt := ui.NewTester(func(c *ui.Context) {
				a.theme(c)
				ui.Column(c).Fill().Background(c.Theme().Surface).Label("终端内容").Children(func() {
					ui.Button(c, "继续操作终端").Width(120).OnClick(func() { clicked = true })
				})
				a.transferOverlays(c)
			}, 900, 500)
			for _, text := range []string{"rz 上传", "sz 下载", "release.tar.gz", "服务日志.txt", "15 秒后自动关闭"} {
				if !tt.HasText(text) {
					t.Fatal("missing", text)
				}
			}
			r, ok := tt.Find("rz 上传")
			if !ok || r.X < 500 || r.Y < 38 || r.Y > 90 {
				t.Fatalf("not top right: %+v", r)
			}
			before, _ := tt.Find("终端内容")
			if err := tt.Click("继续操作终端"); err != nil || !clicked {
				t.Fatal("overlay intercepted terminal", err)
			}
			snapshot(t, tt, "zmodem-overlay-"+theme+".png")
			if err := tt.Click("关闭传输进度"); err != nil {
				t.Fatal(err)
			}
			if download.transfer.visible || !upload.transfer.visible {
				t.Fatal("closed wrong transfer")
			}
			upload.transfer.visible = false
			tt.Frame()
			after, _ := tt.Find("终端内容")
			if before != after {
				t.Fatal("overlay resized terminal", before, after)
			}
		})
	}
}
