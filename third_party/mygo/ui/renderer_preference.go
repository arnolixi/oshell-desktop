package ui

import "time"

// UseSoftwareRenderer selects CPU drawing for this window when enabled; false
// restores automatic GPU selection and its existing software fallback. Call it
// on the UI thread while building the view. It preserves widget/terminal state.
// Memory-only test views already render in software and ignore this preference.
func (c *Context) UseSoftwareRenderer(enabled bool) {
	rt := c.runtime()
	if rt == nil {
		return
	}
	if h, ok := rt.host.(*windowHost); ok {
		h.setSoftwareRenderer(enabled)
	}
}
func (h *windowHost) setSoftwareRenderer(enabled bool) {
	if h.detached || h.forceSoftware == enabled {
		return
	}
	h.forceSoftware = enabled
	if h.gpu != nil {
		h.gpu.Release()
		h.gpu = nil
	}
	h.soft.Release()
	h.gpuTried = false
	h.onGPU = false
	h.degraded = false
	h.retryAt = time.Time{}
	h.gpuSince = time.Time{}
	h.backoff = 0
	h.wide = false
	h.wideUntil = time.Time{}
	// present consumes the preference in the current frame; no new timer or view.
}
