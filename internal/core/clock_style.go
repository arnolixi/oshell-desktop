package core

import (
	"encoding/hex"
	"math"
	"strings"
	"unicode"
)

func (s *Settings) ClampClockStyle() {
	if math.IsNaN(s.TerminalClockFontSize) || math.IsInf(s.TerminalClockFontSize, 0) {
		s.TerminalClockFontSize = 12
	}
	s.TerminalClockFontSize = max(8, min(96, s.TerminalClockFontSize))
	if math.IsNaN(s.TerminalClockOpacity) || math.IsInf(s.TerminalClockOpacity, 0) {
		s.TerminalClockOpacity = .5
	}
	s.TerminalClockOpacity = max(0, min(1, s.TerminalClockOpacity))
	if len(s.TerminalClockColor) != 7 || !strings.HasPrefix(s.TerminalClockColor, "#") {
		s.TerminalClockColor = ""
	} else if _, e := hex.DecodeString(s.TerminalClockColor[1:]); e != nil {
		s.TerminalClockColor = ""
	}
	if len(s.TerminalClockFontName) > 256 || strings.ContainsFunc(s.TerminalClockFontName, unicode.IsControl) {
		s.TerminalClockFontName = ""
	}
}
