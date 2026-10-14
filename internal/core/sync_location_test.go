package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectorySyncScheduleActivationAndCancel(t *testing.T) {
	local, target := t.TempDir(), t.TempDir()
	store := &Store{Dir: local}
	if err := store.Save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(local, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ScheduleSyncDirectory(local, target, "create", []byte("fixture-master")); err != nil {
		t.Fatal(err)
	}
	pending, err := LoadPendingSyncDirectory(local)
	if err != nil || pending == nil {
		t.Fatal("pending choice missing", err)
	}
	options, _ := LoadSyncOptions(local)
	if options.Enabled {
		t.Fatal("schedule activated immediately")
	}
	if err = ActivatePendingSyncDirectory(local); err != nil {
		t.Fatal(err)
	}
	options, err = LoadSyncOptions(local)
	if err != nil || !options.Enabled || options.Backend != "directory" || options.RequireExisting {
		t.Fatal("activation failed", err)
	}
	pending, err = LoadPendingSyncDirectory(local)
	if err != nil || pending != nil {
		t.Fatal("pending not cleared", err)
	}
	after, _ := os.ReadFile(filepath.Join(local, "config.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("local database moved/changed")
	}
	second := t.TempDir()
	if err = ScheduleSyncDirectory(local, second, "create", []byte("fixture-master")); err != nil {
		t.Fatal(err)
	}
	if err = CancelPendingSyncDirectory(local); err != nil {
		t.Fatal(err)
	}
	options, _ = LoadSyncOptions(local)
	canonicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if options.Address != canonicalTarget {
		t.Fatal("cancel changed active destination")
	}
}
func TestDirectorySyncChoiceRejectsUnsafeOrChangedTargets(t *testing.T) {
	local := t.TempDir()
	nested := filepath.Join(local, "nested")
	os.Mkdir(nested, 0700)
	if err := ScheduleSyncDirectory(local, nested, "create", []byte("master")); err == nil {
		t.Fatal("nested directory accepted")
	}
	target := t.TempDir()
	if err := ScheduleSyncDirectory(local, target, "create", nil); err == nil {
		t.Fatal("missing master accepted")
	}
	if err := ScheduleSyncDirectory(local, target, "existing", []byte("master")); err == nil {
		t.Fatal("missing encrypted config accepted")
	}
	if err := ScheduleSyncDirectory(local, target, "create", []byte("master")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(target, "unexpected"), []byte("keep"), 0600)
	if err := ActivatePendingSyncDirectory(local); err == nil {
		t.Fatal("changed destination activated")
	}
	pending, _ := LoadPendingSyncDirectory(local)
	if pending == nil {
		t.Fatal("failed activation lost pending choice")
	}
}
func TestConnectExistingDirectoryValidatesEncryption(t *testing.T) {
	target := t.TempDir()
	engine, cfg, master := syncEngineFixture(t, target)
	plan, err := engine.Prepare(context.Background(), cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(context.Background(), nil, master); err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()
	if err = ScheduleSyncDirectory(local, target, "existing", []byte("wrong-password")); err == nil {
		t.Fatal("wrong master accepted")
	}
	if err = ScheduleSyncDirectory(local, target, "existing", master); err != nil {
		t.Fatal(err)
	}
	if err = ActivatePendingSyncDirectory(local); err != nil {
		t.Fatal(err)
	}
	options, _ := LoadSyncOptions(local)
	if !options.RequireExisting {
		t.Fatal("existing data allowed recreation")
	}
}
