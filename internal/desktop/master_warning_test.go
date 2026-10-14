package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestMasterWarningDismissalPersistsWithoutChangingProtection(t *testing.T) {
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	revision := a.vault.Revision()
	tt := ui.NewTester(a.masterWarning, 1000, 100)
	if err := tt.Click("我已知晓，不再提醒"); err != nil {
		t.Fatal(err)
	}
	disk, err := store.Load()
	if err != nil || !disk.Settings.MasterWarningAcknowledged {
		t.Fatal("choice not saved", err)
	}
	if a.vault.Revision() != revision {
		t.Fatal("dismissal changed credential protection")
	}
	restarted := newApp(store, disk)
	next := ui.NewTester(restarted.masterWarning, 1000, 100)
	if next.HasText("我已知晓，不再提醒") {
		t.Fatal("dismissal lost after restart")
	}
}
func TestMasterWarningSetupAndVisibility(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	tt := ui.NewTester(a.masterWarning, 1000, 100)
	if err := tt.Click("立即设置主密码"); err != nil {
		t.Fatal(err)
	}
	if !a.features.vaultOpen || a.features.vaultAction != "set" {
		t.Fatal("setup entry missing")
	}
	a.focusFullscreen.chrome = true
	hidden := ui.NewTester(a.masterWarning, 1000, 100)
	if hidden.HasText("立即设置主密码") {
		t.Fatal("warning visible in focus mode")
	}
	a.focusFullscreen.chrome = false
	if err := a.vault.SetMaster("fixture-master-password"); err != nil {
		t.Fatal(err)
	}
	hidden = ui.NewTester(a.masterWarning, 1000, 100)
	if hidden.HasText("立即设置主密码") {
		t.Fatal("warning visible with master protection")
	}
}
