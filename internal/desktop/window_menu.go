package desktop

import "github.com/egoist/mygo"

func (h *applicationHub) installDockMenu() {
	mygo.App.Dock.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "新建窗口", Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
		if !h.quitting {
			h.newWindow()
		}
	}}}))
}
func (h *applicationHub) workspaceMenuItem(label string, action func(*App)) *mygo.MenuItem {
	return &mygo.MenuItem{Label: label, Click: func(_ *mygo.MenuItem, window *mygo.Window) {
		if h.quitting {
			return
		}
		a := h.focused(window)
		if a.workspaceMenuBlocked() {
			return
		}
		action(a)
		if a.win != nil {
			a.win.Invalidate()
		}
	}}
}
