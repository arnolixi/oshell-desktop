package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestTabDropCapturesPositionBeforeQueuedWindowCreation(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	previous := captureTabWindowPosition
	t.Cleanup(func() { captureTabWindowPosition = previous })
	calls := 0
	captureTabWindowPosition = func() *tabWindowPosition {
		calls++
		return &tabWindowPosition{x: -1200, y: 240}
	}
	a.queueDetachedTab(&tab{id: "already-closed"})
	if calls != 1 {
		t.Fatal("release did not capture pointer immediately")
	}
	// Queued creation must not sample a newer mouse position. A vanished tab
	// must also remain harmless when the queued operation is drained.
	captureTabWindowPosition = func() *tabWindowPosition { t.Fatal("pointer queried after release"); return nil }
	a.drain()
	if len(a.tabs) != 0 {
		t.Fatal("stale drop restored a closed tab")
	}
}
