package desktop

import "github.com/egoist/mygo/ui"

func (n *node) fitSplit(extent float32) bool {
	// Preserve an existing explicit allocation when adopting proportional sizing.
	if extent > 1 && n.splitExtent == 0 && n.splitRatio == 0 && n.size > 0 && n.size < extent-1 {
		n.splitRatio = (n.size + 1) / (extent + 1)
		n.splitExtent, n.splitApplied = extent, n.size
		return false
	}
	return fitSplitAllocation(extent, &n.size, &n.splitRatio, &n.splitExtent, &n.splitApplied)
}

func (a *App) paneSplit(c *ui.Context, n *node, first, second func()) {
	ui.Box(c).Fill().MinWidth(0).MinHeight(0).Draw(func(_ *ui.Painter, bounds ui.Rect) {
		n.bounds = bounds
		extent := bounds.W
		if n.vertical {
			extent = bounds.H
		}
		if n.fitSplit(extent) {
			c.Invalidate()
		}
	}).Children(func() { a.terminalSplit(c, &n.size, n.vertical, first, second) })
}
