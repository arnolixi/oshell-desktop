package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestQuickConnectionMenusShareNestedFilteredTree(t *testing.T) {
	makeProfile := func(name, group string, visible bool) core.Profile {
		p := core.NewProfile()
		p.Name = name
		p.Group = group
		p.Host = "fixture.invalid"
		p.QuickConnect = visible
		return p
	}
	profiles := []core.Profile{makeProfile("node10", "生产/数据库", true), makeProfile("node2", "生产/数据库", true), makeProfile("root", "", true), makeProfile("hidden", "不显示/子目录", false)}
	tree := quickConnectionTree(profiles)
	if len(tree) != 2 || tree[0].label != "生产" || tree[1].id != profiles[2].ID {
		t.Fatal("root/folder hierarchy incorrect")
	}
	folder := tree[0].children
	if len(folder) != 1 || folder[0].label != "数据库" || len(folder[0].children) != 2 {
		t.Fatal("nested folder missing")
	}
	if folder[0].children[0].id != profiles[1].ID {
		t.Fatal("numeric ordering missing")
	}
	cfg := core.DefaultConfig()
	cfg.Profiles = profiles
	hub := &applicationHub{config: cfg}
	menu := hub.nativeQuickConnectionMenu()
	if menu.Label != "快捷连接" || menu.Submenu[0].Submenu[0].Submenu[0].Label != "node2 · SSH" {
		t.Fatal("native menu differs from tree")
	}
	if menu.Submenu[len(menu.Submenu)-1].Label != "管理会话…" {
		t.Fatal("manager action missing")
	}
}
func TestStaleQuickConnectionCannotConnectDeletedOrDisabledProfile(t *testing.T) {
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "fixture.invalid"
	p.QuickConnect = false
	cfg.Profiles = []core.Profile{p}
	a := newApp(&core.Store{Dir: t.TempDir()}, cfg)
	for _, id := range []string{p.ID, "deleted"} {
		a.connectQuickID(id)
		if a.authOpen || len(a.tabs) != 0 || a.status != "快捷会话已被删除或停用，请重新选择" {
			t.Fatal("stale menu opened connection")
		}
	}
}
