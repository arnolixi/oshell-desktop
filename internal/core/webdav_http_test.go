package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebDAVExplicitHTTPAddressValidation(t *testing.T) {
	for _, address := range []string{"http://dav.example.invalid/base", "HTTP://192.0.2.1:8080/dav", "http://[2001:db8::1]/", "https://dav.example.invalid/dav"} {
		w, err := NewWebDAVTransport(address, "fixture", "secret", false)
		if err != nil || !strings.HasSuffix(w.directory.Path, "/") {
			t.Fatalf("explicit address: %s: %v", address, err)
		}
	}
	for _, address := range []string{"ftp://dav.invalid/", "dav.invalid/", "http://user:secret@dav.invalid/", "http://dav.invalid:0/", "http://dav.invalid:65536/", "http://dav.invalid:/", "http://dav.invalid/?", "http://dav.invalid/#", "http://dav.invalid/?token=secret"} {
		if _, err := NewWebDAVTransport(address, "", "", false); err == nil {
			t.Fatalf("invalid address accepted: %s", address)
		}
	}
	for _, login := range [][2]string{{"bad:user", "secret"}, {"bad\x00user", "secret"}, {"fixture", "bad\x00secret"}, {"fixture", "secret\r\n"}, {strings.Repeat("x", 4097), "secret"}, {"fixture", strings.Repeat("x", 4097)}} {
		if _, err := NewWebDAVTransport("http://dav.invalid/", login[0], login[1], false); err == nil {
			t.Fatal("invalid credential accepted")
		}
	}
	if _, err := NewWebDAVTransport("http://dav.invalid/", strings.Repeat("x", 4096), strings.Repeat("x", 4096), false); err != nil {
		t.Fatal("boundary credentials rejected", err)
	}
}

func TestWebDAVHTTPCollectionDefaultPort(t *testing.T) {
	u, _ := url.Parse("http://dav.invalid/dav/oshell/")
	for _, address := range []string{"http://dav.invalid:80/dav/oshell/", "/dav/oshell/"} {
		if !validDAVCollection([]byte(davCollectionXML(address, "200 OK", `<d:collection/>`)), u) {
			t.Fatal("HTTP default port not recognized")
		}
	}
	if validDAVCollection([]byte(davCollectionXML("http://dav.invalid:443/dav/oshell/", "200 OK", `<d:collection/>`)), u) {
		t.Fatal("wrong HTTP port accepted")
	}
}

func TestWebDAVHTTPRejectsRedirectAndHTTPSDowngrade(t *testing.T) {
	var called atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called.Add(1) }))
	defer target.Close()
	for _, secure := range []bool{false, true} {
		for _, status := range []int{301, 302, 303, 307, 308} {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(status)
			})
			var server *httptest.Server
			if secure {
				server = httptest.NewTLSServer(handler)
			} else {
				server = httptest.NewServer(handler)
			}
			w, err := NewWebDAVTransport(server.URL, "fixture", "secret", false)
			if err != nil {
				t.Fatal(err)
			}
			if secure {
				w.client.Transport = server.Client().Transport
			}
			_, err = w.Fetch(context.Background())
			server.Close()
			wantSyncFailure(t, err, "redirect")
		}
	}
	if called.Load() != 0 {
		t.Fatal("credentials sent to a redirect target")
	}
}

func TestWebDAVHTTPCancellationAndErrors(t *testing.T) {
	for _, mode := range []string{"headers", "body", "authentication", "server", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "headers", "body":
					if mode == "body" {
						w.WriteHeader(200)
						w.(http.Flusher).Flush()
					}
					close(started)
					<-r.Context().Done()
				case "authentication":
					w.WriteHeader(401)
				case "server":
					w.WriteHeader(500)
				case "oversized":
					w.Header().Set("Content-Length", "999999999")
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			w, err := NewWebDAVTransport(server.URL, "fixture", "secret", false)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { _, err := w.Fetch(ctx); result <- err }()
			if mode == "headers" || mode == "body" {
				select {
				case <-started:
				case <-time.After(3 * time.Second):
					t.Fatal("request never started")
				}
				cancel()
			}
			select {
			case err := <-result:
				switch mode {
				case "headers", "body":
					if !errors.Is(err, context.Canceled) {
						t.Fatal("cancel not preserved", err)
					}
				case "authentication":
					wantSyncFailure(t, err, "authentication")
				case "server":
					wantSyncFailure(t, err, "status")
				case "oversized":
					wantSyncFailure(t, err, "size")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("request did not finish promptly")
			}
		})
	}
}

func TestWebDAVHTTPDoesNotUseCertificateTrust(t *testing.T) {
	options := SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "http://dav.invalid/", TrustedServerCertificate: WebDAVCertificate{Host: "old.invalid", Port: 443, CertificateDER: "invalid"}}
	if _, err := options.Transport("", t.TempDir()); err != nil {
		t.Fatal("HTTP used stale TLS pin", err)
	}
	dir, master := t.TempDir(), []byte("synthetic-master")
	if err := SaveSyncOptions(dir, options, master); err != nil {
		t.Fatal(err)
	}
	saved, err := LoadSyncOptions(dir, master)
	if err != nil || saved.TrustedServerCertificate != (WebDAVCertificate{}) {
		t.Fatal("stale TLS pin persisted on HTTP", err)
	}
	if _, err := InspectWebDAVCertificate(context.Background(), options.Address); err == nil {
		t.Fatal("attempted TLS inspection on HTTP")
	}
}
