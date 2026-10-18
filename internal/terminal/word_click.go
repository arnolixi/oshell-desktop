package terminal

import (
	"time"

	"oshell-desktop/internal/terminal/internal/vt"
)

const wordWhitespace = " \t\r\n\v\f\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000"

// Keep the completed double click across a pause, but only for the same cell
// in unchanged terminal contents. A lone click, drag or input starts over.
type wordClickState struct {
	last, revision uint64
	x, y           float64
	ref            vt.GridRef
	clicks         int
	wide           bool
}

func (s *wordClickState) press(ref vt.GridRef, revision uint64, x, y, distance float64, ns uint64) (count int, whitespace bool) {
	dx, dy := x-s.x, y-s.y
	same := s.clicks > 0 && s.ref == ref && s.revision == revision && dx*dx+dy*dy <= distance*distance
	if !same || ns < s.last || ns-s.last > uint64(500*time.Millisecond) {
		wide := same && (s.clicks == 2 || s.clicks >= 4)
		*s = wordClickState{clicks: 1, wide: wide}
	} else {
		s.clicks = min(s.clicks+1, 4)
	}
	s.last, s.revision, s.ref, s.x, s.y = ns, revision, ref, x, y
	if s.clicks >= 4 {
		s.wide = true
		return 2, true
	}
	return s.clicks, s.wide
}
