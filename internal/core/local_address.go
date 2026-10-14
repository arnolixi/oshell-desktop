package core

import (
	"net"
	"net/netip"
	"sort"
)

type localAddressCandidate struct {
	device  string
	address netip.Addr
}

func preferredLocalAddress(candidates []localAddressCandidate, primary map[string]bool) string {
	rank := func(c localAddressCandidate) int {
		r := 4
		if primary[c.device] {
			r = 0
		}
		if c.address.Is6() {
			r++
		}
		return r
	}
	usable := make([]localAddressCandidate, 0, len(candidates))
	for _, c := range candidates {
		c.address = c.address.Unmap()
		if !c.address.IsValid() || c.address.IsLoopback() || c.address.IsLinkLocalUnicast() || !c.address.IsGlobalUnicast() {
			continue
		}
		usable = append(usable, c)
	}
	sort.Slice(usable, func(i, j int) bool {
		a, b := usable[i], usable[j]
		if rank(a) != rank(b) {
			return rank(a) < rank(b)
		}
		return a.address.String() < b.address.String()
	})
	if len(usable) == 0 {
		return ""
	}
	return usable[0].address.String()
}

// Reads interface state without DNS lookups, subprocesses or network traffic.
func LocalIdentitySnapshot() (LocalHostname, HostIdentity) {
	names := LocalHostnameSnapshot()
	var candidates []localAddressCandidate
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil {
				continue
			}
			candidates = append(candidates, localAddressCandidate{iface.Name, prefix.Addr()})
		}
	}
	return names, HostIdentity{Hostname: names.Hostname, Address: preferredLocalAddress(candidates, primaryLocalInterfaces())}
}

// Preserve old local aliases because existing shells retain their startup name.
func (n LocalHostname) WithPrevious(previous LocalHostname) LocalHostname {
	seen := map[string]bool{}
	for _, alias := range n.Aliases {
		seen[alias] = true
	}
	for _, alias := range previous.Aliases {
		if !seen[alias] {
			n.Aliases = append(n.Aliases, alias)
			seen[alias] = true
		}
	}
	return n
}
