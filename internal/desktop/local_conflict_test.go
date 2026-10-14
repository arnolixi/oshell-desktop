package desktop

import (
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
)

func localConflictUIFixture(t *testing.T) (*App, core.Profile, []byte) {
	return localConflictUIFixtureProtected(t, "")
}
func localConflictUIFixtureProtected(t *testing.T, master string) (*App, core.Profile, []byte) {
	t.Helper()
	t.Setenv("OSHELL_TEST_IGNORE_SSH_CONFIG", "1")
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name = "source"
	p.Username = "tester"
	p.Host = "local.invalid"
	p.Group = ""
	cfg.Profiles = []core.Profile{p}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	if err := a.vault.PutFor(p, "local-password", p.Identity()); err != nil {
		t.Fatal(err)
	}
	if master != "" {
		if err := a.vault.SetMaster(master); err != nil {
			t.Fatal(err)
		}
		revision, _ := store.Revision()
		if err := store.ProtectConfiguration(cfg, []byte(master), revision); err != nil {
			t.Fatal(err)
		}
	}
	remoteStore := &core.Store{Dir: store.Dir}
	var remote core.Config
	var err error
	if master != "" {
		remote, err = remoteStore.UnlockConfiguration([]byte(master))
	} else {
		remote, err = remoteStore.Load()
	}
	if err != nil {
		t.Fatal(err)
	}
	remoteVault, err := core.OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if master != "" {
		if err = remoteVault.Unlock(master); err != nil {
			t.Fatal(err)
		}
	}
	remote.Profiles[0].Host = "disk.invalid"
	if err = remoteVault.PutFor(remote.Profiles[0], "disk-password", remote.Profiles[0].Identity()); err != nil {
		t.Fatal(err)
	}
	if err = remoteStore.Save(remote); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(store.Dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	return a, p, data
}
func TestLocalConflictUIAutomaticSaveChoiceAndCancel(t *testing.T) {
	for _, action := range []string{"merge", "reload", "cancel"} {
		t.Run(action, func(t *testing.T) {
			a, p, before := localConflictUIFixture(t)
			oldVault := a.vault
			live, _ := operatorPane(t, a, "live-terminal")
			term := live.term
			a.edit(p)
			a.draft.Name = "local rename"
			a.saveDraft(false)
			waitOperator(t, a, func() bool { return a.features.localConflict.plan != nil && !a.features.localConflict.busy })
			f := &a.features.localConflict
			tt := ui.NewTester(a.view, 1100, 800)
			if !tt.HasText("本地配置发生变化") {
				t.Fatal("save did not open conflict dialog")
			}
			switch action {
			case "merge":
				for _, value := range f.choices {
					*value = "local"
				}
				tt.Frame()
				if err := tt.Click("按选择合并并保存"); err != nil {
					t.Fatal(err)
				}
			case "reload":
				if err := tt.Click("放弃草稿并读取磁盘"); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				if err := tt.Click("取消处理"); err != nil {
					t.Fatal(err)
				}
			}
			waitOperator(t, a, func() bool { return !f.busy && !f.active })
			if a.configurationBusy() || live.term != term || live.closed {
				t.Fatal("operation gate or live connection changed")
			}
			after, _ := os.ReadFile(filepath.Join(a.store.Dir, "config.json"))
			if action == "cancel" {
				if string(after) != string(before) || !a.editing || a.vault != oldVault {
					t.Fatal("cancel changed config/vault/editor")
				}
				return
			}
			host, password := "local.invalid", "local-password"
			if action == "reload" {
				host, password = "disk.invalid", "disk-password"
				if string(after) != string(before) {
					t.Fatal("reload wrote disk")
				}
			}
			saved := a.config.Profiles[0]
			if saved.Host != host || a.vault == oldVault {
				t.Fatal("wrong resolved state")
			}
			if secret, err := a.vault.GetFor(saved, saved.Identity()); err != nil || secret != password {
				t.Fatal("configuration/credential pairing lost", err)
			}
		})
	}
}

func TestLocalConflictKeepsPendingPasswordEdit(t *testing.T) {
	a, p, _ := localConflictUIFixture(t)
	a.edit(p)
	a.draft.Name = "pending password"
	a.features.sessionPassword.password = "new-local-password"
	a.features.sessionPassword.remember = true
	a.saveDraft(false)
	waitOperator(t, a, func() bool { return a.features.localConflict.plan != nil && !a.features.localConflict.busy })
	f := &a.features.localConflict
	for _, value := range f.choices {
		*value = "local"
	}
	a.resolveLocalConflict(false)
	waitOperator(t, a, func() bool { return !f.active && !f.busy })
	saved := a.config.Profiles[0]
	if secret, err := a.vault.GetFor(saved, saved.Identity()); err != nil || secret != "new-local-password" {
		t.Fatal("pending password edit lost", err, a.status)
	}
	if a.features.sessionPassword.password != "" || len(f.changes) != 0 {
		t.Fatal("completed conflict retained plaintext inputs")
	}
}
func TestLocalConflictReadFailureCanBeCancelled(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	if err := os.WriteFile(filepath.Join(store.Dir, "config.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	a.beginLocalConflict(cfg, nil)
	waitOperator(t, a, func() bool { return !a.features.localConflict.busy })
	if a.features.localConflict.error == "" {
		t.Fatal("read error hidden")
	}
	a.features.localConflict.open = false
	tt := ui.NewTester(a.localConflictDialog, 900, 600)
	tt.Frame()
	if a.configurationBusy() || a.features.localConflict.active {
		t.Fatal("failed preview left operation gate locked")
	}
}

func TestLocalConflictCancelDuringPreparationAndAutomaticMerge(t *testing.T) {
	a, _, before := localConflictUIFixture(t)
	a.beginLocalConflict(a.config, nil)
	a.cancelLocalConflict()
	waitOperator(t, a, func() bool { return !a.features.localConflict.active && !a.features.localConflict.busy })
	if a.configurationBusy() || a.features.localConflict.plan != nil {
		t.Fatal("cancelled preparation retained locks or keys")
	}
	after, _ := os.ReadFile(filepath.Join(a.store.Dir, "config.json"))
	if string(before) != string(after) {
		t.Fatal("cancelled preview wrote config")
	}
	a.changeConfig(func(cfg *core.Config) error { cfg.Settings.FontSize = 19; return nil })
	waitOperator(t, a, func() bool { return a.features.localConflict.plan != nil && !a.features.localConflict.busy })
	if len(a.features.localConflict.plan.Merge.Conflicts) != 0 {
		t.Fatal("independent settings/session changes conflicted")
	}
	a.resolveLocalConflict(false)
	waitOperator(t, a, func() bool { return !a.features.localConflict.active && !a.features.localConflict.busy })
	if a.config.Settings.FontSize != 19 || a.config.Profiles[0].Host != "disk.invalid" {
		t.Fatal("automatic merged values missing")
	}
}

func TestLocalConflictDraftResumesAfterDesktopRestart(t *testing.T) {
	for _, master := range []string{"", "fixture-draft-master"} {
		t.Run(map[bool]string{false: "local", true: "master"}[master != ""], func(t *testing.T) {
			a, p, _ := localConflictUIFixtureProtected(t, master)
			a.edit(p)
			a.draft.Name = "restart draft"
			a.features.sessionPassword.password = "restart-draft-password"
			a.features.sessionPassword.remember = true
			a.saveDraft(false)
			waitOperator(t, a, func() bool { return a.features.localConflict.plan != nil && !a.features.localConflict.busy })
			a.cancelLocalConflict()
			if !a.store.LocalConflictDraftExists() {
				t.Fatal("cancel lost durable draft")
			}
			store := &core.Store{Dir: a.store.Dir}
			cfg, err := startupConfiguration(store)
			if err != nil {
				t.Fatal(err)
			}
			restarted := newApp(store, cfg)
			if master != "" {
				restarted.features.startupUnlock = true
				restarted.features.vaultAction = "unlock"
				restarted.features.master = master
				restarted.runVaultAction()
			} else {
				restarted.maybeResumeLocalConflict()
			}
			waitOperator(t, restarted, func() bool {
				return restarted.features.localConflict.plan != nil && !restarted.features.localConflict.busy
			})
			f := &restarted.features.localConflict
			for _, choice := range f.choices {
				*choice = "local"
			}
			restarted.resolveLocalConflict(false)
			waitOperator(t, restarted, func() bool { return !f.active && !f.busy })
			if store.LocalConflictDraftExists() || restarted.config.Profiles[0].Name != "restart draft" {
				t.Fatal("resumed draft not committed/cleared", restarted.status)
			}
			saved := restarted.config.Profiles[0]
			if secret, err := restarted.vault.GetFor(saved, saved.Identity()); err != nil || secret != "restart-draft-password" {
				t.Fatal("restart lost pending password", err)
			}
		})
	}
}
