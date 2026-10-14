package core

import "testing"

func TestMasterWarningSharedPreferenceRoundtrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.MasterWarningAcknowledged = true
	doc := SharedDocument{Preferences: preferencesForShared(cfg)}
	restored, err := doc.Configuration()
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Settings.MasterWarningAcknowledged {
		t.Fatal("shared preferences lost warning choice")
	}
}
