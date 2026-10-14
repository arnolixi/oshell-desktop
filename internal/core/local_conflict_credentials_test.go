package core

import (
	"context"
	"strings"
	"testing"
)

func TestLocalConflictChoiceKeepsProfileAndCredentialTogether(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "original.invalid"
	p.Username = "tester"
	cfg.Profiles = []Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	localVault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = localVault.SetMaster("fixture-master-password"); err != nil {
		t.Fatal(err)
	}
	if err = localVault.PutFor(p, "local-secret-marker", p.Identity()); err != nil {
		t.Fatal(err)
	}
	revision, _ := store.Revision()
	if err = store.ProtectConfiguration(cfg, []byte("fixture-master-password"), revision); err != nil {
		t.Fatal(err)
	}
	otherStore := &Store{Dir: dir}
	disk, err := otherStore.UnlockConfiguration([]byte("fixture-master-password"))
	if err != nil {
		t.Fatal(err)
	}
	remoteVault, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = remoteVault.Unlock("fixture-master-password"); err != nil {
		t.Fatal(err)
	}
	disk.Profiles[0].Host = "remote.invalid"
	if err = remoteVault.PutFor(disk.Profiles[0], "remote-secret-marker", disk.Profiles[0].Identity()); err != nil {
		t.Fatal(err)
	}
	if err = otherStore.Save(disk); err != nil {
		t.Fatal(err)
	}
	local := cfg.Clone()
	local.Profiles[0].Name = "local rename"
	plan, err := store.PrepareLocalResolution(context.Background(), local, localVault, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if len(plan.Merge.Conflicts) != 1 {
		t.Fatal("profile/credential conflict missing", len(plan.Merge.Conflicts))
	}
	conflict := plan.Merge.Conflicts[0]
	if strings.Contains(conflict.LocalDescription, "local-secret-marker") || strings.Contains(conflict.RemoteDescription, "remote-secret-marker") {
		t.Fatal("secret exposed by preview")
	}
	for _, choice := range []SharedMergeChoice{SharedKeepLocal, SharedKeepRemote} {
		merged, updates, err := plan.ResolveCredentials(map[string]SharedMergeChoice{conflict.Key: choice})
		if err != nil {
			t.Fatal(err)
		}
		wantHost, wantSecret := "original.invalid", "local-secret-marker"
		if choice == SharedKeepRemote {
			wantHost, wantSecret = "remote.invalid", "remote-secret-marker"
		}
		found := false
		for i := range updates {
			if updates[i].Profile.ID == p.ID {
				found = true
				if updates[i].Password != wantSecret || updates[i].Identity.Host != wantHost {
					t.Fatal("credential crossed versions")
				}
			}
			updates[i].Password = ""
		}
		if !found || merged.Profiles[0].Host != wantHost {
			t.Fatal("configuration crossed versions")
		}
	}
	plan.Close()
	if _, _, err = plan.ResolveCredentials(map[string]SharedMergeChoice{conflict.Key: SharedKeepLocal}); err == nil {
		t.Fatal("closed secret snapshot remained usable")
	}
}
