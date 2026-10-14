// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"bytes"
	"encoding/json"
)

type InputPreferences struct {
	CopyOnSelect     bool `json:"copyOnSelect"`
	RightClickPaste  bool `json:"rightClickPaste"`
	TrimLeading      bool `json:"trimLeading"`
	TrimTrailing     bool `json:"trimTrailing"`
	ConfirmMultiline bool `json:"confirmMultiline"`
}
type HighlightRule struct {
	Color         string `json:"color"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Pattern       string `json:"pattern"`
	Regex         bool   `json:"regex"`
	CaseSensitive bool   `json:"caseSensitive"`
	Bold          bool   `json:"bold"`
	Foreground    string `json:"foreground"`
	Background    string `json:"background"`
	Enabled       bool   `json:"enabled"`
}

func DefaultSettings() Settings {
	return Settings{Metal: true, PreferColorEmoji: true, TerminalClockPosition: "topRight", TerminalClockFontSize: 12, TerminalClockOpacity: 0.5, LinksVisible: true, HighlightSetID: StandardHighlightSetID, QuickSendScope: "current", QuickSendVisible: true, AutoZmodem: true, InterfaceTheme: "system", ColorSchemeID: "oshell-dark", Dark: false, FontSize: 13, ScrollbackMB: 4, ScrollbackLines: 3000, Input: InputPreferences{CopyOnSelect: true, RightClickPaste: true, ConfirmMultiline: true}}
}
func (s *Settings) UnmarshalJSON(b []byte) error {
	type plain Settings
	v := plain(DefaultSettings())
	if e := json.Unmarshal(b, &v); e != nil {
		return e
	}
	var keys map[string]json.RawMessage
	json.Unmarshal(b, &keys)
	if _, ok := keys["interfaceTheme"]; !ok {
		v.InterfaceTheme = "light"
		if v.Dark {
			v.InterfaceTheme = "dark"
		}
	}
	if _, ok := keys["colorSchemeID"]; !ok {
		v.ColorSchemeID = "oshell-light"
		if v.Dark {
			v.ColorSchemeID = "oshell-dark"
		}
	}
	if raw, exists := keys["highlightSetID"]; exists && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		v.HighlightSetID = ""
	}
	*s = Settings(v)
	s.ClampClockStyle()
	return nil
}
