package core

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func sharedFixture(t *testing.T, value string) []byte {
	t.Helper()
	data, e := SealShared([]byte(value), []byte("synthetic-master"))
	if e != nil {
		t.Fatal(e)
	}
	return data
}
func wantSyncFailure(t *testing.T, err error, kind string) {
	t.Helper()
	var failure *SyncFailure
	if !errors.As(err, &failure) || failure.Kind != kind {
		t.Fatalf("wanted %s, got %v", kind, err)
	}
}
func TestSharedEnvelopeAuthenticated(t *testing.T) {
	data := sharedFixture(t, "private-configuration")
	if bytes.Contains(data, []byte("private-configuration")) {
		t.Fatal("plaintext leaked")
	}
	plain, e := OpenShared(data, []byte("synthetic-master"))
	if e != nil || string(plain) != "private-configuration" {
		t.Fatalf("roundtrip %s %v", plain, e)
	}
	if _, e = OpenShared(data, []byte("wrong")); e == nil {
		t.Fatal("accepted wrong master")
	}
}
func TestDirectorySyncCompareAndSwap(t *testing.T) {
	ctx := context.Background()
	transport := DirectoryTransport{t.TempDir()}
	first := sharedFixture(t, "first")
	second := sharedFixture(t, "second")
	if e := transport.Put(ctx, first, ""); e != nil {
		t.Fatal(e)
	}
	before, e := transport.Fetch(ctx)
	if e != nil {
		t.Fatal(e)
	}
	wantSyncFailure(t, transport.Put(ctx, second, ""), "conflict")
	if e = transport.Put(ctx, second, before.ETag); e != nil {
		t.Fatal(e)
	}
	wantSyncFailure(t, transport.Put(ctx, first, before.ETag), "conflict")
	after, e := transport.Fetch(ctx)
	if e != nil || !bytes.Equal(after.Data, second) {
		t.Fatal("stale write changed data", e)
	}
	if e = os.WriteFile(filepath.Join(transport.Directory, "credential-key"), []byte("key"), 0600); e != nil {
		t.Fatal(e)
	}
	_, e = transport.Fetch(ctx)
	wantSyncFailure(t, e, "directory")
}
func TestWebDAVConditionalWrite(t *testing.T) {
	var mu sync.Mutex
	var data []byte
	etag := ""
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		u, p, ok := r.BasicAuth()
		if !ok || u != "test-user" || p != "test-password" {
			w.WriteHeader(401)
			return
		}
		if r.URL.Path != "/vault/oshell-vault.json" {
			t.Errorf("path %s", r.URL.Path)
		}
		switch r.Method {
		case "GET":
			if data == nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("ETag", etag)
			w.Write(data)
		case "PUT":
			if data == nil {
				if r.Header.Get("If-None-Match") != "*" {
					t.Error("missing create precondition")
				}
			} else if r.Header.Get("If-Match") != etag {
				w.WriteHeader(412)
				return
			}
			var b bytes.Buffer
			b.ReadFrom(r.Body)
			data = b.Bytes()
			etag = `"next"`
			writes++
			w.WriteHeader(204)
		}
	}))
	defer server.Close()
	transport, e := NewWebDAVTransport(server.URL+"/vault", "test-user", "test-password", true)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	obj, e := transport.Fetch(ctx)
	if e != nil || obj != nil {
		t.Fatal("initial fetch", e)
	}
	first := sharedFixture(t, "first")
	if e = transport.Put(ctx, first, ""); e != nil {
		t.Fatal(e)
	}
	obj, e = transport.Fetch(ctx)
	if e != nil || !bytes.Equal(obj.Data, first) {
		t.Fatal("fetch", e)
	}
	wantSyncFailure(t, transport.Put(ctx, first, `"stale"`), "conflict")
	if writes != 1 {
		t.Fatal("stale upload was accepted")
	}
}
func TestWebDAVNoCredentialRedirect(t *testing.T) {
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	transport, e := NewWebDAVTransport(source.URL, "private-user", "private-password", true)
	if e != nil {
		t.Fatal(e)
	}
	_, e = transport.Fetch(context.Background())
	wantSyncFailure(t, e, "redirect")
	if called {
		t.Fatal("followed redirect")
	}
}
