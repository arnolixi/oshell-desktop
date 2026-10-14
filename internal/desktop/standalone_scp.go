package desktop

import (
	"context"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path"
	"path/filepath"
	"strings"
)

type standaloneSCPDraft struct {
	open        bool
	pane        *pane
	remote, err string
}

func (a *App) openStandaloneSCP(p *pane) {
	if p == nil || p.closed || p.conn == nil || !p.conn.Alive() || p.files == nil || p.files.busy {
		return
	}
	a.features.scpDraft = &standaloneSCPDraft{open: true, pane: p, remote: p.files.dir}
}
func (a *App) standaloneSCPDialog(c *ui.Context) {
	draft := a.features.scpDraft
	if draft == nil {
		return
	}
	run := func(upload bool) {
		p := draft.pane
		remote := draft.remote
		if p.closed || p.conn == nil || !p.conn.Alive() || p.files == nil || p.files.busy {
			draft.err = "连接不可用"
			return
		}
		if strings.ContainsAny(remote, "\x00\r\n") || !core.SafeTransferName(path.Base(remote)) {
			draft.err = "请填写完整的远程文件或目录路径"
			return
		}
		a.features.scpDraft = nil
		connection := p.conn
		a.withFileOperation(p, p.files.client != nil, func(ctx context.Context) error {
			if upload {
				paths, err := chooseUploadItems(a.win, false)
				if err != nil {
					return err
				}
				if len(paths) == 0 {
					return context.Canceled
				}
				if len(paths) != 1 {
					return fmt.Errorf("独立 SCP 每次请选择一个文件或目录")
				}
				return connection.SCPUpload(ctx, paths[0], remote, a.transferProgress(p, ctx))
			}
			directories, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "选择 SCP 下载保存目录", Directory: true})
			if err != nil {
				return err
			}
			if len(directories) == 0 {
				return context.Canceled
			}
			entries, err := os.ReadDir(directories[0])
			if err != nil {
				return err
			}
			names := map[string]bool{}
			for _, entry := range entries {
				names[entry.Name()] = true
			}
			target := filepath.Join(directories[0], uniqueTransferName(path.Base(remote), names))
			return connection.SCPDownload(ctx, remote, target, false, a.transferProgress(p, ctx))
		})
	}
	ui.Modal(c.Key("standalone-scp"), &draft.open, func() {
		ui.Column(c).Width(530).Gap(10).Children(func() {
			ui.Text(c, "SCP 文件传输").Bold().FontSize(18)
			ui.Text(c, "可用于没有 SFTP 子系统的 SSH 服务器。填写完整远程路径；上传可能覆盖该路径，下载重名自动编号。")
			ui.TextInput(c, &draft.remote).Label("SCP 远程路径").AutoFocus()
			if draft.err != "" {
				ui.Text(c, draft.err).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				ui.Button(c, "取消").OnClick(func() { a.features.scpDraft = nil })
				ui.Button(c, "上传…").OnClick(func() { run(true) })
				ui.Button(c, "下载…").OnClick(func() { run(false) })
			})
		})
	})
	if !draft.open {
		a.features.scpDraft = nil
	}
}
