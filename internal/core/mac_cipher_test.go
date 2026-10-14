package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type cipherFixture struct {
	Profile  Profile        `json:"profile"`
	Master   string         `json:"master"`
	Password string         `json:"password"`
	Envelope LegacyPassword `json:"envelope"`
}

func TestMac0264CipherInteroperability(t *testing.T) {
	data, e := os.ReadFile("testdata/oshell-0.2.64-password.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture cipherFixture
	if e = json.Unmarshal(data, &fixture); e != nil {
		t.Fatal(e)
	}
	plain, e := DecryptMacPassword(fixture.Envelope, []byte(fixture.Master), fixture.Profile)
	if e != nil || plain != fixture.Password {
		t.Fatal("Swift ciphertext did not decrypt", e)
	}
	changed := fixture.Profile
	changed.Host = "changed.example.invalid"
	if _, e = DecryptMacPassword(fixture.Envelope, []byte(fixture.Master), changed); e == nil {
		t.Fatal("identity tampering accepted")
	}
	fixture.Envelope, e = EncryptMacPassword(fixture.Password, []byte(fixture.Master), fixture.Profile, fixture.Envelope.Identity)
	if e != nil {
		t.Fatal(e)
	}
	if p, e := DecryptMacPassword(fixture.Envelope, []byte(fixture.Master), fixture.Profile); e != nil || p != fixture.Password {
		t.Fatal(e)
	}
	if dir := os.Getenv("OSHELL_INTEROP_OUTPUT"); dir != "" {
		b, _ := json.MarshalIndent(fixture, "", "  ")
		if e = os.WriteFile(filepath.Join(dir, "go-password.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
func TestVaultResolvedIdentityBinding(t *testing.T) {
	v, e := OpenVault(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p := NewProfile()
	p.Host = "alias"
	p.Username = ""
	identity := SSHIdentity{Host: "actual.example.invalid", User: "actual", Port: 22}
	if e = v.PutFor(p, "test-secret", identity); e != nil {
		t.Fatal(e)
	}
	if got, e := v.GetFor(p, identity); e != nil || got != "test-secret" {
		t.Fatal(got, e)
	}
	different := identity
	different.Host = "other.example.invalid"
	if got, _ := v.GetFor(p, different); got != "" {
		t.Fatal("credential reused for changed alias")
	}
	entry := v.data.Entries[p.ID]
	entry.Resolved = &different
	v.data.Entries[p.ID] = entry
	if _, e = v.GetFor(p, different); e == nil {
		t.Fatal("resolved identity not authenticated with ciphertext")
	}
}
