package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func TestManagerCreatedDirectoryMouseNavigation(t *testing.T) {
	for _, method := range []string{"double-name", "double-cell", "context", "enter"} {
		t.Run(method, func(t *testing.T) {
			cfg := core.DefaultConfig()
			store := &core.Store{Dir: t.TempDir()}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			a := newApp(store, cfg)
			a.openDirectory("新建目录", "")
			a.features.directoryName = "测试目录"
			a.applyDirectory()
			if a.features.directoryOpen {
				t.Fatal(a.features.directoryError)
			}
			tt := ui.NewTester(a.managerView, 900, 580)
			r, ok := tt.Find("测试目录")
			if !ok {
				t.Fatal("created folder missing")
			}
			x, y := r.X+45, r.Y+r.H/2
			switch method {
			case "double-name", "double-cell":
				if method == "double-cell" {
					x = r.X + 260
				}
				tt.ClickAt(x, y)
				tt.ClickAt(x, y)
			case "context":
				tt.RightClickAt(x, y)
				if err := tt.ChooseMenuItem("打开目录"); err != nil {
					t.Fatal(err)
				}
			case "enter":
				tt.ClickAt(x, y)
				tt.Key(0, ui.KeyEnter)
			}
			tt.Frame()
			if a.managerGroup != "测试目录" {
				t.Fatalf("folder did not open: group=%q row=%d selection=%v", a.managerGroup, a.managerRow, a.selectedManagerRows())
			}
			if a.managerRow != -1 || a.managerSelection.Len() != 0 {
				t.Fatal("navigation retained the previous directory's selection")
			}
			if err := tt.RightClick("../"); err != nil {
				t.Fatal(err)
			}
			if err := tt.ChooseMenuItem("上一级"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if a.managerGroup != "" {
				t.Fatal("did not return to root")
			}
			// Re-enter after a previous frame has measured and retained this list.
			for range 3 {
				if err := tt.RightClick("测试目录"); err != nil {
					t.Fatal(err)
				}
				if err := tt.ChooseMenuItem("打开目录"); err != nil {
					t.Fatal(err)
				}
				if a.managerGroup != "测试目录" {
					t.Fatal("re-enter failed")
				}
				if err := tt.Click("../"); err != nil {
					t.Fatal(err)
				}
				if a.managerGroup != "" {
					t.Fatal("parent click failed")
				}
			}
		})
	}
}
