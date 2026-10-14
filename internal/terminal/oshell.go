// OShell extensions to the MIT-licensed mygo terminal adapter.
package terminal

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal/internal/vt"
)

// ReplayUserInput applies a semantic event through this terminal's own encoder.
// Device replies, mouse reports and IME preedit are never synchronized.
// Call on the UI thread, as with View.
func (t *Terminal) ReplayUserInput(ev ui.InputEvent) {
	if !t.ManagedInputAvailable() {
		return
	}
	if ev.Kind != ui.InputKeyDown && ev.Kind != ui.InputKeyUp && ev.Kind != ui.InputText {
		return
	}
	if t.v == nil {
		t.v = &view{t: t, blinkStart: time.Now()}
		t.v.shaped.init(4096)
	}
	t.v.input(ev)
}
func (t *Terminal) RequestPaste(text string) {
	if text == "" || !t.Available() {
		return
	}
	if t.opts.BeforePaste != nil && !t.opts.BeforePaste(text) {
		return
	}
	t.ConfirmedPaste(text)
}
func (t *Terminal) ConfirmedPaste(text string) {
	if !t.paste(text) {
		return
	}
	if t.opts.OnUserPaste != nil {
		t.opts.OnUserPaste(text)
	}
}
func (t *Terminal) cleanCopy(s string) string {
	if t.opts.TrimCopyLeading {
		s = strings.TrimLeftFunc(s, unicode.IsSpace)
	}
	if t.opts.TrimCopyTrailing {
		s = strings.TrimRightFunc(s, unicode.IsSpace)
	}
	return s
}
func (t *Terminal) SetInputPolicy(copyOnSelect, rightPaste, trimLeading, trimTrailing bool) {
	t.opts.CopyOnSelect = copyOnSelect
	t.opts.RightClickPaste = rightPaste
	t.opts.TrimCopyLeading = trimLeading
	t.opts.TrimCopyTrailing = trimTrailing
}

func (t *Terminal) observeOutput(b []byte) {
	t.logMu.Lock()
	if t.logFile != nil {
		if _, err := t.logFile.Write(b); err != nil {
			t.logError = err
			t.logFile.Close()
			t.logFile = nil
		}
	}
	t.logMu.Unlock()
	if t.opts.OnOutput != nil {
		t.opts.OnOutput(b)
	}
}
func (t *Terminal) StartLog(path string) error {
	t.logMu.Lock()
	defer t.logMu.Unlock()
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	if t.logFile != nil {
		t.logFile.Close()
	}
	t.logFile = f
	t.logPath = path
	t.logError = nil
	return nil
}
func (t *Terminal) StopLog() {
	t.logMu.Lock()
	defer t.logMu.Unlock()
	if t.logFile != nil {
		t.logFile.Close()
		t.logFile = nil
	}
}
func (t *Terminal) LogState() (string, bool, error) {
	t.logMu.Lock()
	defer t.logMu.Unlock()
	return t.logPath, t.logFile != nil, t.logError
}

type Match struct {
	Row, Start, End int
	Text            string
	Revision        uint64
}

func (t *Terminal) Search(query string, regex, caseSensitive bool) ([]Match, error) {
	if query == "" {
		return nil, nil
	}
	pattern := query
	if !regex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}
	compiled, e := core.CompilePattern(pattern, 10*time.Millisecond)
	if e != nil {
		return nil, e
	}
	defer compiled.Close()
	t.mu.Lock()
	if t.term == nil {
		t.mu.Unlock()
		return nil, nil
	}
	first := t.term.HistoryStart(t.historyLines)
	text := t.term.FormattedTextFrom(first, false)
	revision := t.revision
	t.mu.Unlock()
	var results []Match
	row := first
	for line := range strings.SplitSeq(text, "\n") {
		previousByte, previousRune := 0, 0
		matches, e := compiled.FindAllStringIndex(line, 5000-len(results))
		if e != nil {
			return nil, e
		}
		for _, m := range matches {
			if m[0] == m[1] {
				continue
			}
			start := previousRune + utf8.RuneCountInString(line[previousByte:m[0]])
			end := start + utf8.RuneCountInString(line[m[0]:m[1]])
			results = append(results, Match{row, start, end, line, revision})
			previousByte, previousRune = m[1], end
			if len(results) == 5000 {
				return results, nil
			}
		}
		row++
	}
	return results, nil
}
func (t *Terminal) Reveal(match Match) error {
	t.mu.Lock()
	err := func() error {
		if t.term == nil {
			return fmt.Errorf("终端已关闭")
		}
		if match.Revision != t.revision {
			return fmt.Errorf("终端输出已变化，请刷新匹配")
		}
		t.term.ScrollToRow(match.Row)
		offset := int(t.term.Scrollbar().Offset)
		row := match.Row - offset
		text, columns := t.term.RowText(row)
		if match.Start < 0 || match.End <= match.Start || match.End > len(text) {
			return fmt.Errorf("匹配位置已失效")
		}
		if !t.term.SelectCells(columns[match.Start], row, columns[match.End-1], row) {
			return fmt.Errorf("无法定位匹配")
		}
		return nil
	}()
	t.mu.Unlock()
	if err == nil {
		t.redraw()
	}
	return err
}

type HighlightRule struct {
	Pattern                    string
	Regex, CaseSensitive, Bold bool
	Foreground, Background     ui.Color
}
type compiledHighlight struct {
	pattern *core.Pattern
	rule    HighlightRule
}

func (t *Terminal) SetHighlights(rules []HighlightRule) error {
	compiled := make([]compiledHighlight, 0, len(rules))
	if len(rules) > 32 {
		return fmt.Errorf("突出显示规则超过 32 条")
	}
	for _, r := range rules {
		if r.Pattern == "" {
			continue
		}
		p := r.Pattern
		if !r.Regex {
			p = regexp.QuoteMeta(p)
		}
		if !r.CaseSensitive {
			p = "(?i)" + p
		}
		re, e := core.CompilePattern(p, 2*time.Millisecond)
		if e != nil {
			return e
		}
		compiled = append(compiled, compiledHighlight{re, r})
	}
	t.mu.Lock()
	old := t.highlights
	t.highlights = compiled
	t.highlightVersion++
	t.mu.Unlock()
	for _, rule := range old {
		rule.pattern.Close()
	}
	t.redraw()
	return nil
}
func (v *view) rowHighlights(rs *vt.RenderState, cells []vt.Cell) map[int]HighlightRule {
	if len(v.rules) == 0 {
		return nil
	}
	var text []rune
	var columns []int
	var buf []rune
	for x, cell := range cells {
		if cell.Wide == vt.SpacerHead || cell.Wide == vt.SpacerTail {
			continue
		}
		runes := []rune{rune(cell.Codepoint)}
		if cell.Codepoint == 0 {
			runes[0] = ' '
		}
		if cell.Grapheme {
			buf = rs.CellGraphemes(x, buf[:0])
			runes = buf
		}
		for _, r := range runes {
			text = append(text, r)
			columns = append(columns, x)
		}
	}
	if len(text) > 4096 {
		return nil
	}
	raw := string(text)
	remaining := 256
	out := map[int]HighlightRule{}
	deadline := time.Now().Add(2 * time.Millisecond)
	for _, h := range v.rules {
		if time.Now().After(deadline) {
			break
		}
		matches, e := h.pattern.FindAllStringIndex(raw, remaining)
		if e != nil {
			break
		}
		for _, m := range matches {
			if m[0] == m[1] {
				continue
			}
			remaining--
			start := utf8.RuneCountInString(raw[:m[0]])
			end := start + utf8.RuneCountInString(raw[m[0]:m[1]])
			for i := start; i < end && i < len(columns); i++ {
				out[columns[i]] = h.rule
			}
			if remaining == 0 {
				return out
			}
		}
	}
	return out
}

func (t *Terminal) SetHistoryLimit(lines int) error {
	if lines < 500 || lines > 100000 {
		return fmt.Errorf("历史行数范围为 500–100000")
	}
	t.mu.Lock()
	if t.term == nil {
		t.mu.Unlock()
		return nil
	}
	if t.historyLines == lines {
		t.mu.Unlock()
		return nil
	}
	err := t.term.SetHistoryLines(lines)
	if err == nil {
		t.historyLines = lines
		t.term.SetSelection(nil)
		t.term.ClampHistory(lines)
		t.revision++
	}
	t.mu.Unlock()
	t.redraw()
	return err
}

// SendManagedText batches a composer dispatch into one input packet and never
// echoes it through the synchronized-keyboard callback. Unlike clipboard paste,
// reviewed composer text is sent as command lines even in bracketed-paste mode.
func (t *Terminal) SendManagedText(text string, appendReturn bool) {
	if !t.Available() {
		return
	}
	data := core.ManagedInput(text, appendReturn)
	t.mu.Lock()
	if t.term == nil {
		t.mu.Unlock()
		return
	}
	t.term.ScrollToBottom()
	t.mu.Unlock()
	t.Send(data)
	t.redraw()
}

func (t *Terminal) ManagedInputAvailable() bool {
	if f := t.fallback.Load(); f != nil && !f.ManagedInputAllowed() {
		return false
	}
	return t.Available() && (t.opts.ManagedInputAllowed == nil || t.opts.ManagedInputAllowed())
}

// Edit routes the native Edit menu through this view, on the UI thread.
func (t *Terminal) Edit(command string) {
	if t.v != nil {
		t.v.input(ui.InputEvent{Kind: ui.InputCommand, Text: command})
	}
}

func (t *Terminal) SetVisible(visible bool) {
	was := t.hidden.Swap(!visible)
	if !visible {
		t.stopScrollbarFade()
	}
	if !visible && !was {
		t.visibilityEpoch.Add(1)
	}
	if visible && was {
		t.redraw()
	}
}

// HostPrompt reads the active screen rather than the scrolled viewport.
func (t *Terminal) HostPrompt() (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil || t.term.AltScreen() {
		return "", false
	}
	return t.term.CursorLine(), true
}
