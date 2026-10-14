// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type SearchMatch struct {
	Line                 int
	Text                 string
	Start, End           int
	Before, Match, After string
}

func Search(text, query string, regex, caseSensitive bool) ([]SearchMatch, bool, error) {
	if query == "" {
		return nil, false, nil
	}
	pattern := query
	if !regex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}
	r, e := CompilePattern(pattern, 10*time.Millisecond)
	if e != nil {
		return nil, false, e
	}
	defer r.Close()
	var out []SearchMatch
	line := 0
	for s := range strings.SplitSeq(text, "\n") {
		line++
		previousByte, previousRune := 0, 0
		matches, e := r.FindAllStringIndex(s, 5001-len(out))
		if e != nil {
			return nil, false, e
		}
		for _, m := range matches {
			if m[0] == m[1] {
				continue
			}
			if len(out) == 5000 {
				return out, true, nil
			}
			start := previousRune + utf8.RuneCountInString(s[previousByte:m[0]])
			end := start + utf8.RuneCountInString(s[m[0]:m[1]])
			previousByte, previousRune = m[1], end
			lo, hi := max(0, m[0]-400), min(len(s), m[1]+600)
			for lo < m[0] && !utf8.RuneStart(s[lo]) {
				lo++
			}
			for hi < len(s) && hi > m[1] && !utf8.RuneStart(s[hi]) {
				hi--
			}
			cut := min(m[1], m[0]+768)
			for cut > m[0] && cut < len(s) && !utf8.RuneStart(s[cut]) {
				cut--
			}
			match := s[m[0]:cut]
			if cut < m[1] {
				match += "…"
			}
			out = append(out, SearchMatch{Line: line, Text: s, Start: start, End: end, Before: s[lo:m[0]], Match: match, After: s[m[1]:hi]})
		}
	}
	return out, false, nil
}
