// SPDX-License-Identifier: GPL-3.0-only
package desktop

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"strconv"
	"strings"
)

func (a *App) newBlank() {
	p := core.NewProfile()
	p.Kind = "blank"
	p.Name = "本机网络工具"
	a.open(p, core.Credentials{}, 0)
}
func (a *App) showLiveProperties() {
	p := a.activePane()
	if !canEditLiveKeepAlive(p) {
		return
	}
	f := &a.features
	f.livePane = p
	f.liveKeep = p.profile.KeepAlive
	f.liveInterval = strconv.Itoa(f.liveKeep.IdleInterval)
	f.liveError = ""
	f.liveOpen = true
}
func (a *App) livePropertiesDialog(c *ui.Context) {
	f := &a.features
	ui.Modal(c, &f.liveOpen, func() {
		ui.Column(c).Width(430).Gap(12).Children(func() {
			ui.Text(c, "当前会话空闲保活").FontSize(18).Bold()
			ui.Checkbox(c, &f.liveKeep.IdleEnabled, "空闲时发送文本")
			ui.TextInput(c, &f.liveInterval).Label("间隔（秒）")
			ui.TextInput(c, &f.liveKeep.IdleText).Label(`发送文本（支持 \n、\r、\t、\e 和 \\）`)
			ui.Text(c, "仅影响当前连接，不改写保存的会话配置。").TextColor(c.Theme().TextMuted)
			if f.liveError != "" {
				ui.Text(c, f.liveError).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				ui.Button(c, "取消").OnClick(func() { f.liveOpen = false })
				ui.PrimaryButton(c, "应用到当前连接").OnClick(func() {
					v, e := strconv.Atoi(f.liveInterval)
					if e != nil {
						f.liveError = "间隔需为整数"
						return
					}
					p := f.livePane
					if p == nil || p.closed || p.conn == nil {
						f.liveError = "连接已关闭"
						return
					}
					f.liveKeep.IdleInterval = v
					if e = p.conn.UpdateIdleKeepAlive(f.liveKeep); e != nil {
						f.liveError = e.Error()
						return
					}
					p.profile.KeepAlive = f.liveKeep
					f.liveOpen = false
				})
			})
		})
	})
}
func (a *App) reconnectActive() {
	p := a.activePane()
	if p == nil {
		return
	}
	if p.profile.Kind == "ftp" && p.ftp != nil {
		a.reconnectFTPFiles(p)
		return
	}
	if a.opensAsFileSession(p.profile) && !p.terminalOnly {
		a.reopenSSHFileSession(p, true)
		return
	}

	begin := func() {
		profile := p.profile
		// Saved keepalive options apply on a new connection. Reconnecting an
		// existing tab must not silently retain its old disabled/default values.
		// Keep the current host identity and one-off connections unchanged.
		if !p.externalLaunch {
			for _, saved := range a.config.Profiles {
				if strings.EqualFold(saved.ID, profile.ID) && saved.Kind == profile.Kind && saved.Identity() == profile.Identity() {
					profile.KeepAlive = saved.KeepAlive
					break
				}
			}
		}
		a.features.reconnectTarget = p
		a.request(profile, -1, p.terminalOnly)
	}
	if a.win == nil || p.term == nil || !p.term.Available() || (p.term.InLocalToolsMode() && !p.term.ToolsProcessActive()) {
		begin()
		return
	}
	go func() {
		r, e := mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win, Title: "重新连接", Message: "结束当前连接或本机工具，并重新建立原会话？", Buttons: []string{"取消", "重新连接"}, DefaultButton: 0, CancelButton: 0})
		a.post(func() {
			if e == nil && r.Button == 1 && !p.closed {
				begin()
			}
		})
	}()
}

func canEditLiveKeepAlive(p *pane) bool {
	return p != nil && !p.closed && p.conn != nil && p.term != nil && p.term.Available() && !p.term.InLocalToolsMode()
}
