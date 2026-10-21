package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

type syncPresentation struct {
	title, guidance, mark string
	color                 ui.Color
}

func (a *App) syncStatusOwner() *App {
	if a.hub != nil && a.hub.syncOwner != nil {
		return a.hub.syncOwner
	}
	return a
}
func (a *App) syncOperationBusy() bool {
	if a.syncStatusOwner().features.sharedSync.busy {
		return true
	}
	if a.hub != nil {
		for _, w := range a.hub.configurationWindows() {
			if w.features.sharedSync.busy {
				return true
			}
		}
	}
	return a.features.sharedSync.busy
}
func (a *App) invalidateSyncPresentation() {
	if a.hub == nil {
		return
	}
	a.hub.syncOwner = a
	for _, w := range a.hub.configurationWindows() {
		if w.win != nil {
			w.win.Invalidate()
		}
		if w.preferencesWindow != nil {
			w.preferencesWindow.Invalidate()
		}
	}
}
func (a *App) syncPresentation(c *ui.Context) syncPresentation {
	owner := a.syncStatusOwner()
	f := &owner.features.sharedSync
	if !f.loaded && !f.busy {
		owner.loadSharedSyncSettings()
	}
	p := syncPresentation{"未启用同步", "选择一种同步方式；配置目前只保存在本机。", "−", c.Theme().TextMuted}
	if !f.savedEnabled {
		return p
	}
	p.color = c.Theme().Accent
	switch {
	case a.passwordLocked():
		p.title, p.guidance, p.mark = "等待解锁主密码", "解锁后继续同步，本地修改已保留。", "◇"
	case f.busy:
		p.title, p.guidance, p.mark = "正在同步", "正在比较或应用共享配置，请稍候。", "↻"
	case f.pendingDraft || f.plan != nil && (f.plan.InitialChoiceRequired || len(f.plan.Conflicts) > 0):
		p.title, p.guidance, p.mark, p.color = "同步需要确认", "关闭设置后同步，继续首次接入或冲突处理。", "!", c.Theme().Danger
	case f.phase == core.SyncFailed:
		p.title, p.guidance, p.mark, p.color = "同步未完成", "本地数据已保留；检查连接、证书和诊断日志后重试。", "!", c.Theme().Danger
	case f.phase == core.SyncCancelled:
		p.title, p.guidance, p.mark = "本次同步已取消", "本地修改已保留，可以重新安排同步。", "Ⅱ"
	case f.manualQueued || f.deferred || f.pendingLocal:
		p.title, p.guidance, p.mark = "等待同步", "使用已保存的配置；关闭设置后继续，未应用的草稿不会生效。", "◷"
	case f.summary.LastSuccess != nil:
		p.title, p.guidance, p.mark, p.color = "同步完成", "本机与共享配置已对齐。", "✓", ui.Hex("#32B85A")
		if f.savedOptions.Backend == "directory" {
			p.title = "已与本机同步目录对齐"
			p.guidance = "目录文件已对齐；云端上传进度请在云盘客户端查看。"
		}
	default:
		p.title, p.guidance, p.mark = "等待同步", "将比较本机与共享配置，无需连接 SSH。", "◷"
	}
	return p
}
func (a *App) syncToolbar(c *ui.Context) {
	p := a.syncPresentation(c)
	f := &a.syncStatusOwner().features.sharedSync
	status := map[string]string{"−": "未启用", "◇": "待解锁", "↻": "同步中", "!": "需处理", "Ⅱ": "已取消", "◷": "待同步", "✓": "已同步"}[p.mark]
	width, _ := c.Size()
	compact := width < 1120
	color := c.Theme().Accent
	if p.mark == "!" || p.mark == "✓" {
		color = p.color
	}
	button := ui.ButtonBase(c).Label("多设备同步").Role(ui.RoleButton).
		Tooltip(p.title+"\n最近成功："+syncDate(f.summary.LastSuccess)+"\n点击打开同步设置").
		Height(28).Padding(4, 8).Gap(5).Radius(6).Shrink(0).Cursor(ui.CursorPointer).
		Background(color.Alpha(.10)).Border(1, color.Alpha(.24)).
		OnClick(func() { a.showPreferenceSection(3) })
	if button.Pressed() {
		button.Background(color.Alpha(.22))
	} else if button.Hovered() || button.FocusVisible() {
		button.Background(color.Alpha(.17)).BorderColor(color.Alpha(.50))
	}
	button.Children(func() {
		ui.Icon(c, syncIcon).FontSize(17).TextColor(color)
		label := "同步 · " + status
		if compact {
			label = "同步"
		}
		ui.Text(c, label).FontSize(12).Bold().TextColor(color).SingleLine()
		if compact {
			ui.Box(c).Size(5, 5).Radius(3).Background(p.color)
		}
	})
}

func (a *App) sharedSyncFailed(err error) {
	f := &a.features.sharedSync
	if !f.loaded {
		a.loadSharedSyncSettings()
	}
	f.status, f.phase = err.Error(), core.SyncFailed
	a.status = f.status
	a.invalidateSyncPresentation()
}
