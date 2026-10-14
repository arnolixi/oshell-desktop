package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) AdoptNativeStorage(ctx context.Context, snapshot NativeStorageSnapshot, master []byte, previous Config, vault *Vault, configRevision, vaultRevision string) (Config, string, error) {
	if vault == nil {
		return previous, "", fmt.Errorf("密码库不可用")
	}
	protected, locked := vault.State()
	if !protected || locked {
		return previous, "", fmt.Errorf("请先设置并解锁本地主密码")
	}
	options, err := SyncOptionsStatus(s.Dir)
	if err != nil {
		return previous, "", err
	}
	pending, err := LoadPendingSyncDirectory(s.Dir)
	if err != nil {
		return previous, "", err
	}
	if options.Enabled || pending != nil {
		return previous, "", fmt.Errorf("请先停用同步并取消待生效目录设置")
	}
	cfg, updates, err := snapshot.prepareReplacement(ctx, previous, master)
	defer func() {
		for i := range updates {
			updates[i].Password = ""
		}
	}()
	if err != nil {
		return previous, "", err
	}
	for i := range updates {
		if !updates[i].Delete {
			updates[i].Protection = "master"
		}
	}
	backup, err := s.backupNativeAdoption(vault, configRevision, vaultRevision)
	if err != nil {
		return previous, "", err
	}
	if err = ctx.Err(); err == nil {
		err = s.CommitCredentialEdit(ctx, cfg, vault, updates, configRevision, vaultRevision)
	}
	if err != nil {
		return previous, backup, err
	}
	return cfg, backup, nil
}
func (s *Store) backupNativeAdoption(vault *Vault, configRevision, vaultRevision string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault.mu.Lock()
	defer vault.mu.Unlock()
	revision, err := s.Revision()
	if err != nil {
		return "", err
	}
	if revision != configRevision || vault.revisionLocked() != vaultRevision {
		return "", fmt.Errorf("本地配置或密码库已变化，请重新预览迁移")
	}
	if err := vault.checkDiskRevisionLocked(); err != nil {
		return "", err
	}
	if s.pending || vault.blocked {
		return "", ErrPendingTransaction
	}
	cfg, err := readConfigurationFile(filepath.Join(s.Dir, "config.json"))
	if os.IsNotExist(err) {
		cfg, err = json.Marshal(DefaultConfig())
	}
	if err != nil {
		return "", err
	}
	credentials, err := marshalVaultFile(vault.data, vault.key, vault.dir)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(stateTransaction{Version: 1, Config: cfg, Vault: credentials})
	if err != nil {
		return "", err
	}
	directory := filepath.Join(s.Dir, "storage-backups")
	if err = os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	file := filepath.Join(directory, "before-native-"+NewID()+".json")
	if err = AtomicWrite(file, data); err != nil {
		return "", err
	}
	return file, nil
}
