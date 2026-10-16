package desktop

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"path"
)

func managerParent(group string) string {
	parent := path.Dir(group)
	if parent == "." {
		return ""
	}
	return parent
}

func (a *App) navigateManager(group string) {
	a.managerRename = nil
	a.managerGroup = group
	a.managerAll = false
	a.query = ""
	a.managerRow = -1
	a.managerSelection.Clear()
	a.selected = ""
	a.resetManagerList()
}

// Menu callbacks can navigate while Table is still building its cells. Keep
// that frame's ListState alive until layout and pending input have finished.
func (a *App) resetManagerList() {
	a.managerList = &ui.ListState{Columns: a.managerList.Columns}
}

// Keep selection by identity when filtering, never by its old table position.
func (a *App) refreshManagerSearchSelection() {
	rows := a.managerRows()
	// Filtering changes row positions. Drop the old list anchor before assigning
	// the retained identity, otherwise the widget restores a now-hidden row.
	a.resetManagerList()
	a.managerRow = -1
	a.selected = ""
	query := core.NewSessionQuery(a.query)
	retained := []string{}
	for i, row := range rows {
		if (!row.parent || len(query.Terms) == 0) && a.managerSelection.Has(row.id) {
			retained = append(retained, row.id)
			if a.managerRow < 0 {
				a.managerRow = i
			}
		}
	}
	a.managerSelection.Clear()
	for _, id := range retained {
		a.managerSelection.Add(id)
	}
	if a.managerRow < 0 && query.Valid && len(query.Terms) > 0 {
		for i, row := range rows {
			if !row.parent {
				a.managerRow = i
				a.managerSelection.Add(row.id)
				break
			}
		}
	}
	if a.managerRow >= 0 {
		row := rows[a.managerRow]
		if !row.folder {
			a.selected = row.profile.ID
		}
		a.managerList.ScrollTo(a.managerRow, ui.Center)
	}
}

func (a *App) managerResultStatus(profiles, folders int) string {
	query := core.NewSessionQuery(a.query)
	if !query.Valid {
		return "搜索内容过长：最多 4 KB / 32 个关键词"
	}
	status := fmt.Sprintf("%d 个会话 · %d 个目录", profiles, folders)
	if len(query.Terms) > 0 {
		if a.managerSearchScope == "all" {
			status += " · 全部目录"
		} else {
			status += " · 当前目录及子目录"
		}
	}
	return status
}
