package desktop

import (
	"github.com/egoist/mygo/ui"
	"time"
)

func (a *App) tabDragScroll(c *ui.Context, scroll ui.Element, g *tabGroup) {
	// Register the container for drops in trailing space; individual tabs keep priority.
	_, over := ui.DragOver[tabDrag](scroll)
	if !over && !g.dragOver {
		g.scrollTime = time.Time{}
		return
	}
	x, _, ok := scroll.PointerPosition()
	if !ok {
		return
	}
	now := time.Now()
	delta := float32(0)
	if !g.scrollTime.IsZero() {
		delta = float32(min(now.Sub(g.scrollTime).Seconds(), 0.05)) * 350
	}
	g.scrollTime = now
	if x < 28 {
		g.scroll.X = max(0, g.scroll.X-delta)
		c.AnimationFrame()
	} else if x > scroll.Bounds().W-28 {
		g.scroll.X = min(g.scroll.MaxX, g.scroll.X+delta)
		c.AnimationFrame()
	}
	if drag, dropped := ui.Drop[tabDrag](scroll); dropped {
		tabs := a.groupTabs(g)
		if len(tabs) > 0 {
			target := tabs[len(tabs)-1].id
			a.post(func() { a.moveTabRelative(drag.ID, target, true) })
		}
	}
}
