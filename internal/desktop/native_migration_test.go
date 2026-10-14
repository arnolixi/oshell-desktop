package desktop

import (
	"context"
	"encoding/json"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
)

func TestNativeMigrationPreviewAndApply(t *testing.T) {
	source := t.TempDir()
	cfg := core.DefaultConfig()
	profile := core.NewProfile()
	profile.Host = "migration.invalid"
	profile.Username = "tester"
	cfg.Profiles = []core.Profile{profile}
	original, err := core.OpenVault(source)
	if err != nil {
		t.Fatal(err)
	}
	master := []byte("source-master-password")
	if err = original.SetMaster(string(master)); err != nil {
		t.Fatal(err)
	}
	if err = original.PutFor(profile, "source-secret", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	document, _, err := original.ExportSharedDocument(context.Background(), cfg, master, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := document.Encode(master)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(source, "configuration.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	remote := t.TempDir()
	if err = os.WriteFile(filepath.Join(remote, "configuration.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	location, _ := json.Marshal(map[string]any{"current": remote, "replicaVersion": 2})
	if err = os.WriteFile(filepath.Join(source, "storage-location.json"), location, 0600); err != nil {
		t.Fatal(err)
	}
	store := &core.Store{Dir: t.TempDir()}
	if err = store.Save(core.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, core.DefaultConfig())
	if err = a.vault.SetMaster("destination-master-password"); err != nil {
		t.Fatal(err)
	}
	a.showNativeMigration()
	f := &a.features.nativeMigration
	f.directory = source
	f.password = string(master)
	a.runNativeMigration(false)
	waitOperator(t, a, func() bool { return !f.busy })
	if f.preview == nil || f.error != "" || len(a.config.Profiles) != 0 {
		t.Fatal("preview failed or modified config", f.error)
	}
	a.runNativeMigration(true)
	waitOperator(t, a, func() bool { return !f.busy })
	if !f.open || !f.completed || f.error != "" || len(f.master) != 0 || len(a.config.Profiles) != 1 {
		t.Fatal("apply failed", f.error)
	}
	if secret, err := a.vault.GetFor(a.config.Profiles[0], profile.Identity()); err != nil || secret != "source-secret" {
		t.Fatal("password migration failed", err)
	}
	after, _ := os.ReadFile(filepath.Join(source, "configuration.json"))
	if string(after) != string(data) {
		t.Fatal("native source modified")
	}
	a.scheduleMigratedDirectory()
	waitOperator(t, a, func() bool { return !f.busy })
	if f.syncScheduled || f.error == "" || len(a.config.Profiles) != 1 {
		t.Fatal("mismatched sync password accepted or adoption lost")
	}
	if pending, _ := core.LoadPendingSyncDirectory(store.Dir); pending != nil {
		t.Fatal("failed restore scheduled sync")
	}
	if err = a.vault.ChangeMaster("destination-master-password", string(master)); err != nil {
		t.Fatal(err)
	}
	a.scheduleMigratedDirectory()
	waitOperator(t, a, func() bool { return !f.busy })
	if !f.syncScheduled || f.error != "" {
		t.Fatal("valid sync restore failed", f.error)
	}
	pending, err := core.LoadPendingSyncDirectory(store.Dir)
	if err != nil || pending == nil || pending.Mode != "existing" {
		t.Fatal("missing restart choice", err)
	}
	remoteAfter, _ := os.ReadFile(filepath.Join(remote, "configuration.json"))
	if string(remoteAfter) != string(data) {
		t.Fatal("sync restore changed remote")
	}

}
