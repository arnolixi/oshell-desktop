// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SharedSyncEngine struct {
	Manual    bool
	Progress  func(SyncEvent, error)
	Store     *Store
	Vault     *Vault
	Transport SyncTransport
	Replica   SharedReplicaStore
}
type SharedSyncPlan struct {
	protocolUpgrade                  bool
	runID                            string
	started                          time.Time
	originalLocal                    SharedDocument
	Local, Remote                    SharedDocument
	HasRemote, InitialChoiceRequired bool
	Conflicts                        []SharedMergeConflict
	configuration                    Config
	revision, vaultRevision, etag    string
	digests                          map[string]string
	baseline                         *SharedDocument
	merge                            *SharedMerge
	engine                           *SharedSyncEngine
}

func sameShared(a, b SharedDocument) bool {
	left, err := a.canonicalProtocol()
	if err != nil {
		return false
	}
	right, err := b.canonicalProtocol()
	if err != nil {
		return false
	}
	x, _ := json.Marshal(left)
	y, _ := json.Marshal(right)
	return equalSharedJSON(x, y)
}
func (e *SharedSyncEngine) Prepare(ctx context.Context, cfg Config, master []byte, allowCreate bool) (_ *SharedSyncPlan, resultErr error) {
	started := time.Now()
	runID := NewID()
	e.recordDiagnostic(SyncStarted, started, nil, runID)
	defer func() {
		if resultErr != nil {
			event := SyncFailed
			if errors.Is(resultErr, context.Canceled) {
				event = SyncCancelled
			}
			e.recordDiagnostic(event, started, resultErr, runID)
		}
	}()
	protected, locked := e.Vault.State()
	if !protected || locked {
		return nil, fmt.Errorf("同步需要已解锁的主密码保护")
	}
	revision, err := e.Store.Revision()
	if err != nil {
		return nil, err
	}
	vaultRevision := e.Vault.Revision()
	replica, err := e.Replica.Load(master)
	if err != nil {
		return nil, err
	}
	var cached, baseline *SharedDocument
	var digests map[string]string
	if replica != nil {
		cached = replica.CachedLocal
		baseline = replica.Baseline
		digests = replica.CredentialDigests
	}
	local, digests, err := e.Vault.ExportSharedDocument(ctx, cfg, master, cached, digests)
	if err != nil {
		return nil, err
	}
	draft, err := e.ConflictStore().Load(master)
	if err != nil {
		return nil, err
	}
	if draft != nil {
		if draft.Revision != revision || draft.VaultRevision != vaultRevision {
			return nil, fmt.Errorf("本机配置或密码库在冲突后已变化，原草稿已保留；请先确认并放弃旧草稿，再重新同步")
		}
		local = draft.Local
		baseline = draft.Base
	}
	object, err := e.Transport.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	e.recordDiagnostic(SyncFetched, started, nil, runID)
	plan := &SharedSyncPlan{runID: runID, started: started, Local: local, originalLocal: local, configuration: cfg.Clone(), revision: revision, vaultRevision: vaultRevision, digests: digests, baseline: baseline, engine: e}
	if object == nil {
		if baseline != nil || draft != nil || !allowCreate {
			return nil, safeSyncFailure("remote-deleted")
		}
		return plan, nil
	}
	remote, err := DecodeSharedDocument(object.Data, master)
	if err != nil {
		return nil, safeSyncFailure("encryption")
	}
	// Both verifiers were authenticated with the same shared password. Random
	// envelope nonces do not mean that password protection itself changed.
	remote.Verifier = local.Verifier
	plan.Remote = remote
	plan.protocolUpgrade = remote.needsProtocolUpgrade()
	plan.HasRemote = true
	plan.etag = object.ETag
	if sameShared(local, remote) {
		return plan, nil
	}
	if baseline == nil {
		plan.InitialChoiceRequired = true
		e.recordDiagnostic(SyncConflict, started, nil, runID)
		if err = plan.saveConflictDraft(master); err != nil {
			return nil, err
		}
		return plan, nil
	}
	base := *baseline
	base.Verifier = local.Verifier
	plan.merge, err = NewSharedMerge(base, local, remote)
	if err != nil {
		return nil, err
	}
	plan.Conflicts = plan.merge.Conflicts
	if len(plan.Conflicts) > 0 {
		e.recordDiagnostic(SyncConflict, started, nil, runID)
		if err = plan.saveConflictDraft(master); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

// ChooseInitial matches the original first-sync options: use shared data, or
// merge local sessions into shared settings. It does not publish anything.
func (p *SharedSyncPlan) ChooseInitial(mergeLocal bool) error {
	if !p.InitialChoiceRequired {
		return fmt.Errorf("无需首次同步选择")
	}
	p.InitialChoiceRequired = false
	if !mergeLocal {
		p.Local = p.Remote
		return nil
	}
	base, local := p.Remote, p.Remote
	base.Profiles = []PortableProfile{}
	local.Profiles = p.Local.Profiles
	merge, err := NewSharedMerge(base, local, p.Remote)
	if err != nil {
		return err
	}
	p.merge = merge
	p.Conflicts = merge.Conflicts
	return nil
}
func (p *SharedSyncPlan) Commit(ctx context.Context, choices map[string]SharedMergeChoice, master []byte) (_ Config, resultErr error) {
	e := p.engine
	started := p.started
	if started.IsZero() {
		started = time.Now()
	}
	runID := p.runID
	defer func() {
		if resultErr != nil {
			event := SyncFailed
			if errors.Is(resultErr, context.Canceled) {
				event = SyncCancelled
			}
			e.recordDiagnostic(event, started, resultErr, runID)
		}
	}()
	if p.InitialChoiceRequired {
		return p.configuration, fmt.Errorf("请先选择如何处理已有共享数据")
	}
	document := p.Local
	if p.merge != nil {
		var err error
		document, err = p.merge.Resolve(choices, master)
		if err != nil {
			return p.configuration, err
		}
	}
	revision, err := e.Store.Revision()
	if err != nil {
		return p.configuration, err
	}
	if revision != p.revision || e.Vault.Revision() != p.vaultRevision {
		return p.configuration, fmt.Errorf("选择期间本机配置或密码库发生变化，请重新同步")
	}

	if p.HasRemote && !p.protocolUpgrade && sameShared(p.originalLocal, p.Remote) && sameShared(document, p.Remote) {
		if p.baseline == nil || !sameShared(*p.baseline, document) {
			replica := SharedReplica{Version: 1, Target: e.Replica.Target, Baseline: &document, CachedLocal: &p.Local, CredentialDigests: p.digests}
			if err = e.Replica.Save(replica, master); err != nil {
				return p.configuration, err
			}
		}
		if err = e.ConflictStore().Clear(); err != nil {
			return p.configuration, fmt.Errorf("同步完成，但冲突草稿清理失败：%w", err)
		}
		e.recordDiagnostic(SyncUnchanged, started, nil, runID)
		return p.configuration, nil
	}
	next, updates, err := document.PrepareLocalReplacement(ctx, p.configuration, master)
	if err != nil {
		return p.configuration, err
	}
	encrypted, err := document.Encode(master)
	if err != nil {
		return p.configuration, err
	}
	if p.baseline == nil && p.HasRemote {
		backup, err := p.configurationBackup(master)
		if err != nil {
			return p.configuration, err
		}
		dir := filepath.Join(e.Store.Dir, "storage-backups")
		if err = os.MkdirAll(dir, 0700); err != nil {
			return p.configuration, err
		}
		if err = AtomicWrite(filepath.Join(dir, NewID()+".json"), backup); err != nil {
			return p.configuration, err
		}
	}
	if !p.HasRemote || p.protocolUpgrade || !sameShared(document, p.Remote) {
		e.recordDiagnostic(SyncUploading, started, nil, runID)
		if err = e.Transport.Put(ctx, encrypted, p.etag); err != nil {
			return p.configuration, err
		}
	} else {
		// Even a download-only merge must not apply a remote revision that changed
		// while the conflict dialog was open.
		latest, err := e.Transport.Fetch(ctx)
		if err != nil {
			return p.configuration, err
		}
		if latest == nil || latest.ETag != p.etag {
			return p.configuration, safeSyncFailure("conflict")
		}
	}
	e.recordDiagnostic(SyncApplying, started, nil, runID)
	if err = e.Store.commitCredentials(ctx, next, e.Vault, updates, p.revision, p.vaultRevision); err != nil {
		return p.configuration, err
	}
	// The cached export uses local ciphertext digests after the transaction.
	cached, digests, err := e.Vault.ExportSharedDocument(ctx, next, master, &document, p.digests)
	if err != nil {
		return next, fmt.Errorf("配置已同步，但无法更新同步基准：%w", err)
	}
	replica := SharedReplica{Version: 1, Target: e.Replica.Target, Baseline: &document, CachedLocal: &cached, CredentialDigests: digests}
	if err = e.Replica.Save(replica, master); err != nil {
		return next, fmt.Errorf("配置已同步，但无法保存同步基准：%w", err)
	}
	if err = e.ConflictStore().Clear(); err != nil {
		return next, fmt.Errorf("同步完成，但冲突草稿清理失败：%w", err)
	}
	e.recordDiagnostic(SyncCompleted, started, nil, runID)
	return next, nil
}
func (p *SharedSyncPlan) configurationBackup(master []byte) ([]byte, error) {
	// ChooseInitial may replace Local with Remote; export the captured original
	// configuration again so the first-sync backup never contains the replacement.
	document, _, err := p.engine.Vault.ExportSharedDocument(context.Background(), p.configuration, master, nil, nil)
	if err != nil {
		return nil, err
	}
	return document.Encode(master)
}
