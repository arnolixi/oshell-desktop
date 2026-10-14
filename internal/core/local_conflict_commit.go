package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Guard runs while the store and the private disk-vault snapshot are locked.
// It retains the original vault lock until publication completes.
func (p *LocalResolutionPlan) guard(s *Store) (func(), error) {
	p.original.mu.Lock()
	fail := func(err error) (func(), error) { p.original.mu.Unlock(); return nil, err }
	if p.original.blocked || s.pending {
		return fail(ErrPendingTransaction)
	}
	if p.original.data.Master && len(p.original.key) == 0 {
		return fail(ErrVaultLocked)
	}
	if p.original.revisionLocked() != p.originalRevision || s.observedRevision != p.ObservedRevision {
		return fail(ErrConfigurationChanged)
	}
	revision, err := s.Revision()
	if err != nil {
		return fail(err)
	}
	if revision != p.DiskRevision {
		return fail(ErrConfigurationChanged)
	}
	raw, err := ReadBounded(filepath.Join(s.Dir, "credentials.json"))
	if err != nil && !os.IsNotExist(err) {
		return fail(err)
	}
	digest := ""
	if err == nil {
		digest = configurationDigest(raw)
	}
	clear(raw)
	if digest != p.DiskVaultRevision {
		return fail(ErrConfigurationChanged)
	}
	if _, err = os.Lstat(filepath.Join(s.Dir, transactionFile)); err == nil {
		return fail(ErrConfigurationChanged)
	} else if !os.IsNotExist(err) {
		return fail(err)
	}
	return p.original.mu.Unlock, nil
}

// Commit returns the replacement vault only after config/credentials publish.
// The caller must replace its old shared vault and lock that superseded instance.
func (p *LocalResolutionPlan) Commit(ctx context.Context, s *Store, choices map[string]SharedMergeChoice) (Config, *Vault, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cfg, updates, err := p.resolveCredentialsLocked(choices)
	defer func() {
		for i := range updates {
			updates[i].Password = ""
		}
	}()
	if err != nil {
		return Config{}, nil, err
	}
	remote := p.remoteVault
	err = s.commitCredentials(ctx, cfg, remote, updates, p.DiskRevision, remote.Revision(), func() (func(), error) { return p.guard(s) })
	if err != nil {
		return Config{}, nil, err
	}
	p.remoteVault = nil
	p.closed = true
	clear(p.localVault.key)
	p.localVault.key = nil
	clear(p.localVault.master)
	p.localVault.master = nil
	return cfg, remote, nil
}

// ReloadDisk abandons the proposed draft without writing the disk artifacts.
func (p *LocalResolutionPlan) ReloadDisk(s *Store) (Config, *Vault, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return Config{}, nil, fmt.Errorf("冲突计划已关闭")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	release, err := p.guard(s)
	if err != nil {
		return Config{}, nil, err
	}
	defer release()
	unlock, err := lockStateDirectory(context.Background(), s.Dir, false)
	if err != nil {
		return Config{}, nil, err
	}
	defer unlock()
	if err := p.remoteVault.checkDiskRevisionLocked(); err != nil {
		return Config{}, nil, err
	}
	raw, err := readConfigurationFile(filepath.Join(s.Dir, "config.json"))
	if err != nil {
		return Config{}, nil, err
	}
	defer clear(raw)
	if configurationDigest(raw) != p.DiskRevision {
		return Config{}, nil, ErrConfigurationChanged
	}
	s.observeConfiguration(raw, p.Remote)
	remote := p.remoteVault
	p.remoteVault = nil
	p.closed = true
	clear(p.localVault.key)
	p.localVault.key = nil
	clear(p.localVault.master)
	p.localVault.master = nil
	return p.Remote.Clone(), remote, nil
}
