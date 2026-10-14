package desktop

// Include the dedicated file workspace in shared configuration publication.
// Return a snapshot because publication may invalidate or close a window.
func (h *applicationHub) configurationWindows() []*App {
	out := append([]*App(nil), h.windows...)
	if h.fileWindow != nil {
		found := false
		for _, window := range out {
			if window == h.fileWindow {
				found = true
				break
			}
		}
		if !found {
			out = append(out, h.fileWindow)
		}
	}
	return out
}
