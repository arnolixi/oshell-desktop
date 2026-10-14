package core

import (
	"reflect"
	"testing"
)

func TestLinkEditingAndRemovalNeverChangesSource(t *testing.T) {
	c := linkOrderFixture()
	original := c.Clone().Profiles
	id := c.Links[0].ID
	if e := c.UpdateLink(id, "Custom alias", "Folder"); e != nil {
		t.Fatal(e)
	}
	if c.Links[0].Name != "Custom alias" || c.Links[0].Folder != "Folder" || !reflect.DeepEqual(original, c.Profiles) {
		t.Fatal("reference edit modified source")
	}
	if !c.RemoveLink(id) || len(c.Links) != 0 || !reflect.DeepEqual(original, c.Profiles) {
		t.Fatal("reference removal deleted source")
	}
}
func TestLinkFolderDeleteProtectsActualSessions(t *testing.T) {
	c := linkOrderFixture()
	c.Profiles[1].Group = "Links/Folder"
	before := c.Clone()
	if e := c.RemoveLinkFolder("Folder"); e == nil || !reflect.DeepEqual(before, c) {
		t.Fatal("toolbar deleted actual sessions")
	}
	c.Profiles[1].Group = "Links"
	if e := c.RemoveLinkFolder("Folder"); e != nil {
		t.Fatal(e)
	}
	if len(c.Profiles) != 2 {
		t.Fatal("reference folder deletion removed source profiles")
	}
	for _, name := range []string{"", "..", "a/b", "a\\b", "bad\nname"} {
		if _, e := LinkFolderChild("", name); e == nil {
			t.Fatal("invalid folder component accepted")
		}
	}
}
func TestLinkEditRejectsDuplicateDestinationAtomically(t *testing.T) {
	c := linkOrderFixture()
	if e := c.AddLink(c.Profiles[0], "Folder"); e != nil {
		t.Fatal(e)
	}
	before := c.Clone()
	if e := c.UpdateLink(c.Links[0].ID, "new name", "Folder"); e == nil || !reflect.DeepEqual(before, c) {
		t.Fatal("duplicate edit was not atomic")
	}
}
