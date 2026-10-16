package desktop

import "github.com/egoist/mygo/ui"

// Native splitters store the first pane's size in DIPs. Groups retain a ratio
// instead, so arrangement and window resizing do not favor a fixed pixel size.
func (n *groupNode) fitSplit(extent float32) bool {
	return fitSplitAllocation(extent, &n.size, &n.splitRatio, &n.splitExtent, &n.splitApplied)
}

func fitSplitAllocation(extent float32, size, ratio, previousExtent, applied *float32) bool {
	if extent <= 1 {
		return false
	}
	if *ratio <= 0 || *ratio >= 1 {
		*ratio = .5
	}
	if *previousExtent > 1 && *size != *applied {
		// Keyboard/drag input changed the native splitter since our last allocation.
		*ratio = (*size + 1) / (*previousExtent + 1)
	}
	if *previousExtent == extent {
		*applied = *size
		return false
	}
	*previousExtent = extent
	// Include the separator when weighting nested trees: three horizontal groups
	// need 1:2 subtree space (including its separator), not two equal subtrees.
	next := max(float32(0), min(extent-1, (extent+1)*(*ratio)-1))
	changed := *size != next
	*size, *applied = next, next
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
