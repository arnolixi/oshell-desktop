package core

import (
	"strings"
	"testing"
)

func TestZOCLaunchConnectionOnly(t *testing.T) {
	request, e := ParseZOCLaunch([]string{"/DEV:SSH", "/CONNECT:alice:one-time@[2001:db8::1]:2222", "/EMU:Xterm", "/TITLE:USM"})
	if e != nil {
		t.Fatal(e)
	}
	profile, e := request.ProfileValue()
	if e != nil || profile.Host != "2001:db8::1" || profile.Port != 2222 || profile.Username != "alice" || request.Password == nil || *request.Password != "one-time" || profile.QuickConnect {
		t.Fatal("connection parse failed", e)
	}
	for _, args := range [][]string{{"/SSH:host", "/RUN:bad"}, {"/SSH:host", "/CONNECT:host"}, {"/SSH:host", "/SSH:other"}, {"/DEV:TELNET", "/CONNECT:host"}, {"/SSH:host", "/ENCODING:GBK"}, {"/SSH:host", "/SSHPASSWORD:test"}} {
		if _, e = ParseZOCLaunch(args); e == nil {
			t.Fatal("accepted invalid args")
		}
	}
}
func TestFileLaunchURLAndSelectedSite(t *testing.T) {
	request, e := ParseFileLaunch([]string{"sftp://alice:p%40ss@[2001:db8::1]:2222/srv/test"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if request.Profile.InitialDirectory != "/srv/test" || *request.Password != "p@ss" {
		t.Fatal("URL decoding")
	}
	data := []byte(`<FileZilla3><Servers><Folder>Group<Server><Name>Selected</Name><Host>example.invalid</Host><Port>22</Port><Protocol>1</Protocol><Logontype>1</Logontype><User>alice</User><Pass encoding="base64">c2VjcmV0</Pass><RemoteDir>private serialization</RemoteDir></Server><Server><Name>Other</Name><Host>other.invalid</Host></Server></Folder></Servers></FileZilla3>`)
	request, e = ParseFileZillaSite("0/Group/Selected", data)
	if e != nil {
		t.Fatal(e)
	}
	if request.Profile.Host != "example.invalid" || *request.Password != "secret" || request.Profile.InitialDirectory != "." {
		t.Fatal("wrong site or unsafe remote path")
	}
	duplicate := strings.Replace(string(data), "<Host>example.invalid</Host>", "<Host>one</Host><Host>two</Host>", 1)
	if _, e = ParseFileZillaSite("0/Group/Selected", []byte(duplicate)); e == nil {
		t.Fatal("duplicate field accepted")
	}
	if _, e = ParseFileZillaSite("0/Group/Selected", append([]byte(`<!DOCTYPE x [<!ENTITY x SYSTEM "file:///no-access">]>`), data...)); e == nil {
		t.Fatal("external entity accepted")
	}
	for _, url := range []string{"ftps://host/", "sftp://host/?query=x", "ftp://host/#fragment"} {
		if _, e = ParseFileLaunch([]string{url}, nil); e == nil {
			t.Fatal("unsupported URL accepted")
		}
	}
}
