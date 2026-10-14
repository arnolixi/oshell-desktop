package core

import (
	"context"
	"crypto/subtle"
	"fmt"
)

func (s NativeStorageSnapshot) validatePlainNative(master []byte) error {
	d := s.Document
	if d.Profiles == nil || d.Preferences == nil {
		return fmt.Errorf("原版配置缺少会话或偏好字段")
	}
	archive := PortableArchive{Format: "OShell.sessions", Version: 4, Profiles: d.Profiles, Proxies: d.Proxies, Directories: d.Directories, Links: d.Links.Entries}
	if err := archive.Validate(); err != nil {
		return err
	}
	if d.Verifier != nil {
		text, err := DecryptMacPassword(*d.Verifier, master, masterVerifierProfile())
		if err != nil || subtle.ConstantTimeCompare([]byte(text), []byte(verifierMarker)) != 1 {
			return fmt.Errorf("原版主密码校验失败")
		}
	}
	cfg, err := d.Configuration()
	if err != nil {
		return err
	}
	return cfg.Validate()
}
func (s NativeStorageSnapshot) prepareReplacement(ctx context.Context, previous Config, master []byte) (Config, []CredentialUpdate, error) {
	if !s.PlainLocal && !s.LocalCredentials {
		return s.Document.PrepareLocalReplacement(ctx, previous, master)
	}
	if err := s.validatePlainNative(master); err != nil {
		return previous, nil, err
	}
	keys := map[string]LocalCredentialKey{}
	return s.Document.prepareCredentialReplacement(ctx, previous, func(profile Profile, envelope LegacyPassword) (string, error) {
		if envelope.LocalKeyID == "" {
			return DecryptMacPassword(envelope, master, profile)
		}
		key, ok := keys[envelope.LocalKeyID]
		if !ok {
			var err error
			key, err = LoadOShellLocalCredentialKey(s.Directory, envelope.LocalKeyID, false)
			if err != nil {
				return "", err
			}
			keys[envelope.LocalKeyID] = key
		}
		return DecryptMacLocalPassword(envelope, key, profile)
	})
}
