package desktop

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"golang.org/x/crypto/ssh"
	"os"
	"oshell-desktop/internal/core"
	"strings"
	"sync/atomic"
	"testing"
)

func proxyAuthenticationFixture(t *testing.T) (*App, core.Profile, *atomic.Int32) {
	t.Helper()
	gateway, hop, _ := savedConnectionFixture(t, func(config *ssh.ServerConfig, _ ssh.Signer) {
		config.PasswordCallback = func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if string(password) != "gateway-fixture-password" {
				return nil, fmt.Errorf("wrong gateway password")
			}
			return nil, nil
		}
	})
	a, p, accepted := savedConnectionFixture(t)
	proxy := core.NewProxyProfile()
	proxy.Name = "Fixture gateway"
	proxy.Settings.Kind = "jump"
	proxy.Settings.Host = hop.Host
	proxy.Settings.Port = hop.Port
	proxy.Settings.Username = hop.Username
	proxy.Settings.SSHAuthentication = "password"
	p.ProxyID = proxy.ID()
	a.config.Profiles[0] = p
	a.config.Proxies = []core.ProxyProfile{proxy}
	if err := a.store.Save(a.config); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(a.trust.Path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(gateway.trust.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(a.trust.Path, append(first, second...), 0600); err != nil {
		t.Fatal(err)
	}
	return a, p, accepted
}

func TestProxyDialogNeverOffersTargetPasswordSaving(t *testing.T) {
	a, p, accepted := proxyAuthenticationFixture(t)
	a.request(p, 0)
	waitOperator(t, a, func() bool { return len(a.challenges) == 1 })
	q := a.challenges[0]
	if q.request.Title != "SSH 跳板机认证" || q.allowSave || q.request.Identity.Port == p.Port {
		t.Fatal("gateway prompt not distinguished from target")
	}
	q.answers[0] = "gateway-fixture-password"
	tt := ui.NewTester(a.view, 1000, 700)
	if err := tt.Click("继续认证"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return len(a.challenges) == 1 })
	q = a.challenges[0]
	if q.request.Title != "" || !q.allowSave || q.request.Identity != p.Identity() {
		t.Fatal("target prompt inherited gateway state")
	}
	q.answers[0] = "saved-fixture-password"
	tt = ui.NewTester(a.view, 1000, 700)
	if err := tt.Click("密码认证成功后保存登录密码"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("继续认证"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.tabs[0].active.term != nil && strings.Contains(a.tabs[0].active.term.Text(), "SAVED_CREDENTIAL_READY")
	})
	got, err := a.vault.GetFor(p, p.Identity())
	if err != nil || got != "saved-fixture-password" || accepted.Load() != 1 {
		t.Fatal("target save failed", err)
	}
	if a.vault.HasStoredCredential(core.ProxyCredentialProfile(a.config.Proxies[0].Settings).ID) {
		t.Fatal("proxy password persisted by target consent")
	}
}
