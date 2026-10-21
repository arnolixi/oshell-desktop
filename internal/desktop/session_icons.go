package desktop

import (
	"path"
	"strings"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

// Presentation only: shared session types and configuration stay unchanged.
var (
	sessionSSHIcon    = glyph(`<rect x="2" y="2" width="16" height="16" rx="2"/><path d="M2 13h16M5 6l3 2-3 2m6 0h3"/><circle cx="5.5" cy="15.5" r=".6"/><circle cx="8.5" cy="15.5" r=".6"/>`)
	sessionLocalIcon  = glyph(`<rect x="4" y="3" width="12" height="10" rx="1.5"/><path d="m4 13-2 4h16l-2-4"/>`)
	sessionFolderIcon = glyph(`<path d="M2 17V3h6l2 2h8v12z"/>`)
	sessionSFTPIcon   = glyph(`<path d="M2 17V3h6l2 2h8v12z"/><rect x="6" y="10" width="8" height="5" rx="1"/><path d="M7.5 10V9a2.5 2.5 0 0 1 5 0v1m-2.5 2v1"/>`)
	sessionFTPIcon    = glyph(`<path d="M2 17V3h6l2 2h8v12zM6 9h8l-2-2m2 2-2 2M14 13H6l2-2m-2 2 2 2"/>`)
	sessionParentIcon = glyph(`<path d="M16 15V9H4m4-4L4 9l4 4"/>`)
)

type sessionIconStyle struct {
	image              *ui.SVG
	label, light, dark string
}

func sessionIconFor(kind string, shortcut, folder, parent bool) sessionIconStyle {
	if parent {
		return sessionIconStyle{sessionParentIcon, "上一级目录", "#666B75", "#B3B8C2"}
	}
	if folder {
		return sessionIconStyle{sessionFolderIcon, "目录", "#9C6E1F", "#E8BA5C"}
	}
	style := sessionIconStyle{sessionSSHIcon, "SSH · 远程终端", "#1F66BF", "#66B0FF"}
	switch kind {
	case "local":
		style = sessionIconStyle{sessionLocalIcon, "本地终端", "#8252B8", "#C29CF7"}
	case "sftp":
		style = sessionIconStyle{sessionSFTPIcon, "SFTP · 安全文件传输", "#1F8257", "#63C99C"}
	case "ftp":
		style = sessionIconStyle{sessionFTPIcon, "FTP · 文件传输", "#B85C1A", "#FFA861"}
	}
	if shortcut {
		style.image, style.light, style.dark = linkIcon, "#057D91", "#5CCFE0"
		style.label = "Link · 快捷引用 · " + style.label
	}
	return style
}

func (s sessionIconStyle) draw(c *ui.Context, selected bool) {
	color := ui.Hex(s.light)
	if c.Theme().Dark {
		color = ui.Hex(s.dark)
	}
	if selected {
		color = c.Theme().AccentText
	}
	ui.Icon(c, s.image).FontSize(20).Size(20, 20).Shrink(0).TextColor(color).Label(s.label)
}

func sessionListTooltip(p core.Profile, shortcut bool) string {
	text := sessionIconFor(p.Kind, shortcut, false, false).label
	if shortcut {
		text += "\n来源：" + path.Join("/", p.Group, p.Name)
	}
	return text + "\n" + core.SessionSearchMetadata(p)
}

func sessionTreeItem(c *ui.Context, name string, open *bool, children func(), selected bool, style sessionIconStyle) ui.Element {
	// Keep the toolkit's arrow, keyboard navigation and tree accessibility;
	// append the shared icon/name presentation after its empty label.
	return ui.TreeItem(c, "", open, children).Label(name).Selected(selected).MinWidth(0).Children(func() {
		style.draw(c, selected)
		ui.Text(c, name).SingleLine().Grow(1).MinWidth(0)
	})
}

func sidebarSessionStyle(e sidebarEntry) sessionIconStyle {
	return sessionIconFor(e.profile.Kind, strings.HasPrefix(e.id, "link:"), false, false)
}
