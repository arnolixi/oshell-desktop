package desktop

import (
	"errors"
	"oshell-desktop/internal/core"
)

func startupConfiguration(store *core.Store) (core.Config, error) {
	cfg, err := store.Load()
	if errors.Is(err, core.ErrConfigurationLocked) {
		return core.DefaultConfig(), nil
	}
	return cfg, err
}
func (a *App) lockWorkspaceSecrets() {
	if a.store != nil {
		a.store.LockConfiguration()
	}
	if a.vault != nil {
		a.vault.Lock()
	}
	a.invalidateSecurityWindows()
}
