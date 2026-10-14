package desktop

import (
	"os"
	"oshell-desktop/internal/core"
	"testing"
)

func TestNativeLinksFixture(t *testing.T) {
	directory := os.Getenv("OSHELL_NATIVE_LINK_FIXTURE_DIR")
	if directory == "" {
		t.Skip("native links acceptance fixture only")
	}
	if e := os.MkdirAll(directory, 0700); e != nil {
		t.Fatal(e)
	}
	cfg := core.DefaultConfig()
	bookmark := core.NewProfile()
	bookmark.Name = "Bookmark"
	bookmark.Host = "bookmark.invalid"
	bookmark.Username = "fixture"
	direct := core.NewProfile()
	direct.Name = "RootSession"
	direct.Host = "session.invalid"
	direct.Username = "fixture"
	direct.Group = "Links"
	cfg.Profiles = []core.Profile{bookmark, direct}
	if e := cfg.AddDirectory("Links/Folder"); e != nil {
		t.Fatal(e)
	}
	if e := cfg.AddLink(bookmark, ""); e != nil {
		t.Fatal(e)
	}
	if e := (&core.Store{Dir: directory}).Save(cfg); e != nil {
		t.Fatal(e)
	}
}
