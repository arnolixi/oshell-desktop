package desktop

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

type managerDrag struct {
	owner *App
	rows  []sessionRow
}

func (a *App) managerDrop(element ui.Element, destination string) {
	if drag, over := ui.DragOver[managerDrag](element); over && drag.owner == a {
		element.BorderWidth(1).BorderColor(ui.Hex("#786DFF"))
	}
	if drag, dropped := ui.Drop[managerDrag](element); dropped {
		a.moveManagerRows(drag, destination)
	}
}
func (a *App) moveManagerRows(drag managerDrag, destination string) {
	if drag.owner != a || len(drag.rows) == 0 || len(drag.rows) > 10000 {
		return
	}
	if a.changeConfig(func(cfg *core.Config) error {
		var profiles, dirs, links []string
		for _, r := range drag.rows {
			if r.parent {
				return fmt.Errorf("上级目录行只能用于导航或作为拖放目标")
			}
			if r.folder {
				dirs = append(dirs, r.group)
			} else if r.linkID != "" {
				links = append(links, r.linkID)
			} else {
				found := false
				for _, p := range cfg.Profiles {
					if p.ID == r.profile.ID && p.Group == r.group {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("所选会话已变化，请重新拖动")
				}
				profiles = append(profiles, r.profile.ID)
			}
		}
		return cfg.MoveSessionItems(profiles, dirs, links, destination)
	}) {
		a.managerRow = -1
		a.managerSelection.Clear()
		a.selected = ""
		if a.manager != nil {
			a.manager.Invalidate()
		}
	}
}
