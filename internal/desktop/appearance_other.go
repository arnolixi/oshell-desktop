//go:build !darwin

package desktop

import "github.com/egoist/mygo"

func systemInterfaceDark(fallback bool) bool     { return fallback }
func previewWindowAppearance(*mygo.Window, bool) {}
