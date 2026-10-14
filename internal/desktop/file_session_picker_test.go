package desktop

import (
	"fmt"
	"oshell-desktop/internal/core"
	"testing"
)

func TestFileSessionPickerFiltersProtocolsAndSearchesEntireCatalog(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	for i := 0; i < 205; i++ {
		p := core.NewProfile()
		p.Name = fmt.Sprintf("server-%03d", i)
		p.Host = "fixture.invalid"
		p.Username = "tester"
		p.Group = "production"
		a.config.Profiles = append(a.config.Profiles, p)
	}
	local := core.NewProfile()
	local.Kind = "local"
	local.Name = "local-only"
	a.config.Profiles = append(a.config.Profiles, local)
	a.showFileSessionPicker()
	if entries := a.switchEntries(); len(entries) != 205 {
		t.Fatal("file picker included non-file profile", len(entries))
	}
	a.features.switcher.query = "server-204 tester production"
	entries := a.switchEntries()
	if len(entries) != 1 || entries[0].Profile.Name != "server-204" {
		t.Fatal("search limited before filtering")
	}
	a.showQuickSwitch()
	a.features.switcher.query = "local-only"
	if entries := a.switchEntries(); len(entries) != 1 {
		t.Fatal("file filter leaked to normal switcher")
	}
}
