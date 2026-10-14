package desktop

import (
	"strings"
	"testing"
)

func TestSFTPProfileLaunchesTerminalAndPreservesMode(t *testing.T) {
	a, profile, accepted := savedConnectionFixture(t)
	profile.Kind = "sftp"
	a.config.Profiles[0] = profile
	if err := a.vault.PutFor(profile, "saved-fixture-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(profile, 0, true)
	ready := func() bool {
		p := a.activePane()
		return p != nil && p.term != nil && strings.Contains(p.term.Text(), "SAVED_CREDENTIAL_READY")
	}
	waitOperator(t, a, ready)
	original := a.activePane()
	if !original.terminalOnly || original.profile.Kind != "sftp" || original.files != nil {
		t.Fatal("launch mutated profile or opened file browser")
	}
	a.reconnectActive()
	waitOperator(t, a, func() bool { return a.activePane() != original && ready() })
	if accepted.Load() != 2 || !a.activePane().terminalOnly {
		t.Fatal("reconnect lost launch mode")
	}
	a.cloneActive()
	waitOperator(t, a, func() bool { return len(a.tabs) == 2 && ready() })
	if accepted.Load() != 2 || !a.activePane().terminalOnly {
		t.Fatal("channel clone lost mode or reauthenticated")
	}
	a.performAction("splitVertical")
	waitOperator(t, a, func() bool { return a.activeTab().root.pane == nil && ready() })
	if accepted.Load() != 3 || !a.activePane().terminalOnly {
		t.Fatal("split lost terminal mode")
	}
	if a.config.Profiles[0].Kind != "sftp" {
		t.Fatal("saved protocol changed")
	}
	if secret, err := a.vault.GetFor(profile, profile.Identity()); err != nil || secret != "saved-fixture-password" {
		t.Fatal("credential binding changed", err)
	}
}
