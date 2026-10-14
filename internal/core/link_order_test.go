package core

import (
	"reflect"
	"strings"
	"testing"
)

func linkOrderFixture() Config {
	c := DefaultConfig()
	first := NewProfile()
	first.Name = "remote"
	first.Host = "example.invalid"
	first.Username = "tester"
	second := NewProfile()
	second.Name = "root session"
	second.Host = "other.invalid"
	second.Username = "tester"
	second.Group = "Links"
	c.Profiles = []Profile{first, second}
	c.Directories = []string{"Links/Folder"}
	c.Links = []SessionLink{{ID: NewID(), ProfileID: first.ID, Name: "bookmark"}}
	return c
}
func TestLinkRootOrderPersistsMixedItemsWithoutCopyingProfiles(t *testing.T) {
	c := linkOrderFixture()
	original := c.Clone()
	items := c.OrderedLinkRootItems()
	if len(items) != 3 || items[0].Kind != "folder" || items[1].Kind != "link" || items[2].Kind != "session" {
		t.Fatal(items)
	}
	if changed, e := c.ReorderLinkRoot(items[2].Key(), items[0].Key()); e != nil || !changed {
		t.Fatal(changed, e)
	}
	if c.OrderedLinkRootItems()[0].ID != c.Profiles[1].ID {
		t.Fatal("session not first")
	}
	if !reflect.DeepEqual(c.Profiles, original.Profiles) || !reflect.DeepEqual(c.Links, original.Links) {
		t.Fatal("reorder modified references or credentials")
	}
	store := &Store{Dir: t.TempDir()}
	if e := store.Save(c); e != nil {
		t.Fatal(e)
	}
	loaded, e := store.Load()
	if e != nil || !reflect.DeepEqual(c.LinkRootOrder, loaded.LinkRootOrder) {
		t.Fatal("order not persisted", e)
	}
	if changed, e := c.ReorderLinkRoot(items[2].Key(), items[2].Key()); e != nil || changed {
		t.Fatal("self move changed order")
	}
	before := append([]string(nil), c.LinkRootOrder...)
	if _, e := c.ReorderLinkRoot("link:missing", items[0].Key()); e == nil || !reflect.DeepEqual(before, c.LinkRootOrder) {
		t.Fatal("stale drag changed order")
	}
}
func TestLinkRootOrderFollowsRenameRemovalAndUUIDCase(t *testing.T) {
	c := linkOrderFixture()
	items := c.OrderedLinkRootItems()
	c.LinkRootOrder = []string{strings.ToLower(items[1].Key()), items[0].Key(), items[1].Key(), "folder:missing"}
	c.NormalizeLinkRootOrder()
	if len(c.LinkRootOrder) != 3 || c.LinkRootOrder[0] != items[1].Key() {
		t.Fatal(c.LinkRootOrder)
	}
	if e := c.MoveDirectory("Links/Folder", "Links/Renamed"); e != nil {
		t.Fatal(e)
	}
	if c.LinkRootOrder[1] != "folder:Renamed" {
		t.Fatal("rename lost root position", c.LinkRootOrder)
	}
	if e := c.DeleteDirectory("Links/Renamed"); e != nil {
		t.Fatal(e)
	}
	if len(c.LinkRootOrder) != 2 {
		t.Fatal("deleted folder remained", c.LinkRootOrder)
	}
	c.RemoveProfiles(map[string]bool{c.Profiles[0].ID: true})
	if len(c.LinkRootOrder) != 1 || !strings.HasPrefix(c.LinkRootOrder[0], "session:") {
		t.Fatal("dangling bookmark remained", c.LinkRootOrder)
	}
}
func TestLinkFolderSurvivesLastReferenceDeletion(t *testing.T) {
	c := linkOrderFixture()
	if e := c.AddLink(c.Profiles[0], "Parent/Child"); e != nil {
		t.Fatal(e)
	}
	id := c.Profiles[0].ID
	c.RemoveProfiles(map[string]bool{id: true})
	seen := map[string]bool{}
	for _, d := range c.AllDirectories() {
		seen[d] = true
	}
	if !seen["Links/Parent"] || !seen["Links/Parent/Child"] {
		t.Fatal("empty bookmark folders lost")
	}
}

func TestDirectoryMutationPreservesImplicitParentsAndRejectsStalePaths(t *testing.T) {
	c := DefaultConfig()
	p := NewProfile()
	p.Host = "example.invalid"
	p.Username = "tester"
	p.Group = "Parent/Child"
	c.Profiles = []Profile{p}
	if e := c.DeleteDirectory("Parent/Child"); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, dir := range c.AllDirectories() {
		found = found || dir == "Parent"
	}
	if !found {
		t.Fatal("deleting child also deleted unconfirmed parent")
	}
	before := c.Clone()
	if e := c.MoveDirectory("missing", "New"); e == nil {
		t.Fatal("missing source accepted")
	}
	if !reflect.DeepEqual(c, before) {
		t.Fatal("stale move mutated config")
	}
	if e := c.DeleteDirectory("missing"); e == nil {
		t.Fatal("missing deletion accepted")
	}
	if e := c.MoveDirectory("Parent", "MissingParent/Parent"); e == nil {
		t.Fatal("missing destination accepted")
	}
}

func TestConfigurationClonePreservesEmptySlicesAndRuntimeIsolation(t *testing.T) {
	c := DefaultConfig()
	c.Profiles = []Profile{}
	c.Links = []SessionLink{}
	c.LinkRootOrder = []string{}
	c.Proxies = []ProxyProfile{}
	c.Directories = []string{}
	if !reflect.DeepEqual(c, c.Clone()) {
		t.Fatal("clone changes empty collections into null values")
	}
	p := NewProfile()
	p.Tunnels = []TunnelRule{}
	p.RuntimeProxyRoute = []ProxySettings{{ID: NewID(), Kind: "socks5", Host: "proxy.example", Port: 1080}}
	p.resolvedIdentityFiles = []string{"/fixture/key"}
	c.Profiles = []Profile{p}
	copy := c.Clone()
	if !reflect.DeepEqual(c, copy) {
		t.Fatal("profile clone changes wire shape")
	}
	copy.Profiles[0].RuntimeProxyRoute[0].Host = "changed"
	copy.Profiles[0].resolvedIdentityFiles[0] = "changed"
	if c.Profiles[0].RuntimeProxyRoute[0].Host != "proxy.example" || c.Profiles[0].resolvedIdentityFiles[0] != "/fixture/key" {
		t.Fatal("runtime profile slices shared between clones")
	}
}

func TestFolderCenterMovesReferenceWithoutMovingItsProfile(t *testing.T) {
	c := linkOrderFixture()
	items := c.OrderedLinkRootItems()
	profiles := c.Clone().Profiles
	if changed, e := c.MoveLinkRootIntoFolder(items[1].Key(), "Folder"); e != nil || !changed {
		t.Fatal(changed, e)
	}
	if !reflect.DeepEqual(profiles, c.Profiles) || c.Links[0].Folder != "Folder" {
		t.Fatal("moving reference changed source profile")
	}
	if _, e := c.MoveLinkRootIntoFolder(items[1].Key(), "Folder"); e == nil {
		t.Fatal("stale root drag accepted")
	}
	sessionID := c.Profiles[1].ID
	if _, e := c.MoveLinkRootIntoFolder(items[2].Key(), "Folder"); e != nil {
		t.Fatal(e)
	}
	if c.Profiles[1].ID != sessionID || c.Profiles[1].Group != "Links/Folder" || len(c.Links) != 1 {
		t.Fatal("actual session converted to a reference")
	}
}
