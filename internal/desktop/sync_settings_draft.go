package desktop

import (
	"fmt"
)

func (a *App) syncOptionsChanged() bool {
	f := &a.features.sharedSync
	return f.loaded && (f.options != f.savedOptions || f.password != "")
}

func (a *App) validateSyncDraft() error {
	if err := a.validateDirectoryDraft(); err != nil {
		return err
	}
	f := &a.features.sharedSync
	if !a.syncOptionsChanged() && !f.directoryChanged {
		return nil
	}
	saved, err := a.loadSyncOptions()
	if err != nil {
		return err
	}
	if f.loaded && saved != f.savedOptions {
		return fmt.Errorf("同步设置已被其他窗口修改，请取消并重新打开设置")
	}
	options := f.options
	if !f.loaded {
		options = saved
	}
	if f.directoryDisabled && options.Backend == "directory" {
		options.Enabled = false
	}
	if options.Enabled {
		protected, locked := a.vault.State()
		if !protected || locked {
			return fmt.Errorf("请先启用并解锁主密码保护")
		}
		if _, err := options.Transport(f.password, a.store.Dir); err != nil {
			return err
		}
	}
	if options.Enabled && options.Backend == "webdav" {
		if f.pendingDirectory != nil || saved.Enabled && saved.Backend == "directory" && !f.directoryDisabled {
			return fmt.Errorf("请先停用目录同步并取消待生效目录设置，再启用 WebDAV")
		}
	}
	if f.directoryChanged && f.pendingDirectory != nil && options.Enabled && options.Backend == "webdav" {
		return fmt.Errorf("请先取消启用 WebDAV，再选择目录同步")
	}
	return nil
}

func (a *App) applySyncDraft(done func()) {
	f := &a.features.sharedSync
	if err := a.validateSyncDraft(); err != nil {
		a.features.settingsError = err.Error()
		return
	}
	changed := a.syncOptionsChanged()
	if f.directoryDisabled && f.options.Backend == "directory" {
		f.options.Enabled = false
		changed = f.loaded && f.options != f.savedOptions
	}
	if err := a.applyDirectoryDraft(); err != nil {
		a.features.settingsError = err.Error()
		return
	}
	if changed {
		a.saveSharedSyncSettingsThen(done)
		return
	}
	done()
}
