package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestManagerDeletionCommitsCredentialsAndRejectsStaleConfirmation(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	cfg.Profiles = []core.Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	if err := a.vault.Put(p, "saved-secret"); err != nil {
		t.Fatal(err)
	}
	next, summary, err := cfg.PlanSessionDeletion([]string{p.ID}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	revision, _ := store.Revision()
	vaultRevision := a.vault.Revision()
	a.config.Profiles[0].Name = "changed"
	a.commitManagerDeletion(cfg, next, summary, revision, vaultRevision)
	if !a.vault.HasStoredCredential(p.ID) || len(a.config.Profiles) != 1 {
		t.Fatal("stale confirmation applied")
	}
	a.config = cfg.Clone()
	a.commitManagerDeletion(cfg, next, summary, revision, vaultRevision)
	waitOperator(t, a, func() bool { return !a.configurationBusy() })
	if len(a.config.Profiles) != 0 || a.vault.HasStoredCredential(p.ID) {
		t.Fatal("configuration and credential deletion diverged", a.status)
	}
}
