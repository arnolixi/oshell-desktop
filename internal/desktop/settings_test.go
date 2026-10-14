package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func TestSettingsDraftCancelAndApply(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	a.features.settingsDraft.FontSize = 18
	a.closeSettings()
	if a.config.Settings.FontSize == 18 {
		t.Fatal("cancel changed settings")
	}
	a.prepareSettings()
	a.features.settingsDraft.FontSize = 18
	a.features.settingsLines = "6000"
	tt := ui.NewTester(a.settingsView, 800, 650)
	snapshot(t, tt, "settings-general.png")
	a.features.settingsTab = 1
	tt.Frame()
	snapshot(t, tt, "settings-colors.png")
	a.features.colorDraft.Background = "#123456"
	a.applySettings()
	if a.config.Settings.FontSize != 18 || a.config.Settings.ScrollbackLines != 6000 || a.config.Settings.Scheme().Background != "#123456" {
		t.Fatal(a.config.Settings)
	}
	if len(a.config.Settings.CustomSchemes) != 1 {
		t.Fatal("editing a preset did not create a custom copy")
	}
}

func TestSettingsThemePreviewDoesNotMutateSavedAppearance(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.config.Settings.InterfaceTheme = "light"
	a.prepareSettings()
	a.features.settingsDraft.InterfaceTheme = "dark"
	tt := ui.NewTester(a.settingsView, 800, 650)
	pixel := tt.Image().RGBAAt(1, 1)
	if pixel.R != 32 || pixel.G != 33 || pixel.B != 36 {
		t.Fatalf("whole settings background did not preview dark: %+v", pixel)
	}
	snapshot(t, tt, "settings-preview-dark.png")
	if a.config.Settings.InterfaceTheme != "light" {
		t.Fatal("preview mutated saved appearance")
	}
	a.closeSettings()
	if a.config.Settings.InterfaceTheme != "light" {
		t.Fatal("cancel persisted theme")
	}
	a.prepareSettings()
	a.features.settingsDraft.InterfaceTheme = "dark"
	a.applySettings()
	if a.config.Settings.InterfaceTheme != "dark" {
		t.Fatal("apply did not persist theme")
	}
	a.prepareSettings()
	a.features.settingsDraft.InterfaceTheme = "light"
	tt.Frame()
	pixel = tt.Image().RGBAAt(1, 1)
	if pixel.R != 255 || pixel.G != 255 || pixel.B != 255 {
		t.Fatalf("light preview inherited saved dark: %+v", pixel)
	}
	snapshot(t, tt, "settings-preview-light.png")
	a.closeSettings()
	if a.config.Settings.InterfaceTheme != "dark" {
		t.Fatal("cancel of light preview changed dark preference")
	}
}
