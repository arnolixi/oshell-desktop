package core

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedVaultHidesIdentityAndUpgradesLegacyFile(t *testing.T) {
	dir := t.TempDir()
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := NewProfile()
	p.Host = "private-vault-host.invalid"
	p.Username = "private-vault-user"
	identity := p.Identity()
	identity.Host = "resolved-private.invalid"
	if err = v.SetMaster("metadata-master-password"); err != nil {
		t.Fatal(err)
	}
	if err = v.PutFor(p, "private-login-secret", identity); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{p.ID, p.Host, p.Username, identity.Host, "private-login-secret"} {
			if bytes.Contains(data, []byte(secret)) {
				t.Fatal("vault metadata leaked", secret)
			}
		}
	}
	check()
	reopened, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.GetFor(p, identity); err != ErrVaultLocked {
		t.Fatal("protected index exposed while locked", err)
	}
	if err = reopened.Unlock("metadata-master-password"); err != nil {
		t.Fatal(err)
	}
	if got, err := reopened.GetFor(p, identity); err != nil || got != "private-login-secret" {
		t.Fatal("roundtrip lost credential", err)
	}
	legacy, _ := json.Marshal(v.data)
	os.WriteFile(filepath.Join(dir, "credentials.json"), legacy, 0600)
	old, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = old.Unlock("wrong"); err == nil {
		t.Fatal("wrong password accepted")
	}
	unchanged, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if !bytes.Equal(unchanged, legacy) {
		t.Fatal("failed unlock modified legacy file")
	}
	if err = old.Unlock("metadata-master-password"); err != nil {
		t.Fatal(err)
	}
	check()
}
func TestMissingLocalKeyDoesNotLoseMasterProtectedIndex(t *testing.T) {
	dir := t.TempDir()
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = v.SetMaster("metadata-master-password"); err != nil {
		t.Fatal(err)
	}
	p := NewProfile()
	p.Host = "local-index.invalid"
	if err = v.PutForRevision(p, "local-secret", p.Identity(), v.Revision(), "local"); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(dir, "credential-key")); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenVault(dir)
	if err != nil {
		t.Fatal("local key loss blocked all master access", err)
	}
	if err = reopened.Unlock("metadata-master-password"); err != nil {
		t.Fatal(err)
	}
	if !reopened.HasStoredCredential(p.ID) {
		t.Fatal("master index lost local entry")
	}
	if _, err = reopened.GetFor(p, p.Identity()); err == nil {
		t.Fatal("missing local key silently recreated")
	}
	q := NewProfile()
	q.Host = "remaining-master.invalid"
	if err = reopened.PutFor(q, "master-secret", q.Identity()); err != nil {
		t.Fatal("local key loss blocked unrelated master credential", err)
	}
	if _, err = os.Stat(filepath.Join(dir, "credential-key")); !os.IsNotExist(err) {
		t.Fatal("missing local key was recreated")
	}
	again, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = again.Unlock("metadata-master-password"); err != nil || !again.HasStoredCredential(p.ID) {
		t.Fatal("save discarded local credential index", err)
	}

}
