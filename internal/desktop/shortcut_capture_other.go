//go:build !darwin

package desktop

type nativeShortcutMonitor struct{}

func (h *applicationHub) initializeNativeShortcutCapture() {}
func (h *applicationHub) closeNativeShortcutCapture()      {}
