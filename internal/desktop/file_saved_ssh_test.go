package desktop

import (
	"strings"
	"testing"
)

func TestFileWindowOpensSavedSSHWithoutChangingCredentialIdentity(t *testing.T) {
	a, profile, accepted := savedConnectionFixture(t)
	a.hub = &applicationHub{fileWindow: a, store: a.store, config: a.config, vault: a.vault, trust: a.trust}
	if err := a.vault.PutFor(profile, "saved-fixture-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(profile, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.tabs[0].active.files != nil && !a.tabs[0].active.files.busy && a.tabs[0].active.files.client != nil
	})
	p := a.tabs[0].active
	if p.term != nil || p.profile.Kind != "ssh" || a.authOpen || accepted.Load() != 1 {
		t.Fatal("SSH file launch changed identity, opened a terminal, or repeated authentication")
	}
	if password, err := a.vault.GetFor(profile, profile.Identity()); err != nil || password != "saved-fixture-password" {
		t.Fatal("saved SSH password changed", err)
	}
	a.reconnectActive()
	waitOperator(t, a, func() bool {
		next := a.tabs[0].active
		return next != p && next.files != nil && !next.files.busy && next.files.client != nil
	})
	if len(a.tabs) != 1 || a.tabs[0].active.term != nil || accepted.Load() != 1 {
		t.Fatal("reconnect lost file-session mode")
	}
	a.duplicateFileSession()
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 2 && a.tabs[1].active.files != nil && !a.tabs[1].active.files.busy && a.tabs[1].active.files.client != nil
	})
	if accepted.Load() != 1 {
		t.Fatal("file copy authenticated again")
	}
	a.dispose(a.tabs[0].active)
	files := a.tabs[1].active.files
	if _, err := files.client.ReadDir("/"); err != nil {
		t.Fatal("copy lost connection after original closed", err)
	}
	a.tabs[1].active.conn.Close()
	previous := a.activePane()
	a.reconnectActive()
	if a.activePane() != previous || a.authOpen || accepted.Load() != 1 || !strings.Contains(a.status, "无法复用") {
		t.Fatal("dead shared connection silently reauthenticated", a.status)
	}

}
