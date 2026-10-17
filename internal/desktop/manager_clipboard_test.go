package desktop

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func managerClipboardFixture(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	store := &core.Store{Dir: t.TempDir()}
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name, p.Host, p.Group = "clipboard-source", "fixture.invalid", ""
	cfg.Profiles = []core.Profile{p}
	cfg.Directories = []string{"target"}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	if err := a.vault.SetMaster("fixture-master-password"); err != nil {
		t.Fatal(err)
	}
	if err := a.vault.PutForRevision(p, "fixture-secret", p.Identity(), a.vault.Revision(), "local"); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.managerView, 900, 580)
	if err := tt.Click(p.Name); err != nil {
		t.Fatal(err)
	}
	return a, tt
}

func TestManagerClipboardShortcutsPasteAndRebind(t *testing.T) {
	a, tt := managerClipboardFixture(t)
	source := a.config.Profiles[0]
	tt.Key(ui.Ctrl, ui.KeyC)
	tt.Frame()
	if !strings.HasPrefix(tt.Clipboard(), "oshell-session-configuration:") || strings.Contains(tt.Clipboard(), source.Host) || len(a.config.Profiles) != 1 {
		t.Fatal("copy must store only an opaque reference and not create a session")
	}
	a.navigateManager("target")
	tt.Frame()
	if err := tt.Click("会话列表"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		tt.Key(ui.Ctrl, ui.KeyV)
		tt.Frame()
		waitOperator(t, a, func() bool { return !a.configurationBusy() })
		tt.Frame()
		if len(a.config.Profiles) != i+2 {
			t.Fatal("paste did not create session", a.status)
		}
		copy := a.config.Profiles[i+1]
		if copy.ID == source.ID || copy.Group != "target" || !a.managerSelection.Has(copy.ID) || a.managerAll {
			t.Fatal("paste destination, identity or selection incorrect")
		}
		if i == 0 && copy.Name != source.Name || i == 1 && copy.Name != source.Name+" - 副本" {
			t.Fatal("paste name collision handling incorrect")
		}
		secret, err := a.vault.GetFor(copy, source.Identity())
		if err != nil || secret != "fixture-secret" || a.vault.CredentialProtection(copy.ID) != "local" {
			t.Fatal("credential identity or protection lost", err)
		}
	}
	disk, err := a.store.Load()
	if err != nil || len(disk.Profiles) != 3 {
		t.Fatal("paste not persisted", err)
	}
}

func TestManagerClipboardLeavesTextEditingAlone(t *testing.T) {
	a, tt := managerClipboardFixture(t)
	tt.SetClipboard("search text")
	if err := tt.Click("会话搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyV)
	tt.Frame()
	if a.query != "search text" || a.configurationBusy() || len(a.config.Profiles) != 1 {
		t.Fatal("search paste was intercepted")
	}
	a.query = ""
	tt.Frame()
	if err := tt.Click("clipboard-source"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyF2)
	tt.Frame()
	if a.managerRename == nil {
		t.Fatal("rename did not open")
	}
	tt.SetClipboard("rename text")
	tt.Key(ui.Ctrl, ui.KeyV)
	tt.Frame()
	if a.managerClipboard != nil || a.configurationBusy() || !strings.Contains(a.managerRename.name, "rename text") {
		t.Fatal("rename paste was intercepted")
	}
}

func TestManagerClipboardRejectsStaleOrChangedContents(t *testing.T) {
	a, tt := managerClipboardFixture(t)
	tt.Key(ui.Ctrl, ui.KeyC)
	tt.Frame()
	if a.managerClipboard == nil {
		t.Fatal("copy did not capture selection")
	}
	if err := a.vault.Put(a.config.Profiles[0], "changed-secret"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyV)
	tt.Frame()
	if a.configurationBusy() || len(a.config.Profiles) != 1 || !strings.Contains(a.status, "重新复制") {
		t.Fatal("changed credential accepted")
	}
	tt.Key(ui.Ctrl, ui.KeyC)
	tt.Frame()
	tt.SetClipboard("ordinary text")
	tt.Key(ui.Ctrl, ui.KeyV)
	tt.Frame()
	if a.managerClipboard != nil || a.configurationBusy() || len(a.config.Profiles) != 1 {
		t.Fatal("replaced clipboard reused stale configuration")
	}
}

func TestManagerClipboardBatchKeepsProxyAndSelection(t *testing.T) {
	a, tt := managerClipboardFixture(t)
	proxy := core.NewProxyProfile()
	proxy.Name, proxy.Settings.Kind, proxy.Settings.Host = "fixture proxy", "socks5", "proxy.fixture.invalid"
	a.config.Proxies = []core.ProxyProfile{proxy}
	a.config.Profiles[0].ProxyID = proxy.ID()
	second := core.NewProfile()
	second.Name, second.Host, second.Group, second.Kind = "second", "second.fixture.invalid", "", "sftp"
	a.config.Profiles = append(a.config.Profiles, second)
	if err := a.store.Save(a.config); err != nil {
		t.Fatal(err)
	}
	a.managerSelection.Add(a.config.Profiles[0].ID)
	a.managerSelection.Add(second.ID)
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyC)
	tt.Frame()
	if a.managerClipboard == nil || len(a.managerClipboard.profiles) != 2 {
		t.Fatal("batch copy did not capture both profiles")
	}
	a.navigateManager("target")
	tt.Frame()
	if err := tt.Click("会话列表"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyV)
	tt.Frame()
	waitOperator(t, a, func() bool { return !a.configurationBusy() })
	tt.Frame()
	if len(a.config.Profiles) != 4 || a.managerSelection.Len() != 2 || a.managerGroup != "target" || a.managerAll {
		t.Fatal("batch paste destination or selection incorrect", a.status)
	}
	if a.config.Profiles[2].ProxyID != proxy.ID() || len(a.config.Proxies) != 1 || a.config.Profiles[3].Kind != "sftp" {
		t.Fatal("paste lost proxy reference or protocol")
	}
}

func TestManagerClipboardContextMenuAndLock(t *testing.T) {
	a, tt := managerClipboardFixture(t)
	if err := tt.RightClick("clipboard-source"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("复制会话配置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.managerClipboard == nil {
		t.Fatal("context copy did not capture profile")
	}
	a.vault.Lock()
	if a.managerClipboardSnapshot(tt.Clipboard()) != nil || len(a.managerClipboardProfiles()) != 0 {
		t.Fatal("locked workspace allowed session clipboard access")
	}
	if err := a.vault.Unlock("fixture-master-password"); err != nil {
		t.Fatal(err)
	}
	a.navigateManager("target")
	tt.Frame()
	if err := tt.RightClick("会话列表"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("粘贴会话配置"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !a.configurationBusy() })
	if len(a.config.Profiles) != 2 || a.config.Profiles[1].Group != "target" {
		t.Fatal("context paste failed", a.status)
	}
}
