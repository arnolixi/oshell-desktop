package desktop

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

func sidebarContextFixture(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	cfg := core.DefaultConfig()
	p := core.NewProfile()
	p.Name, p.Host, p.Group = "sidebar-source", "fixture.invalid", "team/dev"
	cfg.Profiles = []core.Profile{p}
	cfg.Directories = []string{"team/dev", "target", "Links/shortcuts"}
	cfg.Links = []core.SessionLink{{ID: "reference", ProfileID: p.ID, Name: "sidebar-alias", Folder: "shortcuts"}}
	store := &core.Store{Dir: t.TempDir()}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.managerGroup, a.query, a.managerProtocol = "elsewhere", "unrelated", "ftp"
	a.managerSelection.Add("unrelated-id")
	a.revealSidebar(p.ID)
	a.revealSidebar("link:reference")
	tt := sidebarContextTester(a)
	return a, tt
}

func sidebarContextTester(a *App) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		// Match the workspace's stretching row so the sidebar scroll area has
		// height and pointer events reach the visible tree items.
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() { a.sidebarView(c) })
		a.sidebarRenameDialog(c)
	}, 500, 600)
}

func TestSidebarContextSessionActionsIgnoreManagerSelection(t *testing.T) {
	a, tt := sidebarContextFixture(t)
	if err := tt.RightClick("sidebar-source"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"连接", "打开 SFTP 文件", "属性…", "复制会话", "复制会话配置", "粘贴会话配置", "添加到快捷链接…", "移动到…", "重命名…", "删除…", "导出此会话…", "导入会话到当前目录…"} {
		if !slices.Contains(tt.Menu(), label) {
			t.Fatalf("missing %s: %v", label, tt.Menu())
		}
	}
	if a.sidebar.context != nil || a.managerSelection.Len() != 1 || !a.managerSelection.Has("unrelated-id") {
		t.Fatal("sidebar menu leaked its target into the manager")
	}
	if err := tt.ChooseMenuItem("属性…"); err != nil {
		t.Fatal(err)
	}
	if !a.editing || a.draft.ID != a.config.Profiles[0].ID || !a.features.managerReturn.sidebar {
		t.Fatal("wrong editor target or return destination")
	}
	a.editing = false
	a.restoreManagerReturn(nil)
	if a.managerGroup != "elsewhere" || a.query != "unrelated" || a.managerProtocol != "ftp" || !a.managerSelection.Has("unrelated-id") {
		t.Fatal("sidebar edit changed manager filters or selection")
	}
}

func TestSidebarDirectoryCreationUsesClickedFolder(t *testing.T) {
	a, tt := sidebarContextFixture(t)
	if err := tt.RightClick("dev"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("新建 SSH 会话…"); err != nil {
		t.Fatal(err)
	}
	if a.draft.Group != "team/dev" || !a.features.managerReturn.sidebar {
		t.Fatal("new session used the manager directory")
	}
	a.editing = false
	a.restoreManagerReturn(nil)
	if err := tt.RightClick("dev"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("新建目录…"); err != nil {
		t.Fatal(err)
	}
	if a.features.directorySource != "team/dev" || !a.features.directorySidebar {
		t.Fatal("new folder lost its parent")
	}
	a.features.directoryName = "nested"
	a.applyDirectory()
	if a.features.directoryOpen || !slices.Contains(a.config.AllDirectories(), "team/dev/nested") || a.managerGroup != "elsewhere" {
		t.Fatal("folder creation failed or changed manager location", a.status)
	}
	if a.sidebar.selected != "folder:team/dev/nested" {
		t.Fatal("new folder was not revealed")
	}
}

func TestSidebarMoveCapturesOnlyClickedRow(t *testing.T) {
	a, tt := sidebarContextFixture(t)
	if err := tt.RightClick("sidebar-source"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("移动实际会话到…"); err != nil {
		t.Fatal(err)
	}
	rows := a.features.directoryRows
	if len(rows) != 1 || rows[0].profile.ID != a.config.Profiles[0].ID {
		t.Fatal("move captured another window's selection")
	}
	a.managerSelection.Clear()
	a.features.directoryName = "/target"
	a.applyDirectory()
	if a.features.directoryOpen || a.config.Profiles[0].Group != "target" || a.config.Links[0].Folder != "shortcuts" {
		t.Fatal("move failed or changed a reference", a.status)
	}
}

func TestSidebarClipboardRebindsPasswordAndPreservesManager(t *testing.T) {
	a, _ := managerClipboardFixture(t)
	source := a.config.Profiles[0]
	a.managerGroup, a.query, a.managerAll = "elsewhere", "unrelated", true
	tt := sidebarContextTester(a)
	if err := tt.RightClick(source.Name); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("复制会话配置"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tt.Clipboard(), "oshell-session-configuration:") || strings.Contains(tt.Clipboard(), source.Host) {
		t.Fatal("clipboard exposed configuration")
	}
	if err := tt.RightClick("target"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("粘贴会话配置"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !a.configurationBusy() })
	if len(a.config.Profiles) != 2 {
		t.Fatal("paste failed", a.status)
	}
	copy := a.config.Profiles[1]
	secret, err := a.vault.GetFor(copy, source.Identity())
	if copy.ID == source.ID || copy.Group != "target" || secret != "fixture-secret" || err != nil {
		t.Fatal("paste lost destination or credential identity", err)
	}
	if a.managerGroup != "elsewhere" || a.query != "unrelated" || !a.managerAll || a.sidebar.selected != copy.ID {
		t.Fatal("paste updated the manager instead of revealing the sidebar copy")
	}
}

func TestSidebarRenameUsesSharedValidationAndKeepsReferenceIdentity(t *testing.T) {
	a, tt := sidebarContextFixture(t)
	if err := tt.RightClick("sidebar-alias"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("重命名…"); err != nil {
		t.Fatal(err)
	}
	if a.sidebar.rename == nil || a.managerRename != nil || !a.modalActive() {
		t.Fatal("sidebar rename did not open its own editor")
	}
	tt.Frame()
	tt.Type("bad/name")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.sidebar.rename == nil || a.sidebar.rename.error == "" || a.config.Links[0].Name != "sidebar-alias" {
		t.Fatal("invalid rename accepted")
	}
	if err := tt.Click("新名称"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("renamed-alias")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.sidebar.rename != nil || a.config.Links[0].Name != "renamed-alias" || a.config.Profiles[0].Name != "sidebar-source" || a.authOpen {
		t.Fatal("rename changed the source or activated a connection", a.status)
	}
	if err := tt.RightClick("renamed-alias"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("导出此快捷引用…"); err != nil {
		t.Fatal(err)
	}
	export := a.features.archive.exportConfig
	if export == nil || len(export.Links) != 1 || export.Links[0].ID != "reference" || len(export.Profiles) != 1 {
		t.Fatal("reference export did not preserve its source")
	}
}

func TestSidebarContextRejectsStaleRowsAndProtectsLinksRoot(t *testing.T) {
	a, tt := sidebarContextFixture(t)
	called := false
	if a.withSidebarContext("missing", func() { called = true }) || called {
		t.Fatal("missing sidebar target reused manager selection")
	}
	a.withSidebarContext("folder:Links", func() {
		if a.managerSelectionMutable() {
			t.Fatal("Links root became mutable")
		}
		a.beginManagerRename(a.sidebar.context.row)
	})
	if a.sidebar.rename != nil {
		t.Fatal("Links root can be renamed")
	}
	if err := tt.RightClick("会话侧栏列表"); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(tt.Menu(), "删除…") || slices.Contains(tt.Menu(), "导出此会话…") {
		t.Fatal("blank space retained a stale target", tt.Menu())
	}
	if err := tt.ChooseMenuItem("新建 FTP 会话…"); err != nil {
		t.Fatal(err)
	}
	if a.draft.Group != "" {
		t.Fatal("blank-space creation used stale directory")
	}
}

func TestSidebarFolderRenameAndEscape(t *testing.T) {
	a, tt := sidebarContextFixture(t)
	if err := tt.Click("dev"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyF2)
	tt.Frame()
	if a.sidebar.rename == nil {
		t.Fatal("F2 did not start folder rename")
	}
	tt.Type("cancelled")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.sidebar.rename != nil || a.config.Profiles[0].Group != "team/dev" {
		t.Fatal("Escape changed the directory")
	}
	if err := tt.RightClick("dev"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("重命名…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("renamed")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.sidebar.rename != nil || a.config.Profiles[0].Group != "team/renamed" || a.config.Links[0].ProfileID != a.config.Profiles[0].ID {
		t.Fatal("folder rename lost contents or reference identity", a.status)
	}
	if !tt.HasText("sidebar-source") || a.sidebar.selected != "folder:team/renamed" {
		t.Fatal("renamed directory was not expanded")
	}
}

func TestSidebarEmptyFolderDeletionKeepsSessions(t *testing.T) {
	a, tt := sidebarContextFixture(t)
	if err := tt.RightClick("target"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除…"); err != nil {
		t.Fatal(err)
	}
	waitOperator(t, a, func() bool { return !a.configurationBusy() })
	if slices.Contains(a.config.AllDirectories(), "target") || len(a.config.Profiles) != 1 || len(a.config.Links) != 1 {
		t.Fatal("empty folder deletion affected unrelated sessions", a.status)
	}
}
