package desktop

import (
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func drainSharedSync(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for a.features.sharedSync.busy {
		a.drain()
		if time.Now().After(deadline) {
			t.Fatal("sync timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
	a.drain()
}
func TestSharedSyncSettingsAndDirectoryFlow(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Host = "example.invalid"
	p.Username = "tester"
	cfg.Profiles = []core.Profile{p}
	if e := store.Save(cfg); e != nil {
		t.Fatal(e)
	}
	a := newApp(store, cfg)
	if e := a.vault.SetMaster("synthetic-shared-password"); e != nil {
		t.Fatal(e)
	}
	a.prepareSettings()
	a.features.settingsTab = 3
	tt := ui.NewTester(a.settingsView, 800, 650)
	if !tt.HasText("多设备同步") || !tt.HasText("同步方式") {
		t.Fatal(tt.Texts())
	}
	remote := t.TempDir()
	a.features.sharedSync.options = core.SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: remote}
	a.saveSharedSyncSettings()
	drainSharedSync(t, a)
	if a.features.sharedSync.status != "同步设置已保存" {
		t.Fatal(a.features.sharedSync.status)
	}
	a.startSavedSharedSync()
	drainSharedSync(t, a)
	if a.features.sharedSync.status != "同步完成" {
		t.Fatal(a.features.sharedSync.status)
	}
	a.prepareSettings()
	a.loadSharedSyncSettings()
	tt.Frame()
	if !tt.HasText("已与本机同步目录对齐") || tt.HasText("上次成功：暂无") {
		t.Fatal("success status card missing", tt.Texts())
	}
	if a.features.sharedSync.summary.LastSuccess == nil {
		t.Fatal("last success missing")
	}
	a.features.sharedSync.logsOpen = true
	tt.Frame()
	snapshot(t, tt, "sync-diagnostics.png")
	if !tt.HasText("同步诊断日志") || !tt.HasText("同步完成") {
		t.Fatal("stage history missing")
	}
	a.features.sharedSync.logsOpen = false
	data, e := os.ReadFile(filepath.Join(remote, "configuration.json"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(data), "example.invalid") {
		t.Fatal("unencrypted shared config")
	}
	a.features.vaultAction = "remove"
	a.runVaultAction()
	if a.features.vaultBusy || a.features.vaultError == "" {
		t.Fatal("removed protection during enabled sync")
	}
	tt.Frame()
	snapshot(t, tt, "settings-sync.png")
}

func TestSharedSyncSchedulerStopsWhenDisabled(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	a := newApp(store, cfg)
	hub := &applicationHub{store: store, vault: a.vault, windows: []*App{a}}
	a.hub = hub
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: t.TempDir()}
	if e := core.SaveSyncOptions(store.Dir, options); e != nil {
		t.Fatal(e)
	}
	hub.scheduleSharedSync(time.Hour)
	if hub.syncTimer == nil {
		t.Fatal("enabled sync not scheduled")
	}
	options.Enabled = false
	if e := core.SaveSyncOptions(store.Dir, options); e != nil {
		t.Fatal(e)
	}
	hub.scheduleSharedSync(time.Hour)
	if hub.syncTimer != nil {
		hub.syncTimer.Stop()
		t.Fatal("disabled sync retains timer")
	}
}

func TestSavedSyncClosesSettingsWithoutApplyingDraft(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	original := a.config.Settings.FontSize
	a.features.settingsDraft.FontSize = original + 7
	a.startSavedSharedSync() // Disabled sync is sufficient to test the entry contract.
	if a.settingsOpen || a.config.Settings.FontSize != original {
		t.Fatal("manual sync applied unsaved settings")
	}
}

func TestSharedSyncDefersForAnotherWindowsSettingsDraft(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	a := newApp(store, cfg)
	if e := a.vault.SetMaster("synthetic-shared-password"); e != nil {
		t.Fatal(e)
	}
	other := newApp(store, cfg)
	other.vault = a.vault
	other.prepareSettings()
	other.features.settingsDraft.FontSize = 23
	hub := &applicationHub{store: store, vault: a.vault, windows: []*App{a, other}, settingsOwner: other}
	a.hub = hub
	other.hub = hub
	options := core.SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: t.TempDir()}
	if e := core.SaveSyncOptions(store.Dir, options); e != nil {
		t.Fatal(e)
	}
	hub.scheduleSharedSync(time.Millisecond)
	first := hub.syncTimer
	waitOperator(t, a, func() bool { return hub.syncTimer != first })
	if hub.syncTimer != nil {
		hub.syncTimer.Stop()
	}
	if a.features.sharedSync.busy || other.features.settingsDraft.FontSize != 23 || !other.settingsOpen {
		t.Fatal("background sync disturbed another window's settings draft")
	}
	if _, e := os.Stat(filepath.Join(options.Address, "configuration.json")); !os.IsNotExist(e) {
		t.Fatal("background sync wrote while settings were open")
	}
}
