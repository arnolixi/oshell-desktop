package terminal

import (
	"github.com/egoist/mygo/ui"
	"math"
	"oshell-desktop/internal/terminal/internal/vt"
	"time"
)

const scrollbarHold = 800 * time.Millisecond
const scrollbarFade = 200 * time.Millisecond

type overlayScrollbar struct {
	activity        time.Time
	dragging        bool
	grab            float32
	width, height   float32
	visibilityEpoch uint64
}

func (s *overlayScrollbar) hide() { s.activity = time.Time{}; s.dragging = false }
func (s *overlayScrollbar) alpha(now time.Time) float32 {
	if s.activity.IsZero() {
		return 0
	}
	if s.dragging {
		return .45
	}
	elapsed := now.Sub(s.activity)
	if elapsed <= scrollbarHold {
		return .45
	}
	if elapsed >= scrollbarHold+scrollbarFade {
		return 0
	}
	return .45 * (1 - float32(elapsed-scrollbarHold)/float32(scrollbarFade))
}
func scrollbarKnob(sb vt.Scrollbar, height float32) (y, h float32) {
	if sb.Total <= sb.Len || height <= 4 {
		return 0, 0
	}
	h = min(height, max(24, height*float32(sb.Len)/float32(sb.Total)))
	y = (height - h) * float32(sb.Offset) / float32(sb.Total-sb.Len)
	return y, h
}

// Called only by user scrolling, never by incoming terminal output.
// The terminal mutex is held by the caller.
func (v *view) scrollingActivity() {
	if v.t.term.AltScreen() || v.t.term.Scrollbar().Total <= v.t.term.Scrollbar().Len {
		v.overlay.hide()
		return
	}
	v.overlay.activity = time.Now()
	if v.overlay.dragging {
		v.t.stopScrollbarFade()
	} else {
		v.t.armScrollbarFade()
	}
}
func (v *view) resetHiddenScrollbar() {
	s := &v.overlay
	epoch := v.t.visibilityEpoch.Load()
	if epoch != s.visibilityEpoch || v.t.hidden.Load() {
		s.hide()
		s.visibilityEpoch = epoch
	}
}

// The overlay's twelve-point hit area exists only while visible. It does not
// take terminal columns or create another focusable control. t.mu is held.
func (v *view) scrollbarInput(ev ui.InputEvent) bool {
	s := &v.overlay
	if v.t.term.AltScreen() || v.t.term.Scrollbar().Total <= v.t.term.Scrollbar().Len {
		s.hide()
		return false
	}
	sb := v.t.term.Scrollbar()
	y, h := scrollbarKnob(sb, s.height)
	if s.dragging {
		switch ev.Kind {
		case ui.InputPointerUp:
			s.dragging = false
			v.scrollingActivity()
			return true
		case ui.InputPointerMove:
			ratio := max(float32(0), min(float32(1), (ev.Y-s.grab)/max(float32(1), s.height-h)))
			v.t.term.ScrollToRow(int(math.Round(float64(ratio) * float64(sb.Total-sb.Len))))
			v.scrollingActivity()
			return true
		}
	}
	if ev.Kind != ui.InputPointerDown || ev.Button != 0 || s.alpha(time.Now()) == 0 || ev.X < s.width-12 || ev.X > s.width {
		return false
	}
	if ev.Y >= y && ev.Y <= y+h {
		s.dragging = true
		s.grab = ev.Y - y
	} else {
		page := int(sb.Len)
		if ev.Y < y {
			page = -page
		}
		v.t.term.ScrollBy(page)
	}
	v.scrollingActivity()
	return true
}

// Wake the host independently of focus/cursor blinking. Some native surfaces
// suppress scheduled repaint-only frames while the caret is outside the viewport.
func (t *Terminal) armScrollbarFade() {
	t.scrollTimerMu.Lock()
	defer t.scrollTimerMu.Unlock()
	t.scrollTimerGeneration++
	generation := t.scrollTimerGeneration
	if t.scrollTimer != nil {
		t.scrollTimer.Stop()
	}
	end := time.Now().Add(scrollbarHold + scrollbarFade)
	var wake func()
	wake = func() {
		t.scrollTimerMu.Lock()
		defer t.scrollTimerMu.Unlock()
		if generation != t.scrollTimerGeneration || t.hidden.Load() {
			return
		}
		select {
		case <-t.done:
			return
		default:
		}
		t.redraw()
		remaining := time.Until(end)
		if remaining > 0 {
			t.scrollTimer = time.AfterFunc(min(16*time.Millisecond, remaining), wake)
		} else {
			t.scrollTimer = nil
		}
	}
	t.scrollTimer = time.AfterFunc(scrollbarHold, wake)
}
func (t *Terminal) stopScrollbarFade() {
	t.scrollTimerMu.Lock()
	defer t.scrollTimerMu.Unlock()
	t.scrollTimerGeneration++
	if t.scrollTimer != nil {
		t.scrollTimer.Stop()
		t.scrollTimer = nil
	}
}
