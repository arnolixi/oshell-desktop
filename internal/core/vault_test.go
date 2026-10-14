package core

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVaultRoundtripAndRekey(t *testing.T) {
	dir := t.TempDir()
	v, e := OpenVault(dir)
	if e != nil {
		t.Fatal(e)
	}
	p := NewProfile()
	p.Host = "example.invalid"
	p.Username = "tester"
	if e = v.Put(p, "never-plaintext"); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if bytes.Contains(b, []byte("never-plaintext")) {
		t.Fatal("plaintext persisted")
	}
	v, e = OpenVault(dir)
	if e != nil {
		t.Fatal(e)
	}
	if password, e := v.Get(p); e != nil || password != "never-plaintext" {
		t.Fatal(password, e)
	}
	if e = v.SetMaster("correct horse battery"); e != nil {
		t.Fatal(e)
	}
	v, e = OpenVault(dir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = v.Get(p); e != ErrVaultLocked {
		t.Fatal("vault not locked")
	}
	if e = v.Unlock("wrong"); e == nil {
		t.Fatal("wrong password accepted")
	}
	if e = v.Unlock("correct horse battery"); e != nil {
		t.Fatal(e)
	}
	q := p
	q.Host = "other.invalid"
	if password, e := v.Get(q); e != nil || password != "" {
		t.Fatal("credentials leaked to changed identity")
	}
	if e = v.SetMaster("replacement password"); e != nil {
		t.Fatal(e)
	}
	v.Lock()
	if e = v.Unlock("correct horse battery"); e == nil {
		t.Fatal("old password accepted")
	}
	if e = v.Unlock("replacement password"); e != nil {
		t.Fatal(e)
	}
	if e = v.SetMaster(""); e != nil {
		t.Fatal(e)
	}
	v, e = OpenVault(dir)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := v.Get(p); e != nil || got != "never-plaintext" {
		t.Fatal(got, e)
	}
	if e = v.Delete([]string{p.ID}); e != nil {
		t.Fatal(e)
	}
	if got, _ := v.Get(p); got != "" {
		t.Fatal("delete failed")
	}
}
