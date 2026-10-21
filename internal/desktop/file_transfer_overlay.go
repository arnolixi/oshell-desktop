package desktop

import (
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type fileTransferNotice struct {
	transferNoticeState
	protocol, recordID string
}

func (task *fileTransfer) beginNotice(source, protocol, recordID string, upload bool, total int64) {
	if task == nil || task.pane == nil {
		return
	}
	postPane(task.pane, func(*App) {
		if task.pane.files != task.files || task.files.activeTransfer != task {
			return
		}
		if task.notice == nil {
			task.notice = &fileTransferNotice{}
		}
		*task.notice = fileTransferNotice{
			transferNoticeState: transferNoticeState{active: true, visible: true, upload: upload, hasProgress: true,
				status: "正在传输", progress: transferNoticeProgress{Name: path.Base(strings.ReplaceAll(source, "\\", "/")), Total: total}},
			protocol: protocol, recordID: recordID,
		}
		task.files.notice = task.notice
		task.lastBytes, task.lastProgress = 0, time.Now()
	})
}

func (a *App) fileTransferCard(c *ui.Context, p *pane, now time.Time) {
	f, notice := p.files, p.files.notice
	direction := notice.protocol + " 下载"
	if notice.upload {
		direction = notice.protocol + " 上传"
	}
	var resumable, paused bool
	if f.records != nil {
		for _, r := range f.records.snapshot() {
			if r.ID == notice.recordID {
				resumable, paused = r.Resume, r.Status == "已暂停"
				break
			}
		}
	}
	transferNoticeCard(c, &notice.transferNoticeState, direction, p.profile.Name, now, func() {
		if notice.active {
			if resumable {
				ui.Button(c, "暂停").Height(24).OnClick(func() {
					if f.activeTransfer != nil && f.cancel != nil {
						f.activeTransfer.pause.Store(true)
						f.cancel()
					}
				})
			}
			ui.Button(c, "取消").Label("取消传输").Height(24).OnClick(func() {
				if f.cancel != nil {
					f.cancel()
				}
			})
		} else if resumable {
			label := "重试"
			if paused {
				label = "继续"
			}
			ui.Button(c, label).Height(24).Disabled(f.busy).OnClick(func() { a.resumeTransfer(p, notice.recordID) })
		}
	})
}
