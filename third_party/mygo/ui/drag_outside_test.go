package ui

import (
	"github.com/egoist/mygo/internal/platform"
	"testing"
)

func TestValueDragOutsideOnlyOnRelease(t *testing.T) {
	outside, dropped := 0, 0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Children(func() {
			coreBox(c).Size(90, 40).Label("source").Drag("tab").OnDragOutside(func() { outside++ })
			dst := coreBox(c).Size(100, 50).Label("target")
			if _, ok := coreDrop[string](dst); ok {
				dropped++
			}
		})
	}, 300, 200)
	tt.Press(10, 10)
	tt.Move(330, 30)
	if outside != 0 {
		t.Fatal("detached before release")
	}
	tt.Release(330, 30)
	if outside != 1 {
		t.Fatal("outside release ignored")
	}
	tt.Press(10, 10)
	tt.Move(-20, 30)
	tt.Key(0, KeyEscape)
	tt.Release(-20, 30)
	if outside != 1 {
		t.Fatal("Escape detached a tab")
	}
	drag(tt, 10, 10, 10, 65)
	if dropped != 1 || outside != 1 {
		t.Fatal("inside drop also detached")
	}
	drag(tt, 10, 10, 200, 100)
	if outside != 1 {
		t.Fatal("inside blank area detached")
	}
	tt.Press(10, 10)
	tt.Move(-20, 30)
	tt.send(platform.SurfaceEvent{Kind: platform.SurfaceBlur})
	tt.Release(-20, 30)
	if outside != 1 {
		t.Fatal("capture loss detached a tab")
	}
}
