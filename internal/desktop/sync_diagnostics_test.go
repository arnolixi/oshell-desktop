package desktop

import (
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
	"time"
)

func TestSyncDiagnosticsWindowCanBeReopened(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	for i := 0; i < 2; i++ {
		a.prepareSettings()
		f := &a.features.sharedSync
		f.loaded = true
		f.logsOpen = true
		f.logEntries = []core.SyncDiagnosticEntry{{Date: time.Now(), Backend: "directory", Event: core.SyncCompleted}}
		tt := ui.NewTester(a.settingsView, 800, 650)
		if !tt.HasText("同步诊断日志") || !tt.HasText("同步完成") {
			t.Fatal("missing reopened log")
		}
		a.closeSettings()
	}
}

func TestSavedChangesQueueSyncWithoutLosingSuccessHistory(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	if err := a.vault.SetMaster("synthetic-shared-password"); err != nil {
		t.Fatal(err)
	}
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: t.TempDir()}
	if err := core.SaveSyncOptions(store.Dir, options); err != nil {
		t.Fatal(err)
	}
	diagnostic := core.SyncDiagnostics{Directory: store.Dir, Target: options.Target(), Backend: options.Backend}
	original := a.config.Settings.FontSize
	if err := diagnostic.Record(core.SyncCompleted, true, 0, nil, core.SyncLocalFingerprint(store, a.vault)); err != nil {
		t.Fatal(err)
	}
	before, err := diagnostic.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !a.changeConfig(func(cfg *core.Config) error { cfg.Settings.FontSize = original + 1; return nil }) {
		t.Fatal(a.status)
	}
	clientOnly, err := diagnostic.Snapshot()
	if err != nil || len(clientOnly.Entries) != len(before.Entries) || a.features.sharedSync.pendingLocal {
		t.Fatal("client-only edit queued session synchronization", err)
	}
	if !a.changeConfig(func(cfg *core.Config) error { cfg.Directories = append(cfg.Directories, "new-directory"); return nil }) {
		t.Fatal(a.status)
	}
	if !a.features.sharedSync.pendingLocal || a.features.sharedSync.phase != core.SyncQueued {
		t.Fatal("saved change did not queue status")
	}
	after, err := diagnostic.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Entries) != len(before.Entries)+2 || after.Entries[len(after.Entries)-2].Event != core.SyncLocalChanged || after.Summary.Outcome != core.SyncQueued || !after.Summary.LastSuccess.Equal(*before.Summary.LastSuccess) {
		t.Fatal("queued history invalid")
	}
	if !a.save() {
		t.Fatal(a.status)
	}
	unchanged, err := diagnostic.Snapshot()
	if err != nil || len(unchanged.Entries) != len(after.Entries) {
		t.Fatal("unchanged save duplicated queue events", err)
	}
	// A later successful state invalidates deduplication, even if another
	// window eventually revisits the same previously queued configuration.
	if err = diagnostic.Record(core.SyncCompleted, true, 0, nil, before.Summary.LocalFingerprint); err != nil {
		t.Fatal(err)
	}
	a.markSharedSyncChanged()
	latest, err := diagnostic.Snapshot()
	if err != nil || latest.Summary.Outcome != core.SyncQueued {
		t.Fatal("old queue identity suppressed a new change", err)
	}
}

func TestSyncDiagnosticsShowsMemoryFallbackAndHistory(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	if err := a.vault.SetMaster("memory-fixture-master"); err != nil {
		t.Fatal(err)
	}
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: t.TempDir()}
	if err := core.SaveSyncOptions(store.Dir, options); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(store.Dir, "logs")
	if err := os.WriteFile(blocked, []byte("fixture obstruction"), 0600); err != nil {
		t.Fatal(err)
	}
	d := core.SyncDiagnostics{Directory: store.Dir, Target: options.Target(), Backend: options.Backend}
	if err := d.Record(core.SyncCompleted, true, 0, nil, core.SyncLocalFingerprint(store, a.vault)); err == nil {
		t.Fatal("fixture should reject writes")
	}
	a.prepareSettings()
	a.features.settingsTab = 3
	tt := ui.NewTester(a.settingsView, 800, 650)
	if !tt.HasText("已与本机同步目录对齐") {
		t.Fatal("logging failure hid successful sync state")
	}
	snapshot(t, tt, "sync-status-memory-fallback.png")
	if !tt.HasText("当前运行仍可查看和导出内存记录") {
		t.Fatal("missing fallback explanation")
	}
	if err := tt.Click("查看同步日志…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("同步完成") {
		t.Fatal("retained history missing from dialog")
	}
	snapshot(t, tt, "sync-diagnostics-memory-fallback.png")
	// Restore the fixture directory and flush so no process-local pending cache
	// survives this test after its temporary directory is removed.
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := d.Record(core.SyncUnchanged, true, 0, nil, ""); err != nil {
		t.Fatal(err)
	}
	a.loadSyncDiagnostics()
	if a.features.sharedSync.logIssue != "" {
		t.Fatal("stale disk failure remained after recovery")
	}
}
