package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestColorSettingsPickerPreviewAndDraft(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	a.prepareSettings()
	original := a.config.Settings.Scheme().Foreground
	tt := ui.NewTester(a.colorSettings, 800, 1000)
	if !tt.HasText("选中文字") || !tt.HasText("亮红 Aa") {
		t.Fatal("color examples missing", tt.Texts())
	}
	if err := tt.Click("文字颜色选择器"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Red"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.features.colorDraft.Foreground != "#EF4444" {
		t.Fatal("picker failed to write color draft", a.features.colorDraft.Foreground)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	snapshot(t, tt, "color-settings-preview.png")
	if a.config.Settings.Scheme().Foreground != original {
		t.Fatal("picker changed saved scheme before Apply")
	}
	a.features.colorDraft.Background = "invalid"
	tt.Frame()
	if !tt.HasText("请输入 #RRGGBB") {
		t.Fatal("invalid color feedback missing")
	}
	a.closeSettings()
	a.prepareSettings()
	if a.features.colorDraft.Foreground != original {
		t.Fatal("Cancel retained draft color")
	}
}
func TestColorInputRejectsInvalidHexWithoutPanic(t *testing.T) {
	for _, value := range []string{"", "#", "#12345Z", "#12345678", "ffffff"} {
		if _, ok := parseSettingColor(value); ok {
			t.Fatal(value)
		}
	}
	color, ok := parseSettingColor("#12aBcD")
	if !ok || color.R != 0x12 || color.G != 0xab || color.B != 0xcd {
		t.Fatal("valid hex rejected")
	}
}
