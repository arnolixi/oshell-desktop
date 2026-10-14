package desktop

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal"
	"strings"
	"testing"
	"time"
)

func TestDisconnectedBannerPreservesTerminalInputFocus(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	profile := core.NewProfile()
	profile.Host = "fixture"
	profile.Username = "tester"
	p := &pane{id: core.NewID(), profile: profile, hostname: "fixture", status: "已连接"}
	p.owner.Store(a)
	conn := &operatorConn{testConn: testConn{done: make(chan struct{})}}
	term, e := terminal.New(terminal.Options{Conn: conn, LocalToolsOnExit: true, NoBlink: true,
		OnSessionEnd: func() { a.post(func() { p.status = "已断开 · 本机工具模式" }) },
		OnExit: func(int) {
			a.post(func() {
				if owner := a.ownerTab(p); owner != nil {
					a.closeEndedPane(owner, p)
				}
			})
		},
	})
	if e != nil {
		t.Fatal(e)
	}
	defer term.Close()
	p.term = term
	a.tabs = []*tab{{id: core.NewID(), name: "fixture", active: p, root: &node{id: core.NewID(), pane: p}}}
	a.current = 0
	tt := ui.NewTester(a.view, 1000, 700)
	if e = tt.Click("Terminal"); e != nil {
		t.Fatal(e)
	}
	conn.Close()
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for !condition() && time.Now().Before(deadline) {
			tt.Frame()
			time.Sleep(time.Millisecond)
		}
		if !condition() {
			t.Fatal("condition timed out", tt.Texts())
		}
	}
	wait(func() bool {
		return strings.HasPrefix(p.status, "已断开") && strings.Contains(term.Text(), "oshell>")
	})
	tt.Frame()
	tt.Type("exit")
	tt.Key(0, ui.KeyEnter)
	wait(func() bool { return len(a.tabs) == 0 })
}

func TestEndedSessionClosePreservesCommandEditorFocus(t *testing.T) {
	for _, editor := range []string{"quick", "composer"} {
		for _, split := range []bool{false, true} {
			t.Run(editor+fmt.Sprint(split), func(t *testing.T) {
				a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
				first, left := operatorPane(t, a, "first")
				second, right := operatorPane(t, a, "second")
				a.current = 1
				if split {
					tab := a.tabs[0]
					tab.root = &node{id: core.NewID(), first: tab.root, second: a.tabs[1].root, size: 300}
					tab.active = second
					a.tabs = a.tabs[:1]
					a.current = 0
				}
				label := "快捷命令"
				if editor == "composer" {
					a.openComposer("", true)
					label = "撰写命令内容"
				}
				tt := ui.NewTester(a.view, 1100, 900)
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Type("retained")
				a.closeEndedPane(a.ownerTab(second), second)
				tt.Frame()
				tt.Type("-focus")
				value := a.quick
				if editor == "composer" {
					value = a.features.operators.composerText
				}
				if value != "retained-focus" {
					t.Fatal("command editor focus lost", value)
				}
				if first.closed || left.text() != "" || right.text() != "" {
					t.Fatal("typing went to terminal or closed wrong pane")
				}
			})
		}
	}
}
