package core

import "testing"

func TestLocalConfigThreeWayMergeAndConflictingSession(t *testing.T) {
	base := DefaultConfig()
	for _, name := range []string{"one", "two"} {
		p := NewProfile()
		p.Name = name
		p.Host = name + ".invalid"
		base.Profiles = append(base.Profiles, p)
	}
	local, remote := base.Clone(), base.Clone()
	local.Profiles[0].Name = "local rename"
	remote.Profiles[1].Host = "remote-change.invalid"
	plan, err := NewLocalConfigMerge(base, local, remote)
	if err != nil || len(plan.Conflicts) != 0 {
		t.Fatal("independent changes conflicted", err)
	}
	merged, err := plan.Resolve(nil)
	if err != nil || merged.Profiles[0].Name != "local rename" || merged.Profiles[1].Host != "remote-change.invalid" {
		t.Fatal("independent changes lost", err)
	}
	remote.Profiles[0].Host = "same-session-change.invalid"
	plan, err = NewLocalConfigMerge(base, local, remote)
	if err != nil || len(plan.Conflicts) != 1 {
		t.Fatal("same-session conflict missing", err)
	}
	if _, err = plan.Resolve(nil); err == nil {
		t.Fatal("unresolved conflict accepted")
	}
	chosen, err := plan.Resolve(map[string]SharedMergeChoice{plan.Conflicts[0].Key: SharedKeepRemote})
	if err != nil || chosen.Profiles[0].Host != "same-session-change.invalid" || chosen.Profiles[0].Name != "one" {
		t.Fatal("session was partially mixed", err)
	}
}
func TestStoreBaselineIsImmutableAndTracksCommittedConfig(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	cfg := DefaultConfig()
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if _, revision, err := store.ConfigurationBaseline(); err != nil || revision != "" {
		t.Fatal("missing-file baseline not captured", err)
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	baseline, revision, err := store.ConfigurationBaseline()
	if err != nil || revision == "" {
		t.Fatal(err)
	}
	baseline.Settings.FontSize = 24
	again, _, _ := store.ConfigurationBaseline()
	if again.Settings.FontSize == 24 {
		t.Fatal("caller mutated store baseline")
	}
}
