package core

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func updateFixture(t *testing.T) (UpdateManifest, ed25519.PrivateKey, string, []byte) {
	t.Helper()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	archive := []byte("synthetic signed application archive")
	digest := sha256.Sum256(archive)
	manifest := UpdateManifest{Repository: "owner/repo", ApplicationID: "dev.oshell.desktop", Version: "0.4.1", Build: "41", Notes: "fixture", Platforms: map[string]UpdateArchive{"darwin-arm64": {URL: "https://github.com/owner/repo/releases/download/v0.4.1/OShell-Desktop-0.4.1-darwin-arm64.tar.gz", Size: int64(len(archive)), SHA256: hex.EncodeToString(digest[:]), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, digest[:])), MinimumOS: "13.0"}}}
	return manifest, key, base64.StdEncoding.EncodeToString(pub), archive
}
func TestUpdateSourceAndSignedProductBinding(t *testing.T) {
	source, e := ParseUpdateSource("https://github.com/Owner/Repo.git")
	if e != nil || source.MetadataURL() != "https://owner.github.io/Repo/updates/latest.json" {
		t.Fatal(source, e)
	}
	user, e := ParseUpdateSource("Owner/Owner.github.io")
	if e != nil || user.MetadataURL() != "https://owner.github.io/updates/latest.json" {
		t.Fatal(user, e)
	}
	for _, input := range []string{"owner/../repo", "https://github.com/owner/repo?token=x", "https://github.com:443/owner/repo", "https://evil.invalid/owner/repo", "owner/.."} {
		if _, e = ParseUpdateSource(input); e == nil {
			t.Fatal("accepted bad source", input)
		}
	}
	manifest, key, pub, _ := updateFixture(t)
	data, e := SignUpdateManifest(manifest, key)
	if e != nil {
		t.Fatal(e)
	}
	source, _ = ParseUpdateSource("OWNER/REPO")
	decode := func(b []byte, app, version, build string) (*VerifiedUpdate, error) {
		return DecodeSignedUpdate(b, source, "darwin-arm64", app, version, build, pub)
	}
	update, e := decode(data, "dev.oshell.desktop", "0.4.0", "40")
	if e != nil || update == nil {
		t.Fatal("signed update rejected", e)
	}
	if rebuilt, e := decode(data, "dev.oshell.desktop", "0.4.1", "40"); e != nil || rebuilt == nil {
		t.Fatal("new signed build of same version rejected", e)
	}
	if _, e = decode(data, "app.oshell.mac", "0.4.0", "40"); e == nil {
		t.Fatal("wrong product accepted")
	}
	if candidate, e := decode(data, "dev.oshell.desktop", "0.4.1", "41"); e != nil || candidate != nil {
		t.Fatal("rollback accepted", e)
	}
	var envelope signedUpdateEnvelope
	json.Unmarshal(data, &envelope)
	envelope.Payload[0] ^= 1
	tampered, _ := json.Marshal(envelope)
	if _, e = decode(tampered, "dev.oshell.desktop", "0.4.0", "40"); e == nil {
		t.Fatal("tampered metadata accepted")
	}
}
func TestUpdateCacheExpiryAndBackoff(t *testing.T) {
	now := time.Unix(1000, 0)
	cache := &UpdateMetadataCache{clock: func() time.Time { return now }}
	loads := 0
	failure := false
	load := func(context.Context) ([]byte, error) {
		loads++
		if failure {
			return nil, &UpdateHTTPError{Status: 429, RetryAt: now.Add(2 * time.Hour)}
		}
		return []byte("verified"), nil
	}
	data, e := cache.Value(context.Background(), load)
	if e != nil {
		t.Fatal(e)
	}
	data[0] = 'x'
	again, _ := cache.Value(context.Background(), load)
	if loads != 1 || string(again) != "verified" {
		t.Fatal("cache aliased or did not reuse")
	}
	now = now.Add(301 * time.Second)
	failure = true
	if _, e = cache.Value(context.Background(), load); e == nil {
		t.Fatal("stale success returned")
	}
	now = now.Add(time.Minute)
	cache.Value(context.Background(), load)
	if loads != 2 {
		t.Fatal("rate-limit backoff ignored")
	}
	now = now.Add(2 * time.Hour)
	failure = false
	if _, e = cache.Value(context.Background(), load); e != nil || loads != 3 {
		t.Fatal("retry never resumed", e)
	}
}

type updateRoundTrip func(*http.Request) (*http.Response, error)

func (f updateRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestUpdateDownloadSignatureAndCleanup(t *testing.T) {
	manifest, key, pub, archive := updateFixture(t)
	data, _ := SignUpdateManifest(manifest, key)
	source, _ := ParseUpdateSource("owner/repo")
	update, e := DecodeSignedUpdate(data, source, "darwin-arm64", "dev.oshell.desktop", "0.4.0", "40", pub)
	if e != nil {
		t.Fatal(e)
	}
	content := archive
	update.Archive.URL = "https://untrusted.invalid/changed" // display fields cannot change verified authority
	client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "github.com" {
			t.Fatal("mutable display field changed download authority")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(content))), ContentLength: int64(len(content)), Header: make(http.Header)}, nil
	})}
	dir := t.TempDir()
	result, e := update.download(context.Background(), dir, nil, client)
	if e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(result.ArchivePath())
	if e != nil || string(got) != string(archive) {
		t.Fatal("download differs", e)
	}
	result.Remove()
	content = append([]byte(nil), archive...)
	content[0] ^= 1
	if _, e = update.download(context.Background(), dir, nil, client); e == nil {
		t.Fatal("corrupt archive accepted")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 0 {
		t.Fatal("failed download left data")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = update.download(ctx, dir, nil, client); !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation lost", e)
	}
}
