// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"fmt"
	"github.com/dlclark/regexp2"
	"regexp"
	"sync"
	"time"
)

// Common patterns stay on Go's linear-time engine. Lookarounds and backreferences
// use a bounded engine; neither renderer nor search runs unbounded backtracking.
type nativeMatcher interface {
	FindAllStringIndex(string, int) ([][]int, error)
	Close()
}
type Pattern struct {
	native   nativeMatcher
	linear   *regexp.Regexp
	advanced *regexp2.Regexp
}

var regexClock sync.Once

func CompilePattern(source string, budget time.Duration) (*Pattern, error) {
	if len(source) > 65536 {
		return nil, fmt.Errorf("正则表达式超过 64 KiB")
	}
	if native, supported, e := compileNativePattern(source, budget); supported {
		if e != nil {
			return nil, e
		}
		return &Pattern{native: native}, nil
	}
	if linear, e := regexp.Compile(source); e == nil {
		return &Pattern{linear: linear}, nil
	}
	regexClock.Do(func() { regexp2.SetTimeoutCheckPeriod(time.Millisecond) })
	advanced, e := regexp2.Compile(source, regexp2.None)
	if e != nil {
		return nil, fmt.Errorf("正则表达式无效：%v", e)
	}
	if budget <= 0 {
		budget = 2 * time.Millisecond
	}
	advanced.MatchTimeout = budget
	return &Pattern{advanced: advanced}, nil
}
func (p *Pattern) FindAllStringIndex(text string, limit int) ([][]int, error) {
	if p.native != nil {
		return p.native.FindAllStringIndex(text, limit)
	}
	if limit == 0 {
		return nil, nil
	}
	if p.linear != nil {
		return p.linear.FindAllStringIndex(text, limit), nil
	}
	var offsets []int
	for at := range text {
		offsets = append(offsets, at)
	}
	offsets = append(offsets, len(text))
	match, e := p.advanced.FindStringMatch(text)
	var out [][]int
	for match != nil && e == nil && (limit < 0 || len(out) < limit) {
		if match.Index >= 0 && match.Index+match.Length < len(offsets) {
			out = append(out, []int{offsets[match.Index], offsets[match.Index+match.Length]})
		}
		if limit >= 0 && len(out) >= limit {
			break
		}
		match, e = p.advanced.FindNextMatch(match)
	}
	if e != nil {
		return nil, fmt.Errorf("正则匹配超时，请简化表达式或缩小搜索范围")
	}
	return out, nil
}

func (p *Pattern) Close() {
	if p != nil && p.native != nil {
		p.native.Close()
	}
}
