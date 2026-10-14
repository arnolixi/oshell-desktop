package core

import (
	"encoding/json"
	"testing"
)

func TestClockStyleDefaultsAndRoundTrip(t *testing.T) {
	var s Settings
	if err := json.Unmarshal([]byte(`{"terminalClockEnabled":true}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.TerminalClockFontSize != 12 || s.TerminalClockOpacity != .5 {
		t.Fatal("legacy defaults lost", s)
	}
	s.TerminalClockFontName = "Menlo"
	s.TerminalClockFontSize = 96
	s.TerminalClockOpacity = 0
	s.TerminalClockColor = "#123456"
	raw, _ := json.Marshal(s)
	var loaded Settings
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.TerminalClockOpacity != 0 || loaded.TerminalClockColor != "#123456" || loaded.TerminalClockFontSize != 96 {
		t.Fatal("clock style lost")
	}
	cfg := DefaultConfig()
	cfg.Settings = s
	d := SharedDocument{Preferences: preferencesForShared(cfg)}
	result, err := d.Configuration()
	if err != nil {
		t.Fatal(err)
	}
	if result.Settings.TerminalClockFontName != "Menlo" || result.Settings.TerminalClockOpacity != 0 {
		t.Fatal("native shared style lost")
	}
	loaded.TerminalClockFontSize = 999
	loaded.TerminalClockOpacity = -1
	loaded.ClampClockStyle()
	if loaded.TerminalClockFontSize != 96 || loaded.TerminalClockOpacity != 0 {
		t.Fatal("clock range not constrained")
	}
}
