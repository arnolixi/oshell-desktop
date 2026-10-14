package core

import (
	"os"
	"strings"
)

type LocalHostname struct {
	Hostname string
	Aliases  []string
}

func LocalHostnameSnapshot() LocalHostname {
	runtime, _ := os.Hostname()
	configured := configuredLocalHostname()
	return LocalHostnameWithNames(configured, runtime)
}
func LocalHostnameWithNames(configured, runtime string) LocalHostname {
	preferred := runtime
	if ValidHostnameHint(configured) {
		preferred = configured
	}
	names := []string{preferred, preferred + ".local", runtime}
	for i := range names {
		names[i] = strings.ToLower(strings.Trim(names[i], "."))
	}
	return LocalHostname{preferred, names}
}
func (n LocalHostname) Canonical(hint string) string {
	key := strings.ToLower(strings.Trim(hint, "."))
	for _, alias := range n.Aliases {
		if alias == key {
			return n.Hostname
		}
	}
	return hint
}
