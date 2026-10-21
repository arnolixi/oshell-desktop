package core

import (
	"sort"
	"strings"
)

// The v2 wire contract remains complete for older clients. Only these sections
// participate in synchronization; client sections are compatibility payload.
func sessionSyncDocument(d SharedDocument) SharedDocument {
	defaults := DefaultConfig()
	d.Preferences = preferencesForShared(defaults)
	d.QuickCommands = []QuickCommand{}
	d.HighlightSets = defaults.HighlightSets
	d.Defaults = defaults.SessionDefaults
	d.Links.Visible = defaults.Settings.LinksVisible
	return d
}

func sessionSyncWire(sessions SharedDocument, remote *SharedDocument) SharedDocument {
	wire := sessionSyncDocument(sessions)
	if remote != nil {
		wire.Preferences = remote.Preferences
		wire.QuickCommands = remote.QuickCommands
		wire.HighlightSets = remote.HighlightSets
		wire.Defaults = remote.Defaults
		wire.Links.Visible = remote.Links.Visible
	}
	return wire
}

func applySessionSync(shared, local Config) Config {
	result := local.Clone()
	result.Profiles = shared.Profiles
	result.Proxies = shared.Proxies
	result.Directories = shared.Directories
	result.Links = shared.Links
	result.LinkRootOrder = shared.LinkRootOrder
	return result
}

// First-use merge must retain dependencies belonging to disjoint local and
// remote sessions. Conflicting definitions still require an explicit choice.
func newInitialSessionMerge(local, remote SharedDocument) (*SharedMerge, error) {
	local, remote = sessionSyncDocument(local), sessionSyncDocument(remote)
	remote.Verifier = local.Verifier
	local.Proxies, remote.Proxies = sessionUnion(local.Proxies, remote.Proxies, func(p PortableProxy) string { return strings.ToUpper(p.Definition.ID()) })
	local.Links.Entries, remote.Links.Entries = sessionUnion(local.Links.Entries, remote.Links.Entries, func(l SessionLink) string { return strings.ToUpper(l.ID) })
	directories := unionSessionStrings(remote.Directories, local.Directories)
	sort.Strings(directories)
	local.Directories, remote.Directories = directories, directories
	folders := unionSessionStrings(remote.Links.Folders, local.Links.Folders)
	sort.Strings(folders)
	local.Links.Folders, remote.Links.Folders = folders, folders
	order := unionSessionStrings(remote.Links.RootOrder, local.Links.RootOrder)
	local.Links.RootOrder, remote.Links.RootOrder = order, order
	base := remote
	base.Profiles = []PortableProfile{}
	base.Proxies = []PortableProxy{}
	base.Links.Entries = []SessionLink{}
	return NewSharedMerge(base, local, remote)
}

func sessionUnion[T any](local, remote []T, key func(T) string) ([]T, []T) {
	own, other := map[string]T{}, map[string]T{}
	order := []string{}
	for _, item := range remote {
		id := key(item)
		order = append(order, id)
		other[id] = item
	}
	for _, item := range local {
		id := key(item)
		own[id] = item
		if _, exists := other[id]; !exists {
			order = append(order, id)
		}
	}
	l, r := make([]T, 0, len(order)), make([]T, 0, len(order))
	for _, id := range order {
		lv, lok := own[id]
		rv, rok := other[id]
		if !lok {
			lv = rv
		}
		if !rok {
			rv = lv
		}
		l, r = append(l, lv), append(r, rv)
	}
	return l, r
}

func unionSessionStrings(remote, local []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, values := range [][]string{remote, local} {
		for _, value := range values {
			// Root-order UUIDs are case insensitive; directory names are not.
			key := value
			if prefix, id, ok := strings.Cut(value, ":"); ok && uuidText.MatchString(id) {
				key = prefix + ":" + strings.ToUpper(id)
			}
			if !seen[key] {
				seen[key] = true
				out = append(out, value)
			}
		}
	}
	return out
}
