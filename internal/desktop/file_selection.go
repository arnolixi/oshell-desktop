package desktop

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"os"
	"strings"
)

func (f *fileState) isSelected(name string) bool {
	if f.selection == nil {
		return f.selected == name
	}
	return f.selection[name]
}
func (f *fileState) selectedNames() []string {
	entries := f.visibleEntries()
	var names []string
	for _, entry := range entries {
		if f.isSelected(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	return names
}
func (f *fileState) selectEntry(index int, extend, toggle bool) {
	entries := f.visibleEntries()
	if index < 0 || index >= len(entries) {
		return
	}
	name := entries[index].Name()
	if f.selection == nil {
		f.selection = map[string]bool{}
		if f.selected != "" {
			f.selection[f.selected] = true
		}
	}
	if extend {
		anchor := index
		for i, entry := range entries {
			if entry.Name() == f.selectionAnchor {
				anchor = i
				break
			}
		}
		if !toggle {
			f.selection = map[string]bool{}
		}
		for i := min(index, anchor); i <= max(index, anchor); i++ {
			f.selection[entries[i].Name()] = true
		}
	} else if toggle {
		if f.selection[name] {
			delete(f.selection, name)
		} else {
			f.selection[name] = true
		}
		f.selectionAnchor = name
	} else {
		f.selection = map[string]bool{name: true}
		f.selectionAnchor = name
	}
	f.selected = ""
	if f.selection[name] {
		f.selected = name
	} else if names := f.selectedNames(); len(names) > 0 {
		f.selected = names[0]
	}
}
func (f *fileState) selectAll() {
	entries := f.visibleEntries()
	f.selected = ""
	f.selectionAnchor = ""
	f.selection = map[string]bool{}
	for _, entry := range entries {
		f.selection[entry.Name()] = true
	}
	if len(entries) > 0 {
		f.selected = entries[0].Name()
		f.selectionAnchor = f.selected
	}
}

// Filter only changes on input or a directory response; cached rows keep idle
// rendering and selection checks from repeatedly case-folding every filename.
func (f *fileState) visibleEntries() []os.FileInfo {
	if f.filter == "" {
		f.visibleCache = nil
		f.filterReady = false
		return f.entries
	}
	if f.filterReady && f.visibleFilter == f.filter {
		return f.visibleCache
	}
	fold := cases.Fold()
	query := fold.String(norm.NFC.String(f.filter))
	f.visibleCache = nil
	for _, entry := range f.entries {
		if strings.Contains(fold.String(norm.NFC.String(entry.Name())), query) {
			f.visibleCache = append(f.visibleCache, entry)
		}
	}
	f.visibleFilter = f.filter
	f.filterReady = true
	return f.visibleCache
}
