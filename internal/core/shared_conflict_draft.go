package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type SharedConflictDraft struct {
	Version       int             `json:"version"`
	Target        string          `json:"target"`
	Local         SharedDocument  `json:"local"`
	Base          *SharedDocument `json:"base,omitempty"`
	Revision      string          `json:"revision"`
	VaultRevision string          `json:"vaultRevision"`
}
type SharedConflictStore struct{ Directory, Target string }

func (s SharedConflictStore) path() string {
	digest := sha256.Sum256([]byte(s.Target))
	return filepath.Join(s.Directory, "sync-conflicts", hex.EncodeToString(digest[:])+".json")
}
func (s SharedConflictStore) Exists() bool { _, err := os.Lstat(s.path()); return err == nil }
func (s SharedConflictStore) Load(master []byte) (*SharedConflictDraft, error) {
	file, err := openImportFile(s.path())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxSharedVault+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxSharedVault {
		return nil, errors.New("同步冲突草稿超过大小限制")
	}
	plain, err := OpenShared(data, master)
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	var draft SharedConflictDraft
	if err = json.Unmarshal(plain, &draft); err != nil {
		return nil, errors.New("同步冲突草稿无效")
	}
	if err = s.validate(draft, master); err != nil {
		return nil, err
	}
	return &draft, nil
}
func (s SharedConflictStore) validate(d SharedConflictDraft, master []byte) error {
	if d.Version != 1 || d.Target != s.Target || d.Revision == "" || d.VaultRevision == "" {
		return errors.New("同步冲突草稿与目标不匹配")
	}
	if err := d.Local.Validate(master); err != nil {
		return err
	}
	if d.Base != nil {
		return d.Base.Validate(master)
	}
	return nil
}
func (s SharedConflictStore) Save(d SharedConflictDraft, master []byte) error {
	if err := s.validate(d, master); err != nil {
		return err
	}
	plain, err := json.Marshal(d)
	if err != nil {
		return err
	}
	defer clear(plain)
	if len(plain) > MaxSharedVault {
		return fmt.Errorf("同步冲突草稿超过大小限制")
	}
	encrypted, err := SealShared(plain, master)
	if err != nil {
		return err
	}
	if len(encrypted) > MaxSharedVault {
		return fmt.Errorf("同步冲突草稿超过大小限制")
	}
	if err = os.MkdirAll(filepath.Dir(s.path()), 0700); err != nil {
		return err
	}
	return AtomicWrite(s.path(), encrypted)
}
func (s SharedConflictStore) Clear() error {
	err := os.Remove(s.path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (e *SharedSyncEngine) ConflictStore() SharedConflictStore {
	return SharedConflictStore{Directory: e.Store.Dir, Target: e.Replica.Target}
}
func (p *SharedSyncPlan) saveConflictDraft(master []byte) error {
	return p.engine.ConflictStore().Save(SharedConflictDraft{Version: 1, Target: p.engine.Replica.Target, Local: p.originalLocal, Base: p.baseline, Revision: p.revision, VaultRevision: p.vaultRevision}, master)
}

func (p *SharedSyncPlan) ConflictDraftExists() bool { return p.engine.ConflictStore().Exists() }
