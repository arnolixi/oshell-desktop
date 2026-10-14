package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func TestAuthenticatedPasswordSaveRejectsStaleSessionAndVault(t *testing.T) {
	for _, kind := range []string{"ftp", "sftp", "ssh"} {
		for _, change := range []string{"none", "rename", "host", "protocol", "inline-proxy", "deleted", "vault-data", "vault-replaced"} {
			t.Run(kind+"/"+change, func(t *testing.T) {
				vault, err := core.OpenVault(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				profile := core.NewProfile()
				profile.Kind = kind
				profile.Host = "fixture.invalid"
				profile.Username = "tester"
				cfg := core.DefaultConfig()
				cfg.Profiles = []core.Profile{profile}
				a := &App{config: cfg, vault: vault}
				revision := vault.Revision()
				switch change {
				case "rename":
					a.config.Profiles[0].Name = "renamed"
				case "host":
					a.config.Profiles[0].Host = "changed.invalid"
				case "protocol":
					a.config.Profiles[0].Kind = "blank"
				case "inline-proxy":
					a.config.Profiles[0].Proxy.Host = "changed.proxy.invalid"
				case "deleted":
					a.config.Profiles = nil
				case "vault-data":
					if err = vault.PutFor(profile, "newer-fixture-password", profile.Identity()); err != nil {
						t.Fatal(err)
					}
				case "vault-replaced":
					a.vault, err = core.OpenVault(t.TempDir())
					if err != nil {
						t.Fatal(err)
					}
				}
				a.saveAuthenticatedPassword(profile, "login-fixture-password", profile.Identity(), vault, revision)
				got, err := vault.GetFor(profile, profile.Identity())
				if err != nil {
					t.Fatal(err)
				}
				want := ""
				if change == "none" || change == "rename" {
					want = "login-fixture-password"
				}
				if change == "vault-data" {
					want = "newer-fixture-password"
				}
				if got != want {
					t.Fatal("late login saved over changed credentials")
				}
				if change != "none" && change != "rename" && !strings.Contains(a.status, "保存") {
					t.Fatal("missing save result")
				}
			})
		}
	}
}

func TestFileWindowDisplaysCredentialSaveFailure(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.status = "密码库已变化，本次密码未保存"
	tt := ui.NewTester(a.fileWindowView, 1080, 720)
	if !tt.HasText(a.status) {
		t.Fatal("file window hid credential-save failure")
	}
}

func TestAuthenticatedTemporarySessionAndResolvedUsername(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "temporary", true: "existing-empty-user"}[existing], func(t *testing.T) {
			store := &core.Store{Dir: t.TempDir()}
			cfg := core.DefaultConfig()
			profile := core.NewProfile()
			profile.Host = "fixture.invalid"
			profile.Username = ""
			if existing {
				cfg.Profiles = []core.Profile{profile}
			}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			a := newApp(store, cfg)
			identity := profile.Identity()
			identity.User = "resolved-user"
			saved := a.saveAuthenticatedSessionPassword(profile, "verified-password", identity, a.vault, a.vault.Revision(), existing, "local")
			if saved == nil || saved.Username != "resolved-user" || len(a.config.Profiles) != 1 {
				t.Fatal("profile not saved with login identity", a.status)
			}
			disk, err := store.Load()
			if err != nil || disk.Profiles[0].Username != "resolved-user" {
				t.Fatal("resolved user not persisted", err)
			}
			password, err := a.vault.GetFor(*saved, identity)
			if err != nil || password != "verified-password" {
				t.Fatal("password binding mismatch", err)
			}
		})
	}
}
