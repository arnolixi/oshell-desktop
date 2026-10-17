package core

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestSharedSyncLegacyFTPConflictPreservesBothSides(t *testing.T) {
	ctx := context.Background()
	engine, cfg, master := syncEngineFixture(t, t.TempDir())
	cfg.Profiles[0].Kind, cfg.Profiles[0].Port, cfg.Profiles[0].InitialDirectory = "ftp", 21, "/"
	if err := engine.Store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	plan, err := engine.Prepare(ctx, cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = plan.Commit(ctx, nil, master)
	if err != nil {
		t.Fatal(err)
	}
	object, err := engine.Transport.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := OpenShared(object.Data, master)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err = json.Unmarshal(plain, &raw); err != nil {
		t.Fatal(err)
	}
	p := cfg.Profiles[0]
	envelope, err := EncryptMacPassword("legacy-fixture-secret", master, p, p.Identity())
	if err != nil {
		t.Fatal(err)
	}
	raw["ftpProfiles"] = []any{map[string]any{"id": p.ID, "name": p.Name, "host": p.Host, "port": p.Port, "username": p.Username, "initialDirectory": p.InitialDirectory, "encryptedPassword": envelope}}
	plain, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	conflicting, err := SealShared(plain, master)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Transport.Put(ctx, conflicting, object.ETag); err != nil {
		t.Fatal(err)
	}
	revision, err := engine.Store.Revision()
	if err != nil {
		t.Fatal(err)
	}
	vaultRevision := engine.Vault.Revision()
	if _, err = engine.Prepare(ctx, cfg, master, false); err == nil {
		t.Fatal("legacy credential collision allowed synchronization")
	}
	after, err := engine.Transport.Fetch(ctx)
	if err != nil || !bytes.Equal(after.Data, conflicting) {
		t.Fatal("remote legacy data changed", err)
	}
	current, err := engine.Store.Revision()
	if err != nil || current != revision || engine.Vault.Revision() != vaultRevision {
		t.Fatal("local configuration or credentials changed", err)
	}
}

func legacyFTPDocument(t *testing.T, current []PortableProfile, legacy map[string]any) []byte {
	t.Helper()
	if current == nil {
		current = []PortableProfile{}
	}
	data, err := json.Marshal(map[string]any{"profiles": current, "ftpProfiles": []any{legacy}, "preferences": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLegacyFTPMigrationIsStable(t *testing.T) {
	legacy := map[string]any{"id": NewID(), "name": "Legacy FTP", "host": "fixture.invalid", "port": 21, "username": "fixture", "initialDirectory": "/"}
	data := legacyFTPDocument(t, nil, legacy)
	a, err := decodeNativeDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := decodeNativeDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	if !sameShared(a, b) {
		t.Fatal("reading the same legacy FTP document created a new sync revision")
	}
}

func TestLegacyFTPDuplicateCannotDiscardFields(t *testing.T) {
	p := NewProfile()
	p.Kind, p.Name, p.Host, p.Username, p.Port, p.InitialDirectory = "ftp", "Legacy FTP", "fixture.invalid", "fixture", 21, "/"
	envelope, err := EncryptMacPassword("fixture-secret", []byte("fixture-master"), p, p.Identity())
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"same", "name", "initialDirectory", "encryptedPassword"} {
		t.Run(field, func(t *testing.T) {
			legacy := map[string]any{"id": p.ID, "name": p.Name, "host": p.Host, "port": p.Port, "username": p.Username, "initialDirectory": p.InitialDirectory}
			switch field {
			case "name":
				legacy[field] = "Another name"
			case "initialDirectory":
				legacy[field] = "/another"
			case "encryptedPassword":
				legacy[field] = envelope
			}
			doc, err := decodeNativeDocument(legacyFTPDocument(t, []PortableProfile{{Definition: p}}, legacy))
			if field == "same" {
				if err != nil || len(doc.Profiles) != 1 {
					t.Fatal("identical legacy entry must deduplicate", err)
				}
			} else if err == nil {
				t.Fatal("conflicting legacy field was silently discarded")
			}
		})
	}
}
