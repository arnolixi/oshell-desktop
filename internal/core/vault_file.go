package core

import (
	"encoding/json"
	"fmt"
)

type protectedVaultFile struct {
	Version          int    `json:"version"`
	Master           bool   `json:"master"`
	Salt             []byte `json:"salt"`
	Check            []byte `json:"check"`
	ProtectedEntries []byte `json:"protectedEntries"`
	LocalEntries     []byte `json:"localEntries,omitempty"`
}

func marshalVaultFile(data vaultData, key []byte, dir string) ([]byte, error) {
	if !data.Master {
		return json.MarshalIndent(data, "", "  ")
	}
	if len(key) == 0 {
		return nil, ErrVaultLocked
	}
	protected, local := map[string]vaultEntry{}, map[string]vaultEntry{}
	for id, entry := range data.Entries {
		if entry.Local {
			local[id] = entry
		}
		protected[id] = entry
	}
	encode := func(entries map[string]vaultEntry, key []byte, aad string) ([]byte, error) {
		plain, err := json.Marshal(entries)
		if err != nil {
			return nil, err
		}
		defer clear(plain)
		return seal(key, plain, aad)
	}
	out := protectedVaultFile{Version: 2, Master: true, Salt: data.Salt, Check: data.Check}
	var err error
	out.ProtectedEntries, err = encode(protected, key, "OShell vault protected metadata v2")
	if err != nil {
		return nil, err
	}
	if len(local) > 0 {
		if localKey, keyErr := loadLocalVaultKey(dir); keyErr == nil {
			out.LocalEntries, err = encode(local, localKey, "OShell vault local metadata v2")
			clear(localKey)
			if err != nil {
				return nil, err
			}
		}
		// If the local key is missing, the master-protected index still retains all entries.
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if len(result) > MaxArchive {
		return nil, fmt.Errorf("密码库过大")
	}
	return result, err
}
func decodeVaultEntries(ciphertext, key []byte, aad string, local bool) (map[string]vaultEntry, error) {
	plain, err := unseal(key, ciphertext, aad)
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	var entries map[string]vaultEntry
	if err = json.Unmarshal(plain, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, fmt.Errorf("密码库索引无效")
	}
	for id, entry := range entries {
		if id == "" || local && !entry.Local {
			return nil, fmt.Errorf("密码库索引保护方式无效")
		}
	}
	return entries, nil
}
func validVaultFile(data []byte) bool {
	var header struct{ Version int }
	if json.Unmarshal(data, &header) != nil {
		return false
	}
	if header.Version == 1 {
		var v vaultData
		return json.Unmarshal(data, &v) == nil && v.Entries != nil
	}
	var v protectedVaultFile
	return header.Version == 2 && json.Unmarshal(data, &v) == nil && v.Master && len(v.Salt) == 32 && len(v.Check) >= 28 && len(v.ProtectedEntries) >= 28 && (len(v.LocalEntries) == 0 || len(v.LocalEntries) >= 28)
}
