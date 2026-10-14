package desktop

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func (a *App) syncLocationSettings(c *ui.Context) {
	f := &a.features.sharedSync
	ui.Text(c, "本地数据目录（始终使用）").Bold()
	ui.Text(c, a.store.Dir).Wrap()
	ui.Row(c).Gap(8).Children(func() {
		ui.Button(c, "打开本地目录").OnClick(func() {
			if err := mygo.Shell.OpenPath(a.store.Dir); err != nil {
				f.status = err.Error()
			}
		})
		ui.Button(c, "设置 / 解锁主密码…").OnClick(a.showVault)
	})
	ui.Text(c, "iCloud / 自定义同步目录").Bold()
	ui.Row(c).Gap(8).Children(func() {
		ui.Button(c, "新建同步目录…").Disabled(f.busy || a.configurationBusy()).OnClick(func() { a.chooseSyncDirectory("create") })
		ui.Button(c, "连接已有同步目录…").Disabled(f.busy || a.configurationBusy()).OnClick(func() { a.chooseSyncDirectory("existing") })
	})
	if f.pendingDirectory != nil {
		ui.Text(c, "重启后生效："+f.pendingDirectory.Path).Wrap()
		ui.Button(c, "取消待生效设置").Disabled(f.busy).OnClick(func() {
			f.pendingDirectory = nil
			f.directoryChanged = true
			f.status = "取消目录设置已暂存，点击应用后生效"
		})
	}
	ui.Button(c, "停用目录同步").Disabled(f.busy || a.configurationBusy()).OnClick(func() {
		f.directoryDisabled = true
		f.directoryChanged = true
		f.pendingDirectory = nil
		f.status = "停用目录同步已暂存，点击应用后生效"
	})
	ui.Text(c, "会话始终在本地读写。新目录需为空，已有目录需包含可解密的同步数据；点击应用后保存，重启生效。").FontSize(11).TextColor(c.Theme().TextMuted)
}
func (a *App) chooseSyncDirectory(mode string) {
	f := &a.features.sharedSync
	if f.busy || a.configurationBusy() {
		return
	}
	master, err := a.vault.MasterSecret()
	if err != nil {
		f.status = err.Error()
		return
	}
	f.busy = true
	a.setConfigurationBusy(true)
	parent := a.preferencesWindow
	generation := f.directoryGeneration
	go func() {
		defer clear(master)
		var pending *core.PendingSyncDirectory
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "选择加密同步目录", Directory: true})
		if err == nil && len(paths) > 0 {
			pending, err = core.ValidateSyncDirectorySelection(a.store.Dir, paths[0], mode, master)
		}
		a.post(func() {
			f.busy = false
			a.setConfigurationBusy(false)
			if generation != f.directoryGeneration || !a.settingsOpen {
				return
			}
			if err != nil {
				f.status = err.Error()
				return
			}
			if len(paths) > 0 {
				f.pendingDirectory = pending
				f.directoryChanged = true
				f.directoryDisabled = false
				f.status = "目录选择已暂存，点击应用后保存，重启后启用"
			}
		})
	}()
}
