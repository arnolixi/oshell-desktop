package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeLocalKeyStorageMigratesWithoutSourceMaster(t *testing.T) {
	source := t.TempDir()
	key := LocalCredentialKey{ID: NewID(), Secret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, 32))}
	keyData, _ := json.Marshal(key)
	if err := os.WriteFile(filepath.Join(source, "local-credential-key.json"), keyData, 0600); err != nil {
		t.Fatal(err)
	}
	p := NewProfile()
	p.Host = "native-local.invalid"
	p.Username = "tester"
	envelope, err := EncryptMacPassword("native-local-secret", []byte(key.Secret), p, p.Identity())
	if err != nil {
		t.Fatal(err)
	}
	envelope.LocalKeyID = key.ID
	doc := SharedDocument{HighlightSets: DefaultConfig().HighlightSets, Profiles: []PortableProfile{{Definition: p, Password: &envelope}}, Preferences: preferencesForShared(DefaultConfig()), Defaults: DefaultSessionDefaults(), Links: SharedLinks{Visible: true}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(source, "configuration.json"), data, 0600)
	snapshot, err := ReadNativeStorage(source, nil)
	if err != nil || !snapshot.PlainLocal {
		t.Fatal("local config not recognized", err)
	}
	store := &Store{Dir: t.TempDir()}
	previous := DefaultConfig()
	if err = store.Save(previous); err != nil {
		t.Fatal(err)
	}
	vault, err := OpenVault(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.SetMaster("destination-master-password"); err != nil {
		t.Fatal(err)
	}
	revision, _ := store.Revision()
	cfg, _, err := store.AdoptNativeStorage(context.Background(), snapshot, nil, previous, vault, revision, vault.Revision())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := vault.GetFor(cfg.Profiles[0], p.Identity()); err != nil || got != "native-local-secret" {
		t.Fatal("local secret not converted", err)
	}
	after, _ := os.ReadFile(filepath.Join(source, "local-credential-key.json"))
	if !bytes.Equal(after, keyData) {
		t.Fatal("source key altered")
	}
	wrapped, err := SealShared(data, []byte("source-wrapper-password"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(source, "configuration.json"), wrapped, 0600)
	wrappedSnapshot, err := ReadNativeStorage(source, []byte("source-wrapper-password"))
	if err != nil {
		t.Fatal("encrypted native local envelope rejected", err)
	}
	_, updates, err := wrappedSnapshot.prepareReplacement(context.Background(), cfg, []byte("source-wrapper-password"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range updates {
		if updates[i].Password == "native-local-secret" {
			found = true
		}
		updates[i].Password = ""
	}
	if !found {
		t.Fatal("wrapped local credential lost")
	}
	key.ID = NewID()
	wrong, _ := json.Marshal(key)
	os.WriteFile(filepath.Join(source, "local-credential-key.json"), wrong, 0600)
	revision, _ = store.Revision()
	before := vault.Revision()
	if _, _, err = store.AdoptNativeStorage(context.Background(), snapshot, nil, cfg, vault, revision, before); err == nil {
		t.Fatal("mismatched key accepted")
	}
	if vault.Revision() != before {
		t.Fatal("failed migration changed vault")
	}
}
