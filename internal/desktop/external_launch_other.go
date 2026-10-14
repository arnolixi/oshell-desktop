//go:build !darwin

package desktop

// Other-platform launcher integration is deferred until macOS acceptance.
type pendingExternalLaunch struct{}

func (h *applicationHub) startExternalLaunch() {}
func (h *applicationHub) drainExternalLaunch() {}
