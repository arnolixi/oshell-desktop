package core

import "fmt"

// Empty mode preserves the existing entry and the legacy vault default.
func (v *Vault) localProtectionLocked(id, mode string) (bool, error) {
	switch mode {
	case "":
		return v.data.Entries[id].Local, nil
	case "local":
		return true, nil
	case "master":
		if !v.data.Master || len(v.key) == 0 {
			return false, ErrVaultLocked
		}
		return false, nil
	default:
		return false, fmt.Errorf("密码保护方式无效")
	}
}
func (v *Vault) CredentialProtection(id string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	if entry, ok := v.data.Entries[id]; ok {
		if entry.Local || !v.data.Master {
			return "local"
		}
		return "master"
	}
	return "local"
}
func (v *Vault) decryptEntryLocked(id string, entry vaultEntry) ([]byte, error) {
	key := v.key
	if entry.Local {
		var err error
		key, err = loadLocalVaultKey(v.dir)
		if err != nil {
			return nil, err
		}
		defer clear(key)
	}
	if len(key) == 0 {
		return nil, ErrVaultLocked
	}
	return unseal(key, entry.Data, entryAAD(id, entry))
}
func (v *Vault) encryptEntryLocked(id string, entry vaultEntry, plain []byte) ([]byte, error) {
	key := v.key
	if entry.Local {
		var err error
		key, err = localVaultKey(v.dir)
		if err != nil {
			return nil, err
		}
		defer clear(key)
	}
	if len(key) == 0 {
		return nil, ErrVaultLocked
	}
	return seal(key, plain, entryAAD(id, entry))
}
