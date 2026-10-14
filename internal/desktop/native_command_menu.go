package desktop

import (
	"github.com/egoist/mygo"
	"sort"
)

func (a *App) loadSavedCommandID(id string) {
	if a.passwordLocked() {
		a.showVault()
		return
	}
	for _, q := range a.config.QuickCommands {
		if q.ID == id {
			a.loadCommandInComposer(q)
			return
		}
	}
	a.status = "快速命令已被删除，请重新选择"
}
func (h *applicationHub) nativeCommandMenu() *mygo.MenuItem {
	root := &mygo.MenuItem{Label: "快速命令", Submenu: []*mygo.MenuItem{
		h.workspaceMenuItem("快速命令管理器…", func(a *App) {
			if a.passwordLocked() {
				a.showVault()
			} else {
				a.openCommands()
			}
		}), mygo.Separator(),
	}}
	groups := map[string][]*mygo.MenuItem{}
	for _, q := range h.config.QuickCommands {
		id := q.ID
		groups[q.Group] = append(groups[q.Group], h.workspaceMenuItem(q.Name, func(a *App) { a.loadSavedCommandID(id) }))
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		label := name
		if label == "" {
			label = "未分组"
		}
		root.Submenu = append(root.Submenu, &mygo.MenuItem{Label: label, Submenu: groups[name]})
	}
	return root
}
func (a *App) refreshActiveHostFromMenu() {
	if a.passwordLocked() {
		a.showVault()
		return
	}
	a.refreshHostIdentity(a.activePane())
}
