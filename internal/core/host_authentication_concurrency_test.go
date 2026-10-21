package core

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/ssh/knownhosts"
)

func TestHostApprovalDoesNotBlockTrustedConnections(t *testing.T) {
	trust := testTrust(t)
	trustedKey, newKey := key(t).PublicKey(), key(t).PublicKey()
	trustedHost, newHost := "127.0.0.1:2201", "127.0.0.1:2202"
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2201}
	if err := os.WriteFile(trust.Path, []byte(knownhosts.Line([]string{knownhosts.Normalize(trustedHost)}, trustedKey)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	result := make(chan error, 1)
	go func() {
		result <- trust.Callback(ctx, func(p HostPrompt) bool {
			close(started)
			select {
			case <-release:
				return true
			case <-p.Context.Done():
				return false
			}
		})(newHost, &net.TCPAddr{IP: addr.IP, Port: 2202}, newKey)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("host prompt not opened")
	}
	knownResult := make(chan error, 1)
	go func() { knownResult <- trust.Callback(ctx, nil)(trustedHost, addr, trustedKey) }()
	select {
	case err := <-knownResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("trusted connection waited for another host's approval")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled host was trusted")
		}
	case <-time.After(time.Second):
		t.Fatal("approval did not cancel")
	}
}

func TestHostApprovalRechecksConcurrentKeyChanges(t *testing.T) {
	for _, different := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-key", true: "changed-key"}[different], func(t *testing.T) {
			trust := testTrust(t)
			original, concurrent := key(t).PublicKey(), key(t).PublicKey()
			if !different {
				concurrent = original
			}
			host := "127.0.0.1:2203"
			addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2203}
			err := trust.Callback(context.Background(), func(HostPrompt) bool {
				// This callback represents a second connection finishing approval
				// while the first operator's confirmation is still open.
				if err := trust.Callback(context.Background(), acceptHost)(host, addr, concurrent); err != nil {
					t.Fatal(err)
				}
				return true
			})(host, addr, original)
			if (err != nil) != different {
				t.Fatal("host key race was not revalidated", err)
			}
			data, err := os.ReadFile(trust.Path)
			want := knownhosts.Line([]string{knownhosts.Normalize(host)}, concurrent) + "\n"
			if err != nil || string(data) != want {
				t.Fatal("duplicate or mismatched host key written", err)
			}
		})
	}
}
