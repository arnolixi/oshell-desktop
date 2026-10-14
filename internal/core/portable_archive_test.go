package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableArchiveCredentialsRebindAndCommit(t *testing.T) {
	source := NewProfile()
	source.Host = "alias"
	source.Username = "user"
	source.Proxy = ProxySettings{ID: NewID(), Kind: "socks5", Host: "proxy.example", Port: 1080, Username: "proxy-user"}
	cfg := DefaultConfig()
	cfg.Profiles = []Profile{source}
	cfg.Links = []SessionLink{{ID: NewID(), ProfileID: source.ID, Name: "shortcut", Folder: "work"}}
	vault, e := OpenVault(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	resolved := SSHIdentity{Host: "server.example", User: "resolved-user", Port: 2222}
	if e = vault.PutFor(source, "login-test", resolved); e != nil {
		t.Fatal(e)
	}
	proxy := ProxyCredentialProfile(source.Proxy)
	if e = vault.Put(proxy, "proxy-test"); e != nil {
		t.Fatal(e)
	}
	archive, e := vault.PortableArchive(context.Background(), cfg, true, []byte("export-test-master"))
	if e != nil {
		t.Fatal(e)
	}
	data, e := archive.Encode()
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(data), "login-test") || strings.Contains(string(data), "proxy-test") {
		t.Fatal("plaintext password leaked")
	}
	decoded, e := DecodePortableArchive(data)
	if e != nil {
		t.Fatal(e)
	}
	if decoded.PasswordCount() != 2 {
		t.Fatal("lost credentials")
	}
	if _, _, e = decoded.Merge(context.Background(), cfg, "", true, []byte("wrong-master")); e == nil {
		t.Fatal("wrong password accepted")
	}
	next, updates, e := decoded.Merge(context.Background(), cfg, "Imported", true, []byte("export-test-master"))
	if e != nil {
		t.Fatal(e)
	}
	if len(next.Profiles) != 2 || next.Profiles[1].ID == source.ID || next.Profiles[1].Proxy.ID == source.Proxy.ID || next.Links[1].ProfileID != next.Profiles[1].ID {
		t.Fatal("IDs not rebound")
	}
	store := &Store{Dir: t.TempDir()}
	destination, _ := OpenVault(store.Dir)
	if e = store.CommitCredentials(context.Background(), next, destination, updates, ""); e != nil {
		t.Fatal(e)
	}
	got, e := destination.GetFor(next.Profiles[1], resolved)
	if e != nil || got != "login-test" {
		t.Fatal(got, e)
	}
	route, e := ProxyRoute(next.Profiles[1].ProxyID, next.Proxies)
	if e != nil || len(route) != 1 {
		t.Fatal(route, e)
	}
	got, e = destination.GetFor(ProxyCredentialProfile(route[0].Settings), proxy.Identity())
	if e != nil || got != "proxy-test" {
		t.Fatal(got, e)
	}
	if _, e = os.Stat(filepath.Join(store.Dir, transactionFile)); !os.IsNotExist(e) {
		t.Fatal("journal not removed")
	}
	loaded, e := store.Load()
	if e != nil || len(loaded.Profiles) != 2 {
		t.Fatal(e)
	}
	stale := next.Clone()
	stale.Profiles[1].Name = "changed"
	if e = store.CommitCredentials(context.Background(), stale, destination, nil, ""); e == nil {
		t.Fatal("stale revision accepted")
	}
}
func TestPortableArchiveRejectsMalformedDefinitions(t *testing.T) {
	for _, data := range []string{`{"format":"OShell.sessions","version":2,"profiles":[],"directories":null}`, `{"format":"OShell.sessions","version":2,"profiles":[{"name":"missing identity"}],"directories":[]}`} {
		if _, e := DecodePortableArchive([]byte(data)); e == nil {
			t.Fatal("malformed archive accepted")
		}
	}
	empty := PortableArchive{Format: "OShell.sessions", Version: 2}
	b, e := empty.Encode()
	if e != nil || strings.Contains(string(b), "null") {
		t.Fatal(string(b), e)
	}
}
func TestTransactionRecoveryAppliesWholeState(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir}
	vault, _ := OpenVault(dir)
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "example.invalid"
	p.Username = "user"
	cfg.Profiles = []Profile{p}
	// A directory at the final config path forces commit to stop after its durable intent.
	if e := os.Mkdir(filepath.Join(dir, "config.json"), 0700); e != nil {
		t.Fatal(e)
	}
	// Construct the same persisted journal as an interrupted successful staging step.
	cfgBytes, _ := json.Marshal(cfg)
	vault.Put(p, "test-password")
	vaultBytes, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	journal, _ := json.Marshal(stateTransaction{1, cfgBytes, vaultBytes})
	os.WriteFile(filepath.Join(dir, transactionFile), journal, 0600)
	if e := recoverTransaction(dir); e == nil {
		t.Fatal("failed write not detected")
	}
	os.Remove(filepath.Join(dir, "config.json"))
	if e := recoverTransaction(dir); e != nil {
		t.Fatal(e)
	}
	loaded, e := store.Load()
	if e != nil || len(loaded.Profiles) != 1 {
		t.Fatal(e)
	}
	restored, e := OpenVault(dir)
	if e != nil {
		t.Fatal(e)
	}
	secret, e := restored.Get(p)
	if e != nil || secret != "test-password" {
		t.Fatal(secret, e)
	}
}
