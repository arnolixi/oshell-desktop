package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeStorageReadsLegacyExternalAndModernLocalReplica(t *testing.T) {
	remote := t.TempDir()
	engine, cfg, master := syncEngineFixture(t, remote)
	plan, err := engine.Prepare(context.Background(), cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(context.Background(), nil, master); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(remote, "configuration.json"))
	if err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()
	state, err := json.Marshal(map[string]any{"current": remote, "pending": map[string]string{"path": "/not-activated", "mode": "existing"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(local, "storage-location.json"), state, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadNativeStorage(local, master)
	if err != nil || !snapshot.LegacyExternal {
		t.Fatal("legacy source not detected", err)
	}
	if _, err = ReadNativeStorage(local, []byte("wrong")); err == nil {
		t.Fatal("wrong master accepted")
	}
	if err = os.WriteFile(filepath.Join(local, "configuration.json"), original, 0600); err != nil {
		t.Fatal(err)
	}
	state, _ = json.Marshal(map[string]any{"current": "/missing-remote", "replicaVersion": 2})
	os.WriteFile(filepath.Join(local, "storage-location.json"), state, 0600)
	snapshot, err = ReadNativeStorage(local, master)
	if err != nil || snapshot.LegacyExternal {
		t.Fatal("modern source did not use local replica", err)
	}
	after, _ := os.ReadFile(filepath.Join(remote, "configuration.json"))
	if string(after) != string(original) {
		t.Fatal("source modified")
	}
}
