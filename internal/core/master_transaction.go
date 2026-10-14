package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// ChangeProtection commits the re-keyed credential vault and complete config in
// one durable intent. Recovery can finish publication without retaining a password.
func (s *Store) ChangeProtection(ctx context.Context, cfg Config, v *Vault, current, next, configRevision, vaultRevision string) error {
	if v == nil {
		return fmt.Errorf("密码库不可用")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.publishConfigurationState()
	v.mu.Lock()
	defer v.mu.Unlock()
	if s.pending || v.blocked {
		return ErrPendingTransaction
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	revision, err := s.Revision()
	if err != nil {
		return err
	}
	if revision != configRevision || v.revisionLocked() != vaultRevision {
		return fmt.Errorf("配置或密码库已变化，请重新设置主密码")
	}
	if _, err = s.encodingMasterLocked(); err != nil {
		return err
	}
	if s.LocalConflictDraftExists() {
		return fmt.Errorf("请先处理或放弃本地配置冲突草稿，再修改主密码")
	}
	options, err := SyncOptionsStatus(s.Dir)
	if err != nil {
		return err
	}
	pending, err := LoadPendingSyncDirectory(s.Dir)
	if err != nil {
		return err
	}
	if options.Enabled || pending != nil {
		return fmt.Errorf("请先停用同步并取消待生效目录设置")
	}
	if v.data.Master {
		if current == "" {
			return fmt.Errorf("请输入当前主密码")
		}
		if current == next {
			return fmt.Errorf("新主密码不能与原主密码相同")
		}
		if err = v.unlockLocked(current); err != nil {
			return err
		}
	} else if next == "" {
		return fmt.Errorf("尚未设置主密码")
	}
	data, key, err := v.prepareMasterLocked(next)
	if err != nil {
		return err
	}
	adopted := false
	defer func() {
		if !adopted {
			clear(key)
		}
	}()
	cfgBytes, err := EncodeConfiguration(cfg, []byte(next))
	if err != nil {
		return err
	}
	defer clear(cfgBytes)
	vaultBytes, err := marshalVaultFile(data, key, v.dir)
	if err != nil {
		return err
	}
	if len(vaultBytes) > MaxArchive {
		return fmt.Errorf("密码库过大")
	}
	txBytes, err := json.Marshal(stateTransaction{1, cfgBytes, vaultBytes})
	if err != nil {
		return err
	}
	defer clear(txBytes)
	if len(txBytes) > 48<<20 {
		return fmt.Errorf("配置事务过大")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	release, lockErr := lockStateDirectory(ctx, s.Dir, false)
	if lockErr != nil {
		return lockErr
	}
	defer release()
	if err = s.checkConfigurationRevision(configRevision); err != nil {
		return err
	}
	if err = v.checkDiskRevisionLocked(); err != nil {
		return err
	}
	if s.LocalConflictDraftExists() {
		return fmt.Errorf("请先处理或放弃本地配置冲突草稿，再修改主密码")
	}
	if err = AtomicWrite(filepath.Join(s.Dir, transactionFile), txBytes); err != nil {
		return err
	}
	v.acceptMasterLocked(data, key, next)
	adopted = true
	clear(s.master)
	s.master = []byte(next)
	s.protected = next != ""
	if err = recoverTransactionLocked(s.Dir); err != nil {
		s.pending = true
		v.blocked = true
		return fmt.Errorf("%w: %v", ErrPendingTransaction, err)
	}
	v.diskRevision = configurationDigest(vaultBytes)
	s.observeConfiguration(cfgBytes, cfg)
	return nil
}
