package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestAuthenticationPersistsChosenProtection(t *testing.T) {
	for _, mode := range []string{"local", "master"} {
		for _, dynamic := range []bool{false, true} {
			name := mode + "/initial"
			if dynamic {
				name = mode + "/challenge"
			}
			t.Run(name, func(t *testing.T) {
				a, p, _ := savedConnectionFixture(t)
				p.Kind = "sftp"
				a.config.Profiles[0] = p
				if err := a.vault.SetMaster("fixture-master-password"); err != nil {
					t.Fatal(err)
				}
				cred := core.Credentials{Password: "saved-fixture-password", SavePassword: true, PasswordProtection: mode}
				if dynamic {
					cred = core.Credentials{}
				}
				a.open(p, cred, 0)
				if dynamic {
					waitOperator(t, a, func() bool { return len(a.challenges) == 1 })
					q := a.challenges[0]
					q.answers[0] = "saved-fixture-password"
					q.remember = true
					q.protection = mode
					tt := ui.NewTester(a.fileWindowView, 1080, 720)
					if err := tt.Click("继续认证"); err != nil {
						t.Fatal(err)
					}
				}
				waitOperator(t, a, func() bool { return a.vault.HasStoredCredential(p.ID) })
				if a.vault.CredentialProtection(p.ID) != mode {
					t.Fatal("chosen protection lost", a.status)
				}
				a.vault.Lock()
				password, err := a.vault.GetFor(p, p.Identity())
				if mode == "local" && (err != nil || password != "saved-fixture-password") {
					t.Fatal("local login credential depends on master", err)
				}
				if mode == "master" && err != core.ErrVaultLocked {
					t.Fatal("master login credential not protected", err)
				}
			})
		}
	}
}

func TestProxyEditorConvertsPasswordProtection(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	if err := a.vault.SetMaster("fixture-master-password"); err != nil {
		t.Fatal(err)
	}
	a.editProxy(nil)
	f := &a.features.proxyManager
	f.draft.Name = "Proxy fixture"
	f.draft.Settings.Kind = "http"
	f.draft.Settings.Host = "proxy.invalid"
	f.draft.Settings.Username = "tester"
	f.port = "8080"
	f.password = "proxy-secret"
	f.remember = true
	f.protection = "local"
	a.saveProxy()
	waitOperator(t, a, func() bool { return !f.busy })
	if f.editing {
		t.Fatal(f.error)
	}
	saved := a.config.Proxies[0]
	profile := core.ProxyCredentialProfile(saved.Settings)
	if a.vault.CredentialProtection(profile.ID) != "local" {
		t.Fatal("proxy local choice lost")
	}
	a.editProxy(&saved)
	f.protection = "master"
	a.saveProxy()
	waitOperator(t, a, func() bool { return !f.busy })
	if f.editing || a.vault.CredentialProtection(profile.ID) != "master" {
		t.Fatal("proxy protection conversion failed", f.error)
	}
	password, err := a.vault.GetFor(profile, profile.Identity())
	if err != nil || password != "proxy-secret" {
		t.Fatal("conversion lost proxy password", err)
	}
}
