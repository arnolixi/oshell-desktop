package desktop

import "fmt"

func (a *App) groupUnread(g *tabGroup) bool {
	if g == nil {
		return false
	}
	for _, tab := range a.tabs {
		if tab.group == g.id && tab.unread {
			return true
		}
	}
	return false
}
func (a *App) hiddenUnreadGroups() int {
	count := 0
	for _, g := range a.groups.root.leaves() {
		if g.hidden && a.groupUnread(g) {
			count++
		}
	}
	return count
}
func (a *App) groupMenuTitle(g *tabGroup) string {
	title := fmt.Sprintf("%s（%d）", a.groupTitle(g), len(a.groupTabs(g)))
	if g.hidden {
		title += " · 隐藏"
	}
	if a.groupUnread(g) {
		title += " ●"
	}
	return title
}
