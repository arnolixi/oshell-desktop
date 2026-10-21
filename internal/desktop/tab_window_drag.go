package desktop

import (
	"runtime"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

const terminalTabFormat transfer.Format = "application/vnd.oshell.terminal-tab"

func (a *App) tabDragSource(id string) *App {
	if a.tabByID(id) != nil {
		return a
	}
	if a.hub != nil {
		for _, source := range a.hub.windows {
			if source != a && !source.stopped.Load() && source.tabByID(id) != nil {
				return source
			}
		}
	}
	return nil
}

func (a *App) acceptsTabDrag(d tabDrag, g *tabGroup) bool {
	source := a.tabDragSource(d.ID)
	if source == nil || g == nil || g.hidden || a.passwordLocked() || source.passwordLocked() || a.stopped.Load() {
		return false
	}
	if source != a && (a.nonAuthenticationModalActive() || source.nonAuthenticationModalActive() || tabMoveDialogOpen(a.win) || tabMoveDialogOpen(source.win)) {
		return false
	}
	return true
}

func (a *App) receiveTabDrag(d tabDrag, g *tabGroup, target string, after bool) {
	if !a.acceptsTabDrag(d, g) || a.groupByID(g.id) != g {
		return
	}
	source := a.tabDragSource(d.ID)
	if source == a {
		if target == "" {
			tabs := a.groupTabs(g)
			if len(tabs) == 0 {
				a.moveToGroup(a.tabByID(d.ID), g)
			} else {
				a.moveTabRelative(d.ID, tabs[len(tabs)-1].id, true)
			}
		} else {
			a.moveTabRelative(d.ID, target, after)
		}
		return
	}
	source.transferTabAt(source.tabByID(d.ID), a, g.id, target, after)
}

func (a *App) terminalTabDrag(e ui.Element, t *tab) {
	e.Drag(tabDrag{t.id}).OnDragOutside(func() { a.queueDetachedTab(t) })
	// Native data drags carry the Go value only inside this process. The
	// advertised format contains no session definition, host or credentials.
	// Keep headless views and the unsupported macOS build on value dragging.
	if a.win == nil || runtime.GOOS == "darwin" {
		return
	}
	e.DragDataFrom(func() transfer.Data {
		a.tabNativeDrag = true
		return transfer.New(transfer.NewItem(transfer.Bytes(terminalTabFormat, []byte("tab"))))
	}, transfer.DragOptions{Operations: transfer.Move, Done: func(result transfer.Result) {
		a.tabNativeDrag = false
		if result.Err != nil {
			a.status = "标签拖动失败：" + result.Err.Error()
		}
		if result.Err == nil && !result.Canceled && result.Operation == transfer.None {
			a.queueDetachedTab(t)
		}
		a.finishTabMoveClose()
	}})
}

func (a *App) tabStripDropSpace(e ui.Element, g *tabGroup) {
	if d, over := ui.DragOver[tabDrag](e); over && a.acceptsTabDrag(d, g) {
		e.BorderWidth(0, 0, 0, 2).BorderColor(terminalFlame)
	}
	if d, dropped := ui.Drop[tabDrag](e); dropped {
		a.post(func() { a.receiveTabDrag(d, g, "", false) })
	}
}
