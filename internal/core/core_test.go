package core

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func key(t *testing.T) ssh.Signer {
	t.Helper()
	_, p, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	s, e := ssh.NewSignerFromKey(p)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestTrustFirstUseAndChangedKey(t *testing.T) {
	store := &TrustStore{Path: filepath.Join(t.TempDir(), "known_hosts")}
	k := key(t)
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2222}
	calls := 0
	cb := store.Callback(context.Background(), func(p HostPrompt) bool {
		calls++
		if !strings.HasPrefix(p.Fingerprint, "SHA256:") {
			t.Fatal(p)
		}
		return true
	})
	if e := cb("localhost:2222", addr, k.PublicKey()); e != nil {
		t.Fatal(e)
	}
	if e := cb("localhost:2222", addr, k.PublicKey()); e != nil {
		t.Fatal(e)
	}
	if e := cb("localhost:2222", addr, key(t).PublicKey()); e == nil {
		t.Fatal("changed host key accepted")
	}
	if calls != 1 {
		t.Fatalf("approval calls: %d", calls)
	}
	reject := store.Callback(context.Background(), func(HostPrompt) bool { return false })
	if e := reject("other:2222", addr, key(t).PublicKey()); e == nil {
		t.Fatal("rejected host accepted")
	}
}
func TestStoreAndMacImport(t *testing.T) {
	archive := `{"format":"OShell.sessions","version":3,"profiles":[{"name":"prod","kind":"ssh","host":"example.com","port":22,"username":"dev","group":"生产","encryptedPassword":{"ciphertext":"secret"}},{"name":"jump","kind":"ssh","host":"private","port":22,"username":"dev","jumpHost":"bastion"},{"name":"FTP","kind":"ftp"}],"directories":[],"links":[]}`
	profiles, warnings, e := ImportMac([]byte(archive))
	if e != nil {
		t.Fatal(e)
	}
	if len(profiles) != 2 || len(warnings) != 2 {
		t.Fatalf("%+v %+v", profiles, warnings)
	}
	if len(profiles[0].ID) != 36 {
		t.Fatal("ID must be a macOS-compatible UUID")
	}
	s := &Store{Dir: t.TempDir()}
	c := DefaultConfig()
	c.Profiles = profiles
	if e = s.Save(c); e != nil {
		t.Fatal(e)
	}
	loaded, e := s.Load()
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(loaded.Profiles[0], profiles[0]) {
		t.Fatal("roundtrip changed profile")
	}
	data, _ := os.ReadFile(filepath.Join(s.Dir, "config.json"))
	if strings.Contains(string(data), "secret") {
		t.Fatal("credential persisted")
	}
	if _, _, e = ImportMac([]byte(`{"format":"wrong","version":2}`)); e == nil {
		t.Fatal("invalid archive accepted")
	}
	os.WriteFile(filepath.Join(s.Dir, "config.json"), []byte("broken"), 0600)
	if _, e = s.Load(); e == nil {
		t.Fatal("corrupted configuration silently accepted")
	}
}
func TestProfileIPv6AndValidation(t *testing.T) {
	p := NewProfile()
	p.Username = "test"
	p.Host = "::1"
	if p.Address() != "[::1]:22" {
		t.Fatal(p.Address())
	}
	for _, h := range []string{"", "a b", "-bad", "x\ny"} {
		p.Host = h
		if p.Validate() == nil {
			t.Fatal(h)
		}
	}
	var out map[string]any
	b, _ := json.Marshal(p)
	json.Unmarshal(b, &out)
	if _, ok := out["jumpHost"]; !ok {
		t.Fatal("mac importer requires jumpHost")
	}
}

func TestUpgradePreservesOldConfiguration(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	original := []byte(`{"version":1,"profiles":[],"settings":{"dark":true,"fontSize":15,"scrollbackMB":8}}`)
	os.WriteFile(filepath.Join(s.Dir, "config.json"), original, 0600)
	cfg, e := s.Load()
	if e != nil {
		t.Fatal(e)
	}
	if cfg.Version != 3 || !cfg.Settings.Dark || cfg.Settings.FontSize != 15 {
		t.Fatal(cfg)
	}
	if e = s.Save(cfg); e != nil {
		t.Fatal(e)
	}
	backup, e := os.ReadFile(filepath.Join(s.Dir, "config-v1.backup.json"))
	if e != nil || string(backup) != string(original) {
		t.Fatal("upgrade backup missing")
	}
}

// A loopback SSH/SFTP server exercises authentication, trust, PTY, resizing,
// bidirectional bytes, file access and cancellation without real credentials.
func TestSSHAndSFTP(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OSHELL_TEST_NO_AGENT", "1")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		if c.User() == "tester" && string(p) == "test-only" {
			return nil, nil
		}
		return nil, fmt.Errorf("denied")
	}}
	cfg.AddHostKey(key(t))
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		nc, e := ln.Accept()
		if e != nil {
			return
		}
		sc, channels, requests, e := ssh.NewServerConn(nc, cfg)
		if e != nil {
			return
		}
		defer sc.Close()
		go ssh.DiscardRequests(requests)
		for ch := range channels {
			if ch.ChannelType() != "session" {
				ch.Reject(ssh.UnknownChannelType, "session only")
				continue
			}
			c, reqs, e := ch.Accept()
			if e != nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer c.Close()
				for r := range reqs {
					switch r.Type {
					case "pty-req", "window-change":
						r.Reply(true, nil)
					case "shell":
						r.Reply(true, nil)
						io.Copy(c, c)
						return
					case "subsystem":
						var s struct{ Name string }
						ssh.Unmarshal(r.Payload, &s)
						if s.Name != "sftp" {
							r.Reply(false, nil)
							continue
						}
						r.Reply(true, nil)
						server, e := sftp.NewServer(c)
						if e == nil {
							server.Serve()
							server.Close()
						}
						return
					default:
						r.Reply(false, nil)
					}
				}
			}()
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	p := NewProfile()
	p.Host = host
	p.Username = "tester"
	fmt.Sscan(port, &p.Port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	trust := &TrustStore{Path: filepath.Join(t.TempDir(), "known_hosts")}
	conn, e := Dial(ctx, p, Credentials{Password: "test-only"}, trust, func(HostPrompt) bool { return true })
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	if e = conn.Resize(120, 40); e != nil {
		t.Fatal(e)
	}
	if _, e = conn.Write([]byte("hello SSH\n")); e != nil {
		t.Fatal(e)
	}
	got := make(chan string, 1)
	go func() {
		b := make([]byte, 10)
		_, e := io.ReadFull(conn, b)
		if e != nil {
			got <- e.Error()
		} else {
			got <- string(b)
		}
	}()
	select {
	case s := <-got:
		if s != "hello SSH\n" {
			t.Fatal(s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SSH read blocked")
	}
	fs, e := sftp.NewClient(conn.Client)
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	file := filepath.Join(root, "transfer.txt")
	f, e := fs.Create(file)
	if e != nil {
		t.Fatal(e)
	}
	f.Write([]byte("SFTP roundtrip"))
	f.Close()
	b, e := os.ReadFile(file)
	if e != nil || string(b) != "SFTP roundtrip" {
		t.Fatalf("%s %v", b, e)
	}
	fs.Close()
	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("transport leaked after cancellation")
	}
}
