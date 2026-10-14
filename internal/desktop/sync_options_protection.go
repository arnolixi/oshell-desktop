package desktop

import (
	"fmt"
	"oshell-desktop/internal/core"
)

func (a *App) syncOptionsMaster() ([]byte, error) {
	protected, locked := a.vault.State()
	if !protected {
		return nil, nil
	}
	if locked {
		return nil, core.ErrSyncSettingsLocked
	}
	return a.vault.MasterSecret()
}
func (a *App) loadSyncOptions() (core.SyncOptions, error) {
	master, err := a.syncOptionsMaster()
	if err != nil {
		return core.SyncOptions{}, err
	}
	defer clear(master)
	return core.LoadAndProtectSyncOptions(a.store.Dir, master)
}
func (a *App) saveSyncOptions(options core.SyncOptions) error {
	master, err := a.syncOptionsMaster()
	if err != nil {
		return err
	}
	defer clear(master)
	if options.Enabled && options.Backend == "webdav" && len(master) == 0 {
		return fmt.Errorf("请先设置并解锁主密码")
	}
	if err := core.SaveSyncOptions(a.store.Dir, options, master); err != nil {
		return err
	}
	if options.Backend == "webdav" && !options.Enabled {
		id := options.CredentialProfile().ID
		if a.vault.HasStoredCredential(id) {
			return a.vault.Delete([]string{id})
		}
	}
	return nil
}
