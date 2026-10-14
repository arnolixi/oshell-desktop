package core

import (
	"encoding/json"
	"testing"
)

func TestProfileInitialDirectoryMatchesNativeValidation(t *testing.T) {
	for _, kind := range []string{"ssh", "sftp", "ftp"} {
		p := DefaultSessionDefaults().MakeProfile(kind, "")
		p.Host = "fixture.invalid"
		p.InitialDirectory = ""
		if err := p.Validate(); err == nil {
			t.Fatal(kind, "accepted empty directory")
		}
		for _, directory := range []string{".", "/", "/with spaces/目录"} {
			p.InitialDirectory = directory
			if err := p.Validate(); err != nil {
				t.Fatal(kind, directory, err)
			}
		}
	}
	p := NewProfile()
	p.Kind = "local"
	p.InitialDirectory = ""
	if err := p.Validate(); err != nil {
		t.Fatal("local session requires remote directory", err)
	}
	d := DefaultSessionDefaults()
	d.SSHDirectory = ""
	if err := d.Validate(); err == nil {
		t.Fatal("defaults accepted empty directory")
	}
	// Missing legacy field keeps the default; explicit empty values are invalid.
	var legacy Profile
	if err := json.Unmarshal([]byte(`{"host":"fixture.invalid","name":"fixture","kind":"ssh"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.InitialDirectory != "." || legacy.Validate() != nil {
		t.Fatal("missing legacy field lost default")
	}
}
