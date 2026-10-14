package desktop

import (
	"context"
	"oshell-desktop/internal/core"
	"testing"
)

func TestWebDAVProbeRequiresMasterAndDoesNotSave(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.features.sharedSync.options = core.SyncOptions{Backend: "webdav", Address: "https://draft.invalid/"}
	a.testWebDAVConnection()
	if a.features.sharedSync.probing || a.features.sharedSync.probeStatus != "请先启用并解锁主密码保护" {
		t.Fatal("probe bypassed master guard")
	}
	saved, err := core.LoadSyncOptions(a.store.Dir)
	if err != nil || saved.Address != "" {
		t.Fatal("probe saved draft", err)
	}
}
func TestCloseSettingsCancelsWebDAVProbe(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	ctx, cancel := context.WithCancel(context.Background())
	f := &a.features.sharedSync
	f.probing = true
	f.probeCancel = cancel
	generation := f.probeGeneration
	a.closeSettings()
	if ctx.Err() == nil || f.probing || f.probeGeneration == generation {
		t.Fatal("probe not cancelled and invalidated")
	}
}
