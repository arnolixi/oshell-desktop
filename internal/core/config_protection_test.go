package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedConfigurationRoundtripAndLockedRecognition(t *testing.T) {
	cfg := DefaultConfig()
	p := NewProfile()
	p.Name = "private-session-name"
	p.Host = "private-host.invalid"
	cfg.Profiles = []Profile{p}
	cfg.Directories = []string{"private-directory"}
	master := []byte("fixture-master-password")
	encrypted, err := EncodeConfiguration(cfg, master)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{p.Name, p.Host, "private-directory"} {
		if bytes.Contains(encrypted, []byte(private)) {
			t.Fatal("metadata exposed", private)
		}
	}
	if _, protected, err := DecodeConfiguration(encrypted, nil); !protected || !errors.Is(err, ErrConfigurationLocked) {
		t.Fatal("encrypted config became defaults", err)
	}
	if _, protected, err := DecodeConfiguration(encrypted, []byte("wrong")); !protected || err == nil {
		t.Fatal("wrong password accepted", err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	if err = os.WriteFile(file, encrypted, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = (&Store{Dir: dir}).Load(); !errors.Is(err, ErrConfigurationLocked) {
		t.Fatal("store silently loaded encrypted config as defaults", err)
	}
	if err = (&Store{Dir: dir}).Save(cfg); !errors.Is(err, ErrConfigurationLocked) {
		t.Fatal("locked config was overwritten by plain save", err)
	}
	unchanged, _ := os.ReadFile(file)
	if !bytes.Equal(unchanged, encrypted) {
		t.Fatal("locked store modified config")
	}
	loaded, protected, err := DecodeConfiguration(encrypted, master)
	if err != nil || !protected || loaded.Profiles[0].ID != p.ID || loaded.Profiles[0].Host != p.Host {
		t.Fatal("protected roundtrip failed", err)
	}
	plain, err := EncodeConfiguration(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	loaded, protected, err = DecodeConfiguration(plain, nil)
	if err != nil || protected || loaded.Profiles[0].ID != p.ID {
		t.Fatal("plain compatibility failed", err)
	}
	damaged := append([]byte(nil), encrypted...)
	damaged[len(damaged)/2] ^= 1
	if _, _, err = DecodeConfiguration(damaged, master); err == nil {
		t.Fatal("damaged config accepted")
	}
}
