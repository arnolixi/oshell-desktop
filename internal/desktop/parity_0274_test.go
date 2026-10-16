package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestInlineRenamePreservesDirectoryContentsAndReferences(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	p.Name = "original"
	p.Group = "team/sub"
	cfg.Profiles = []core.Profile{p}
	cfg.Directories = []string{"team", "team/sub"}
	cfg.Links = []core.SessionLink{{ID: core.NewID(), ProfileID: p.ID, Name: "alias"}}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.beginManagerRename(sessionRow{id: "folder:team", name: "team", group: "team", folder: true})
	a.managerRename.name = "renamed"
	a.commitManagerRename()
	if a.config.Profiles[0].Group != "renamed/sub" {
		t.Fatal(a.status)
	}
	a.beginManagerRename(sessionRow{id: p.ID, name: p.Name, profile: p})
	a.managerRename.name = "new name"
	a.commitManagerRename()
	if a.config.Profiles[0].Name != "new name" || a.config.Links[0].ProfileID != p.ID {
		t.Fatal(a.status)
	}
	link := a.config.Links[0]
	a.beginManagerRename(sessionRow{id: "link:" + link.ID, linkID: link.ID, name: link.Name, profile: p})
	a.managerRename.name = "new alias"
	a.commitManagerRename()
	if a.config.Links[0].Name != "new alias" || a.config.Profiles[0].Name != "new name" {
		t.Fatal(a.status)
	}
}
func TestManagerRenameEscapeKeepsWindowAndData(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name = "fixture"
	p.Group = ""
	p.Host = "fixture.invalid"
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	tt := ui.NewTester(a.managerView, 900, 580)
	if err := tt.RightClick("fixture"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("重命名…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	snapshot(t, tt, "manager-inline-rename.png")
	if a.managerRename == nil {
		t.Fatal("no inline editor")
	}
	a.managerRename.name = "cancelled"
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.managerRename != nil || a.config.Profiles[0].Name != "fixture" {
		t.Fatal("escape saved rename")
	}
}
func TestRevealedPasswordKeepsUnicodeAndSelection(t *testing.T) {
	value := "秘密🔐-é"
	show := false
	tt := ui.NewTester(func(c *ui.Context) { revealPasswordInput(c, &value, &show).AutoFocus() }, 600, 90)
	tt.Key(ui.Cmd, ui.KeyA)
	if err := tt.Click("显示密码"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !show || value != "秘密🔐-é" {
		t.Fatal("toggle changed text")
	}
	tt.Type("replacement")
	if value != "replacement" {
		t.Fatal("toggle lost selection", value)
	}
}
func TestTransferRateInput(t *testing.T) {
	for _, tc := range []struct {
		s, u string
		want int64
	}{{"0", "KiB/s", 0}, {"1.5", "MiB/s", 1572864}, {"1024", "MiB/s", 1 << 30}} {
		n, err := parseTransferRate(tc.s, tc.u)
		if err != nil || n != tc.want {
			t.Fatal(tc, n, err)
		}
	}
	for _, s := range []string{"NaN", "Inf", "-1", "0.1", "1048577", "bad"} {
		if _, err := parseTransferRate(s, "KiB/s"); err == nil {
			t.Fatal("accepted", s)
		}
	}
}

func TestTabInsertionAfterAndAcrossGroupsPreservesLiveModels(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.ensureGroups()
	left := a.groups.root
	right := groupLeaf("right")
	a.groups.root = &groupNode{first: left, second: right}
	p, q, r := &pane{id: "p"}, &pane{id: "q"}, &pane{id: "r"}
	first := &tab{id: "first", group: left.group.id, active: p, root: &node{pane: p}}
	second := &tab{id: "second", group: left.group.id, active: q, root: &node{pane: q}}
	third := &tab{id: "third", group: right.group.id, active: r, root: &node{pane: r}}
	a.tabs = []*tab{first, second, third}
	a.setCurrentTab(first, false)
	a.moveTabRelative("first", "second", true)
	if a.tabs[1] != first || a.activePane() != p {
		t.Fatal("after insertion lost active terminal")
	}
	a.moveTabRelative("first", "third", true)
	if a.tabs[2] != first || first.group != third.group || a.activePane() != p || first.root.pane != p {
		t.Fatal("cross-group insertion replaced state")
	}
}
func TestManagerRenameBlurDoesNotSave(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name = "fixture"
	p.Host = "fixture.invalid"
	p.Group = ""
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	a.beginManagerRename(sessionRow{id: p.ID, name: p.Name, profile: p})
	tt := ui.NewTester(a.managerView, 900, 580)
	tt.Frame()
	a.managerRename.name = "cancelled"
	if err := tt.Click("新建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.config.Profiles[0].Name != "fixture" {
		t.Fatal("blur saved name")
	}
}

func TestFileRateControlsCompact(t *testing.T) {
	f := &fileState{rateChoice: "自定义", rateCustom: "1.5", rateUnit: "MiB/s"}
	tt := ui.NewTester(func(c *ui.Context) { ui.Column(c).Width(330).Children(func() { fileRateControls(c, f) }) }, 340, 150)
	snapshot(t, tt, "file-rate-compact.png")
	if err := tt.Click("应用限速"); err != nil {
		t.Fatal(err)
	}
	if f.limiter.Rate() != 1572864 {
		t.Fatal("rate not applied")
	}
}

func TestManagerF2EnterRenamesWithoutConnecting(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name = "fixture"
	p.Host = "fixture.invalid"
	p.Group = ""
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	tt := ui.NewTester(a.managerView, 900, 580)
	if err := tt.Click("fixture"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyF2)
	tt.Frame()
	if a.managerRename == nil {
		t.Fatal("F2 did not start editing")
	}
	tt.Key(ui.Super, ui.KeyA)
	tt.Type("renamed")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.managerRename != nil || a.config.Profiles[0].Name != "renamed" || a.authOpen || len(a.tabs) > 0 {
		t.Fatalf("Enter: rename=%+v name=%s auth=%v tabs=%d status=%s", a.managerRename, a.config.Profiles[0].Name, a.authOpen, len(a.tabs), a.status)
	}
}
