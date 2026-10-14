package desktop

import (
	"github.com/egoist/mygo"
	"oshell-desktop/internal/core"
	"strings"
)

// Folder buttons open on click/release rather than press, so the same surface
// can initiate a root-order drag without a native menu stealing the pointer.
func (a *App) nativeLinkFolderMenu(folder string) *mygo.Menu {
	return mygo.NewMenu(a.nativeLinkFolderItems(folder))
}
func (a *App) nativeLinkFolderItems(folder string) []*mygo.MenuItem {
	full := "Links"
	if folder != "" {
		full += "/" + folder
	}
	items := []*mygo.MenuItem{}
	for _, directory := range a.config.AllDirectories() {
		if rest, ok := strings.CutPrefix(directory, full+"/"); ok && rest != "" && !strings.Contains(rest, "/") {
			nested := strings.TrimPrefix(directory, "Links/")
			items = append(items, &mygo.MenuItem{Label: rest, Submenu: a.nativeLinkFolderItems(nested)})
		}
	}
	for _, link := range a.config.Links {
		if link.Folder == folder {
			if _, ok := a.profileByID(link.ProfileID); ok {
				items = append(items, &mygo.MenuItem{Label: link.Name, Click: func(*mygo.MenuItem, *mygo.Window) {
					if current, ok := a.currentLinkProfile(link.ID); ok {
						a.request(current, 0)
					}
					if a.win != nil {
						a.win.Invalidate()
					}
				}})
			}
		}
	}
	for _, profile := range a.config.Profiles {
		if profile.Group == full {
			items = append(items, &mygo.MenuItem{Label: profile.Name, Click: func(*mygo.MenuItem, *mygo.Window) {
				if current, ok := a.profileByID(profile.ID); ok {
					a.request(current, 0)
				}
				if a.win != nil {
					a.win.Invalidate()
				}
			}})
		}
	}
	items = append(items, mygo.Separator(), &mygo.MenuItem{Label: "管理此快捷目录…", Click: func(*mygo.MenuItem, *mygo.Window) {
		a.managerGroup = full
		a.managerAll = false
		a.query = ""
		a.openManager()
	}})
	return items
}

func linkDropDestination(source string, target core.LinkRootItem, index int, items []core.LinkRootItem, fraction float32) (before, folder string) {
	if !strings.HasPrefix(source, "folder:") && target.Kind == "folder" && fraction > 0.2 && fraction < 0.8 {
		return "", target.Folder
	}
	if fraction < 0.5 {
		return target.Key(), ""
	}
	if index+1 < len(items) {
		return items[index+1].Key(), ""
	}
	return "", ""
}
func (a *App) moveLinkIntoFolder(source, folder string) {
	a.changeConfig(func(config *core.Config) error { _, e := config.MoveLinkRootIntoFolder(source, folder); return e })
}

func (a *App) currentLinkProfile(id string) (core.Profile, bool) {
	for _, link := range a.config.Links {
		if link.ID == id {
			return a.profileByID(link.ProfileID)
		}
	}
	return core.Profile{}, false
}
