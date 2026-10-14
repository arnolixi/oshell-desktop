package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWebDAVSettingsProtectionAndDisable(t *testing.T) {
	dir := t.TempDir()
	master := []byte("fixture-master")
	options := SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "https://private.invalid/path", Username: "private-user"}
	if err := SaveSyncOptions(dir, options, master); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "sync-settings.json")
	raw, err := os.ReadFile(file)
	if err != nil || !IsSharedVault(raw) || bytes.Contains(raw, []byte(options.Address)) || bytes.Contains(raw, []byte(options.Username)) {
		t.Fatal("metadata not encrypted", err)
	}
	if _, err := LoadSyncOptions(dir); !errors.Is(err, ErrSyncSettingsLocked) {
		t.Fatal(err)
	}
	status, err := SyncOptionsStatus(dir)
	if err != nil || !status.Enabled || status.Address != "" || status.Username != "" {
		t.Fatal("locked status exposed metadata", err)
	}
	got, err := LoadSyncOptions(dir, master)
	if err != nil || got != options {
		t.Fatal("roundtrip failed", err)
	}
	for _, key := range [][]byte{nil, []byte("wrong")} {
		if err := SaveSyncOptions(dir, options, key); err == nil {
			t.Fatal("invalid key accepted")
		}
		current, _ := os.ReadFile(file)
		if !bytes.Equal(current, raw) {
			t.Fatal("failure changed file")
		}
	}
	options.Enabled = false
	if err := SaveSyncOptions(dir, options, master); err != nil {
		t.Fatal(err)
	}
	got, err = LoadSyncOptions(dir)
	if err != nil || got.Enabled || got.Address != "" || got.Username != "" {
		t.Fatal("disable retained login settings", err)
	}
}
