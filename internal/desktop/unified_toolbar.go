package desktop

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) toolbar(c *ui.Context) {
	if runtime.GOOS != "windows" {
		a.legacyToolbar(c)
		return
	}
	if p := a.activePane(); p != nil && p.term != nil {
		if _, _, err := p.term.LogState(); err != nil {
			a.status = "日志记录失败：" + err.Error()
		}
	}
	ui.Toolbar(c, func() {
		for _, title := range []string{"OShell", "会话", "快捷连接", "编辑", "视图", "窗口", "快速命令", "工具"} {
			if title == "编辑" {
				a.headerEditButton(c)
				continue
			}
			ui.MenuButton(c, title, func(m *ui.Menu) { a.headerMenu(m, title) }).Height(25).FontSize(12)
		}
		label := "标签组"
		if n := a.hiddenUnreadGroups(); n > 0 {
			label = fmt.Sprintf("标签组 · %d", n)
		}
		groups := ui.MenuButton(c, label, a.groupsMenu).Label("标签组").Tooltip("新建、命名和显示/隐藏标签组").Height(25)
		if a.hiddenUnreadGroups() > 0 {
			groups.TextColor(c.Theme().Accent)
		}
		ui.MenuButton(c, "排列", a.headerArrangementMenu).Height(25).Disabled(len(a.tabs) == 0)
		ui.Spacer(c)
		ui.Box(c).Width(1).Height(18).Background(c.Theme().Border)
		toolButton(c, "会话管理", terminalIcon, "会话管理").Shrink(0).OnClick(func() { a.performAction("sessionManager") })
		toolButton(c, "文件管理", folderIcon, "文件管理").Shrink(0).OnClick(func() { a.performAction("fileManager") })
	}).Height(38).Padding(6, 8).Gap(5).Background(c.Theme().Background).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Label("主工具栏")
}

// Build on opening so saved sessions, commands and shortcuts are current.
// Reuse the application menu's actions, including modal and password guards.
func (a *App) headerItems(title string) []*mygo.MenuItem {
	h := a.hub
	if h == nil {
		h = &applicationHub{config: a.config, windows: []*App{a}}
	}
	for _, root := range h.menuItems() {
		if root.Label != title {
			continue
		}
		var out []*mygo.MenuItem
		for _, item := range root.Submenu {
			omit := false
			switch title {
			case "OShell":
				omit = item.ID == "newWindow" || item.ID == "newBlank" || item.ID == "local" || item.ID == "newSession"
			case "会话":
				omit = item.ID == "newWindow" || item.ID == "quickSwitch" || item.ID == "closePane" || item.ID == "closeTab"
			case "视图":
				omit = item.ID == "newGroup" || item.ID == "showGroups" || strings.HasPrefix(item.ID, "arrange")
				if item.ID == "splitVertical" {
					item.Label = "左右分屏"
				}
				if item.ID == "splitHorizontal" {
					item.Label = "上下分屏"
				}
			case "工具":
				omit = item.ID == "quickCommands" || item.ID == "appearance"
			}
			if omit || item.Type == mygo.MenuItemSeparator && (len(out) == 0 || out[len(out)-1].Type == mygo.MenuItemSeparator) {
				continue
			}
			out = append(out, item)
		}
		if len(out) > 0 && out[len(out)-1].Type == mygo.MenuItemSeparator {
			out = out[:len(out)-1]
		}
		return out
	}
	return nil
}

func (a *App) headerMenu(m *ui.Menu, title string) {
	if title == "快捷连接" {
		a.connectionMenu(m)
		return
	}
	a.headerMenuItems(m, a.headerItems(title))
	switch title {
	case "会话":
		m.Separator()
		for _, format := range []string{"SecureCRT", "Xshell"} {
			m.Submenu("导入 "+format, func(child *ui.Menu) {
				if child.Item("选择会话文件…").Chosen() {
					a.importThirdParty(format, false)
				}
				if child.Item("选择会话目录…").Chosen() {
					a.importThirdParty(format, true)
				}
			})
		}
		if m.Item("复制 SSH 渠道（免认证）").Disabled(a.activePane() == nil || a.activePane().conn == nil).Chosen() {
			a.cloneActive()
		}
	case "窗口":
		m.Separator()
		if m.Item("移动当前标签到新窗口").Disabled(a.activeTab() == nil).Chosen() {
			a.moveTabToNewWindow()
		}
	case "工具":
		m.Separator()
		if m.Item("保存终端历史文本…").Disabled(a.activePane() == nil || a.activePane().term == nil).Chosen() {
			a.exportScreen()
		}
	}
}

func (a *App) headerMenuItems(m *ui.Menu, items []*mygo.MenuItem) {
	for _, item := range items {
		if item.Hidden {
			continue
		}
		if item.Type == mygo.MenuItemSeparator {
			m.Separator()
			continue
		}
		if len(item.Submenu) > 0 {
			m.Submenu(item.Label, func(child *ui.Menu) { a.headerMenuItems(child, item.Submenu) }).Disabled(item.Disabled)
			continue
		}
		entry := m.Item(item.Label).Disabled(item.Disabled)
		switch item.ID {
		case "links":
			entry.Checked(a.config.Settings.LinksVisible)
		case "quickSend":
			entry.Checked(a.config.Settings.QuickSendVisible)
		case "composer":
			entry.Checked(a.features.operators.composerOpen)
		case "syncInput":
			entry.Checked(a.features.syncEnabled)
		case "focusFullscreen":
			entry.Checked(a.focusFullscreen.requested)
		case "logging":
			p := a.activePane()
			entry.Disabled(p == nil || p.term == nil)
			if p != nil && p.term != nil {
				_, active, _ := p.term.LogState()
				entry.Checked(active)
			}
		}
		if item.Type == mygo.MenuItemCheckbox || item.Type == mygo.MenuItemRadio {
			entry.Checked(item.Checked)
		}
		if item.ID != "" {
			bindings := a.config.Settings.KeyboardShortcuts.Bindings(item.ID, runtime.GOOS)
			if len(bindings) > 0 && strings.TrimSpace(item.Accelerator) != "" {
				entry.Shortcut(shortcutMods(bindings[0]), shortcutKey(bindings[0].KeyCode))
			}
		}
		if entry.Chosen() && item.Click != nil {
			item.Click(item, a.win)
		}
	}
}

// Native Edit roles route to the focused terminal or text input. Restore that
// input after activating the toolbar button, before opening the native menu.
func (a *App) headerEditButton(c *ui.Context) {
	button := ui.Button(c, "编辑").Height(25).FontSize(12).Role(ui.RoleMenuButton)
	open := func() {
		a.lastInputFocus.Focus()
		if a.win == nil || !mygo.App.IsReady() {
			return
		}
		bounds := button.Bounds()
		menu, window := mygo.NewMenu(a.headerItems("编辑")), a.win
		go mygo.RunOnMain(func() {
			if !a.stopped.Load() && window.IsVisible() {
				menu.PopupAt(window, int(bounds.X), int(bounds.Y+bounds.H))
			}
		})
	}
	button.OnClick(open).HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyDown {
			open()
			return true
		}
		return false
	})
}

func (a *App) headerArrangementMenu(m *ui.Menu) {
	a.arrangementMenu(m)
	m.Separator()
	for _, choice := range [][2]string{{"horizontal", "均分左右"}, {"vertical", "均分上下"}, {"tile", "瓷砖平铺"}} {
		if m.Item(choice[1]).Chosen() {
			a.arrangeActive(choice[0])
		}
	}
	m.Separator()
	if m.Item("当前分屏移到独立标签").Chosen() {
		a.detachActive()
	}
}
