package desktop

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSidebarFiltersLinksAndRetainsAncestors(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name, p.Group, p.Host, p.Username, p.Port = "开发机器", "团队/开发", "192.0.2.9", "tester", 2222
	cfg.Profiles = []core.Profile{p}
	cfg.Links = []core.SessionLink{{ID: "reference", ProfileID: p.ID, Name: "快捷入口", Folder: "常用"}}
	for _, query := range []string{"开发", "192.0.2.9", "tester", "2222"} {
		entries, folders := sidebarEntries(cfg, query)
		if len(entries) == 0 || !folders["团队"] || !folders["团队/开发"] {
			t.Fatalf("%s: %#v %#v", query, entries, folders)
		}
	}
	entries, folders := sidebarEntries(cfg, "快捷入口")
	if len(entries) != 1 || entries[0].profile.ID != p.ID || !folders["Links/常用"] || !folders["Links"] {
		t.Fatal(entries, folders)
	}
	if entries, _ := sidebarEntries(cfg, "not-present"); len(entries) != 0 {
		t.Fatal(entries)
	}
}
func TestSidebarLocalSettingsStayOutsideSharedConfig(t *testing.T) {
	dir := t.TempDir()
	a := newApp(&core.Store{Dir: dir}, core.DefaultConfig())
	before, _ := json.Marshal(a.config)
	preferences := a.localUI
	preferences.WordSeparators = "/:"
	preferences.SidebarVisible = false
	if !a.saveLocalPreferences(preferences) {
		t.Fatal(a.status)
	}
	after, _ := json.Marshal(a.config)
	if string(before) != string(after) {
		t.Fatal("local UI settings changed shared config")
	}
	if got := loadLocalPreferences(dir); !reflect.DeepEqual(got, preferences) {
		t.Fatal(got)
	}
	os.WriteFile(filepath.Join(dir, "desktop-preferences.json"), []byte(`{"sidebarVisible":false,"wordSeparators":5}`), 0600)
	if got := loadLocalPreferences(dir); !reflect.DeepEqual(got, defaultLocalPreferences()) {
		t.Fatal("partial corrupt preferences applied", got)
	}
}
func TestSidebarSearchRestoresExpansionAndHeaderSettings(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := core.NewProfile()
	p.Name, p.Group = "fixture", "parent/child"
	a.config.Profiles = []core.Profile{p}
	collapsed := false
	a.sidebar.open = map[string]*bool{"parent": &collapsed}
	a.sidebar.query = "fixture"
	tt := ui.NewTester(a.sidebarView, 235, 500)
	tt.Frame()
	if !tt.HasText("fixture") {
		t.Fatal("search did not expand ancestors")
	}
	a.sidebar.query = ""
	tt.Frame()
	if collapsed || tt.HasText("fixture") {
		t.Fatal("search changed original expansion")
	}
	header := ui.NewTester(a.view, 880, 560)
	header.Frame()
	if _, ok := header.Find("设置"); !ok {
		t.Fatal("settings header button missing")
	}
	snapshot(t, header, "sidebar-header.png")
	if err := header.Click("设置"); err != nil || !a.settingsOpen {
		t.Fatal("settings button did not open settings", err)
	}
	a.features.settingsTab = 8
	settings := ui.NewTester(a.settingsView, 800, 650)
	settings.Frame()
	snapshot(t, settings, "selection-settings.png")
}
