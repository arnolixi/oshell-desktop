package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func TestManagerSearchScopeDirectoriesAndSelection(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{"team", "team/server2", "team/server10", "team-other"}
	for _, group := range []string{"team/server2", "team-other"} {
		p := core.NewProfile()
		p.Name = "server"
		p.Group = group
		cfg.Profiles = append(cfg.Profiles, p)
	}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.managerGroup = "team"
	a.query = "server"
	rows := a.managerRows()
	if len(rows) != 4 || !rows[0].parent || rows[1].name != "server2" || rows[2].name != "server10" || rows[3].profile.ID != cfg.Profiles[0].ID {
		t.Fatalf("scoped results: %+v", rows)
	}
	a.refreshManagerSearchSelection()
	if a.managerRow != 1 || !a.managerSelection.Has(rows[1].id) {
		t.Fatal("must select first non-parent result")
	}
	a.managerSearchScope = "all"
	if len(a.managerRows()) != 5 {
		t.Fatal("global scope did not include sibling")
	}
	a.enterRow(rows[1])
	if a.query != "" || a.managerRow != -1 || a.managerSelection.Len() != 0 || a.managerGroup != "team/server2" {
		t.Fatal("navigation retained stale search/selection")
	}
	a.query = strings.Repeat("x", 4097)
	a.refreshManagerSearchSelection()
	if a.managerRow != -1 || !strings.Contains(a.managerResultStatus(0, 0), "4 KB") {
		t.Fatal("invalid query not explained")
	}
}

func TestManagerRootAndFolderButton(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{".team/child"}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.navigateManager(".team/child")
	if managerParent(a.managerGroup) != ".team" {
		t.Fatal("parent mangled dot-prefixed directory")
	}
	tt := ui.NewTester(a.managerView, 900, 580)
	if err := tt.Click("所有会话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.managerGroup != "" || a.managerAll {
		t.Fatal("root button flattened hierarchy")
	}
	for i, row := range a.managerRows() {
		if row.group == ".team" {
			a.managerRow = i
			a.managerSelection.Add(row.id)
		}
	}
	tt.Frame()
	if err := tt.Click("打开"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.managerGroup != ".team" || a.managerRow != -1 {
		t.Fatal("folder button failed")
	}
}

func TestManagerSearchReturnOpensDirectoryAndRetainsScopeOnEdit(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{"team/target"}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.managerGroup = "team"
	a.managerSearchScope = "all"
	tt := ui.NewTester(a.managerView, 900, 580)
	if err := tt.Click("会话搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Type("target")
	tt.Frame()
	if a.managerRow != 1 {
		t.Fatalf("search selection = %d", a.managerRow)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.managerGroup != "team/target" || a.query != "" {
		t.Fatal("search Return did not navigate")
	}
	state := a.captureManagerReturn()
	a.managerSearchScope = ""
	a.features.managerReturn = state
	a.restoreManagerReturn(nil)
	if a.managerSearchScope != "all" {
		t.Fatal("editor return lost search scope")
	}
}
