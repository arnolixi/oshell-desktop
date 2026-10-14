package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func cloneShared(t *testing.T, d SharedDocument) SharedDocument {
	t.Helper()
	b, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	var out SharedDocument
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func mergeFixture(t *testing.T) (SharedDocument, []byte) {
	t.Helper()
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "example.invalid"
	p.Username = "tester"
	cfg.Profiles = []Profile{p}
	vault, e := OpenVault(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	master := []byte("synthetic-shared-master")
	d, _, e := vault.ExportSharedDocument(context.Background(), cfg, master, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	return d, master
}
func TestSharedMergeWholeSessionAndDelete(t *testing.T) {
	base, master := mergeFixture(t)
	local, remote := cloneShared(t, base), cloneShared(t, base)
	local.Profiles[0].Definition.Name = "local name"
	remote.Profiles[0].Definition.Host = "remote.invalid"
	merge, e := NewSharedMerge(base, local, remote)
	if e != nil {
		t.Fatal(e)
	}
	if len(merge.Conflicts) != 1 {
		t.Fatalf("different fields of same session must conflict: %+v", merge.Conflicts)
	}
	if _, e = merge.Resolve(nil, master); e == nil {
		t.Fatal("unresolved conflict accepted")
	}
	result, e := merge.Resolve(map[string]SharedMergeChoice{merge.Conflicts[0].Key: SharedKeepLocal}, master)
	if e != nil {
		t.Fatal(e)
	}
	if result.Profiles[0].Definition.Host != "example.invalid" || result.Profiles[0].Definition.Name != "local name" {
		t.Fatal("mixed session revisions")
	}
	remote = cloneShared(t, base)
	remote.Profiles = nil
	merge, e = NewSharedMerge(base, local, remote)
	if e != nil {
		t.Fatal(e)
	}
	if len(merge.Conflicts) != 1 {
		t.Fatal("delete/edit lost conflict")
	}
	result, e = merge.Resolve(map[string]SharedMergeChoice{merge.Conflicts[0].Key: SharedKeepRemote}, master)
	if e != nil {
		t.Fatal(e)
	}
	if len(result.Profiles) != 0 {
		t.Fatal("deleted session retained")
	}
}
func TestSharedMergeIndependentSessionEdits(t *testing.T) {
	base, master := mergeFixture(t)
	p := base.Profiles[0]
	p.Definition.ID = NewID()
	p.Definition.Name = "second"
	base.Profiles = append(base.Profiles, p)
	local, remote := cloneShared(t, base), cloneShared(t, base)
	local.Profiles[0].Definition.Name = "local"
	remote.Profiles[1].Definition.Name = "remote"
	merge, e := NewSharedMerge(base, local, remote)
	if e != nil {
		t.Fatal(e)
	}
	if len(merge.Conflicts) != 0 {
		t.Fatalf("independent sessions conflicted: %+v", merge.Conflicts)
	}
	result, e := merge.Resolve(nil, master)
	if e != nil {
		t.Fatal(e)
	}
	if result.Profiles[0].Definition.Name != "local" || result.Profiles[1].Definition.Name != "remote" {
		t.Fatal("lost independent edit")
	}
}
func TestSharedMergeRedactionAndVerifier(t *testing.T) {
	value := json.RawMessage(`{"password":{"ciphertext":"sensitive-cipher"},"nested":{"apiToken":"private-token"},"name":"visible"}`)
	description := describeSharedJSON(value)
	if strings.Contains(description, "sensitive-cipher") || strings.Contains(description, "private-token") || !strings.Contains(description, "visible") {
		t.Fatal("unsafe conflict details", description)
	}
	base, _ := mergeFixture(t)
	remote := cloneShared(t, base)
	remote.Verifier = nil
	if _, e := NewSharedMerge(base, base, remote); e == nil {
		t.Fatal("merged a changed protection state")
	}
	duplicate := cloneShared(t, base)
	duplicate.Profiles = append(duplicate.Profiles, duplicate.Profiles[0])
	duplicate.Profiles[1].Definition.ID = strings.ToLower(duplicate.Profiles[1].Definition.ID)
	if _, e := NewSharedMerge(base, base, duplicate); e == nil {
		t.Fatal("accepted duplicate UUID")
	}
}
