package desktop

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"oshell-desktop/internal/core"
	"sort"
	"strings"
)

type quickConnectionNode struct {
	label, id string
	children  []*quickConnectionNode
}

func quickConnectionTree(profiles []core.Profile) []*quickConnectionNode {
	root := &quickConnectionNode{}
	for _, p := range profiles {
		if !p.QuickConnect {
			continue
		}
		node := root
		for _, part := range strings.Split(strings.Trim(p.Group, "/"), "/") {
			if part == "" {
				continue
			}
			var child *quickConnectionNode
			for _, n := range node.children {
				if n.id == "" && n.label == part {
					child = n
					break
				}
			}
			if child == nil {
				child = &quickConnectionNode{label: part}
				node.children = append(node.children, child)
			}
			node = child
		}
		kind := strings.ToUpper(p.Kind)
		if p.Kind == "local" {
			kind = "本地"
		}
		node.children = append(node.children, &quickConnectionNode{label: p.Name + " · " + kind, id: p.ID})
	}
	order := collate.New(language.Und, collate.Numeric)
	var arrange func(*quickConnectionNode)
	arrange = func(node *quickConnectionNode) {
		sort.SliceStable(node.children, func(i, j int) bool {
			l, r := node.children[i], node.children[j]
			if (l.id == "") != (r.id == "") {
				return l.id == ""
			}
			return order.CompareString(l.label, r.label) < 0
		})
		for _, n := range node.children {
			arrange(n)
		}
	}
	arrange(root)
	return root.children
}
func (a *App) connectQuickID(id string) {
	p, exists := a.profileByID(id)
	if !exists || !p.QuickConnect {
		a.status = "快捷会话已被删除或停用，请重新选择"
		return
	}
	a.request(p, 0)
}
func (a *App) populateQuickConnections(menu *ui.Menu, nodes []*quickConnectionNode) {
	for _, n := range nodes {
		if n.id == "" {
			menu.Submenu(n.label, func(child *ui.Menu) { a.populateQuickConnections(child, n.children) })
		} else if menu.Item(n.label).Chosen() {
			a.connectQuickID(n.id)
		}
	}
}
func (h *applicationHub) nativeQuickConnectionMenu() *mygo.MenuItem {
	var items func([]*quickConnectionNode) []*mygo.MenuItem
	items = func(nodes []*quickConnectionNode) []*mygo.MenuItem {
		var out []*mygo.MenuItem
		for _, n := range nodes {
			if n.id == "" {
				out = append(out, &mygo.MenuItem{Label: n.label, Submenu: items(n.children)})
			} else {
				id := n.id
				out = append(out, h.workspaceMenuItem(n.label, func(a *App) { a.connectQuickID(id) }))
			}
		}
		return out
	}
	children := items(quickConnectionTree(h.config.Profiles))
	if len(children) == 0 {
		children = append(children, &mygo.MenuItem{Label: "暂无快捷会话", Disabled: true})
	}
	children = append(children, mygo.Separator(), h.workspaceMenuItem("管理会话…", func(a *App) { a.openManager() }))
	return &mygo.MenuItem{Label: "快捷连接", Submenu: children}
}
