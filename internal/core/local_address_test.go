package core

import (
	"net/netip"
	"testing"
)

func TestLocalAddressPreference(t *testing.T) {
	candidate := func(device, address string) localAddressCandidate {
		return localAddressCandidate{device, netip.MustParseAddr(address)}
	}
	candidates := []localAddressCandidate{candidate("lo0", "127.0.0.1"), candidate("en0", "169.254.1.1"), candidate("en0", "fe80::1"), candidate("other", "10.0.0.1"), candidate("en0", "2001:db8::2"), candidate("en0", "192.168.1.5"), candidate("other", "224.0.0.1")}
	if got := preferredLocalAddress(candidates, map[string]bool{"en0": true}); got != "192.168.1.5" {
		t.Fatal(got)
	}
	if got := preferredLocalAddress(candidates[:5], map[string]bool{"en0": true}); got != "2001:db8::2" {
		t.Fatal("primary IPv6 must outrank other IPv4", got)
	}
	if got := preferredLocalAddress(candidates[:3], nil); got != "" {
		t.Fatal("invalid interfaces selected", got)
	}
	if got := preferredLocalAddress(candidates, nil); got != "10.0.0.1" {
		t.Fatal("fallback ordering", got)
	}
}
func TestLocalIdentitySnapshotUsesNumericAddress(t *testing.T) {
	names, identity := LocalIdentitySnapshot()
	if identity.Hostname != names.Hostname {
		t.Fatal("hostname mismatch")
	}
	if identity.Address != "" {
		if _, err := netip.ParseAddr(identity.Address); err != nil {
			t.Fatal(err)
		}
	}
}
func TestLocalHostnameRetainsEarlierNetworkAliases(t *testing.T) {
	old := LocalHostnameWithNames("my-mac", "vpn-runtime")
	current := LocalHostnameWithNames("renamed-mac", "current-runtime").WithPrevious(old)
	if current.Canonical("VPN-RUNTIME.") != "renamed-mac" || current.Canonical("my-mac.local") != "renamed-mac" {
		t.Fatal("lost prior local alias")
	}
	if current.Canonical("remote.example") != "remote.example" {
		t.Fatal("remote host rewritten")
	}
	next := current.WithPrevious(old)
	if len(next.Aliases) != len(current.Aliases) {
		t.Fatal("alias growth on repeated updates")
	}
}
