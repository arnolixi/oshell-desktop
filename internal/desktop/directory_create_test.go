package desktop

import (
	"oshell-desktop/internal/core"
	"slices"
	"testing"
)

func TestCreateSessionDirectoryUsesSingleNameInCapturedParent(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.Directories = []string{"production"}
	store := &core.Store{Dir: t.TempDir()}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	a := newApp(store, cfg)
	a.openDirectory("新建目录", "production")
	a.managerGroup = ""
	for _, bad := range []string{"../escape", "nested/child", "..", ""} {
		a.features.directoryName = bad
		a.applyDirectory()
		if !a.features.directoryOpen || a.features.directoryError == "" || len(a.config.Directories) != 1 {
			t.Fatal("invalid name accepted", bad)
		}
	}
	a.features.directoryName = "数据库"
	a.applyDirectory()
	if a.features.directoryOpen || !slices.Contains(a.config.Directories, "production/数据库") {
		t.Fatal("wrong destination", a.features.directoryError)
	}
	a.openDirectory("新建目录", "")
	a.features.directoryName = "root-child"
	a.applyDirectory()
	if !slices.Contains(a.config.Directories, "root-child") {
		t.Fatal("root create failed")
	}
	a.openDirectory("新建目录", "removed")
	a.features.directoryName = "child"
	a.applyDirectory()
	if !a.features.directoryOpen || a.features.directoryError == "" {
		t.Fatal("missing parent recreated")
	}
}
