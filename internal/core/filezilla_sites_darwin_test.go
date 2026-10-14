//go:build darwin

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileZillaReadRejectsSymlinkAndPreservesSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.xml")
	data := []byte(`<FileZilla3><Servers><Server><Name>one</Name><Host>example.invalid</Host><Protocol>1</Protocol><Logontype>2</Logontype><User>alice</User></Server></Servers></FileZilla3>`)
	if e := os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("OSHELL_FILEZILLA_CONFIG", path)
	got, e := ReadFileZillaSites("0/one")
	if e != nil || string(got) != string(data) {
		t.Fatal("read failed", e)
	}
	alias := filepath.Join(dir, "alias.xml")
	if e = os.Symlink(path, alias); e != nil {
		t.Fatal(e)
	}
	t.Setenv("OSHELL_FILEZILLA_CONFIG", alias)
	if _, e = ReadFileZillaSites("0/one"); e == nil {
		t.Fatal("followed symlink")
	}
	original, e := os.ReadFile(path)
	if e != nil || string(original) != string(data) {
		t.Fatal("rewrote source")
	}
}
func TestExternalTerminalTypeIsRuntimeOnly(t *testing.T) {
	request, e := ParseZOCLaunch([]string{"/SSH:alice@example.invalid", "/EMU:VT100"})
	if e != nil {
		t.Fatal(e)
	}
	profile, e := request.ProfileValue()
	if e != nil || profile.RuntimeTerminalType != "vt100" {
		t.Fatal("lost terminal type", e)
	}
	data, e := json.Marshal(profile)
	if e != nil || strings.Contains(string(data), "vt100") {
		t.Fatal("runtime launch field persisted", e)
	}
}
