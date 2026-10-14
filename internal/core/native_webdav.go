package core

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
)

func decodeNativeWebDAV(data, master []byte) (SyncOptions, string, error) {
	options := SyncOptions{Version: 1, Backend: "webdav", Enabled: true}
	plain, err := OpenShared(data, master)
	if err != nil {
		return options, "", err
	}
	defer clear(plain)
	var stored struct{ Address, Username, Password string }
	if err = json.Unmarshal(plain, &stored); err != nil {
		return options, "", err
	}
	options.Address = stored.Address
	options.Username = stored.Username
	if _, err = options.Transport(stored.Password, ""); err != nil {
		return options, "", err
	}
	return options, stored.Password, nil
}

// Restoring enables the existing endpoint without making a network request.
// The ordinary sync engine handles its first fetch/merge after this returns.
func (s *Store) RestoreNativeWebDAV(snapshot NativeStorageSnapshot, sourceMaster []byte, vault *Vault) error {
	if vault == nil {
		return fmt.Errorf("密码库不可用")
	}
	expectedRevision := vault.Revision()
	if snapshot.WebDAV == nil || len(snapshot.webDAVData) == 0 {
		return fmt.Errorf("原版没有 WebDAV 设置")
	}
	master, err := vault.MasterSecret()
	if err != nil {
		return err
	}
	defer clear(master)
	if subtle.ConstantTimeCompare(master, sourceMaster) != 1 {
		return fmt.Errorf("当前主密码与原版不同，不能恢复原 WebDAV 同步；请先统一主密码或使用新的同步目标")
	}
	active, err := LoadSyncOptions(s.Dir, master)
	if err != nil {
		return err
	}
	pending, err := LoadPendingSyncDirectory(s.Dir)
	if err != nil {
		return err
	}
	if active.Enabled || pending != nil {
		return fmt.Errorf("请先停用当前同步并取消待生效目录")
	}
	options, password, err := decodeNativeWebDAV(snapshot.webDAVData, sourceMaster)
	if err != nil {
		return err
	}
	profile := options.CredentialProfile()
	if err = vault.PutForRevision(profile, password, profile.Identity(), expectedRevision, "master"); err != nil {
		return err
	}
	password = ""
	return SaveSyncOptions(s.Dir, options, master)
}
