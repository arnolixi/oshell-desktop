package desktop

import (
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"testing"
)

func TestLinksMenuCreatesActualProfilesForEveryProtocol(t *testing.T) {
	for _, entry := range [][2]string{{"ssh", "新建 SSH 会话…"}, {"sftp", "新建 SFTP 会话…"}, {"ftp", "新建 FTP 会话…"}} {
		t.Run(entry[0], func(t *testing.T) {
			store := &core.Store{Dir: t.TempDir()}
			cfg := core.DefaultConfig()
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			a := newApp(store, cfg)
			a.managerGroup = "Links"
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Fill().Label("blank").ContextMenu(func(m *ui.Menu) { a.managerContextMenu(c, m, "") })
			}, 700, 400)
			if err := tt.RightClick("blank"); err != nil {
				t.Fatal(err)
			}
			if err := tt.ChooseMenuItem(entry[1]); err != nil {
				t.Fatal(err)
			}
			if !a.editing || a.draft.Kind != entry[0] || a.draft.Group != "Links" {
				t.Fatal("wrong protocol or directory")
			}
			a.draft.Host = "fixture.invalid"
			a.draft.Name = "actual"
			a.saveDraft(false)
			if a.editing || len(a.config.Profiles) != 1 || a.config.Profiles[0].Group != "Links" || len(a.config.Links) != 0 {
				t.Fatal("actual session became reference", a.formError)
			}
		})
	}
}
