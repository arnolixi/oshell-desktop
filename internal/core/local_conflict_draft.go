package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/scrypt"
)

const localDraftFile = "local-conflict-draft.json"
const maxLocalDraft = 64 << 20
const maxLocalDraftFile = 96 << 20

type localDraftEnvelope struct {
	Version    int
	ID, Mode   string
	Salt, Data []byte
}
type localDraftPayload struct {
	Version   int
	Base      Config
	BaseUnits map[string]json.RawMessage
	Local     Config
	Vault     vaultData
}

func (s *Store) LocalConflictDraftExists() bool {
	_, err := os.Lstat(filepath.Join(s.Dir, localDraftFile))
	return !os.IsNotExist(err)
}
func (s *Store) SaveLocalConflictDraft(ctx context.Context, p *LocalResolutionPlan) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return "", fmt.Errorf("冲突计划已关闭")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	payload := localDraftPayload{1, p.Base, p.baseUnits, p.Local, p.localVault.data}
	plain, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	defer clear(plain)
	if len(plain) > maxLocalDraft {
		return "", fmt.Errorf("本地冲突草稿过大")
	}
	envelope := localDraftEnvelope{Version: 1, ID: NewID(), Mode: "local"}
	var key []byte
	if p.localVault.data.Master {
		envelope.Mode = "master"
		envelope.Salt = append([]byte(nil), p.localVault.data.Salt...)
		key = append([]byte(nil), p.localVault.key...)
		if len(key) == 0 {
			return "", ErrVaultLocked
		}
	} else {
		key, err = localVaultKey(s.Dir)
		if err != nil {
			return "", err
		}
	}
	defer clear(key)
	envelope.Data, err = seal(key, plain, "OShell local conflict draft v1\n"+envelope.ID)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	if len(data) > maxLocalDraftFile {
		return "", fmt.Errorf("本地冲突草稿过大")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = AtomicWrite(filepath.Join(s.Dir, localDraftFile), data); err != nil {
		return "", err
	}
	return envelope.ID, nil
}
func (s *Store) readLocalDraftEnvelope() (localDraftEnvelope, error) {
	var envelope localDraftEnvelope
	file, err := openImportFile(filepath.Join(s.Dir, localDraftFile))
	if err != nil {
		return envelope, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return envelope, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxLocalDraftFile {
		return envelope, fmt.Errorf("草稿文件类型或大小无效")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxLocalDraftFile+1))
	if err != nil {
		return envelope, err
	}
	defer clear(data)
	if len(data) > maxLocalDraftFile || json.Unmarshal(data, &envelope) != nil || envelope.Version != 1 || !uuidText.MatchString(envelope.ID) || len(envelope.Data) < 28 || len(envelope.Data) > maxLocalDraft+28 || (envelope.Mode != "master" && envelope.Mode != "local") {
		return envelope, fmt.Errorf("本地冲突草稿格式无效，原文件已保留")
	}
	if envelope.Mode == "master" && len(envelope.Salt) != 32 {
		return envelope, fmt.Errorf("草稿主密码参数无效")
	}
	return envelope, nil
}
func (s *Store) ResumeLocalResolution(ctx context.Context, current *Vault) (*LocalResolutionPlan, string, error) {
	if current == nil {
		return nil, "", fmt.Errorf("密码库不可用")
	}
	envelope, err := s.readLocalDraftEnvelope()
	if err != nil {
		return nil, "", err
	}
	var key, master []byte
	if envelope.Mode == "master" {
		master, err = current.MasterSecret()
		if err != nil {
			return nil, "", err
		}
		defer clear(master)
		key, err = scrypt.Key(master, envelope.Salt, 32768, 8, 1, 32)
	} else {
		if protected, _ := current.State(); protected {
			return nil, "", fmt.Errorf("当前已启用主密码，不能恢复仅由本机密钥保护的草稿；原文件已保留")
		}
		key, err = loadLocalVaultKey(s.Dir)
	}
	if err != nil {
		return nil, "", err
	}
	defer clear(key)
	plain, err := unseal(key, envelope.Data, "OShell local conflict draft v1\n"+envelope.ID)
	if err != nil {
		return nil, "", fmt.Errorf("草稿无法解密，需原主密码或本机密钥；原文件已保留")
	}
	defer clear(plain)
	var payload localDraftPayload
	if err = json.Unmarshal(plain, &payload); err != nil || payload.Version != 1 || payload.Vault.Version != 1 || payload.Vault.Entries == nil || payload.BaseUnits == nil {
		return nil, "", fmt.Errorf("草稿内容无效")
	}
	if err = payload.Base.Validate(); err != nil {
		return nil, "", err
	}
	if err = payload.Local.Validate(); err != nil {
		return nil, "", err
	}
	units, _, err := localConfigUnits(payload.Base)
	if err != nil {
		return nil, "", err
	}
	if len(units) != len(payload.BaseUnits) {
		return nil, "", fmt.Errorf("草稿基线不匹配")
	}
	for id, value := range units {
		if !equalSharedJSON(value, stripCredentialMarker(payload.BaseUnits[id])) {
			return nil, "", fmt.Errorf("草稿基线不匹配")
		}
	}
	restored := &Vault{dir: s.Dir, data: payload.Vault}
	if restored.data.Master {
		if envelope.Mode != "master" {
			return nil, "", fmt.Errorf("草稿保护方式不匹配")
		}
		err = restored.unlockLocked(string(master))
	} else {
		restored.key, err = loadLocalVaultKey(s.Dir)
	}
	if err != nil {
		return nil, "", err
	}
	owned := false
	defer func() {
		if !owned {
			clear(restored.key)
			clear(restored.master)
		}
	}()
	plan, err := s.PrepareLocalResolution(ctx, payload.Local, current, nil)
	if err != nil {
		return nil, "", err
	}
	localUnits, order, err := credentialConfigUnits(payload.Local, restored)
	if err != nil {
		plan.Close()
		return nil, "", err
	}
	remoteUnits, extra, err := credentialConfigUnits(plan.Remote, plan.remoteVault)
	if err != nil {
		plan.Close()
		return nil, "", err
	}
	clear(plan.localVault.key)
	clear(plan.localVault.master)
	plan.localVault = restored
	plan.Base = payload.Base
	plan.baseUnits = payload.BaseUnits
	plan.Merge = mergeLocalUnits(payload.BaseUnits, localUnits, remoteUnits, order, extra)
	// Ensure another process did not replace the draft while it was decoded.
	latest, err := s.readLocalDraftEnvelope()
	if err != nil || latest.ID != envelope.ID || !bytes.Equal(latest.Data, envelope.Data) {
		plan.Close()
		return nil, "", ErrConfigurationChanged
	}
	owned = true
	return plan, envelope.ID, nil
}

// Empty expectedID is reserved for an explicitly confirmed discard of unreadable data.
func (s *Store) ClearLocalConflictDraft(expectedID string) error {
	if expectedID != "" {
		envelope, err := s.readLocalDraftEnvelope()
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if envelope.ID != expectedID {
			return ErrConfigurationChanged
		}
	}
	err := os.Remove(filepath.Join(s.Dir, localDraftFile))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
