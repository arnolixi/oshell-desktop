//go:build darwin

package desktop

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// Called from UI actions on the main thread. AppKit owns the native sheet state,
// including file pickers opened by either MyGo or the mixed-upload picker.
func nativeTabMoveDialogOpen(window *mygo.Window) bool {
	if !mygo.App.IsReady() {
		return false
	}
	application := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	if application.Send(objc.RegisterName("modalWindow")) != 0 {
		return true
	}
	if window == nil {
		return false
	}
	handle := objc.ID(window.NativeHandle())
	return handle != 0 && handle.Send(objc.RegisterName("attachedSheet")) != 0
}
