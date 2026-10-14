package desktop

import (
	"github.com/egoist/mygo/ui"
	"runtime"
	"testing"
)

func TestPasswordSelectAllUnicodeWithoutClipboard(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS password editing policy")
	}
	for _, mods := range []ui.Modifiers{ui.Ctrl, ui.Super} {
		text := "fixture-中文-🔐-é-👨‍👩‍👧-password"
		tt := ui.NewTester(func(c *ui.Context) { passwordInput(c, &text).AutoFocus().FillWidth() }, 600, 100)
		tt.SetClipboard("clipboard-sentinel")
		tt.Key(mods, ui.KeyA)
		tt.Type("replacement")
		if text != "replacement" {
			t.Fatalf("%v did not replace entire Unicode password: %q", mods, text)
		}
		if tt.Clipboard() != "clipboard-sentinel" {
			t.Fatal("selection changed clipboard")
		}
		tt.Key(mods, ui.KeyA)
		tt.Key(ui.Super, ui.KeyC)
		if tt.Clipboard() != "clipboard-sentinel" {
			t.Fatal("password copy exposed secret")
		}
		tt.Key(mods, ui.KeyA)
		tt.Key(0, ui.KeyBackspace)
		tt.Key(mods, ui.KeyA)
		tt.Type("empty-replacement")
		if text != "empty-replacement" {
			t.Fatal("empty selection failed", text)
		}
	}
}
func TestPasswordSelectionDoesNotChangeOrdinaryControlA(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS editing policy")
	}
	text := "ordinary"
	tt := ui.NewTester(func(c *ui.Context) { ui.TextInput(c, &text).AutoFocus() }, 500, 100)
	tt.Key(0, ui.KeyEnd)
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("X")
	if text == "X" {
		t.Fatal("ordinary field was selected by password behavior")
	}
}
