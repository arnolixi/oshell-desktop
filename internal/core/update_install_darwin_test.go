//go:build darwin

package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testUpdateBundle(t *testing.T, parent, name, identifier, version, build string, signed bool) string {
	t.Helper()
	bundle := filepath.Join(parent, name)
	if e := os.MkdirAll(filepath.Join(bundle, "Contents/MacOS"), 0755); e != nil {
		t.Fatal(e)
	}
	plist := fmt.Sprintf(`<?xml version="1.0"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>%s</string><key>CFBundleExecutable</key><string>OShellDesktop</string><key>CFBundleShortVersionString</key><string>%s</string><key>CFBundleVersion</key><string>%s</string><key>LSMinimumSystemVersion</key><string>13.0</string><key>CFBundlePackageType</key><string>APPL</string></dict></plist>`, identifier, version, build)
	if e := os.WriteFile(filepath.Join(bundle, "Contents/Info.plist"), []byte(plist), 0644); e != nil {
		t.Fatal(e)
	}
	data := []byte("fixture executable")
	if signed {
		var e error
		data, e = os.ReadFile("/usr/bin/true")
		if e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(bundle, "Contents/MacOS/OShellDesktop"), data, 0755); e != nil {
		t.Fatal(e)
	}
	if signed {
		if out, e := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--timestamp=none", bundle).CombinedOutput(); e != nil {
			t.Fatalf("sign fixture: %v %s", e, out)
		}
	}
	return bundle
}
func archiveTestBundle(t *testing.T, bundle string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zip := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(zip)
	e := filepath.WalkDir(bundle, func(file string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, e = os.Readlink(file)
			if e != nil {
				return e
			}
		}
		header, e := tar.FileInfoHeader(info, link)
		if e != nil {
			return e
		}
		relative, _ := filepath.Rel(filepath.Dir(bundle), file)
		header.Name = filepath.ToSlash(relative)
		if e = writer.WriteHeader(header); e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			data, e := os.ReadFile(file)
			if e != nil {
				return e
			}
			_, e = writer.Write(data)
			return e
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if e = writer.Close(); e != nil {
		t.Fatal(e)
	}
	if e = zip.Close(); e != nil {
		t.Fatal(e)
	}
	return buffer.Bytes()
}
func verifiedTestDownload(t *testing.T, archive []byte) *VerifiedDownload {
	t.Helper()
	manifest, key, pub, _ := updateFixture(t)
	target := "darwin-" + runtime.GOARCH
	digest := sha256.Sum256(archive)
	manifest.Platforms = map[string]UpdateArchive{target: {URL: "https://github.com/owner/repo/releases/download/v0.4.1/OShell-Desktop-0.4.1-" + target + ".tar.gz", Size: int64(len(archive)), SHA256: hex.EncodeToString(digest[:]), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, digest[:])), MinimumOS: "13.0"}}
	data, e := SignUpdateManifest(manifest, key)
	if e != nil {
		t.Fatal(e)
	}
	source, _ := ParseUpdateSource("owner/repo")
	update, e := DecodeSignedUpdate(data, source, target, "dev.oshell.desktop", "0.4.0", "40", pub)
	if e != nil {
		t.Fatal(e)
	}
	client := &http.Client{Transport: updateRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: int64(len(archive)), Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
	})}
	result, e := update.download(context.Background(), t.TempDir(), nil, client)
	if e != nil {
		t.Fatal(e)
	}
	return result
}
func TestUpdateNativeSignedBundleAtomicInstallAndRollback(t *testing.T) {
	parent := t.TempDir()
	current := testUpdateBundle(t, parent, "Desktop.app", "dev.oshell.desktop", "0.4.0", "40", true)
	incoming := testUpdateBundle(t, t.TempDir(), "Release.app", "dev.oshell.desktop", "0.4.1", "41", true)
	sentinel := filepath.Join(parent, "user-settings.json")
	os.WriteFile(sentinel, []byte("preserve configuration"), 0600)
	download := verifiedTestDownload(t, archiveTestBundle(t, incoming))
	prepared, e := download.PrepareMacOS(context.Background(), current)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := ReadAppBundleInfo(current)
	if before.Version != "0.4.0" {
		t.Fatal("prepare modified app")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = prepared.Commit(cancelled); e == nil {
		t.Fatal("cancelled install committed")
	}
	installed, e := prepared.Commit(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	next, e := ReadAppBundleInfo(current)
	if e != nil || next.Version != "0.4.1" {
		t.Fatal("atomic replacement failed", e)
	}
	old, e := ReadAppBundleInfo(installed.backup)
	if e != nil || old.Version != "0.4.0" {
		t.Fatal("rollback copy missing", e)
	}
	if e = installed.Rollback(context.Background()); e != nil {
		t.Fatal(e)
	}
	restored, _ := ReadAppBundleInfo(current)
	if restored.Version != "0.4.0" {
		t.Fatal("rollback failed")
	}
	if e = installed.Finalize(); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(prepared.work); !os.IsNotExist(e) {
		t.Fatal("staging not cleaned")
	}
	data, _ := os.ReadFile(sentinel)
	if string(data) != "preserve configuration" {
		t.Fatal("settings changed")
	}
}
func TestUpdatePrepareRejectsWrongProductAndMutatedStages(t *testing.T) {
	verify := func(context.Context, string, string) error { return nil }
	for _, kind := range []string{"product", "stage", "installed"} {
		t.Run(kind, func(t *testing.T) {
			current := testUpdateBundle(t, t.TempDir(), "Desktop.app", "dev.oshell.desktop", "0.4.0", "40", false)
			id := "dev.oshell.desktop"
			if kind == "product" {
				id = "app.oshell.mac"
			}
			incoming := testUpdateBundle(t, t.TempDir(), "Release.app", id, "0.4.1", "41", false)
			download := verifiedTestDownload(t, archiveTestBundle(t, incoming))
			prepared, e := download.prepareMacOS(context.Background(), current, verify)
			if kind == "product" {
				if e == nil {
					prepared.Discard()
					t.Fatal("original OShell bundle accepted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			defer prepared.Discard()
			changed := prepared.bundle
			if kind == "installed" {
				changed = current
			}
			os.WriteFile(filepath.Join(changed, "Contents/MacOS/OShellDesktop"), []byte("changed since verification"), 0755)
			if _, e = prepared.Commit(context.Background()); e == nil {
				t.Fatal("mutated app accepted")
			}
			info, _ := ReadAppBundleInfo(current)
			if info.Version != "0.4.0" {
				t.Fatal("failed install changed current app")
			}
		})
	}
}
func TestUpdateArchiveRejectsTraversalLinksAndDuplicates(t *testing.T) {
	for _, headers := range [][]*tar.Header{
		{{Name: "App.app/../escape", Mode: 0644, Typeflag: tar.TypeReg}},
		{{Name: "/App.app/absolute", Mode: 0644, Typeflag: tar.TypeReg}},
		{{Name: "App.app/link", Linkname: "../../escape", Typeflag: tar.TypeSymlink}},
		{{Name: "App.app/link", Linkname: "/tmp/escape", Typeflag: tar.TypeSymlink}},
		{{Name: "App.app/a", Typeflag: tar.TypeReg}, {Name: "App.app/A", Typeflag: tar.TypeReg}},
		{{Name: "App.app/device", Typeflag: tar.TypeFifo}},
		{{Name: "App.app/a", Typeflag: tar.TypeReg}, {Name: "Other.app/b", Typeflag: tar.TypeReg}},
	} {
		var buffer bytes.Buffer
		zip := gzip.NewWriter(&buffer)
		writer := tar.NewWriter(zip)
		for _, header := range headers {
			if e := writer.WriteHeader(header); e != nil {
				t.Fatal(e)
			}
		}
		writer.Close()
		zip.Close()
		if path, e := extractUpdateBundle(context.Background(), bytes.NewReader(buffer.Bytes()), t.TempDir()); e == nil {
			t.Fatalf("accepted %s as %s", headers[0].Name, path)
		}
	}
}
func TestUpdatePlistRejectsDuplicateIdentity(t *testing.T) {
	bundle := testUpdateBundle(t, t.TempDir(), "Desktop.app", "dev.oshell.desktop", "0.4.1", "41", false)
	path := filepath.Join(bundle, "Contents/Info.plist")
	data, _ := os.ReadFile(path)
	data = []byte(strings.Replace(string(data), "</dict>", "<key>CFBundleIdentifier</key><string>app.oshell.mac</string></dict>", 1))
	os.WriteFile(path, data, 0644)
	if _, e := ReadAppBundleInfo(bundle); e == nil {
		t.Fatal("duplicate identity accepted")
	}
}

func TestUpdateAtomicFailureLeavesOriginal(t *testing.T) {
	parent := t.TempDir()
	current := testUpdateBundle(t, parent, "Desktop.app", "dev.oshell.desktop", "0.4.0", "40", false)
	incoming := testUpdateBundle(t, t.TempDir(), "Release.app", "dev.oshell.desktop", "0.4.1", "41", false)
	download := verifiedTestDownload(t, archiveTestBundle(t, incoming))
	prepared, e := download.prepareMacOS(context.Background(), current, func(context.Context, string, string) error { return nil })
	if e != nil {
		t.Fatal(e)
	}
	defer prepared.Discard()
	if e = os.Chmod(parent, 0500); e != nil {
		t.Fatal(e)
	}
	_, commitErr := prepared.Commit(context.Background())
	restoreErr := os.Chmod(parent, 0700)
	if restoreErr != nil {
		t.Fatal(restoreErr)
	}
	if commitErr == nil {
		t.Fatal("install unexpectedly succeeded without directory write permission")
	}
	info, e := ReadAppBundleInfo(current)
	if e != nil || info.Version != "0.4.0" {
		t.Fatal("failed atomic operation changed app", e)
	}
}
func TestUpdateFrameworkLinksAndArchiveReverification(t *testing.T) {
	incoming := testUpdateBundle(t, t.TempDir(), "Release.app", "dev.oshell.desktop", "0.4.1", "41", false)
	framework := filepath.Join(incoming, "Contents/Frameworks/Fixture.framework")
	if e := os.MkdirAll(filepath.Join(framework, "Versions/A"), 0755); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(framework, "Versions/A/Fixture"), []byte("fixture"), 0644)
	if e := os.Symlink("A", filepath.Join(framework, "Versions/Current")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("Versions/Current/Fixture", filepath.Join(framework, "Fixture")); e != nil {
		t.Fatal(e)
	}
	archive := archiveTestBundle(t, incoming)
	unpacked, e := extractUpdateBundle(context.Background(), bytes.NewReader(archive), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(filepath.Join(unpacked, "Contents/Frameworks/Fixture.framework/Fixture"))
	if e != nil || string(data) != "fixture" {
		t.Fatal("safe framework link not retained", e)
	}
	download := verifiedTestDownload(t, archive)
	file, e := os.OpenFile(download.ArchivePath(), os.O_WRONLY, 0)
	if e != nil {
		t.Fatal(e)
	}
	file.WriteAt([]byte{0}, 0)
	file.Close()
	current := testUpdateBundle(t, t.TempDir(), "Desktop.app", "dev.oshell.desktop", "0.4.0", "40", false)
	if staged, e := download.prepareMacOS(context.Background(), current, func(context.Context, string, string) error { return nil }); e == nil {
		staged.Discard()
		t.Fatal("archive changed after download was accepted")
	}
}

func TestUpdateReceiptFinalizesAfterVerifiedRelaunch(t *testing.T) {
	parent := t.TempDir()
	state := t.TempDir()
	current := testUpdateBundle(t, parent, "Desktop.app", "dev.oshell.desktop", "0.4.0", "40", false)
	incoming := testUpdateBundle(t, t.TempDir(), "Release.app", "dev.oshell.desktop", "0.4.1", "41", false)
	download := verifiedTestDownload(t, archiveTestBundle(t, incoming))
	prepared, e := download.prepareMacOS(context.Background(), current, func(context.Context, string, string) error { return nil })
	if e != nil {
		t.Fatal(e)
	}
	if _, e = prepared.CommitWithReceipt(context.Background(), state); e != nil {
		t.Fatal(e)
	}
	if e = FinishUpdateRecovery(context.Background(), state, current, "app.oshell.mac"); e == nil {
		t.Fatal("wrong product could clean backup")
	}
	if _, e = os.Stat(prepared.work); e != nil {
		t.Fatal("failed recovery removed backup")
	}
	if e = FinishUpdateRecovery(context.Background(), state, current, "dev.oshell.desktop"); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(prepared.work); !os.IsNotExist(e) {
		t.Fatal("verified relaunch did not clean backup")
	}
	if _, e = os.Stat(filepath.Join(state, "update-installation.json")); !os.IsNotExist(e) {
		t.Fatal("receipt remained")
	}
	info, _ := ReadAppBundleInfo(current)
	if info.Version != "0.4.1" {
		t.Fatal("cleanup touched installed app")
	}
}

func TestUpdateRecoveryDoesNotDeleteUnmarkedSibling(t *testing.T) {
	parent := t.TempDir()
	state := t.TempDir()
	current := testUpdateBundle(t, parent, "Desktop.app", "dev.oshell.desktop", "0.4.0", "40", false)
	incoming := testUpdateBundle(t, t.TempDir(), "Release.app", "dev.oshell.desktop", "0.4.1", "41", false)
	download := verifiedTestDownload(t, archiveTestBundle(t, incoming))
	prepared, e := download.prepareMacOS(context.Background(), current, func(context.Context, string, string) error { return nil })
	if e != nil {
		t.Fatal(e)
	}
	installed, e := prepared.CommitWithReceipt(context.Background(), state)
	if e != nil {
		t.Fatal(e)
	}
	defer installed.Finalize()
	other, e := os.MkdirTemp(parent, ".oshell-update-unrelated-")
	if e != nil {
		t.Fatal(e)
	}
	sentinel := filepath.Join(other, "keep")
	os.WriteFile(sentinel, []byte("unrelated data"), 0600)
	recordFile := filepath.Join(state, "update-installation.json")
	data, _ := os.ReadFile(recordFile)
	var record updateInstallationRecord
	json.Unmarshal(data, &record)
	record.Work = other
	record.Backup = filepath.Join(other, "Release.app")
	changed, _ := json.Marshal(record)
	os.WriteFile(recordFile, changed, 0600)
	if e = FinishUpdateRecovery(context.Background(), state, current, "dev.oshell.desktop"); e == nil {
		t.Fatal("unmarked directory accepted")
	}
	preserved, e := os.ReadFile(sentinel)
	if e != nil || string(preserved) != "unrelated data" {
		t.Fatal("recovery removed unrelated data", e)
	}
}
