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
	if f.pendingDirectory != nil {
		ui.Text(c, "旧版待生效目录："+f.pendingDirectory.Path).Wrap()
		ui.Button(c, "取消待生效设置").OnClick(func() { f.pendingDirectory = nil; f.directoryChanged = true })
	}
	ui.Text(c, "只切换同步目标，本机工作目录和原共享文件均保留。").TextColor(c.Theme().TextMuted)
}

func (a *App) selectSyncMethod(method string) {
	f := &a.features.sharedSync
	f.directoryGeneration++
	f.method = method
	a.cancelWebDAVProbe()
	f.options.Enabled = method != "none"
	if method != "none" && method != f.options.Backend {
		f.options.Backend = method
		f.options.Address, f.options.Username, f.password = "", "", ""
		f.options.TrustedServerCertificate = core.WebDAVCertificate{}
		f.options.RequireExisting = false
	}
	f.status = "尚未应用，当前同步目标保持不变"
	if f.options.Enabled {
		protected, locked := a.vault.State()
		if !protected || locked {
			a.showVault()
		}
	}
}

func (a *App) chooseLiveSyncDirectory() {
	f := &a.features.sharedSync
	generation := f.directoryGeneration
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.preferencesWindow, Title: "选择同步目录（自动识别已有配置）", Directory: true})
		a.post(func() {
			if !a.settingsOpen || generation != f.directoryGeneration || f.options.Backend != "directory" {
				return
			}
			if err != nil {
				f.status = err.Error()
			} else if len(paths) > 0 {
				f.options.Address = paths[0]
				f.status = "目录已选择，点击应用立即生效"
			}
		})
	}()
}
