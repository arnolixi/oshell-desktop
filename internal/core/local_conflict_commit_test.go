package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func localResolutionFixture(t *testing.T) (*Store, *Vault, *Store, Config, *LocalResolutionPlan) {
	return localResolutionFixtureProtected(t, "")
}
func localResolutionFixtureProtected(t *testing.T, master string) (*Store, *Vault, *Store, Config, *LocalResolutionPlan) {
	t.Helper()
	dir := t.TempDir()
	store := &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "local-version.invalid"
	cfg.Profiles = []Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.PutFor(p, "local-password", p.Identity()); err != nil {
		t.Fatal(err)
	}
	if master != "" {
		if err = vault.SetMaster(master); err != nil {
			t.Fatal(err)
		}
		revision, _ := store.Revision()
		if err = store.ProtectConfiguration(cfg, []byte(master), revision); err != nil {
			t.Fatal(err)
		}
	}
	other := &Store{Dir: dir}
	var remote Config
	if master != "" {
		remote, err = other.UnlockConfiguration([]byte(master))
	} else {
		remote, err = other.Load()
	}
	if err != nil {
		t.Fatal(err)
	}
	diskVault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if master != "" {
		if err = diskVault.Unlock(master); err != nil {
			t.Fatal(err)
		}
	}
	remote.Profiles[0].Host = "disk-version.invalid"
	if err = diskVault.PutFor(remote.Profiles[0], "disk-password", remote.Profiles[0].Identity()); err != nil {
		t.Fatal(err)
	}
	if err = other.Save(remote); err != nil {
		t.Fatal(err)
	}
	local := cfg.Clone()
	local.Profiles[0].Name = "local-name"
	plan, err := store.PrepareLocalResolution(context.Background(), local, vault, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(plan.Close)
	return store, vault, other, remote, plan
}
func TestLocalResolutionCommitAndDiskReload(t *testing.T) {
	for _, reload := range []bool{false, true} {
		t.Run(map[bool]string{false: "merge", true: "reload"}[reload], func(t *testing.T) {
			store, _, _, _, plan := localResolutionFixture(t)
			before, _ := os.ReadFile(filepath.Join(store.Dir, "config.json"))
			var cfg Config
			var vault *Vault
			var err error
			if reload {
				cfg, vault, err = plan.ReloadDisk(store)
			} else {
				cfg, vault, err = plan.Commit(context.Background(), store, map[string]SharedMergeChoice{plan.Merge.Conflicts[0].Key: SharedKeepLocal})
			}
			if err != nil {
				t.Fatal(err)
			}
			wantHost, wantPassword := "local-version.invalid", "local-password"
			if reload {
				wantHost, wantPassword = "disk-version.invalid", "disk-password"
			}
			if cfg.Profiles[0].Host != wantHost {
				t.Fatal("configuration choice lost")
			}
			if secret, err := vault.GetFor(cfg.Profiles[0], cfg.Profiles[0].Identity()); err != nil || secret != wantPassword {
				t.Fatal("credential choice lost", err)
			}
			loaded, err := store.Load()
			if err != nil || loaded.Profiles[0].Host != wantHost {
				t.Fatal("disk/config state diverged", err)
			}
			if reload {
				after, _ := os.ReadFile(filepath.Join(store.Dir, "config.json"))
				if string(after) != string(before) {
					t.Fatal("reload wrote disk")
				}
			}
		})
	}
}
func TestLocalResolutionRejectsChangesAfterPreview(t *testing.T) {
	store, _, other, remote, plan := localResolutionFixture(t)
	remote.Settings.FontSize = 19
	if err := other.Save(remote); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plan.Commit(context.Background(), store, map[string]SharedMergeChoice{plan.Merge.Conflicts[0].Key: SharedKeepLocal}); err == nil {
		t.Fatal("stale preview overwrote external change")
	}
	latest, err := other.Load()
	if err != nil || latest.Settings.FontSize != 19 {
		t.Fatal("external change lost", err)
	}
}

func TestLocalResolutionRejectsDiskOnlyCredentialChange(t *testing.T) {
	store, original, _, remote, plan := localResolutionFixture(t)
	if err := original.Put(remote.Profiles[0], "stale-original-write"); err == nil {
		t.Fatal("stale vault overwrote external credentials")
	}
	latest, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	profile := remote.Profiles[0]
	if err = latest.PutFor(profile, "newer-disk-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	if _, _, err = plan.Commit(context.Background(), store, map[string]SharedMergeChoice{plan.Merge.Conflicts[0].Key: SharedKeepLocal}); err == nil {
		t.Fatal("credential-only update lost")
	}
	reopened, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if secret, err := reopened.GetFor(profile, profile.Identity()); err != nil || secret != "newer-disk-password" {
		t.Fatal("newer credential overwritten", err)
	}
}
