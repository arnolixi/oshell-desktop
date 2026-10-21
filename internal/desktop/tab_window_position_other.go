//go:build !windows

package desktop

import "github.com/egoist/mygo"

func (p tabWindowPosition) within(window *mygo.Window) bool {
	r := window.Bounds()
	return p.x >= r.X && p.x < r.X+r.Width && p.y >= r.Y && p.y < r.Y+r.Height
}

func currentTabWindowPosition() *tabWindowPosition {
	p := mygo.Screen.CursorScreenPoint()
	return &tabWindowPosition{x: p.X, y: p.Y}
}

func (p tabWindowPosition) apply(window *mygo.Window) error {
	window.SetPosition(p.x, p.y)
	return nil
}
