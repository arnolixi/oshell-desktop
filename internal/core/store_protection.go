package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func readConfigurationFile(file string) ([]byte, error) {
	input, err := openImportFile(file)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("配置不是普通文件")
	}
	data, err := io.ReadAll(io.LimitReader(input, MaxSharedVault+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxSharedVault {
		return nil, fmt.Errorf("配置文件过大")
	}
	return data, nil
}
func (s *Store) UnlockConfiguration(master []byte) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.publishConfigurationState()
	data, err := readConfigurationFile(filepath.Join(s.Dir, "config.json"))
	if err != nil {
		return DefaultConfig(), err
	}
	defer clear(data)
	if s.protected && !IsSharedVault(data) {
		return DefaultConfig(), fmt.Errorf("受保护配置被替换为未加密内容，已停止载入")
	}
	if IsSharedVault(data) {
		s.protected = true
	}
	cfg, protected, err := DecodeConfiguration(data, master)
	if err != nil {
		return cfg, err
	}
	clear(s.master)
	s.master = nil
	s.protected = protected
	if protected {
		s.master = append([]byte(nil), master...)
	}
	s.observeConfiguration(data, cfg)
	return cfg, nil
}
func (s *Store) LockConfiguration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.publishConfigurationState()
	clear(s.master)
	s.master = nil
}

// ProtectConfiguration is a revision-guarded storage transition. Coordinating it
// with a vault master change requires the master-management transaction path.
func (s *Store) ProtectConfiguration(cfg Config, master []byte, expectedRevision string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.publishConfigurationState()
	if s.pending {
		return ErrPendingTransaction
	}
	if _, err := s.encodingMasterLocked(); err != nil {
		return err
	}
	revision, err := s.Revision()
	if err != nil {
		return err
	}
	if revision != expectedRevision {
		return fmt.Errorf("配置已变化，请重新载入")
	}
	if len(master) == 0 {
		return ErrConfigurationLocked
	}
	data, err := EncodeConfiguration(cfg, master)
	if err != nil {
		return err
	}
	release, lockErr := lockStateDirectory(context.Background(), s.Dir, false)
	if lockErr != nil {
		return lockErr
	}
	defer release()
	if err = s.checkConfigurationRevision(expectedRevision); err != nil {
		return err
	}
	if err = AtomicWrite(filepath.Join(s.Dir, "config.json"), data); err != nil {
		return err
	}
	clear(s.master)
	s.master = append([]byte(nil), master...)
	s.protected = true
	s.observeConfiguration(data, cfg)
	return nil
}
func validTransactionConfiguration(data []byte) bool {
	if IsSharedVault(data) {
		var e sharedEnvelope
		return json.Unmarshal(data, &e) == nil && e.Version == 1 && e.Iterations >= 600000 && e.Iterations <= 2000000 && len(e.Salt) == 16 && len(e.Ciphertext) >= 28 && len(e.Ciphertext) <= MaxArchive+28
	}
	var cfg Config
	return len(data) <= MaxArchive && json.Unmarshal(data, &cfg) == nil && cfg.Validate() == nil
}
func (s *Store) encodingMasterLocked() ([]byte, error) {
	defer s.publishConfigurationState()
	data, err := readConfigurationFile(filepath.Join(s.Dir, "config.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	defer clear(data)
	if IsSharedVault(data) {
		s.protected = true
	}
	if s.protected && len(s.master) == 0 {
		return nil, ErrConfigurationLocked
	}
	if s.protected {
		return s.master, nil
	}
	return nil, nil
}

func (s *Store) ConfigurationProtection() (protected, locked bool) {
	state := s.protectionState.Load()
	return state&1 != 0, state == 1
}
func (s *Store) publishConfigurationState() {
	state := uint32(0)
	if s.protected {
		state = 1
		if len(s.master) > 0 {
			state = 3
		}
	}
	s.protectionState.Store(state)
}
