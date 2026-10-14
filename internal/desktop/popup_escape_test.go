package desktop

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestAuxiliaryEscapeFocusedSearchAndMenuDismissal(t *testing.T) {
	closed := 0
	text := ""
	tt := ui.NewTester(func(c *ui.Context) {
		auxiliaryEscape(c, func() { closed++ })
		ui.TextInput(c, &text).Label("search").AutoFocus()
		ui.MenuButton(c, "menu", func(m *ui.Menu) { m.Item("entry") })
	}, 500, 300)
	if err := tt.Click("search"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Shift, ui.KeyEscape)
	if closed != 0 {
		t.Fatal("modified escape closed popup")
	}
	tt.Key(0, ui.KeyEscape)
	if closed != 1 {
		t.Fatal("focused search swallowed escape")
	}
	if err := tt.Click("menu"); err != nil {
		t.Fatal(err)
	}
	// Tester menus model platform callbacks, not native menu key tracking.
	tt.CloseMenu()
	if closed != 1 || len(tt.Menu()) != 0 {
		t.Fatal("menu escape also closed popup")
	}
	tt.Key(0, ui.KeyEscape)
	if closed != 2 {
		t.Fatal("escape after menu did not close popup")
	}
}

func TestAuxiliaryEscapeClosesInnerModalBeforeParent(t *testing.T) {
	closed := 0
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		auxiliaryEscape(c, func() { closed++ })
		ui.Modal(c, &open, func() { ui.Text(c, "inner") })
	}, 500, 300)
	tt.Key(0, ui.KeyEscape)
	if open || closed != 0 {
		t.Fatal("inner modal escape closed parent")
	}
	tt.Key(0, ui.KeyEscape)
	if closed != 1 {
		t.Fatal("parent did not close after modal")
	}
}
