package core

import "testing"

func TestHostIdentitySemantics(t *testing.T) {
	for _, tc := range []struct{ in, ip string }{{"host|10.1.2.3|192.0.2.3", "10.1.2.3"}, {"host||::1 fe80::1 2001:db8::1 192.0.2.3/24", "192.0.2.3"}, {"host|::|127.0.0.1 2001:db8::2", "2001:db8::2"}} {
		got, ok := ParseHostIdentity(tc.in)
		if !ok || got.Address != tc.ip {
			t.Fatal(got, ok)
		}
	}
	h := HostObservation{Mode: "shellIntegration"}
	h.Integration(HostIdentity{"outer-real", "10.1.2.3"})
	h.Hint("prompt-alias")
	if h.Identity.Hostname != "outer-real" {
		t.Fatal("prompt replaced integrated identity")
	}
	h.Hint("plain-inner")
	if h.Identity.Address != "" || h.Identity.Hostname != "plain-inner" || h.AllowsActiveProbe() {
		t.Fatal("nested host retained old IP or enabled probes")
	}
	h = HostObservation{Mode: "passive"}
	if h.Integration(HostIdentity{"ignored", "1.2.3.4"}) {
		t.Fatal("passive accepted integration")
	}
	names := LocalHostnameWithNames("ConfiguredMac", "vpn-runtime.tail.ts.net")
	if names.Canonical("vpn-runtime.tail.ts.net") != "ConfiguredMac" || names.Canonical("remote.tail.ts.net") != "remote.tail.ts.net" {
		t.Fatal("local normalization stripped remote domain")
	}
	if HostnameFromPrompt("user@server:/tmp$ ") != "server" || HostnameFromPrompt("log user@server: failure") != "" {
		t.Fatal("prompt recognition")
	}
}

func TestHostChangeRecognition(t *testing.T) {
	for _, input := range []string{"user@outer:~$ ssh inner", "[root@outer ~]# /usr/bin/ssh inner", "user@outer ~ % exit"} {
		if !ChangesHost(input, true) {
			t.Fatal("missed transition", input)
		}
	}
	if ChangesHost("user@host:~$ echo ssh", true) || ChangesHost("ssh is mentioned in a log", true) {
		t.Fatal("mistook output for a host transition")
	}
}
