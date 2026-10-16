package desktop

import (
	"github.com/egoist/mygo"
	"oshell-desktop/internal/core"
	"testing"
)

func TestWorkspaceCloseKeepsSyncCompletionAlive(t *testing.T) {
	for _, filesOnly := range []bool{false, true} {
		a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
		hub := &applicationHub{}
		a.hub = hub
		if filesOnly {
			hub.fileWindow = a
		} else {
			hub.windows = []*App{a}
		}
		a.features.sharedSync.busy = true
		a.setConfigurationBusy(true)
		// Even an already-confirmed close/quit must wait for the commit.
		a.features.allowClose = true
		hub.quitting = true
		event := &mygo.CloseEvent{}
		a.onWorkspaceClose(event)
		if !event.DefaultPrevented() {
			t.Fatal("sync owner allowed to close before completion")
		}
		if !a.post(func() { a.features.sharedSync.busy = false; a.setConfigurationBusy(false) }) {
			t.Fatal("completion rejected")
		}
		a.drain()
		other := &App{hub: hub}
		if other.configurationBusy() {
			t.Fatal("global configuration remained busy")
		}
		event = &mygo.CloseEvent{}
		a.onWorkspaceClose(event)
		if event.DefaultPrevented() {
			t.Fatal("completed sync still prevents closing")
		}
	}
}
