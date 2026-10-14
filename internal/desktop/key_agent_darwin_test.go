//go:build darwin

package desktop

import (
	"context"
	"encoding/pem"
	"github.com/egoist/mygo/ui"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"os"
	"oshell-desktop/internal/core"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type signingFixtureAgent struct {
	agent.Agent
	signatures atomic.Int32
}

func (a *signingFixtureAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	a.signatures.Add(1)
	return a.Agent.Sign(key, data)
}
func TestAgentLoadedEncryptedKeyConnectsWithoutRedundantPrompts(t *testing.T) {
	a, p, accepted := savedConnectionFixture(t, keyOnlyFixture)
	raw, err := os.ReadFile(p.IdentityFile)
	if err != nil {
		t.Fatal(err)
	}
	private, err := ssh.ParseRawPrivateKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := ssh.MarshalPrivateKeyWithPassphrase(private, "fixture", []byte("fixture-key-passphrase"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p.IdentityFile, pem.EncodeToMemory(encrypted), 0600); err != nil {
		t.Fatal(err)
	}
	keyring := agent.NewKeyring()
	if err = keyring.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	service := &signingFixtureAgent{Agent: keyring}
	temp, err := os.CreateTemp("", "oshell-agent-")
	if err != nil {
		t.Fatal(err)
	}
	socket := temp.Name()
	temp.Close()
	os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(socket, 0600)
	var mu sync.Mutex
	var clients []net.Conn
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			clients = append(clients, conn)
			mu.Unlock()
			wg.Add(1)
			go func() { defer wg.Done(); defer conn.Close(); agent.ServeAgent(service, conn) }()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		mu.Lock()
		for _, conn := range clients {
			conn.Close()
		}
		mu.Unlock()
		wg.Wait()
		os.Remove(socket)
	})
	t.Setenv("SSH_AUTH_SOCK", socket)
	t.Setenv("OSHELL_TEST_NO_AGENT", "0")
	t.Setenv("OSHELL_TEST_IGNORE_SSH_CONFIG", "1")
	candidate := p
	candidate.IdentityFile = ""
	if !core.SSHKeyAuthenticationCandidate(context.Background(), candidate) {
		t.Fatal("agent public key list not detected")
	}
	a.request(p, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.tabs[0].active.term != nil && strings.Contains(a.tabs[0].active.term.Text(), "SAVED_CREDENTIAL_READY")
	})
	if a.authOpen || len(a.challenges) != 0 || accepted.Load() != 1 || service.signatures.Load() == 0 {
		t.Fatal("agent-backed key required redundant prompt or was not used")
	}
	if a.vault.HasStoredCredential(p.ID) {
		t.Fatal("agent authentication persisted a password")
	}
	next := p
	next.ID = core.NewID()
	next.Name = "Agent-only fixture"
	next.IdentityFile = ""
	a.config.Profiles = append(a.config.Profiles, next)
	a.request(next, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 2 && a.tabs[1].active.term != nil && strings.Contains(a.tabs[1].active.term.Text(), "SAVED_CREDENTIAL_READY")
	})
	if a.authOpen || len(a.challenges) != 0 || a.vault.HasStoredCredential(next.ID) {
		t.Fatal("agent-only session did not connect directly")
	}
	if err = keyring.RemoveAll(); err != nil {
		t.Fatal(err)
	}
	if core.SSHKeyAuthenticationCandidate(context.Background(), candidate) {
		t.Fatal("empty agent falsely treated as usable authentication")
	}

}

func TestEncryptedKeyPromptsOnlyForNecessaryPassphrase(t *testing.T) {
	a, p, _ := savedConnectionFixture(t, keyOnlyFixture)
	raw, err := os.ReadFile(p.IdentityFile)
	if err != nil {
		t.Fatal(err)
	}
	private, err := ssh.ParseRawPrivateKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := ssh.MarshalPrivateKeyWithPassphrase(private, "fixture", []byte("fixture-key-passphrase"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p.IdentityFile, pem.EncodeToMemory(encrypted), 0600); err != nil {
		t.Fatal(err)
	}
	a.request(p, 0)
	waitOperator(t, a, func() bool { return len(a.challenges) == 1 })
	if a.authOpen || a.challenges[0].allowSave || a.challenges[0].request.Name != "解锁 SSH 私钥" {
		t.Fatal("unexpected preconnection password prompt")
	}
	q := a.challenges[0]
	q.answers[0] = "fixture-key-passphrase"
	tt := ui.NewTester(a.view, 1000, 700)
	if err = tt.Click("继续认证"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.tabs[0].active.term != nil && strings.Contains(a.tabs[0].active.term.Text(), "SAVED_CREDENTIAL_READY")
	})
	if len(a.challenges) != 0 || a.vault.HasStoredCredential(p.ID) {
		t.Fatal("passphrase was retained or saved")
	}
}
