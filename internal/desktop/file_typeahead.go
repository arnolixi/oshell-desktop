package desktop

import (
	"github.com/egoist/mygo/ui"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"path"
	"strings"
	"time"
	"unicode"
)

func (f *fileState) findTypedFile(text string, current int, now time.Time) int {
	if text == "" || strings.ContainsFunc(text, unicode.IsControl) {
		return -1
	}
	fold := cases.Fold()
	text = fold.String(norm.NFC.String(text))
	fresh := now.Sub(f.typeTime) > time.Second || f.typeQuery == ""
	repeated := !fresh && f.typeQuery == text
	if fresh || repeated {
		f.typeQuery = text
	} else {
		f.typeQuery += text
	}
	f.typeTime = now
	entries := f.visibleEntries()
	if len(entries) == 0 {
		return -1
	}
	start := current
	if fresh || repeated {
		start++
	}
	if start < 0 || start >= len(entries) {
		start = 0
	}
	for offset := 0; offset < len(entries); offset++ {
		index := (start + offset) % len(entries)
		if strings.HasPrefix(fold.String(norm.NFC.String(entries[index].Name())), f.typeQuery) {
			return index
		}
	}
	return -1
}
func (a *App) fileNavigationInput(p *pane, current int, event ui.InputEvent) bool {
	f := p.files
	if f == nil || f.busy {
		return false
	}
	if event.Kind == ui.InputKeyDown && ((event.Key == ui.KeyUp && event.Mods == ui.Cmd) || (event.Key == ui.KeyBackspace && event.Mods == 0)) {
		f.typeQuery = ""
		a.loadFiles(p, path.Dir(f.dir))
		return true
	}
	if event.Kind == ui.InputText && event.Mods&(ui.Cmd|ui.Ctrl) == 0 {
		if next := f.findTypedFile(event.Text, current, time.Now()); next >= 0 {
			f.selectEntry(next, false, false)
			f.focusRow = f.visibleEntries()[next].Name()
			f.list.ScrollIntoView(next)
		}
		return true
	}
	return false
}

func (f *fileState) filePageIndex(current, direction int) int {
	count := len(f.visibleEntries())
	if count == 0 {
		return -1
	}
	return min(count-1, max(0, current+direction*max(1, f.pageRows)))
}
