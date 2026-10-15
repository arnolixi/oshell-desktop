package desktop

import (
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSessionEditorSavesPreservesAndRemovesPasswordAtomically(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	p := core.NewProfile()
	p.Kind = "ftp"
	p.Port = 21
	p.Host = "fixture.invalid"
	p.Username = "tester"
	p.Name = "Credential fixture"
	a.edit(p)
	a.features.sessionPassword.password = "editor-secret-fixture"
	a.features.sessionPassword.remember = true
	tt := ui.NewTester(a.view, 1000, 750)
	snapshot(t, tt, "session-password-editor.png")
	a.saveDraft(false)
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	if a.editing {
		t.Fatal(a.formError)
	}
	saved, ok := a.profileByID(p.ID)
	if !ok {
		t.Fatal("profile missing")
	}
	secret, err := a.vault.GetFor(saved, saved.Identity())
	if err != nil || secret != "editor-secret-fixture" {
		t.Fatal("saved credential missing", err)
	}
	for _, name := range []string{"config.json", "credentials.json"} {
		data, err := os.ReadFile(filepath.Join(store.Dir, name))
		if err != nil || strings.Contains(string(data), "editor-secret-fixture") {
			t.Fatal("plaintext persisted", name, err)
		}
	}
	a.edit(saved)
	a.draft.Name = "Renamed"
	a.saveDraft(false)
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	if a.editing {
		t.Fatal(a.formError)
	}
	saved, _ = a.profileByID(p.ID)
	secret, err = a.vault.GetFor(saved, saved.Identity())
	if err != nil || secret != "editor-secret-fixture" {
		t.Fatal("blank rename lost credential", err)
	}
	a.edit(saved)
	a.draft.Host = "different.invalid"
	a.saveDraft(false)
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	if a.editing {
		t.Fatal(a.formError)
	}
	saved, _ = a.profileByID(p.ID)
	secret, err = a.vault.GetFor(saved, saved.Identity())
	if err != nil || secret != "editor-secret-fixture" {
		t.Fatal("changed target lost saved password", err)
	}
	a.edit(saved)
	a.features.sessionPassword.remember = false
	a.saveDraft(false)
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	if a.editing || a.vault.HasStoredCredential(p.ID) {
		t.Fatal("credential not removed atomically", a.formError)
	}
	saved, _ = a.profileByID(p.ID)
	if saved.Host != "different.invalid" {
		t.Fatal("configuration not committed with removal")
	}
}
func TestSessionEditorCancelDropsPasswordDraft(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	a.edit(p)
	a.features.sessionPassword.password = "cancelled-secret"
	a.features.sessionPassword.remember = true
	tt := ui.NewTester(a.view, 1000, 750)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.editing || a.features.sessionPassword.password != "" || a.vault.HasStoredCredential(p.ID) {
		t.Fatal("cancel retained or saved password")
	}
}

func TestSessionEditorLateCancelNeverStartsConnection(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	p := core.NewProfile()
	p.Kind = "ftp"
	p.Port = 21
	p.Host = "fixture.invalid"
	p.Username = "tester"
	p.Name = "Late cancel fixture"
	a.edit(p)
	a.features.sessionPassword.password = "fixture-password"
	a.features.sessionPassword.remember = true
	a.saveDraft(true)
	// Do not drain the UI queue until the background transaction is committed.
	// This reproduces cancellation after storage succeeded but before its UI
	// completion can execute the requested follow-up connection.
	deadline := time.Now().Add(3 * time.Second)
	committed := false
	for time.Now().Before(deadline) {
		saved, err := store.Load()
		if err == nil {
			for _, profile := range saved.Profiles {
				if profile.ID == p.ID {
					committed = true
				}
			}
		}
		if committed {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !committed {
		t.Fatal("transaction did not finish")
	}
	a.cancelSessionPassword()
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	if a.authOpen || len(a.tabs) != 0 || len(a.authQueue) != 0 {
		t.Fatal("late cancellation initiated authentication/connection")
	}
	if a.status != "配置已保存，已取消连接" {
		t.Fatal("committed cancellation misreported", a.status)
	}
	if a.features.sessionPassword.password != "" {
		t.Fatal("password draft retained")
	}
	if !a.vault.HasStoredCredential(p.ID) {
		t.Fatal("already committed credential disappeared")
	}
}

func TestSessionEditorCancellationStopsIdentityResolutionBeforeCommit(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS ssh -G cancellation fixture")
	}
	binaryDir := t.TempDir()
	started := filepath.Join(binaryDir, "started")
	script := "#!/bin/sh\n: > \"$OSHELL_SSH_EDIT_STARTED\"\nexec /bin/sleep 30\n"
	if err := os.WriteFile(filepath.Join(binaryDir, "ssh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binaryDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OSHELL_SSH_EDIT_STARTED", started)
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	p.Username = "tester"
	a.edit(p)
	a.features.sessionPassword.password = "fixture-only"
	a.features.sessionPassword.remember = true
	a.saveDraft(true)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(started); err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := os.Stat(started); err != nil {
		a.cancelSessionPassword()
		t.Fatal("resolver fixture did not start")
	}
	tt := ui.NewTester(a.view, 1000, 750)
	tt.Key(0, ui.KeyEscape)
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Profiles) != 0 || a.vault.HasStoredCredential(p.ID) || a.authOpen || len(a.tabs) != 0 {
		t.Fatal("cancelled resolution saved data or started authentication")
	}
	if a.editing || a.configurationBusy() || a.status != "已取消保存" || a.features.sessionPassword.password != "" {
		t.Fatal("cancel state not released", a.status)
	}
}

func TestSessionEditorChangesProtectionWithoutReplacingPassword(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	if err := a.vault.SetMaster("editor-master-password"); err != nil {
		t.Fatal(err)
	}
	p := core.NewProfile()
	p.Kind = "ftp"
	p.Port = 21
	p.Host = "fixture.invalid"
	p.Username = "tester"
	a.edit(p)
	a.features.sessionPassword.remember = true
	a.features.sessionPassword.password = "entry-password"
	a.features.sessionPassword.protection = "local"
	a.saveDraft(false)
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	if a.editing || a.vault.CredentialProtection(p.ID) != "local" {
		t.Fatal("local save failed", a.formError)
	}
	saved, _ := a.profileByID(p.ID)
	a.edit(saved)
	a.features.sessionPassword.protection = "master"
	a.saveDraft(false)
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	if a.editing || a.vault.CredentialProtection(p.ID) != "master" {
		t.Fatal("master conversion failed", a.formError)
	}
	secret, err := a.vault.GetFor(saved, saved.Identity())
	if err != nil || secret != "entry-password" {
		t.Fatal("conversion lost secret", err)
	}
	a.edit(saved)
	a.features.sessionPassword.protection = "local"
	a.saveDraft(false)
	waitOperator(t, a, func() bool { return !a.features.sessionPassword.busy })
	if a.editing {
		t.Fatal(a.formError)
	}
	a.vault.Lock()
	secret, err = a.vault.GetFor(saved, saved.Identity())
	if err != nil || secret != "entry-password" {
		t.Fatal("reverse conversion lost local access", err)
	}
}
