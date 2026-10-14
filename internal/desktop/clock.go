// SPDX-License-Identifier: GPL-3.0-only
package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"sync"
	"time"
)

type workspaceClock struct {
	mu      sync.Mutex
	members map[*App]bool
	stop    chan struct{}
	text    string
}

func (c *workspaceClock) update(members map[*App]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.members = members
	if len(members) == 0 {
		if c.stop != nil {
			close(c.stop)
			c.stop = nil
		}
		return
	}
	if c.stop != nil {
		return
	}
	c.text = core.LocalClockText(time.Now())
	stop := make(chan struct{})
	c.stop = stop
	go c.run(stop)
}
func (c *workspaceClock) run(stop <-chan struct{}) {
	timer := time.NewTimer(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
			text := core.LocalClockText(time.Now())
			c.mu.Lock()
			c.text = text
			members := make([]*App, 0, len(c.members))
			for a := range c.members {
				members = append(members, a)
			}
			c.mu.Unlock()
			for _, a := range members {
				if a.clockPending.CompareAndSwap(false, true) {
					a.post(func() { a.clockPending.Store(false); a.clockText = c.current() })
				}
			}
			timer.Reset(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
		}
	}
}
func (c *workspaceClock) current() string { c.mu.Lock(); defer c.mu.Unlock(); return c.text }
func (h *applicationHub) refreshClock() {
	members := map[*App]bool{}
	if clockVisible(h.config.Settings) {
		for _, a := range h.windows {
			if a.win != nil && !a.stopped.Load() && a.win.IsVisible() && !a.win.IsMinimized() {
				for _, p := range a.visiblePanes() {
					if p.term != nil && !p.closed {
						members[a] = true
						break
					}
				}
			}
		}
	}
	h.clock.update(members)
}
func (a *App) clockOverlay(element ui.Element) {
	if !clockVisible(a.config.Settings) {
		return
	}
	text := a.clockText
	if text == "" {
		if a.hub != nil {
			text = a.hub.clock.current()
		}
		if text == "" {
			text = core.LocalClockText(time.Now())
		}
	}
	drawClockOverlay(element, a.config.Settings, text)
}
func clockVisible(s core.Settings) bool { return s.TerminalClockEnabled && s.TerminalClockOpacity > 0 }
func drawClockOverlay(element ui.Element, settings core.Settings, text string) {
	settings.ClampClockStyle()
	if !clockVisible(settings) {
		return
	}
	family := "monospace"
	if settings.TerminalClockFontName != "" {
		family = settings.TerminalClockFontName + ", monospace"
	}
	font := ui.Font{Family: family, Size: float32(settings.TerminalClockFontSize), Weight: 500}
	colorText := settings.TerminalClockColor
	if colorText == "" {
		colorText = settings.Scheme().Foreground
	}
	color := ui.Hex(colorText).Alpha(float32(settings.TerminalClockOpacity))
	column, row := core.ClockGrid(settings.TerminalClockPosition)
	element.DrawOver(func(p *ui.Painter, r ui.Rect) {
		fit := font
		glyphs := ui.Shape(text, fit)
		metrics := fit.Metrics()
		width := float32(0)
		for _, g := range glyphs {
			width = max(width, g.X+g.Advance)
		}
		height := metrics.Ascent + metrics.Descent
		inset := min(float32(12), max(float32(0), min(r.W, r.H)/8))
		scale := min(float32(1), min(max(float32(1), r.W-2*inset)/max(width, 1), max(float32(1), r.H-2*inset)/max(height, 1)))
		if scale < 1 {
			fit.Size = max(float32(1), font.Size*scale)
			glyphs = ui.Shape(text, fit)
			metrics = fit.Metrics()
			width = 0
			for _, g := range glyphs {
				width = max(width, g.X+g.Advance)
			}
			height = metrics.Ascent + metrics.Descent
		}
		x, y := r.X+inset, r.Y+inset
		if column == 1 {
			x = r.X + (r.W-width)/2
		} else if column == 2 {
			x = r.X + r.W - width - inset
		}
		if row == 1 {
			y = r.Y + (r.H-height)/2
		} else if row == 2 {
			y = r.Y + r.H - height - inset
		}
		p.Clip(r, 0, func() { p.Glyphs(glyphs, x, y+metrics.Ascent, color) })
	})
}
