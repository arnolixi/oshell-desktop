package desktop

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func renderPaneSizing(c *ui.Context, a *App, n *node) {
	if n.pane != nil {
		ui.Box(c).Fill().Label(n.pane.profile.Name)
		return
	}
	a.paneSplit(c, n, func() { renderPaneSizing(c, a, n.first) }, func() { renderPaneSizing(c, a, n.second) })
}

func TestNewPaneSplitEqualAndPreservesDraggedRatioOnResize(t *testing.T) {
	for _, direction := range []int{1, 2} {
		a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
		operatorPane(t, a, "first")
		a.open(core.Profile{ID: core.NewID(), Kind: "blank", Name: "second"}, core.Credentials{}, direction)
		second := a.activePane()
		waitOperator(t, a, func() bool { return second.term != nil })
		t.Cleanup(func() { a.dispose(second) })
		root := a.activeTab().root
		tt := ui.NewTester(func(c *ui.Context) { a.theme(c); renderPaneSizing(c, a, root) }, 1240, 710)
		checkEqual := func() {
			t.Helper()
			first, _ := tt.Find("first")
			second, _ := tt.Find("second")
			x, y := first.W, second.W
			if direction == 2 {
				x, y = first.H, second.H
			}
			if x < 1 || math.Abs(float64(x-y)) > 1 {
				t.Fatalf("new split not equal: %+v / %+v", first, second)
			}
		}
		checkEqual()
		tt.SetSize(890, 560)
		checkEqual()
		divider, ok := tt.Find("Divider")
		if !ok {
			t.Fatal("divider missing")
		}
		x, y := divider.X+divider.W/2, divider.Y+divider.H/2
		tt.Press(x, y)
		if direction == 1 {
			x += 70
		} else {
			y += 70
		}
		tt.Move(x, y)
		tt.Release(x, y)
		ratio := root.splitRatio
		if ratio <= .5 {
			t.Fatal("manual drag was reset")
		}
		tt.SetSize(1370, 900)
		if math.Abs(float64(root.splitRatio-ratio)) > .0001 {
			t.Fatal("window resize lost dragged ratio")
		}
		// Repeated idle frames must not drift or trigger another allocation.
		size := root.size
		for range 5 {
			tt.Frame()
		}
		if size != root.size {
			t.Fatal("idle layout drifted")
		}
	}
}

func TestArrangeThreePanesUsesActualContainerSize(t *testing.T) {
	for _, mode := range []string{"horizontal", "vertical"} {
		a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
		for _, name := range []string{"one", "two", "three"} {
			operatorPane(t, a, name)
		}
		tab := a.tabs[0]
		tab.root = &node{first: tab.root, second: &node{first: a.tabs[1].root, second: a.tabs[2].root}}
		a.tabs = a.tabs[:1]
		a.arrangeActive(mode)
		tt := ui.NewTester(func(c *ui.Context) {
			a.theme(c)
			ui.Row(c).Fill().Children(func() {
				ui.Box(c).Width(190).FillHeight()
				ui.Box(c).Grow(1).FillHeight().Children(func() { renderPaneSizing(c, a, tab.root) })
			})
		}, 1003, 643)
		check := func() {
			t.Helper()
			one, _ := tt.Find("one")
			two, _ := tt.Find("two")
			three, _ := tt.Find("three")
			x, y, z := one.W, two.W, three.W
			if mode == "vertical" {
				x, y, z = one.H, two.H, three.H
			}
			if x < 1 || math.Abs(float64(x-y)) > 1 || math.Abs(float64(y-z)) > 1 {
				t.Fatalf("unequal %s arrangement: %+v / %+v / %+v", mode, one, two, three)
			}
		}
		check()
		tt.SetSize(1371, 891)
		check()
	}
}
