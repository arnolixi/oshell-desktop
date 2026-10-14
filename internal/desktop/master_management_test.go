package desktop

import (
	"oshell-desktop/internal/core"
	"testing"
)

func TestClearMasterRequiresCurrentPasswordEvenWhenUnlocked(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	if err := a.vault.SetMaster("fixture-master-password"); err != nil {
		t.Fatal(err)
	}
	a.features.vaultAction = "remove"
	a.features.masterCurrent = "wrong-password"
	a.runVaultAction()
	waitOperator(t, a, func() bool { return !a.features.vaultBusy })
	if master, _ := a.vault.State(); !master || a.features.vaultError == "" {
		t.Fatal("cleared without verification")
	}
	a.features.masterCurrent = "fixture-master-password"
	a.runVaultAction()
	waitOperator(t, a, func() bool { return !a.features.vaultBusy })
	if master, _ := a.vault.State(); master || a.features.vaultError != "" {
		t.Fatal("verified clear failed", a.features.vaultError)
	}
	if a.features.masterCurrent != "" {
		t.Fatal("dialog retained current password")
	}
}
