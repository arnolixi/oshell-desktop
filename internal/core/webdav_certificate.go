package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// WebDAVCertificate is local connection state, never part of the shared schema.
// DER uses Base64 explicitly so SyncOptions remains a comparable draft value.
type WebDAVCertificate struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	CertificateDER string `json:"certificateDER"`
}

func webDAVEndpoint(u *url.URL) (string, int, error) {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	port := 443
	if strings.EqualFold(u.Scheme, "http") {
		port = 80
	}
	if u.Port() != "" {
		n, err := strconv.Atoi(u.Port())
		if err != nil || n < 1 || n > 65535 {
			return "", 0, safeSyncFailure("tls")
		}
		port = n
	}
	if host == "" {
		return "", 0, safeSyncFailure("tls")
	}
	return host, port, nil
}

func (p WebDAVCertificate) certificate() (*x509.Certificate, error) {
	if len(p.CertificateDER) > 128*1024 {
		return nil, safeSyncFailure("tls")
	}
	der, err := base64.StdEncoding.DecodeString(p.CertificateDER)
	if err != nil {
		return nil, safeSyncFailure("tls")
	}
	if len(der) > 65536 {
		return nil, safeSyncFailure("tls")
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, safeSyncFailure("tls")
	}
	return cert, nil
}

// Trust only this leaf, never its issuer. IP literals may opt in to name/EKU
// exceptions; domain names still require the DNS identity and serverAuth usage.
func (p WebDAVCertificate) validateLeaf(cert *x509.Certificate, now time.Time) error {
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return safeSyncFailure("tls-validity")
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	opts := x509.VerifyOptions{Roots: roots, DNSName: p.Host, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if net.ParseIP(p.Host) != nil {
		opts.DNSName = ""
		opts.KeyUsages = []x509.ExtKeyUsage{x509.ExtKeyUsageAny}
	}
	if _, err := cert.Verify(opts); err != nil {
		return safeSyncFailure("tls")
	}
	return nil
}

func (p WebDAVCertificate) Validate(address string) error {
	u, err := url.Parse(strings.TrimSpace(address))
	if err != nil || u.Scheme != "https" {
		return safeSyncFailure("tls")
	}
	host, port, err := webDAVEndpoint(u)
	if err != nil || host != p.Host || port != p.Port {
		return safeSyncFailure("tls-changed")
	}
	cert, err := p.certificate()
	if err != nil {
		return err
	}
	return p.validateLeaf(cert, time.Now())
}

func (p WebDAVCertificate) Description() string {
	cert, err := p.certificate()
	if err != nil {
		return "证书无法读取"
	}
	digest := sha256.Sum256(cert.Raw)
	return fmt.Sprintf("服务端：%s:%d\n主体：%s\n颁发者：%s\n有效期：%s 至 %s\nSHA-256：%X", p.Host, p.Port, certificateLabel(cert.Subject.String()), certificateLabel(cert.Issuer.String()), cert.NotBefore.UTC().Format(time.RFC3339), cert.NotAfter.UTC().Format(time.RFC3339), digest)
}

func (w *WebDAVTransport) trustCertificate(p WebDAVCertificate) error {
	if err := p.Validate(w.directory.String()); err != nil {
		return err
	}
	pinned, _ := p.certificate()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// This client alone replaces system verification with the exact pin policy.
	// VerifyConnection runs on every handshake, including resumed connections.
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 || !bytes.Equal(cs.PeerCertificates[0].Raw, pinned.Raw) {
				return safeSyncFailure("tls-changed")
			}
			return p.validateLeaf(cs.PeerCertificates[0], time.Now())
		},
	}
	w.client.Transport = transport
	return nil
}

// InspectWebDAVCertificate sends no HTTP request or credentials. The handshake
// is deliberately aborted after capturing the untrusted certificate for review.
func InspectWebDAVCertificate(ctx context.Context, address string) (WebDAVCertificate, error) {
	w, err := NewWebDAVTransport(address, "", "", false)
	if err != nil {
		return WebDAVCertificate{}, err
	}
	if w.directory.Scheme != "https" {
		return WebDAVCertificate{}, fmt.Errorf("仅 HTTPS 地址支持服务器证书检查")
	}
	host, port, err := webDAVEndpoint(w.directory)
	if err != nil {
		return WebDAVCertificate{}, err
	}
	p := WebDAVCertificate{Host: host, Port: port}
	inspected := errors.New("certificate inspection complete")
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 15 * time.Second}, Config: &tls.Config{
		MinVersion: tls.VersionTLS12, ServerName: host, InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) > 0 {
				p.CertificateDER = base64.StdEncoding.EncodeToString(cs.PeerCertificates[0].Raw)
			}
			return inspected
		},
	}}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if conn != nil {
		conn.Close()
	}
	if !errors.Is(err, inspected) || p.CertificateDER == "" {
		if ctx.Err() != nil {
			return WebDAVCertificate{}, ctx.Err()
		}
		return WebDAVCertificate{}, safeSyncFailure("tls")
	}
	return p, nil
}

func webDAVRequestFailure(err error) error {
	var failure *SyncFailure
	var cert *tls.CertificateVerificationError
	var record tls.RecordHeaderError
	if errors.As(err, &failure) {
		return failure
	}
	if errors.As(err, &cert) || errors.As(err, &record) {
		return safeSyncFailure("tls")
	}
	return safeSyncFailure("network")
}

func certificateLabel(s string) string {
	runes := []rune(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s))
	if len(runes) > 512 {
		runes = runes[:512]
	}
	return string(runes)
}
