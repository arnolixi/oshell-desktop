package core

import (
	"encoding/json"
	"testing"
)

func TestRendererPreferenceDefaultsAndNativeMapping(t *testing.T) {
	var settings Settings
	if err := json.Unmarshal([]byte(`{}`), &settings); err != nil || !settings.Metal {
		t.Fatal("legacy config lost GPU default", err)
	}
	document := SharedDocument{Preferences: map[string]json.RawMessage{"metal": json.RawMessage(`false`)}}
	cfg, err := document.Configuration()
	if err != nil || cfg.Settings.Metal {
		t.Fatal("native software preference lost", err)
	}
	if string(preferencesForShared(cfg)["metal"]) != "false" {
		t.Fatal("export lost preference")
	}
	data, err := json.Marshal(cfg.Settings)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &settings); err != nil || settings.Metal {
		t.Fatal("save/load lost software preference", err)
	}
}
