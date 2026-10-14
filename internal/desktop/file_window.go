package desktop

import (
	"context"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"strings"
	"time"
)

// External file launches share a dedicated window. They must never consume a
// terminal workspace tab or silently replace its active SSH session.
func (h *applicationHub) externalFileWindow() *App {
	if h.fileWindow != nil {
		return h.fileWindow
	}
	a := newApp(h.store, h.config.Clone())
	a.hub = h
	a.vault = h.vault
	a.trust = h.trust
	a.win = mygo.NewWindow(mygo.WindowOptions{Title: "文件管理", Width: 1080, Height: 720, MinWidth: 760, MinHeight: 480, Content: ui.View(a.fileWindowView)})
	h.fileWindow = a
	a.win.OnClose(func(event *mygo.CloseEvent) {
		if a.features.localConflict.active {
			event.PreventDefault()
			a.cancelLocalConflict()
			return
		}
		if a.features.sessionPassword.busy {
			event.PreventDefault()
			a.cancelSessionPassword()
			return
		}

		if !h.quitting && !a.features.allowClose && a.activeConnections() > 0 {
			event.PreventDefault()
			a.confirmExit(false)
		}
	})
	a.win.OnClosed(func() {
		a.cancelSessionPassword()
		a.stopped.Store(true)
		for _, tab := range a.tabs {
			a.walk(tab.root, a.dispose)
		}
		a.drain()
		if h.fileWindow == a {
			h.fileWindow = nil
		}
		h.scheduleSharedSync(time.Minute)
	})
	h.scheduleSharedSync(2 * time.Second)
	return a
}
func (a *App) fileWindowView(c *ui.Context) {
	a.drain()
	if auxiliaryEscape(c, func() {
		if a.win != nil {
			a.win.Close()
		}
	}) {
		return
	}
	a.theme(c)
	if a.lockedWorkspace(c) {
		return
	}
	ui.Column(c).Fill().MinHeight(0).Children(func() {
		ui.Row(c).Padding(8).Gap(8).Children(func() {
			ui.MenuButton(c, "新会话…", func(m *ui.Menu) {
				if m.Item("新建 SFTP 会话…").Chosen() {
					a.edit(a.newProfile("sftp"))
				}
				if m.Item("新建 FTP 会话…").Chosen() {
					a.edit(a.newProfile("ftp"))
				}
				m.Separator()
				if m.Item("选择已保存会话…").Chosen() {
					a.showFileSessionPicker()
				}

			})
			ui.Button(c, "复制文件标签").Disabled(a.activePane() == nil).OnClick(a.duplicateFileSession)
			ui.Button(c, "重连").Disabled(a.activePane() == nil).OnClick(a.reconnectActive)
		})
		ui.ScrollHorizontal(c).Height(34).Children(func() {
			ui.Row(c).Gap(4).Children(func() {
				for _, tab := range a.tabs {
					title := tab.name
					if tab.active != nil {
						title = tab.active.profile.Host
					}
					ui.Row(c.Key(tab.id)).Gap(2).Children(func() {
						button := ui.Button(c, title).Tooltip("双击已连接的会话可新建相同文件标签")
						a.tabTitleEvents(button, tab)
						ui.Button(c, "×").Label("关闭文件会话 " + title).OnClick(func() { a.requestSessionClose(tab, nil) })
					})
				}
			})
		})
		if p := a.activePane(); p != nil {
			if (p.conn != nil && !p.conn.Alive()) || strings.HasPrefix(p.status, "连接失败") || strings.HasPrefix(p.status, "FTP 连接失败") {
				ui.Row(c).Padding(8).Gap(8).Children(func() {
					ui.Text(c, "文件连接已断开").Grow(1)
					ui.Button(c, "重新连接").OnClick(a.reconnectActive)
				})
			}
			if p.files != nil {
				a.fileBrowser(c, p, true)
			} else {
				ui.Text(c, p.status).Padding(20)
			}
		} else {
			ui.Text(c, "从会话管理或外部文件启动入口打开 FTP / SFTP 连接。").Padding(24)
		}
		if strings.Contains(a.status, "密码") || strings.Contains(a.status, "SSH 连接") {
			ui.Text(c, a.status).FontSize(11).TextColor(c.Theme().TextMuted).Padding(8)
		}
	})
	a.dialogs(c)
}
func (h *applicationHub) openExternalFile(profile core.Profile, credentials core.Credentials) {
	a := h.externalFileWindow()
	a.open(profile, credentials, 0)
	a.win.Focus()
}

func (h *applicationHub) openFilesFromPane(source *pane) {
	if source == nil || source.closed || source.conn == nil {
		return
	}
	target := h.externalFileWindow()
	profile := source.profile
	profile.Kind = "sftp"
	if source.files != nil && source.files.dir != "" {
		profile.InitialDirectory = source.files.dir
	}
	target.openConnection(profile, core.Credentials{ExternalLaunch: source.externalLaunch}, 0, source.conn)
	target.win.Focus()
}

func observeFileConnection(ctx context.Context, p *pane, conn *core.Connection) {
	go func() {
		if conn.WaitClosed(ctx) != nil {
			return
		}
		postPane(p, func(a *App) {
			if p.closed || p.conn != conn {
				return
			}
			p.status = "文件连接已断开"
			if p.files != nil {
				p.files.status = "文件连接已断开"
			}
		})
	}()
}

// Keep the saved profile identity for credential lookup while opening only SFTP.
func (a *App) opensAsFileSession(profile core.Profile) bool {
	return profile.Kind == "sftp" || profile.Kind == "ssh" && a.hub != nil && a.hub.fileWindow == a
}

func (a *App) reopenSSHFileSession(p *pane, replace bool) {
	if p == nil || p.closed {
		return
	}
	if p.conn == nil || !p.conn.Alive() {
		a.status = "原 SSH 连接已失效，无法复用。请从新会话入口重新连接。"
		if p.files != nil {
			p.files.status = a.status
		}
		return
	}
	profile := p.profile
	if p.files != nil && p.files.dir != "" {
		profile.InitialDirectory = p.files.dir
	}
	split := 0
	if replace {
		split = -1
		a.features.reconnectTarget = p
	}
	a.openConnection(profile, core.Credentials{ExternalLaunch: p.externalLaunch}, split, p.conn)
}
func (a *App) duplicateFileSession() {
	p := a.activePane()
	if p == nil || p.closed {
		return
	}
	if a.opensAsFileSession(p.profile) {
		a.reopenSSHFileSession(p, false)
		return
	}
	if p.profile.Kind == "ftp" && p.ftp != nil {
		profile := p.profile
		if p.files != nil && p.files.dir != "" {
			profile.InitialDirectory = p.files.dir
		}
		a.openConnection(profile, core.Credentials{ExternalLaunch: p.externalLaunch}, 0, nil, p.ftp)
	}
}
