//go:build !windows

package core

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLocalKeyConcurrentCreationPublishesOneCompleteKey(t *testing.T) {
	directory := t.TempDir()
	const workers = 24
	results := make(chan []byte, workers)
	failures := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			key, e := localVaultKey(directory)
			if e != nil {
				failures <- e
			} else {
				results <- key
			}
		}()
	}
	group.Wait()
	close(results)
	close(failures)
	for e := range failures {
		t.Fatal(e)
	}
	var first []byte
	count := 0
	for key := range results {
		if first == nil {
			first = key
		} else if !bytes.Equal(first, key) {
			t.Fatal("concurrent writers obtained different keys")
		}
		if len(key) != 32 {
			t.Fatal("partial key")
		}
		count++
	}
	if count != workers {
		t.Fatal("missing key result")
	}
	stat, e := os.Stat(filepath.Join(directory, "credential-key"))
	if e != nil || stat.Mode().Perm() != 0600 {
		t.Fatal("key permissions", e)
	}
	matches, _ := filepath.Glob(filepath.Join(directory, ".local-key-*"))
	if len(matches) != 0 {
		t.Fatal("temporary key remained")
	}
}
func TestLocalKeyRefusesUnsafeFilesAndDoesNotReplaceMissingKey(t *testing.T) {
	for _, kind := range []string{"permissions", "symlink", "fifo", "missing"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			vault, e := OpenVault(dir)
			if e != nil {
				t.Fatal(e)
			}
			profile := NewProfile()
			profile.Host = "example.invalid"
			profile.Username = "tester"
			if e = vault.Put(profile, "fixture-secret"); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(dir, "credential-key")
			original, e := os.ReadFile(filepath.Join(dir, "credentials.json"))
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "permissions":
				os.Chmod(path, 0644)
			case "symlink":
				backup := filepath.Join(dir, "original-key")
				os.Rename(path, backup)
				if e = os.Symlink(backup, path); e != nil {
					t.Fatal(e)
				}
			case "fifo":
				os.Remove(path)
				if e = unix.Mkfifo(path, 0600); e != nil {
					t.Fatal(e)
				}
			case "missing":
				os.Remove(path)
			}
			if _, e = OpenVault(dir); e == nil {
				t.Fatal("unsafe or missing key accepted")
			}
			after, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
			if !bytes.Equal(after, original) {
				t.Fatal("failed open changed vault")
			}
			if kind == "missing" {
				if _, e = os.Lstat(path); !os.IsNotExist(e) {
					t.Fatal("missing key regenerated despite existing vault")
				}
			}
		})
	}
}
func TestOriginalLocalKeyIDAndExplicitPermissionRepair(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "local-credential-key.json")
	key := LocalCredentialKey{ID: NewID(), Secret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))}
	data, _ := json.Marshal(key)
	if e := os.WriteFile(path, data, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := LoadOShellLocalCredentialKey(directory, key.ID, false); e == nil {
		t.Fatal("broad permissions accepted implicitly")
	}
	if _, e := LoadOShellLocalCredentialKey(directory, NewID(), true); e == nil {
		t.Fatal("wrong key ID accepted")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0644 {
		t.Fatal("failed validation changed permissions")
	}
	loaded, e := LoadOShellLocalCredentialKey(directory, key.ID, true)
	if e != nil || loaded != key {
		t.Fatal("original key failed to load", e)
	}
	info, _ = os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("explicit permission repair failed")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, data) {
		t.Fatal("permission repair rewrote key")
	}
}

func TestNativeLocalEnvelopeMigrationBindsKeyAndIdentity(t *testing.T) {
	key := LocalCredentialKey{ID: NewID(), Secret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, 32))}
	profile := NewProfile()
	profile.Host = "original.example"
	profile.Username = "tester"
	envelope, e := EncryptMacPassword("native-local-fixture", []byte(key.Secret), profile, profile.Identity())
	if e != nil {
		t.Fatal(e)
	}
	envelope.LocalKeyID = key.ID
	password, e := DecryptMacLocalPassword(envelope, key, profile)
	if e != nil || password != "native-local-fixture" {
		t.Fatal("local envelope could not migrate", e)
	}
	wrong := key
	wrong.ID = NewID()
	if _, e = DecryptMacLocalPassword(envelope, wrong, profile); e == nil {
		t.Fatal("wrong key ID accepted")
	}
	changed := profile
	changed.Host = "other.example"
	if _, e = DecryptMacLocalPassword(envelope, key, changed); e == nil {
		t.Fatal("local envelope rebound to a different host")
	}
	if _, e = DecryptMacPassword(envelope, []byte(key.Secret), profile); e == nil {
		t.Fatal("portable decoder accepted a local-key reference")
	}
}
