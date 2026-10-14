//go:build darwin

package desktop

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

type nativeFocusChrome struct {
	registered objc.ID
	saved      bool
	title      int64
	fullSize   bool
	hidden     [3]bool
}

func (s *nativeFocusChrome) apply(w *mygo.Window, enabled bool) {
	window := objc.ID(w.NativeHandle())
	if window == 0 {
		return
	}
	sel := objc.RegisterName
	if enabled {
		if s.saved {
			return
		}
		s.saved = true
		s.title = objc.Send[int64](window, sel("titleVisibility"))
		mask := objc.Send[uint64](window, sel("styleMask"))
		s.fullSize = mask&(1<<15) != 0
		for i := range s.hidden {
			button := window.Send(sel("standardWindowButton:"), uint64(i))
			s.hidden[i] = objc.Send[bool](button, sel("isHidden"))
			button.Send(sel("setHidden:"), true)
		}
		window.Send(sel("setTitleVisibility:"), int64(1))
		window.Send(sel("setStyleMask:"), mask|1<<15)
	} else if s.saved {
		window.Send(sel("setTitleVisibility:"), s.title)
		mask := objc.Send[uint64](window, sel("styleMask"))
		if !s.fullSize {
			window.Send(sel("setStyleMask:"), mask&^(1<<15))
		}
		for i, hidden := range s.hidden {
			window.Send(sel("standardWindowButton:"), uint64(i)).Send(sel("setHidden:"), hidden)
		}
		s.saved = false
	}
}

// These selectors are absent from the pinned mygo delegate. Add only missing
// callbacks, without replacing its resize, focus or completed-transition methods.
var focusWindows = map[objc.ID]*App{}
var focusDelegateClasses = map[objc.Class]bool{}

func (a *App) registerFocusWindow() {
	if a.win == nil {
		return
	}
	window := objc.ID(a.win.NativeHandle())
	if window == 0 {
		return
	}
	a.focusFullscreen.native.registered = window
	focusWindows[window] = a
	class := window.Send(objc.RegisterName("delegate")).Class()
	if focusDelegateClasses[class] {
		return
	}
	focusDelegateClasses[class] = true
	for _, entering := range []bool{true, false} {
		name := "windowDidFailToExitFullScreen:"
		if entering {
			name = "windowDidFailToEnterFullScreen:"
		}
		class.AddMethod(objc.RegisterName(name), objc.NewIMP(func(_ objc.ID, _ objc.SEL, w objc.ID) {
			if owner := focusWindows[w]; owner != nil {
				owner.focusFullscreen.transitioning = false
				owner.focusFullscreen.requested = !entering
				owner.deferFullscreenReconcile()
			}
		}), "v@:@")
	}
}
func (a *App) unregisterFocusWindow() {
	delete(focusWindows, a.focusFullscreen.native.registered)
	a.focusFullscreen.native.registered = 0
}
