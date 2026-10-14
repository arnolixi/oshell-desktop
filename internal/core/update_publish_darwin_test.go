//go:build darwin

package core

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func pinTestUpdateKey(t *testing.T, bundle, pub string) {
	t.Helper()
	path := filepath.Join(bundle, "Contents/Info.plist")
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	updated := strings.Replace(string(data), "</dict>", "<key>OShellDesktopUpdatePublicKey</key><string>"+pub+"</string><key>OShellDesktopUpdateRepository</key><string>owner/repo</string></dict>", 1)
	if e = os.WriteFile(path, []byte(updated), 0644); e != nil {
		t.Fatal(e)
	}
	if out, e := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--timestamp=none", bundle).CombinedOutput(); e != nil {
		t.Fatalf("sign fixture: %v %s", e, out)
	}
}
func TestUpdatePublisherProducesConsumerVerifiedArtifact(t *testing.T) {
	_, key, pub, _ := updateFixture(t)
	bundle := testUpdateBundle(t, t.TempDir(), "Desktop.app", "dev.oshell.desktop", "0.4.1", "41", true)
	pinTestUpdateKey(t, bundle, pub)
	options := UpdatePublishOptions{Application: bundle, Output: t.TempDir(), Repository: "owner/repo", Target: "darwin-" + runtime.GOARCH, ApplicationID: "dev.oshell.desktop", Notes: "release fixture"}
	result, e := PublishMacOSUpdate(context.Background(), options, key)
	if e != nil {
		t.Fatal(e)
	}
	if result.PublicKey != pub {
		t.Fatal("publisher used different key")
	}
	metadata, e := os.ReadFile(result.MetadataPath)
	if e != nil {
		t.Fatal(e)
	}
	source, _ := ParseUpdateSource("owner/repo")
	candidate, e := DecodeSignedUpdate(metadata, source, options.Target, options.ApplicationID, "0.4.0", "40", pub)
	if e != nil || candidate == nil {
		t.Fatal("consumer rejected metadata", e)
	}
	client := &http.Client{Transport: updateRoundTrip(func(*http.Request) (*http.Response, error) {
		file, e := os.Open(result.ArchivePath)
		if e != nil {
			return nil, e
		}
		return &http.Response{StatusCode: 200, ContentLength: candidate.Archive.Size, Body: file, Header: make(http.Header)}, nil
	})}
	downloaded, e := candidate.download(context.Background(), t.TempDir(), nil, client)
	if e != nil {
		t.Fatal(e)
	}
	defer downloaded.Remove()
	current := testUpdateBundle(t, t.TempDir(), "Desktop.app", "dev.oshell.desktop", "0.4.0", "40", true)
	pinTestUpdateKey(t, current, pub)
	prepared, e := downloaded.PrepareMacOS(context.Background(), current)
	if e != nil {
		t.Fatal(e)
	}
	prepared.Discard()
	if _, e = PublishMacOSUpdate(context.Background(), options, key); e == nil {
		t.Fatal("existing version archive overwritten")
	}
	after, _ := os.ReadFile(result.MetadataPath)
	if string(after) != string(metadata) {
		t.Fatal("failed republish changed feed")
	}
}
func TestUpdateSigningKeyFileGuards(t *testing.T) {
	_, key, _, _ := updateFixture(t)
	file := filepath.Join(t.TempDir(), "private-key")
	if e := os.WriteFile(file, []byte(base64.StdEncoding.EncodeToString(key)), 0600); e != nil {
		t.Fatal(e)
	}
	read, e := ReadUpdateSigningKey(file)
	if e != nil {
		t.Fatal(e)
	}
	clear(read)
	if e = os.Chmod(file, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadUpdateSigningKey(file); e == nil {
		t.Fatal("world-readable signing key accepted")
	}
	os.Chmod(file, 0600)
	alias := file + "-alias"
	if e = os.Symlink(file, alias); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadUpdateSigningKey(alias); e == nil {
		t.Fatal("private key symlink followed")
	}
}

func TestUpdatePublisherRejectsWrongPinnedKey(t *testing.T) {
	_, key, pub, _ := updateFixture(t)
	_, other, _, _ := updateFixture(t)
	bundle := testUpdateBundle(t, t.TempDir(), "Desktop.app", "dev.oshell.desktop", "0.4.1", "41", true)
	pinTestUpdateKey(t, bundle, pub)
	output := t.TempDir()
	_, e := PublishMacOSUpdate(context.Background(), UpdatePublishOptions{Application: bundle, Output: output, Repository: "owner/repo", Target: "darwin-" + runtime.GOARCH, ApplicationID: "dev.oshell.desktop"}, other)
	if e == nil {
		t.Fatal("published with a key not pinned in the app")
	}
	files, _ := os.ReadDir(output)
	if len(files) != 0 {
		t.Fatal("key mismatch produced artifacts")
	}
	clear(key)
	clear(other)
}

func TestMacUpdateTargetUsesHardwareForRosetta(t *testing.T) {
	if macUpdateTarget(true, "amd64") != "darwin-arm64" || macUpdateTarget(false, "amd64") != "darwin-amd64" {
		t.Fatal("update target followed emulation instead of hardware")
	}
}

func TestUpdateKeyGenerationIsExplicitPrivateAndNonOverwriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing-key")
	pub, e := GenerateUpdateSigningKeyFile(path)
	if e != nil {
		t.Fatal(e)
	}
	key, e := ReadUpdateSigningKey(path)
	if e != nil {
		t.Fatal(e)
	}
	defer clear(key)
	if pub != base64.StdEncoding.EncodeToString(key[32:]) {
		t.Fatal("public key does not match private file")
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private key permissions", e)
	}
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	defer clear(before)
	if _, e = GenerateUpdateSigningKeyFile(path); e == nil {
		t.Fatal("key generation overwrote existing key")
	}
	after, _ := os.ReadFile(path)
	defer clear(after)
	if string(before) != string(after) {
		t.Fatal("existing key changed")
	}
}
