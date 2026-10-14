package desktop

import (
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
)

func TestStorageSettingsPathsCancelAndApply(t *testing.T) {
	dir := t.TempDir()
	store := &core.Store{Dir: dir}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.prepareSettings()
	a.features.settingsTab = 6
	tt := ui.NewTester(a.settingsView, 800, 650)
	if !tt.HasText(dir) || tt.HasText("config.json") || tt.HasText("credentials.json") {
		t.Fatal("storage page must show directory without individual file paths", tt.Texts())
	}
	snapshot(t, tt, "settings-storage.png")
	target := filepath.Join(t.TempDir(), "custom")
	a.storageDirectoryDraft = target
	a.closeSettings()
	if _, err := os.Stat(filepath.Join(dir, "storage-location.json")); !os.IsNotExist(err) {
		t.Fatal("cancel published location")
	}
	a.prepareSettings()
	a.storageDirectoryDraft = target
	a.applySettings()
	location, err := store.StorageLocation()
	if err != nil || location.Pending == "" {
		t.Fatal("Apply did not schedule", err, a.features.settingsError)
	}
	if a.store.Dir != dir {
		t.Fatal("running store changed before restart")
	}
	a.prepareSettings()
	if a.storageDirectoryDraft != location.Pending {
		t.Fatal("pending location not restored")
	}
	a.storageDirectoryDraft = dir
	a.applySettings()
	location, err = store.StorageLocation()
	if err != nil || location.Pending != "" {
		t.Fatal("cancel migration failed", err)
	}
}

func TestPortableStorageSettingsRemainBesideExecutable(t *testing.T) {
	store := &core.Store{Dir: t.TempDir(), Portable: true}
	a := newApp(store, core.DefaultConfig())
	a.prepareSettings()
	a.features.settingsTab = 6
	tt := ui.NewTester(a.settingsView, 800, 650)
	if !tt.HasText("绿色便携模式") || tt.HasText("自定义数据目录") {
		t.Fatal("portable page permits external relocation", tt.Texts())
	}
}
