package desktop

import "github.com/egoist/mygo/ui"

// Native splitters store the first pane's size in DIPs. Groups retain a ratio
// instead, so arrangement and window resizing do not favor a fixed pixel size.
func (n *groupNode) fitSplit(extent float32) bool {
	if extent <= 1 {
		return false
	}
	if n.splitRatio <= 0 || n.splitRatio >= 1 {
		n.splitRatio = .5
	}
	if n.splitExtent > 1 && n.size != n.splitApplied {
		// Keyboard/drag input changed the native splitter since our last allocation.
		n.splitRatio = (n.size + 1) / (n.splitExtent + 1)
	}
	n.splitExtent = extent
	// Include the separator when weighting nested trees: three horizontal groups
	// need 1:2 subtree space (including its separator), not two equal subtrees.
	next := max(float32(0), min(extent-1, (extent+1)*n.splitRatio-1))
	changed := n.size != next
	n.size, n.splitApplied = next, next
	return changed
}
func (a *App) groupSplit(c *ui.Context, n *groupNode, first, second func()) {
	ui.Box(c).Fill().MinWidth(0).MinHeight(0).Draw(func(_ *ui.Painter, bounds ui.Rect) {
		extent := bounds.W
		if n.vertical {
			extent = bounds.H
		}
		// Draw receives the current measured bounds, including the first layout and
		// resize. Request a build only when allocation changes; idle stays idle.
		if n.fitSplit(extent) {
			c.Invalidate()
		}
	}).Children(func() { a.terminalSplit(c, &n.size, n.vertical, first, second) })
}
