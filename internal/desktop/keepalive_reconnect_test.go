package desktop

import (
	"testing"

	"oshell-desktop/internal/core"
)

func TestReconnectUsesSavedKeepAlive(t *testing.T) {
	for _, mode := range []string{"saved", "other-host", "external", "deleted"} {
		t.Run(mode, func(t *testing.T) { testReconnectKeepAlive(t, mode) })
	}
}

func testReconnectKeepAlive(t *testing.T, mode string) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p, _ := operatorPane(t, a, "ssh")
	p.profile = core.NewProfile()
	p.profile.Host, p.profile.Username = "keepalive.example.invalid", "test"
	saved := p.profile
	saved.KeepAlive.IdleEnabled = true
	saved.KeepAlive.IdleInterval = 30
	saved.KeepAlive.IdleText = `ls \n`
	saved.KeepAlive.Interval = 17
	want := p.profile.KeepAlive
	switch mode {
	case "saved":
		want = saved.KeepAlive
	case "other-host":
		saved.Host = "changed.example.invalid"
	case "external":
		p.externalLaunch = true
	case "deleted":
		saved.ID = core.NewID()
	}
	a.config.Profiles = []core.Profile{saved}
	a.reconnectActive()
	if a.features.authCancel != nil {
		defer a.features.authCancel()
	}
	if a.pendingProfile.KeepAlive != want || a.pendingProfile.Identity() != p.profile.Identity() {
		t.Fatalf("reconnect used stale keepalive: %+v", a.pendingProfile.KeepAlive)
	}
	if p.profile.KeepAlive == saved.KeepAlive {
		t.Fatal("existing connection snapshot was changed")
	}
}
