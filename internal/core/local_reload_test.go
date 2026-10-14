package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRefreshAdoptsDiskConfigAndVaultWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "fixture.invalid"
	cfg.Profiles = []Profile{p}
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Put(p, "old-fixture"); err != nil {
		t.Fatal(err)
	}
	other := &Store{Dir: dir}
	remote, err := other.Load()
	if err != nil {
		t.Fatal(err)
	}
	remote.Profiles[0].Name = "disk-change"
	if err := other.Save(remote); err != nil {
		t.Fatal(err)
	}
	disk, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := disk.Put(p, "new-fixture"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	vaultBefore, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	loaded, replacement, err := s.ReloadLocalConfiguration(context.Background(), cfg, v, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Lock()
	password, err := replacement.Get(p)
	if err != nil || password != "new-fixture" || loaded.Profiles[0].Name != "disk-change" {
		t.Fatal("incoherent reload", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	vaultAfter, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if !bytes.Equal(before, after) || !bytes.Equal(vaultBefore, vaultAfter) {
		t.Fatal("refresh wrote disk")
	}
}
func TestLocalRefreshRejectsUnsavedDraft(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	cfg := DefaultConfig()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Settings.FontSize = 21
	if _, _, err := s.ReloadLocalConfiguration(context.Background(), cfg, v, nil); !errors.Is(err, ErrLocalDraftChanged) {
		t.Fatal(err)
	}
}

func TestLocalRefreshRejectsProtectionDowngrade(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	cfg := DefaultConfig()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.SetMaster("fixture-master"); err != nil {
		t.Fatal(err)
	}
	revision, _ := s.Revision()
	if err := s.ProtectConfiguration(cfg, []byte("fixture-master"), revision); err != nil {
		t.Fatal(err)
	}
	disk, err := OpenVault(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := disk.ChangeMaster("fixture-master", ""); err != nil {
		t.Fatal(err)
	}
	if _, replacement, err := s.ReloadLocalConfiguration(context.Background(), cfg, v, nil); err == nil || replacement != nil {
		t.Fatal("protection downgrade adopted")
	}
	master, locked := v.State()
	if !master || locked {
		t.Fatal("failed reload changed original vault state")
	}
}
func TestLocalRefreshRejectsDiskChangeAfterValidation(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	cfg := DefaultConfig()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.ReloadLocalConfiguration(context.Background(), cfg, v, func(Config) error {
		other := &Store{Dir: s.Dir}
		next, err := other.Load()
		if err != nil {
			return err
		}
		next.Settings.FontSize = 20
		return other.Save(next)
	})
	if !errors.Is(err, ErrConfigurationChanged) {
		t.Fatal("late change not rejected", err)
	}
	baseline, _, err := s.ConfigurationBaseline()
	if err != nil || baseline.Settings.FontSize != cfg.Settings.FontSize {
		t.Fatal("failed reload advanced baseline", err)
	}
}
