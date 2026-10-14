package core

import (
	"context"
	"errors"
)

// LocalStateChanged avoids replacing vault/runtime state on unchanged activation.
func (s *Store) LocalStateChanged(v *Vault) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v == nil {
		return true, nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	release, err := lockStateDirectory(context.Background(), s.Dir, false)
	if err != nil {
		return false, err
	}
	defer release()
	revision, err := s.Revision()
	if err != nil {
		return false, err
	}
	if !s.observed || revision != s.observedRevision {
		return true, nil
	}
	if err := v.checkDiskRevisionLocked(); err != nil {
		if errors.Is(err, ErrVaultChanged) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
