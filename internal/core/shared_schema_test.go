package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUnifiedSharedEnvelopeVersions(t *testing.T) {
	data, err := os.ReadFile("testdata/oshell-swift-shared-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	master := []byte("interop-fixture-master-🔐")
	doc, err := DecodeSharedDocument(data, master)
	if err != nil {
		t.Fatal("v1 migration", err)
	}
	encoded, err := doc.Encode(master)
	if err != nil {
		t.Fatal(err)
	}
	var envelope sharedEnvelope
	if err = json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Version != 2 {
		t.Fatal("shared configuration must write v2")
	}
	if _, err = DecodeSharedDocument(encoded, master); err != nil {
		t.Fatal("v2 decode", err)
	}
	envelope.Version = 1
	downgraded, _ := json.Marshal(envelope)
	if _, err = OpenShared(downgraded, master); err == nil {
		t.Fatal("version downgrade authenticated")
	}
	envelope.Version = 3
	future, _ := json.Marshal(envelope)
	if _, err = OpenShared(future, master); err == nil {
		t.Fatal("unknown version accepted")
	}
}
func TestUnsupportedSharedFieldsCannotReplaceEitherSide(t *testing.T) {
	engine, cfg, master := syncEngineFixture(t, t.TempDir())
	ctx := context.Background()
	plan, err := engine.Prepare(ctx, cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = plan.Commit(ctx, nil, master)
	if err != nil {
		t.Fatal(err)
	}
	before, err := engine.Transport.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := OpenShared(before.Data, master)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	json.Unmarshal(plain, &raw)
	raw["futureProtocolCapability"] = map[string]any{"secret": "sensitive"}
	plain, _ = json.Marshal(raw)
	future, err := sealSharedVersion(plain, master, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Transport.Put(ctx, future, before.ETag); err != nil {
		t.Fatal(err)
	}
	revision, err := engine.Store.Revision()
	if err != nil {
		t.Fatal(err)
	}
	vaultRevision := engine.Vault.Revision()
	if _, err = engine.Prepare(ctx, cfg, master, false); !errors.Is(err, ErrSharedProtocol) {
		t.Fatal("unknown field not reported", err)
	}
	after, err := engine.Transport.Fetch(ctx)
	if err != nil || !bytes.Equal(after.Data, future) {
		t.Fatal("unsupported remote overwritten", err)
	}
	next, err := engine.Store.Revision()
	if err != nil || next != revision || engine.Vault.Revision() != vaultRevision {
		t.Fatal("local data changed", err)
	}
}
func TestSharedSchemaDefaultsAndTypes(t *testing.T) {
	input := []byte(`{"profiles":[],"preferences":{}}`)
	data, err := canonicalConfigurationJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := canonicalConfigurationJSON(data)
	if err != nil || !bytes.Equal(data, twice) {
		t.Fatal("defaults not idempotent", err)
	}
	var root map[string]any
	json.Unmarshal(data, &root)
	prefs := root["preferences"].(map[string]any)
	if prefs["quickSendScope"] != float64(0) || prefs["fontSize"] != float64(13) {
		t.Fatal("defaults")
	}
	for _, value := range []any{true, "13", nil} {
		prefs["fontSize"] = value
		bad, _ := json.Marshal(root)
		_, err := canonicalConfigurationJSON(bad)
		if value != nil && err == nil {
			t.Fatal("invalid font type accepted")
		}
	}
}

func TestSharedSchemaRejectsTrailingJSONAndBadLegacyType(t *testing.T) {
	if _, err := canonicalConfigurationJSON([]byte(`{"profiles":[],"preferences":{}} {}`)); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	p := NewProfile()
	p.Host = "fixture.invalid"
	data, _ := json.Marshal(p)
	var profile map[string]any
	json.Unmarshal(data, &profile)
	profile["activeHostProbe"] = "not-a-boolean"
	raw, _ := json.Marshal(map[string]any{"profiles": []any{profile}, "preferences": map[string]any{}})
	if _, err := canonicalConfigurationJSON(raw); err == nil {
		t.Fatal("invalid legacy field discarded")
	}
	profile["activeHostProbe"] = true
	profile["titleMode"] = nil
	profile["proxy"] = nil
	profile["kind"] = "ftp"
	profile["initialDirectory"] = nil
	raw, _ = json.Marshal(map[string]any{"profiles": []any{profile}, "preferences": map[string]any{}})
	data, err := canonicalConfigurationJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct{ Profiles []Profile }
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Profiles[0].InitialDirectory != "/" || decoded.Profiles[0].TitleMode != "activeProbe" || decoded.Profiles[0].Proxy.ID != absentProxyID(p.ID) {
		t.Fatal("legacy defaults not stable")
	}
}

func TestCrossPlatformShortcutCollisionPreservesConfiguration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.KeyboardShortcuts.Overrides = map[string]ShortcutOverride{
		"find":       {Shortcut: &KeyboardShortcut{KeyCode: 3, Modifiers: 1}},
		"newSession": {Shortcut: &KeyboardShortcut{KeyCode: 3, Modifiers: 4}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal("stored config rejected due to local platform", err)
	}
	if len(cfg.Settings.KeyboardShortcuts.Bindings("find", "windows")) != 0 || len(cfg.Settings.KeyboardShortcuts.Bindings("newSession", "windows")) != 0 {
		t.Fatal("ambiguous Windows shortcut executed")
	}
	if len(cfg.Settings.KeyboardShortcuts.Bindings("find", "darwin")) != 1 || len(cfg.Settings.KeyboardShortcuts.Bindings("newSession", "darwin")) != 1 {
		t.Fatal("distinct macOS bindings lost")
	}
	if err := cfg.Settings.KeyboardShortcuts.Validate("windows"); err == nil {
		t.Fatal("settings editor must still explain local conflict")
	}
}
