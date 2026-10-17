package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// The shared wire representation follows macOS OShell's Int-backed enum.
// Local desktop configuration continues using its descriptive strings.
var sharedSendScopes = []string{"current", "tab", "all", "visible", "selected", "currentGroup", "selectedGroups"}

func sharedQuickSendScope(scope string) int {
	for n, value := range sharedSendScopes {
		if value == scope {
			return n
		}
	}
	return 0
}
func decodeSharedQuickSendScope(raw json.RawMessage) (string, error) {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if n >= 0 && n < len(sharedSendScopes) {
			return sharedSendScopes[n], nil
		}
		return "", fmt.Errorf("共享快速发送范围无效")
	}
	// Accept documents written by older desktop versions, but always write Int.
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		for _, scope := range sharedSendScopes {
			if value == scope {
				return scope, nil
			}
		}
	}
	return "", fmt.Errorf("共享快速发送范围无效")
}

// Copy the preference map before normalization: conflict baselines are immutable.
func (d SharedDocument) canonicalProtocol() (SharedDocument, error) {
	if d.Proxies == nil {
		d.Proxies = []PortableProxy{}
	}
	if d.Preferences == nil {
		return d, nil
	}
	prefs := make(map[string]json.RawMessage, len(d.Preferences))
	for key, value := range d.Preferences {
		prefs[key] = value
	}
	if raw, ok := prefs["quickSendScope"]; ok {
		scope, err := decodeSharedQuickSendScope(raw)
		if err != nil {
			return d, err
		}
		prefs["quickSendScope"], _ = json.Marshal(sharedQuickSendScope(scope))
	}
	// Native OShell derives this legacy marker from the chosen terminal palette.
	// Normalizing baselines avoids an upgrade being treated as a user edit.
	if raw, ok := prefs["colorSchemeID"]; ok {
		settings := DefaultSettings()
		if err := json.Unmarshal(raw, &settings.ColorSchemeID); err != nil {
			return d, err
		}
		if raw, ok := prefs["customColorSchemes"]; ok {
			if err := json.Unmarshal(raw, &settings.CustomSchemes); err != nil {
				return d, err
			}
		}
		prefs["darkTheme"], _ = json.Marshal(sharedSchemeIsDark(settings))
	}
	d.Preferences = prefs
	return d, nil
}

func (d SharedDocument) needsProtocolUpgrade() bool {
	raw := d.Preferences["quickSendScope"]
	var legacy string
	return len(raw) > 0 && json.Unmarshal(raw, &legacy) == nil && !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func sharedSchemeIsDark(s Settings) bool {
	raw, err := hex.DecodeString(strings.TrimPrefix(s.Scheme().Background, "#"))
	if err != nil || len(raw) != 3 {
		return true
	}
	return (0.2126*float64(raw[0]) + 0.7152*float64(raw[1]) + 0.0722*float64(raw[2])) < 128
}
