package desktop

import "oshell-desktop/internal/core"

// Directory choices are only published by the settings Apply action.
func (a *App) applyDirectoryDraft() error {
	f := &a.features.sharedSync
	if !f.directoryChanged {
		return nil
	}
	if f.directoryDisabled {
		options, err := a.loadSyncOptions()
		if err != nil {
			return err
		}
		if options.Backend == "directory" {
			options.Enabled = false
			if err = a.saveSyncOptions(options); err != nil {
				return err
			}
		}
	}
	if f.pendingDirectory == nil {
		if err := core.CancelPendingSyncDirectory(a.store.Dir); err != nil {
			return err
		}
	} else {
		master, err := a.vault.MasterSecret()
		if err != nil {
			return err
		}
		defer clear(master)
		pending := f.pendingDirectory
		if err = core.ScheduleSyncDirectory(a.store.Dir, pending.Path, pending.Mode, master); err != nil {
			return err
		}
	}
	f.directoryChanged = false
	f.directoryDisabled = false
	return nil
}

func (a *App) validateDirectoryDraft() error {
	f := &a.features.sharedSync
	if !f.directoryChanged || f.pendingDirectory == nil {
		return nil
	}
	master, err := a.vault.MasterSecret()
	if err != nil {
		return err
	}
	defer clear(master)
	pending := f.pendingDirectory
	_, err = core.ValidateSyncDirectorySelection(a.store.Dir, pending.Path, pending.Mode, master)
	return err
}
