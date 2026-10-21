package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func davTestCertificate(t *testing.T, dns string, usage x509.ExtKeyUsage, from, until time.Time) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: dns}, DNSNames: []string{dns}, NotBefore: from, NotAfter: until, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
func TestWebDAVExactCertificateTrust(t *testing.T) {
	now := time.Now()
	first := davTestCertificate(t, "localhost", x509.ExtKeyUsageServerAuth, now.Add(-time.Hour), now.Add(time.Hour))
	second := davTestCertificate(t, "localhost", x509.ExtKeyUsageServerAuth, now.Add(-time.Hour), now.Add(time.Hour))
	var active atomic.Pointer[tls.Certificate]
	active.Store(&first)
	var requests atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method == "PROPFIND" {
			writeDAVCollection(w, r.URL.Path)
			return
		}
		w.WriteHeader(404)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return active.Load(), nil }}
	server.StartTLS()
	defer server.Close()
	address := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	untrusted, err := NewWebDAVTransport(address, "private-user", "private-password", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = untrusted.Fetch(context.Background()); err == nil {
		t.Fatal("system trust accepted self signed certificate")
	}
	p, err := InspectWebDAVCertificate(context.Background(), address)
	if err != nil || p.CertificateDER != base64.StdEncoding.EncodeToString(first.Certificate[0]) {
		t.Fatal("inspection", err)
	}
	if requests.Load() != 0 {
		t.Fatal("inspection sent an HTTP request")
	}
	options := SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: address, TrustedServerCertificate: p}
	transport, err := options.Transport("secret", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = transport.Fetch(context.Background()); err != nil {
		t.Fatal("exact pin rejected", err)
	}
	active.Store(&second)
	transport.(*WebDAVTransport).client.CloseIdleConnections()
	if _, err = transport.Fetch(context.Background()); err == nil {
		t.Fatal("rotated certificate accepted")
	}
	var failure *SyncFailure
	if !errors.As(err, &failure) || failure.Kind != "tls-changed" {
		t.Fatal(err)
	}
	safe := safeDiagnosticFailure(err)
	if safe.Kind != "tls" || strings.Contains(safe.Title(), "localhost") {
		t.Fatal("unsafe diagnostic", safe)
	}
	other := p
	other.Port++
	if other.Validate(address) == nil {
		t.Fatal("pin transferred across ports")
	}
	other = p
	other.Host = "elsewhere.invalid"
	if other.Validate(address) == nil {
		t.Fatal("pin transferred across hosts")
	}
}
func TestWebDAVCertificateValidityAndIPExceptions(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name, host, dns string
		usage           x509.ExtKeyUsage
		from, until     time.Time
		accept          bool
	}{
		{"domain", "localhost", "localhost", x509.ExtKeyUsageServerAuth, now.Add(-time.Hour), now.Add(time.Hour), true},
		{"domain-name", "localhost", "wrong.invalid", x509.ExtKeyUsageServerAuth, now.Add(-time.Hour), now.Add(time.Hour), false},
		{"domain-purpose", "localhost", "localhost", x509.ExtKeyUsageClientAuth, now.Add(-time.Hour), now.Add(time.Hour), false},
		{"explicit-IP", "127.0.0.1", "wrong.invalid", x509.ExtKeyUsageClientAuth, now.Add(-time.Hour), now.Add(time.Hour), true},
		{"expired-IP", "127.0.0.1", "wrong.invalid", x509.ExtKeyUsageClientAuth, now.Add(-2 * time.Hour), now.Add(-time.Hour), false},
		{"future", "localhost", "localhost", x509.ExtKeyUsageServerAuth, now.Add(time.Hour), now.Add(2 * time.Hour), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cert := davTestCertificate(t, tc.dns, tc.usage, tc.from, tc.until)
			p := WebDAVCertificate{Host: tc.host, Port: 443, CertificateDER: base64.StdEncoding.EncodeToString(cert.Certificate[0])}
			err := p.Validate("https://" + tc.host + "/dav")
			if (err == nil) != tc.accept {
				t.Fatal("unexpected policy", err)
			}
		})
	}
	u, _ := url.Parse("https://LOCALHOST.:443/dav")
	host, port, err := webDAVEndpoint(u)
	if err != nil || host != "localhost" || port != 443 {
		t.Fatal(host, port, err)
	}
}
func TestWebDAVCertificateStaysLocalAndEncrypted(t *testing.T) {
	now := time.Now()
	cert := davTestCertificate(t, "localhost", x509.ExtKeyUsageServerAuth, now.Add(-time.Hour), now.Add(time.Hour))
	p := WebDAVCertificate{Host: "localhost", Port: 443, CertificateDER: base64.StdEncoding.EncodeToString(cert.Certificate[0])}
	options := SyncOptions{Version: 1, Enabled: true, Backend: "webdav", Address: "https://localhost/dav", Username: "fixture-user", TrustedServerCertificate: p}
	dir := t.TempDir()
	master := []byte("fixture-master")
	if err := SaveSyncOptions(dir, options, master); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "sync-settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !IsSharedVault(raw) || strings.Contains(string(raw), p.CertificateDER) || strings.Contains(string(raw), "fixture-user") {
		t.Fatal("local trust was not encrypted")
	}
	loaded, err := LoadSyncOptions(dir, master)
	if err != nil || loaded != options {
		t.Fatal("roundtrip", err)
	}
	native, _ := json.Marshal(map[string]any{"address": options.Address, "username": options.Username, "password": "fixture-password", "trustedServerCertificate": p})
	sealed, err := SealShared(native, master)
	if err != nil {
		t.Fatal(err)
	}
	imported, _, err := decodeNativeWebDAV(sealed, master)
	if err != nil || imported.TrustedServerCertificate != (WebDAVCertificate{}) {
		t.Fatal("import inherited another device trust", err)
	}
	options.Enabled = false
	if err = SaveSyncOptions(dir, options, master); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadSyncOptions(dir)
	if err != nil || loaded.TrustedServerCertificate != (WebDAVCertificate{}) {
		t.Fatal("disabled trust persisted", err)
	}
}
