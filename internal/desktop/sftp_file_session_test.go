package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
	"time"
)

func TestSavedSFTPConnectsAndManagesDirectories(t *testing.T) {
	a, p, accepted := savedConnectionFixture(t)
	p.Kind = "sftp"
	p.InitialDirectory = "/"
	a.config.Profiles[0] = p
	if err := a.vault.PutFor(p, "saved-fixture-password", p.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(p, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.tabs[0].active.files != nil && !a.tabs[0].active.files.busy && len(a.tabs[0].active.files.entries) == 1
	})
	pane := a.tabs[0].active
	keyboard := ui.NewTester(a.fileWindowView, 1080, 720)
	if err := keyboard.Click("fixture-folder"); err != nil {
		t.Fatal(err)
	}
	keyboard.Key(0, ui.KeyEnter)
	waitOperator(t, a, func() bool { return !pane.files.busy && pane.files.dir == "/fixture-folder" })
	a.loadFiles(pane, "/")
	waitOperator(t, a, func() bool { return !pane.files.busy && pane.files.dir == "/" })
	if a.authOpen || len(a.challenges) != 0 || pane.term != nil || pane.files.entries[0].Name() != "fixture-folder" || accepted.Load() != 1 {
		t.Fatal("saved file session did not enter SFTP directly")
	}
	for _, step := range []struct{ action, name string }{{"新建远程目录", "native-created"}, {"重命名远程项目", "native-renamed"}} {
		a.openFileAction(pane, step.action)
		a.features.fileName = step.name
		tt := ui.NewTester(a.fileWindowView, 1080, 720)
		if err := tt.Click("应用"); err != nil {
			t.Fatal(err)
		}
		waitOperator(t, a, func() bool {
			if pane.files.busy {
				return false
			}
			for _, entry := range pane.files.entries {
				if entry.Name() == step.name {
					return true
				}
			}
			return false
		})
		pane.files.selected = step.name
	}
	a.loadFiles(pane, "/native-renamed")
	waitOperator(t, a, func() bool { return !pane.files.busy && pane.files.dir == "/native-renamed" })
	if len(pane.files.entries) != 0 {
		t.Fatal("new directory not empty")
	}
	if err := pane.conn.Client.Close(); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool {
		return pane.status == "文件连接已断开" && pane.files.status == "文件连接已断开"
	})
	a.status = "文件操作完成"
	pane.files.status = "connection lost"
	tt := ui.NewTester(a.fileWindowView, 1080, 720)
	if tt.HasText("文件操作完成") {
		t.Fatal("stale success footer survived disconnect")
	}
	if !tt.HasText("文件连接已断开") {
		t.Fatal("file disconnect not visible")
	}
	if err := tt.Click("重新连接"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool {
		return a.activePane() != pane && a.activePane().files != nil && !a.activePane().files.busy && a.activePane().files.client != nil
	})
	if len(a.tabs) != 1 || accepted.Load() != 2 || a.activePane().files.dir != "/native-renamed" {
		t.Fatal("explicit reconnect lost file tab or directory")
	}
	// A deliberate new-session action may establish a fresh authentication.
	a.request(p, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 2 && a.activePane() != pane && a.activePane().files != nil && !a.activePane().files.busy && len(a.activePane().files.entries) == 2
	})
	if accepted.Load() != 3 {
		t.Fatal("explicit new session did not authenticate")
	}

}

func TestLateFileDisconnectCannotChangeReplacementConnection(t *testing.T) {
	a, p, _ := savedConnectionFixture(t)
	p.Kind = "sftp"
	p.InitialDirectory = "/"
	a.config.Profiles[0] = p
	if err := a.vault.PutFor(p, "saved-fixture-password", p.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(p, 0)
	waitOperator(t, a, func() bool { return len(a.tabs) == 1 && a.tabs[0].active.files != nil && !a.tabs[0].active.files.busy })
	current := a.tabs[0].active
	old := current.conn
	a.drain()
	old.Client.Close()
	deadline := time.Now().Add(time.Second)
	queued := false
	for time.Now().Before(deadline) {
		a.eventMu.Lock()
		queued = len(a.events) > 0
		a.eventMu.Unlock()
		if queued {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !queued {
		t.Fatal("disconnect event not queued")
	}
	current.conn = &core.Connection{}
	current.status = "replacement-connected"
	current.files.status = "replacement-listing"
	a.drain()
	current.conn = old
	if current.status != "replacement-connected" || current.files.status != "replacement-listing" {
		t.Fatal("old callback changed replacement state")
	}
}
