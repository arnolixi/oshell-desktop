package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncConnectionRollbackAndStaleDraft(t *testing.T) {
	dir := t.TempDir()
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = v.SetMaster("fixture-master"); err != nil {
		t.Fatal(err)
	}
	initial, err := LoadSyncOptions(dir)
	if err != nil {
		t.Fatal(err)
	}
	next := SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "https://fixture.invalid/dav", Username: "fixture-user"}
	if err = SaveSyncConnection(v, initial, next, "old-password", v.Revision()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	revision := v.Revision()
	invalid := next
	invalid.Version = 2 // The settings writer fails after credentials were staged.
	if err = SaveSyncConnection(v, next, invalid, "new-password", revision); err == nil {
		t.Fatal("accepted invalid settings")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if !bytes.Equal(before, after) || v.Revision() != revision {
		t.Fatal("failed settings changed credentials")
	}
	p := next.CredentialProfile()
	if password, err := v.GetFor(p, p.Identity()); err != nil || password != "old-password" {
		t.Fatal("rollback lost credential", err)
	}
	saved, err := LoadSyncOptions(dir, []byte("fixture-master"))
	if err != nil || saved != next {
		t.Fatal("failed settings replaced connection", err)
	}
	if err = SaveSyncConnection(v, initial, next, "stale-password", revision); err == nil {
		t.Fatal("accepted stale options")
	}
	if err = SaveSyncConnection(v, next, next, "changed-password", revision); err != nil {
		t.Fatal(err)
	}
	if err = SaveSyncConnection(v, next, next, "stale-password", revision); err == nil {
		t.Fatal("accepted stale credential revision")
	}
	directory := SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: t.TempDir()}
	if err = SaveSyncConnection(v, next, directory, "", v.Revision()); err != nil {
		t.Fatal(err)
	}
	if password, _ := v.Get(p); password != "" {
		t.Fatal("obsolete webdav credential retained")
	}
}

func TestSyncDirectoryTargetDetectionAndFirstConfirmation(t *testing.T) {
	remote := t.TempDir()
	engine, cfg, master := syncEngineFixture(t, remote)
	local := t.TempDir()
	pending, err := ValidateSyncDirectoryTarget(local, remote, master)
	if err != nil || pending.Mode != "create" {
		t.Fatal("empty detection", err)
	}
	plan, err := engine.Prepare(context.Background(), cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(context.Background(), nil, master); err != nil {
		t.Fatal(err)
	}
	pending, err = ValidateSyncDirectoryTarget(local, remote, master)
	if err != nil || pending.Mode != "existing" {
		t.Fatal("existing detection", err)
	}
	if _, err = ValidateSyncDirectoryTarget(local, remote, []byte("wrong-master")); err == nil {
		t.Fatal("wrong master accepted")
	}
	other, otherConfig, otherMaster := syncEngineFixture(t, remote)
	plan, err = other.Prepare(context.Background(), otherConfig, otherMaster, false)
	if err != nil || !plan.InitialChoiceRequired {
		t.Fatal("existing target skipped first confirmation", err)
	}
	if pending, err = LoadPendingSyncDirectory(local); err != nil || pending != nil {
		t.Fatal("live detection scheduled restart", err)
	}
	unrelated := t.TempDir()
	if err = os.WriteFile(filepath.Join(unrelated, "unrelated.txt"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateSyncDirectoryTarget(local, unrelated, master); err == nil {
		t.Fatal("nonempty folder accepted as empty")
	}
}
