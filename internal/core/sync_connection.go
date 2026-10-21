package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SaveSyncConnection serializes credentials and connection settings against
// other writers and restores the previous credential file on settings failure.
func SaveSyncConnection(v *Vault, expected, next SyncOptions, password string, vaultRevision ...string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(vaultRevision) > 0 && v.revisionLocked() != vaultRevision[0] {
		return fmt.Errorf("同步凭据已被修改，请重新打开设置")
	}
	if v.blocked {
		return ErrPendingTransaction
	}
	if next.Enabled && (!v.data.Master || len(v.master) == 0) {
		return ErrVaultLocked
	}
	release, err := lockStateDirectory(context.Background(), v.dir, false)
	if err != nil {
		return err
	}
	defer release()
	saved, err := LoadSyncOptions(v.dir, v.master)
	if err != nil {
		return err
	}
	if saved != expected {
		return fmt.Errorf("同步设置已被其他窗口修改，请重新打开设置")
	}
	if next.Enabled {
		if _, err = next.Transport(password, v.dir); err != nil {
			return err
		}
	}
	if err = v.checkDiskRevisionLocked(); err != nil {
		return err
	}
	data := v.data
	data.Entries = make(map[string]vaultEntry, len(v.data.Entries))
	for id, entry := range v.data.Entries {
		data.Entries[id] = entry
	}
	p := next.CredentialProfile()
	changed := false
	if next.Enabled && next.Backend == "webdav" && password != "" {
		identity := p.Identity()
		entry := vaultEntry{Identity: credentialIdentity(p), Resolved: &identity}
		entry.Data, err = v.encryptEntryLocked(p.ID, entry, []byte(password))
		if err != nil {
			return err
		}
		data.Entries[p.ID] = entry
		changed = true
	} else if !next.Enabled || next.Backend != "webdav" {
		if _, ok := data.Entries[p.ID]; ok {
			delete(data.Entries, p.ID)
			changed = true
		}
	}
	var previous, encoded []byte
	path := filepath.Join(v.dir, "credentials.json")
	if changed {
		previous, err = ReadBounded(path)
		if err != nil {
			return err
		}
		encoded, err = marshalVaultFile(data, v.key, v.dir)
		if err != nil {
			return err
		}
		if err = AtomicWrite(path, encoded); err != nil {
			return err
		}
	}
	if err = saveSyncOptionsLocked(v.dir, next, v.master); err != nil {
		if changed {
			if rollback := AtomicWrite(path, previous); rollback != nil {
				return errors.Join(err, fmt.Errorf("恢复同步凭据失败：%w", rollback))
			}
		}
		return err
	}
	if changed {
		v.data = data
		v.diskRevision = configurationDigest(encoded)
	}
	return nil
}

// Detect an existing encrypted object without mistaking an evicted cloud file
// or a directory containing unrelated files for an empty target.
func ValidateSyncDirectoryTarget(local, target string, master []byte) (*PendingSyncDirectory, error) {
	mode := "existing"
	if _, err := os.Lstat(filepath.Join(target, "configuration.json")); os.IsNotExist(err) {
		mode = "create"
	} else if err != nil {
		return nil, err
	}
	return ValidateSyncDirectorySelection(local, target, mode, master)
}
