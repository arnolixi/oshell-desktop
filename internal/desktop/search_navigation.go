// SPDX-License-Identifier: GPL-3.0-only
package desktop

import (
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal"
)

func (a *App) navigateSearch(direction int) {
	f := &a.features
	p := a.activePane()
	if p == nil || p.term == nil {
		return
	}
	if f.findQuery == "" {
		a.openSearch()
		return
	}
	var previous *terminal.Match
	if f.searchPane == p && f.findIndex >= 0 && f.findIndex < len(f.terminalMatches) {
		v := f.terminalMatches[f.findIndex]
		previous = &v
	}
	f.findGeneration++
	generation := f.findGeneration
	f.findBusy = true
	query, regex, sensitive, term := f.findQuery, f.findRegex, f.findCase, p.term
	go func() {
		matches, err := term.Search(query, regex, sensitive)
		a.post(func() {
			if generation != f.findGeneration {
				return
			}
			f.findBusy = false
			if p.closed || a.activePane() != p {
				return
			}
			if err != nil {
				f.findError = err.Error()
				f.findOpen = true
				return
			}
			f.searchPane = p
			f.terminalMatches = matches
			f.findMatches = searchDisplayMatches(matches)
			f.findLimited = len(matches) == 5000
			f.findIndex = -1
			if len(matches) == 0 {
				f.findError = "没有找到匹配"
				f.findOpen = true
				return
			}
			index := 0
			if direction < 0 {
				index = len(matches) - 1
			}
			if previous != nil {
				if direction > 0 {
					for i, m := range matches {
						if m.Row > previous.Row || (m.Row == previous.Row && m.Start > previous.Start) {
							index = i
							break
						}
					}
				} else {
					for i := len(matches) - 1; i >= 0; i-- {
						m := matches[i]
						if m.Row < previous.Row || (m.Row == previous.Row && m.Start < previous.Start) {
							index = i
							break
						}
					}
				}
			}
			f.findIndex = index
			f.findError = ""
			a.revealSearch()
		})
	}()
}

func searchDisplayMatches(matches []terminal.Match) []core.SearchMatch {
	out := make([]core.SearchMatch, 0, len(matches))
	for _, m := range matches {
		r := []rune(m.Text)
		lo, hi := max(0, m.Start-100), min(len(r), m.End+160)
		out = append(out, core.SearchMatch{Line: m.Row + 1, Text: m.Text, Start: m.Start, End: m.End, Before: string(r[lo:m.Start]), Match: string(r[m.Start:m.End]), After: string(r[m.End:hi])})
	}
	return out
}
