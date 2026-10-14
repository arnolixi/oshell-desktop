package desktop

import "github.com/egoist/mygo/ui"

// The pinned SearchField hardcodes English child labels; keep this control
// localized while preserving clear/Escape behavior and the input's focus.
func directoryFilter(c *ui.Context, query *string) bool {
	changed := false
	ui.Row(c).Grow(1).MinWidth(80).MaxWidth(320).Gap(4).Children(func() {
		input := ui.TextInput(c.Key("query"), query).Label("筛选当前目录").Placeholder("筛选当前目录").Grow(1).MinWidth(50)
		changed = input.Changed()
		if *query != "" {
			if ui.Button(c.Key("clear"), "×").Label("清除筛选").Tooltip("清除筛选").Clicked() || input.Shortcut(0, ui.KeyEscape) {
				*query = ""
				changed = true
				input.Focus()
			}
		}
	})
	return changed
}
