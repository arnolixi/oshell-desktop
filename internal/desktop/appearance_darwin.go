//go:build darwin

package desktop

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func appearanceString(value string) objc.ID {
	bytes := append([]byte(value), 0)
	return objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), &bytes[0])
}

// NSApp's effective appearance may be forced by saved preferences. Read the
// system preference independently so a System preview cannot inherit that override.
func systemInterfaceDark(fallback bool) bool {
	defaults := objc.ID(objc.GetClass("NSUserDefaults"))
	if defaults == 0 {
		return fallback
	}
	value := defaults.Send(objc.RegisterName("standardUserDefaults")).Send(objc.RegisterName("stringForKey:"), appearanceString("AppleInterfaceStyle"))
	return value != 0 && objc.Send[bool](value, objc.RegisterName("isEqualToString:"), appearanceString("Dark"))
}
func previewWindowAppearance(window *mygo.Window, dark bool) {
	if window == nil {
		return
	}
	win := objc.ID(window.NativeHandle())
	if win == 0 {
		return
	}
	name := "NSAppearanceNameAqua"
	if dark {
		name = "NSAppearanceNameDarkAqua"
	}
	appearance := objc.ID(objc.GetClass("NSAppearance")).Send(objc.RegisterName("appearanceNamed:"), appearanceString(name))
	var apply func(objc.ID)
	apply = func(w objc.ID) {
		if w == 0 {
			return
		}
		w.Send(objc.RegisterName("setAppearance:"), appearance)
		children := w.Send(objc.RegisterName("childWindows"))
		for i, n := uint64(0), objc.Send[uint64](children, objc.RegisterName("count")); i < n; i++ {
			apply(children.Send(objc.RegisterName("objectAtIndex:"), i))
		}
		sheet := w.Send(objc.RegisterName("attachedSheet"))
		if sheet != 0 {
			sheet.Send(objc.RegisterName("setAppearance:"), appearance)
		}
	}
	apply(win)
}
