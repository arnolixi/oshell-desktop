package desktop

// Activation never creates windows or opens unlock/conflict dialogs.
func (h *applicationHub) refreshOnActivation() {
	if h.quitting || h.configBusy {
		return
	}
	owner := h.fileWindow
	if len(h.windows) > 0 {
		owner = h.windows[0]
	}
	for _, window := range h.windows {
		if window.win != nil && window.win.IsFocused() {
			owner = window
			break
		}
	}
	if owner != nil && !owner.stopped.Load() {
		owner.reloadLocalSharedData(false)
	}
}
