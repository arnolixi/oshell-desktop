// SPDX-License-Identifier: GPL-3.0-only
package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal"
	"oshell-desktop/internal/zmodem"
)

type terminalTransferState struct {
	active  bool
	visible bool
	status  string
}

func postPane(p *pane, fn func(*App)) bool {
	if owner := p.owner.Load(); owner != nil {
		return owner.post(func() {
			if current := p.owner.Load(); current != nil && !p.closed {
				fn(current)
			}
		})
	}
	return false
}

func (a *App) configureTerminalTransfer(p *pane, opts *terminal.Options, enabled bool) {
	opts.AutomaticTransfers = enabled
	opts.TransferHandler = func(ctx context.Context, upload bool, conn net.Conn) error {
		window := make(chan *mygo.Window, 1)
		if !postPane(p, func(owner *App) {
			p.transfer = terminalTransferState{active: true, visible: true, status: "ZMODEM · 等待选择文件"}
			window <- owner.win
		}) {
			return context.Canceled
		}
		var parent *mygo.Window
		select {
		case parent = <-window:
		case <-ctx.Done():
			return ctx.Err()
		}
		paths, err := chooseTransferPaths(ctx, parent, upload)
		if err != nil {
			return err
		}
		started, last := time.Now(), time.Time{}
		progress := func(value zmodem.FileProgress) {
			now := time.Now()
			if now.Sub(last) < 100*time.Millisecond && value.Bytes != value.Total {
				return
			}
			last = now
			status := fmt.Sprintf("ZMODEM · %s · %s / %s · %s/s", value.Name, bytesLabel(value.Bytes), bytesLabel(value.Total), bytesLabel(int64(float64(value.Bytes)/max(now.Sub(started).Seconds(), 0.1))))
			postPane(p, func(*App) { p.transfer.status = status })
		}
		if upload {
			var files []zmodem.SendFile
			for _, path := range paths {
				file, e := os.Open(path)
				if e != nil {
					return e
				}
				defer file.Close()
				info, e := file.Stat()
				if e != nil {
					return e
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("只能上传普通文件：%s", info.Name())
				}
				files = append(files, zmodem.SendFile{Name: info.Name(), Size: info.Size(), Reader: file})
			}
			return zmodem.SendFiles(conn, files, progress)
		}
		var rejected []string
		err = zmodem.ReceiveFiles(conn, func(file zmodem.ReceiveFile) (io.WriteCloser, error) {
			writer, e := newTransferFile(paths[0], file.Name)
			if e != nil {
				rejected = append(rejected, file.Name+"："+e.Error())
			}
			return writer, e
		}, progress)
		if len(rejected) > 0 && err == nil {
			postPane(p, func(*App) {
				p.transfer.status = "ZMODEM · 已结束，跳过 " + strings.Join(rejected, "；")
				p.transfer.active = false
			})
		}
		return err
	}
	opts.TransferFinished = func(err error) {
		postPane(p, func(*App) {
			if !p.transfer.active {
				return
			}
			p.transfer.active = false
			switch {
			case err == nil:
				p.transfer.status = "ZMODEM · 传输完成"
			case errors.Is(err, context.Canceled) || errors.Is(err, zmodem.ErrCancelled):
				p.transfer.status = "ZMODEM · 已取消"
			default:
				p.transfer.status = "ZMODEM · 传输失败：" + err.Error()
			}
		})
	}
}

func chooseTransferPaths(ctx context.Context, parent *mygo.Window, upload bool) ([]string, error) {
	type result struct {
		paths []string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		title := "选择 ZMODEM 下载目录（同名文件将跳过）"
		if upload {
			title = "选择 ZMODEM 上传文件"
		}
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: title, Multiple: upload, Directory: !upload, CreateDirectories: !upload})
		ch <- result{paths, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		if len(r.paths) == 0 {
			return nil, context.Canceled
		}
		return r.paths, ctx.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Complete downloads become visible in one operation without replacing an
// existing file. Failed/cancelled transfers leave no partial destination.
type transferFile struct {
	file    *os.File
	target  string
	aborted error
	closed  bool
}

func newTransferFile(dir, name string) (*transferFile, error) {
	if !core.SafeTransferName(name) {
		return nil, fmt.Errorf("无效的文件名")
	}
	target := filepath.Join(dir, name)
	if _, e := os.Lstat(target); !os.IsNotExist(e) {
		return nil, fmt.Errorf("目标已存在或无法访问")
	}
	f, e := os.CreateTemp(dir, ".oshell-zmodem-*")
	if e != nil {
		return nil, e
	}
	return &transferFile{file: f, target: target}, nil
}
func (f *transferFile) Write(b []byte) (int, error) { return f.file.Write(b) }
func (f *transferFile) Abort(e error)               { f.aborted = e }
func (f *transferFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	defer os.Remove(f.file.Name())
	if f.aborted != nil {
		f.file.Close()
		return f.aborted
	}
	if e := f.file.Sync(); e != nil {
		f.file.Close()
		return e
	}
	if e := f.file.Close(); e != nil {
		return e
	}
	return core.PublishDownloadedFile(f.file.Name(), f.target)
}
func (a *App) terminalTransferView(c *ui.Context, p *pane) {
	if !p.transfer.visible {
		return
	}
	ui.Row(c).Padding(5, 8).Gap(8).Background(c.Theme().Surface).Children(func() {
		ui.Text(c, p.transfer.status).SingleLine().Grow(1).FontSize(12)
		if p.transfer.active {
			ui.Button(c, "取消传输").OnClick(p.term.CancelTransfer)
		} else {
			ui.Button(c, "关闭").OnClick(func() { p.transfer.visible = false })
		}
	})
}
