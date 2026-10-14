package desktop

import (
	"github.com/egoist/mygo/ui"
	"math"
	"oshell-desktop/internal/core"
	"testing"
)

func TestGroupSplitEqualAtCreationAndResize(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		t.Run(map[bool]string{false: "horizontal", true: "vertical"}[vertical], func(t *testing.T) {
			a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
			n := &groupNode{vertical: vertical}
			tt := ui.NewTester(func(c *ui.Context) {
				a.groupSplit(c, n,
					func() { ui.Box(c).Fill().Label("first group") }, func() { ui.Box(c).Fill().Label("second group") })
			}, 1100, 700)
			check := func() {
				t.Helper()
				first, _ := tt.Find("first group")
				second, _ := tt.Find("second group")
				x, y := first.W, second.W
				if vertical {
					x, y = first.H, second.H
				}
				if x < 1 || math.Abs(float64(x-y)) > 1 {
					t.Fatalf("groups not equal: %+v %+v", first, second)
				}
			}
			check()
			tt.SetSize(1370, 910)
			check()
			tt.SetSize(860, 570)
			check()
			if err := tt.Click("Divider"); err != nil {
				t.Fatal(err)
			}
			key := ui.KeyRight
			if vertical {
				key = ui.KeyDown
			}
			tt.Key(0, key)
			tt.Frame()
			ratio := n.splitRatio
			if ratio <= .5 {
				t.Fatal("manual splitter adjustment was reset")
			}
			tt.SetSize(1200, 800)
			if math.Abs(float64(n.splitRatio-ratio)) > 0.0001 {
				t.Fatal("resize lost user ratio")
			}
		})
	}
}
func TestGroupSplitThreeGroupsEqual(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	root := &groupNode{splitRatio: 1.0 / 3}
	right := &groupNode{}
	tt := ui.NewTester(func(c *ui.Context) {
		a.groupSplit(c, root,
			func() { ui.Box(c).Fill().Label("one") }, func() {
				a.groupSplit(c, right, func() { ui.Box(c).Fill().Label("two") }, func() { ui.Box(c).Fill().Label("three") })
			})
	}, 1000, 600)
	one, _ := tt.Find("one")
	two, _ := tt.Find("two")
	three, _ := tt.Find("three")
	if math.Abs(float64(one.W-two.W)) > 1 || math.Abs(float64(two.W-three.W)) > 1 {
		t.Fatalf("unequal three-way layout: %v %v %v", one, two, three)
	}
}
