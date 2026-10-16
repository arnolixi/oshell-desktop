package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFileManagerFollowsCurrentSSHConnection(t *testing.T) {
	source, profile, accepted := savedConnectionFixture(t)
	if err := source.vault.PutFor(profile, "saved-fixture-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	// Two independent connections to the same saved profile must not be confused.
	for count := 1; count <= 2; count++ {
		source.request(profile, -2)
		waitOperator(t, source, func() bool {
			return len(source.tabs) == count && source.activePane().term != nil && source.activePane().conn != nil
		})
	}
	files := newApp(source.store, source.config)
	hub := &applicationHub{store: source.store, config: source.config, vault: source.vault, trust: source.trust, fileWindow: files, windows: []*App{source}}
	source.hub, files.hub = hub, hub
	t.Cleanup(func() {
		for _, tab := range files.tabs {
			files.walk(tab.root, files.dispose)
		}
	})
	var fileTabs []*tab
	for i, terminalTab := range source.tabs {
		source.selectTab(terminalTab)
		source.performAction("fileManager")
		if len(files.tabs) != i+1 {
			t.Fatalf("SSH session %d did not open a file tab: count=%d", i, len(files.tabs))
		}
		opened := files.activeTab()
		source.performAction("fileManager")
		if len(files.tabs) != i+1 || files.activeTab() != opened {
			t.Fatal("repeated click duplicated a pending file tab")
		}
		waitOperator(t, files, func() bool { p := opened.active; return p.files != nil && !p.files.busy && p.files.client != nil })
		fileTabs = append(fileTabs, opened)
	}
	if accepted.Load() != 2 {
		t.Fatal("file manager reauthenticated instead of reusing SSH", accepted.Load())
	}
	for _, i := range []int{0, 1, 0} {
		source.selectTab(source.tabs[i])
		source.performAction("fileManager")
		if files.activeTab() != fileTabs[i] || len(files.tabs) != 2 {
			t.Fatal("file manager did not select the current SSH session")
		}
	}
	beforeReconnect := files.activePane()
	files.reconnectActive()
	waitOperator(t, files, func() bool {
		p := files.activePane()
		return p != beforeReconnect && p.files != nil && !p.files.busy && p.files.client != nil
	})
	source.performAction("fileManager")
	if files.activeTab() != fileTabs[0] || len(files.tabs) != 2 {
		t.Fatal("file reconnect lost the originating SSH session")
	}
	// Closing a file tab releases only its channel; opening it again still works.
	files.closeTab(fileTabs[0])
	source.performAction("fileManager")
	waitOperator(t, files, func() bool {
		p := files.activePane()
		return len(files.tabs) == 2 && p.files != nil && !p.files.busy && p.files.client != nil
	})
	if !source.activePane().conn.Alive() || accepted.Load() != 2 {
		t.Fatal("file close affected the SSH terminal")
	}
	// The view must reflect the tab chosen by the external toolbar action.
	tt := ui.NewTester(files.fileWindowView, 1080, 720)
	if !tt.HasText("当前会话：" + fileTabDescription(files.activeTab())) {
		t.Fatal("current file indicator was not refreshed")
	}
}
