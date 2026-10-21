// SPDX-License-Identifier: GPL-3.0-only
package desktop

import "github.com/egoist/mygo/ui"

// Small, consistent outline icons; parsed once, shared across all frames.
func glyph(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	folderIcon          = glyph(`<path d="M2.5 5h5l1.5 2h8.5v9h-15z"/><path d="M2.5 5V3.5h5l1.5 2h7v1.5"/>`)
	sidebarIcon         = glyph(`<rect x="2" y="3" width="16" height="14" rx="2"/><path d="M7 3v14M4 6h1m-1 3h1m-1 3h1"/>`)
	collapseSidebarIcon = glyph(`<rect x="2" y="3" width="16" height="14" rx="2"/><path d="M7 3v14m6-10-3 3 3 3"/>`)
	syncIcon            = glyph(`<path d="M16.5 8a6.8 6.8 0 0 0-11.7-3L2.5 7.5m0-4v4h4M3.5 12a6.8 6.8 0 0 0 11.7 3l2.3-2.5m0 4v-4h-4"/>`)
	terminalIcon        = glyph(`<rect x="2" y="3.5" width="16" height="13" rx="2"/><path d="m5 7 3 3-3 3m5 0h4"/>`)
	plusIcon            = glyph(`<path d="M10 4v12M4 10h12"/>`)
	closeIcon           = glyph(`<path d="m6 6 8 8m0-8-8 8"/>`)
	settingsIcon        = glyph(`<path d="M4 5h12M4 10h12M4 15h12"/><rect x="6" y="3" width="3" height="4" rx="1"/><rect x="12" y="8" width="3" height="4" rx="1"/><rect x="7" y="13" width="3" height="4" rx="1"/>`)
	splitIcon           = glyph(`<rect x="2" y="3" width="16" height="14" rx="2"/><path d="M10 3v14m0-7h8"/>`)
	linkIcon            = glyph(`<path d="m8 12 4-4M7 13l-1 1a3 3 0 0 1-4-4l4-4a3 3 0 0 1 4 0m0 8a3 3 0 0 0 4 0l4-4a3 3 0 0 0-4-4l-1 1"/>`)
	chevronIcon         = glyph(`<path d="m6 8 4 4 4-4"/>`)
	arrowUpIcon         = glyph(`<path d="m5 10 5-5 5 5m-5-5v11"/>`)
	searchIcon          = glyph(`<circle cx="8.5" cy="8.5" r="5"/><path d="m12 12 5 5"/>`)
	editIcon            = glyph(`<path d="m12.5 3.5 4 4M3.5 16.5l1-4 9-9a2.1 2.1 0 0 1 3 3l-9 9z"/>`)
)

func toolButton(c *ui.Context, label string, icon *ui.SVG, text string) ui.Element {
	return ui.Button(c, "").Label(label).Tooltip(label).Height(25).Padding(3, 7).Gap(5).Radius(4).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).FontSize(15).TextColor(c.Theme().Accent)
		}
		if text != "" {
			ui.Text(c, text).FontSize(12).SingleLine()
		}
	})
}
func flatButton(c *ui.Context, label string, icon *ui.SVG, text string) ui.Element {
	return toolButton(c, label, icon, text).Background(ui.Transparent).BorderWidth(0)
}

func rowIconButton(c *ui.Context, label string, icon *ui.SVG, foreground ui.Color) ui.Element {
	button := ui.ButtonBase(c).Role(ui.RoleButton).Label(label).Tooltip(label).
		Size(28, 28).Radius(5).Cursor(ui.CursorPointer)
	if button.Pressed() {
		button.Background(foreground.Alpha(.20))
	} else if button.Hovered() || button.FocusVisible() {
		button.Background(foreground.Alpha(.12))
	}
	return button.Children(func() {
		ui.Icon(c, icon).FontSize(16).TextColor(foreground)
	})
}
