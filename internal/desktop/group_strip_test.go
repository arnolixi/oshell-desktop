package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestGroupStripInlineHeaderAndUnreadSelection(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	left, right := groupLeaf(""), groupLeaf("")
	a.groups.root = &groupNode{first: left, second: right}
	a.groups.active = left.group.id
	a.tabs = []*tab{{id: "left", name: "develop", group: left.group.id, active: &pane{}}, {id: "right", name: "develop", group: right.group.id, active: &pane{}, unread: true}}
	left.group.active = "left"
	right.group.active = "right"
	a.current = 0
	tt := ui.NewTester(func(c *ui.Context) {
		theme := ui.DarkTheme()
		c.SetTheme(theme)
		ui.Row(c).Fill().Background(theme.Surface).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() { a.groupTabBar(c, left.group) })
			ui.Box(c).Width(1).FillHeight().Background(theme.Border)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() { a.groupTabBar(c, right.group) })
		})
	}, 1000, 40)
	if !tt.HasText("分组 1 · 1") || !tt.HasText("● 分组 2 · 1") {
		t.Fatal("group count/unread header missing", tt.Texts())
	}
	header, _ := tt.Find("分组 1")
	tabRect, _ := tt.Find("develop")
	if header.Y+header.H/2 != tabRect.Y+tabRect.H/2 {
		t.Fatalf("header must share tab row: %v %v", header, tabRect)
	}
	snapshot(t, tt, "tab-strip-reference.png")
	if err := tt.Click("分组 2"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("切换到此组"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.groups.active != right.group.id || a.tabs[1].unread {
		t.Fatal("group switch failed to acknowledge unread")
	}
	if err := tt.Click("分组 2"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("重命名标签组…"); err != nil {
		t.Fatal(err)
	}
	if !a.groups.dialog || a.groups.editing != right.group.id {
		t.Fatal("header menu targeted wrong group")
	}
}
