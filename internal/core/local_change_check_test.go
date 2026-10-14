package core

import "testing"

func TestLocalChangeCheckIncludesCredentialOnlyChanges(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	cfg := DefaultConfig()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := s.LocalStateChanged(v); err != nil || changed {
		t.Fatal("fresh state changed", err)
	}
	disk, err := OpenVault(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	p := NewProfile()
	p.Host = "fixture.invalid"
	if err := disk.Put(p, "fixture-secret"); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.LocalStateChanged(v); err != nil || !changed {
		t.Fatal("credential-only change missed", err)
	}
}
