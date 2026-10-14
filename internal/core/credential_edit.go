package core

import (
	"context"
	"fmt"
)

func (v *Vault) HasStoredCredential(id string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.data.Entries[id]
	return ok && len(entry.Data) > 0
}
func sameCredentialDestination(a, b Profile) bool {
	ssh := func(kind string) bool { return kind == "ssh" || kind == "sftp" }
	return a.Host == b.Host && a.Port == b.Port && a.Username == b.Username && (a.Kind == b.Kind || ssh(a.Kind) && ssh(b.Kind))
}

// PrepareCredentialEdit never alters the vault. A blank password preserves an
// existing envelope only for the same destination; caller commits the config
// and returned changes together, guarded by both revisions.
func (v *Vault) PrepareCredentialEdit(original *Profile, destination Profile, password string, remember bool, identity SSHIdentity, protection ...string) ([]CredentialUpdate, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, exists := v.data.Entries[destination.ID]
	mode := ""
	if len(protection) > 0 {
		mode = protection[0]
	}
	local, modeErr := v.localProtectionLocked(destination.ID, mode)
	if remember && modeErr != nil {
		return nil, modeErr
	}
	if !remember {
		if exists {
			return []CredentialUpdate{{Profile: destination, Delete: true}}, nil
		}
		return nil, nil
	}
	if password != "" {
		return []CredentialUpdate{{Profile: destination, Password: password, Identity: identity, Protection: mode}}, nil
	}
	if original == nil || original.ID != destination.ID || !exists || entry.Identity != credentialIdentity(*original) {
		return nil, fmt.Errorf("请输入要保存的密码，或取消保存")
	}
	if !sameCredentialDestination(*original, destination) {
		return nil, fmt.Errorf("连接目标已改变，请重新输入密码或取消保存")
	}
	if original.Kind == destination.Kind && entry.Local == local {
		return nil, nil
	}
	// SSH -> SFTP is one destination, but this vault's AAD also contains kind.
	if entry.Resolved != nil && *entry.Resolved != identity || entry.Resolved == nil && original.Identity() != identity {
		return nil, fmt.Errorf("SSH 配置解析出的目标已改变，请重新输入密码")
	}
	if len(v.key) == 0 && !entry.Local {
		return nil, ErrVaultLocked
	}
	plain, err := v.decryptEntryLocked(original.ID, entry)
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	return []CredentialUpdate{{Profile: destination, Password: string(plain), Identity: identity, Protection: mode}}, nil
}
func (s *Store) CommitCredentialEdit(ctx context.Context, cfg Config, v *Vault, changes []CredentialUpdate, configRevision, vaultRevision string) error {
	return s.commitCredentials(ctx, cfg, v, changes, configRevision, vaultRevision)
}
