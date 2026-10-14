//go:build !darwin

package desktop

import "github.com/egoist/mygo"

type nativeFocusChrome struct{}

func (*nativeFocusChrome) apply(*mygo.Window, bool) {}
func (a *App) registerFocusWindow()                 {}
func (a *App) unregisterFocusWindow()               {}
