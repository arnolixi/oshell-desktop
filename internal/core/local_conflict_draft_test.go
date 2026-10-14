package core

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalConflictDraftResumeKeepsOriginalBaselineAndPassword(t *testing.T) {
	for _, master := range []string{"", "fixture-draft-master"} {
		t.Run(map[bool]string{false: "local", true: "master"}[master != ""], func(t *testing.T) {
			store, _, other, remote, plan := localResolutionFixtureProtected(t, master)
			id, err := store.SaveLocalConflictDraft(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(filepath.Join(store.Dir, localDraftFile))
			for _, text := range []string{"local-version.invalid", "local-password", "local-name"} {
				if bytes.Contains(raw, []byte(text)) {
					t.Fatal("plaintext draft data leaked")
				}
			}
			plan.Close()
			remote.Settings.FontSize = 19
			if err = other.Save(remote); err != nil {
				t.Fatal(err)
			}
			restarted := &Store{Dir: store.Dir}
			var disk Config
			if master != "" {
				disk, err = restarted.UnlockConfiguration([]byte(master))
			} else {
				disk, err = restarted.Load()
			}
			if err != nil {
				t.Fatal(err)
			}
			vault, err := OpenVault(store.Dir)
			if err != nil {
				t.Fatal(err)
			}
			if master != "" {
				if err = vault.Unlock(master); err != nil {
					t.Fatal(err)
				}
			}
			revision, _ := restarted.Revision()
			if err = restarted.ChangeProtection(context.Background(), disk, vault, master, "new-master-password", revision, vault.Revision()); err == nil || !strings.Contains(err.Error(), "草稿") {
				t.Fatal("master change did not protect pending draft", err)
			}
			resumed, loadedID, err := restarted.ResumeLocalResolution(context.Background(), vault)
			if err != nil {
				t.Fatal(err)
			}
			defer resumed.Close()
			if loadedID != id || len(resumed.Merge.Conflicts) != 1 {
				t.Fatal("draft baseline or identity lost")
			}
			cfg, nextVault, err := resumed.Commit(context.Background(), restarted, map[string]SharedMergeChoice{resumed.Merge.Conflicts[0].Key: SharedKeepLocal})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Profiles[0].Host != "local-version.invalid" || cfg.Settings.FontSize != 19 {
				t.Fatal("resume ignored original draft or newer remote changes")
			}
			if secret, err := nextVault.GetFor(cfg.Profiles[0], cfg.Profiles[0].Identity()); err != nil || secret != "local-password" {
				t.Fatal("resumed draft lost password", err)
			}
			if err = restarted.ClearLocalConflictDraft(id); err != nil || restarted.LocalConflictDraftExists() {
				t.Fatal("draft not cleared", err)
			}
		})
	}
}

func TestLocalConflictDraftRejectsWrongCleanupAndPreservesCorruption(t *testing.T) {
	store, vault, _, _, plan := localResolutionFixture(t)
	id, err := store.SaveLocalConflictDraft(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ClearLocalConflictDraft(NewID()); err == nil || !store.LocalConflictDraftExists() {
		t.Fatal("cleanup removed another draft")
	}
	envelope, err := store.readLocalDraftEnvelope()
	if err != nil {
		t.Fatal(err)
	}
	if envelope.ID != id {
		t.Fatal("draft token changed")
	}
	envelope.Data[len(envelope.Data)/2] ^= 1
	corrupted, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(store.Dir, localDraftFile)
	if err = os.WriteFile(file, corrupted, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.ResumeLocalResolution(context.Background(), vault); err == nil {
		t.Fatal("corrupted draft accepted")
	}
	after, _ := os.ReadFile(file)
	if !bytes.Equal(after, corrupted) {
		t.Fatal("failed resume modified draft")
	}
	if err = store.ClearLocalConflictDraft(""); err != nil || store.LocalConflictDraftExists() {
		t.Fatal("explicit discard failed", err)
	}
}

func TestProtectedVaultRejectsLocalOnlyDraftDowngrade(t *testing.T) {
	store, _, _, _, plan := localResolutionFixture(t)
	if _, err := store.SaveLocalConflictDraft(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	current, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = current.SetMaster("new-protected-state"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.ResumeLocalResolution(context.Background(), current); err == nil || !store.LocalConflictDraftExists() {
		t.Fatal("weaker draft mode accepted or draft destroyed")
	}
}
