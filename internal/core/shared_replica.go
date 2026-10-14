// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SharedReplica contains no plaintext credential. The entire replica is also
// encrypted, since preferences and connection identities are private data.
type SharedReplica struct {
	Version           int               `json:"version"`
	Target            string            `json:"target"`
	Baseline          *SharedDocument   `json:"baseline,omitempty"`
	CachedLocal       *SharedDocument   `json:"cachedLocal,omitempty"`
	CredentialDigests map[string]string `json:"credentialDigests,omitempty"`
}
type SharedReplicaStore struct{ Directory, Target string }

func (s SharedReplicaStore) path() string {
	digest := sha256.Sum256([]byte(s.Target))
	return filepath.Join(s.Directory, "sync-replicas", hex.EncodeToString(digest[:])+".json")
}
func (s SharedReplicaStore) Load(master []byte) (*SharedReplica, error) {
	f, e := openImportFile(s.path())
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, MaxSharedVault+1))
	if e != nil {
		return nil, e
	}
	if len(data) > MaxSharedVault {
		return nil, fmt.Errorf("同步基准超过大小限制")
	}
	plain, e := OpenShared(data, master)
	if e != nil {
		return nil, e
	}
	defer clear(plain)
	var replica SharedReplica
	if e = json.Unmarshal(plain, &replica); e != nil {
		return nil, fmt.Errorf("同步基准格式无效")
	}
	if replica.Version != 1 || replica.Target != s.Target {
		return nil, fmt.Errorf("同步基准与目标不匹配")
	}
	for _, document := range []*SharedDocument{replica.Baseline, replica.CachedLocal} {
		if document != nil {
			if e = document.Validate(master); e != nil {
				return nil, e
			}
		}
	}
	return &replica, nil
}
func (s SharedReplicaStore) Save(replica SharedReplica, master []byte) error {
	if replica.Target != s.Target || replica.Version != 1 {
		return fmt.Errorf("同步基准与目标不匹配")
	}
	for _, document := range []*SharedDocument{replica.Baseline, replica.CachedLocal} {
		if document != nil {
			if e := document.Validate(master); e != nil {
				return e
			}
		}
	}
	plain, e := json.Marshal(replica)
	if e != nil {
		return e
	}
	defer clear(plain)
	encrypted, e := SealShared(plain, master)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(s.path()), 0700); e != nil {
		return e
	}
	return AtomicWrite(s.path(), encrypted)
}

// PrepareLocalReplacement decrypts every incoming credential before callers
// commit anything. It retains UUIDs (unlike importing an independent archive)
// and explicitly removes credentials absent from the selected revision.
func (d SharedDocument) PrepareLocalReplacement(ctx context.Context, previous Config, master []byte) (Config, []CredentialUpdate, error) {
	if e := d.Validate(master); e != nil {
		return previous, nil, e
	}
	return d.prepareCredentialReplacement(ctx, previous, func(source Profile, envelope LegacyPassword) (string, error) {
		return DecryptMacPassword(envelope, master, source)
	})
}
func (d SharedDocument) prepareCredentialReplacement(ctx context.Context, previous Config, decrypt func(Profile, LegacyPassword) (string, error)) (Config, []CredentialUpdate, error) {
	cfg, e := d.Configuration()
	if e != nil {
		return previous, nil, e
	}
	// This local renderer memory budget is not part of the source shared schema.
	cfg.Settings.ScrollbackMB = previous.Settings.ScrollbackMB
	byID := map[string]Profile{}
	for _, p := range cfg.Profiles {
		byID[strings.ToUpper(p.ID)] = p
	}
	for _, proxy := range cfg.Proxies {
		p := ProxyCredentialProfile(proxy.Settings)
		byID[strings.ToUpper(p.ID)] = p
	}
	updates := []CredentialUpdate{}
	seen := map[string]bool{}
	bind := func(source Profile, envelope *LegacyPassword) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		key := strings.ToUpper(source.ID)
		destination, ok := byID[key]
		if !ok {
			return nil
		}
		if seen[key] {
			return fmt.Errorf("共享凭据 ID 重复")
		}
		seen[key] = true
		update := CredentialUpdate{Profile: destination, Delete: envelope == nil}
		if envelope != nil {
			secret, e := decrypt(source, *envelope)
			if e != nil {
				return e
			}
			update.Password = secret
			update.Identity = envelope.Identity
		}
		updates = append(updates, update)
		return nil
	}
	for _, p := range d.Profiles {
		if e = bind(p.Definition, p.Password); e != nil {
			return previous, nil, e
		}
		if p.Definition.ProxyID == "" && p.Definition.Proxy.Kind != "none" && p.Definition.Proxy.Kind != "" {
			if e = bind(ProxyCredentialProfile(p.Definition.Proxy), p.ProxyPassword); e != nil {
				return previous, nil, e
			}
		}
	}
	for _, proxy := range d.Proxies {
		if e = bind(ProxyCredentialProfile(proxy.Definition.Settings), proxy.Password); e != nil {
			return previous, nil, e
		}
	}
	for _, p := range previous.Profiles {
		for _, old := range []Profile{p, ProxyCredentialProfile(p.Proxy)} {
			key := strings.ToUpper(old.ID)
			if key != "" && !seen[key] {
				seen[key] = true
				updates = append(updates, CredentialUpdate{Profile: old, Delete: true})
			}
		}
	}
	for _, proxy := range previous.Proxies {
		p := ProxyCredentialProfile(proxy.Settings)
		key := strings.ToUpper(p.ID)
		if !seen[key] {
			seen[key] = true
			updates = append(updates, CredentialUpdate{Profile: p, Delete: true})
		}
	}
	return cfg, updates, nil
}
