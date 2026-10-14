package core

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestDirectSSHObservedPeer(t *testing.T) {
	server := startSSH(t, "test-password")
	p := NewProfile()
	p.Host = "127.0.0.1"
	p.Username = "tester"
	_, portText, _ := net.SplitHostPort(server.address)
	p.Port, _ = strconv.Atoi(portText)
	conn, e := Dial(context.Background(), p, Credentials{Password: "test-password"}, &TrustStore{Path: filepath.Join(t.TempDir(), "known_hosts")}, func(HostPrompt) bool { return true })
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	host, port := conn.ObservedPeer()
	if host != "127.0.0.1" || port != p.Port {
		t.Fatalf("peer %s:%d", host, port)
	}
	clone, e := conn.Clone(context.Background(), p.KeepAlive)
	if e != nil {
		t.Fatal(e)
	}
	defer clone.Close()
	host, port = clone.ObservedPeer()
	if host != "127.0.0.1" || port != p.Port {
		t.Fatal("clone lost observed peer")
	}
}

func TestFileOnlyConnectionDetectsTransportClosureWithoutKeepalive(t *testing.T) {
	server := startSSH(t, "test-password")
	p := NewProfile()
	p.Kind = "sftp"
	p.Host = "127.0.0.1"
	p.Username = "tester"
	_, portText, _ := net.SplitHostPort(server.address)
	p.Port, _ = strconv.Atoi(portText)
	p.KeepAlive.Enabled = false
	conn, e := Dial(context.Background(), p, Credentials{Password: "test-password"}, &TrustStore{Path: filepath.Join(t.TempDir(), "known_hosts")}, func(HostPrompt) bool { return true })
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	if !conn.Alive() {
		t.Fatal("new file connection not alive")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := conn.WaitClosed(cancelled); !errors.Is(err, context.Canceled) || !conn.Alive() {
		t.Fatal("observer cancellation closed the connection", err)
	}
	conn.Client.Close()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := conn.WaitClosed(waitCtx); err != nil {
		t.Fatal("transport closure notification missing", err)
	}
	deadline := time.Now().Add(time.Second)
	for conn.Alive() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if conn.Alive() {
		t.Fatal("file-only transport stayed alive after closure")
	}
}
