package core

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrConfigurationLocked = errors.New("配置已加密，请先输入主密码解锁")

// EncodeConfiguration protects the complete desktop configuration when a master
// is supplied. Password envelopes remain in the separate credential vault.
func EncodeConfiguration(cfg Config, master []byte) ([]byte, error) {
	cfg = cfg.Clone()
	cfg.Version = 3
	cfg.NormalizeLinkRootOrder()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	plain, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(plain) > MaxArchive {
		return nil, fmt.Errorf("配置超过 16 MiB")
	}
	if len(master) == 0 {
		return plain, nil
	}
	defer clear(plain)
	return SealShared(plain, master)
}

// DecodeConfiguration never treats an encrypted document as an empty/default
// configuration. The boolean reports protection even when unlocking fails.
func DecodeConfiguration(data, master []byte) (Config, bool, error) {
	cfg := DefaultConfig()
	if len(data) > MaxSharedVault {
		return cfg, false, fmt.Errorf("配置文件过大")
	}
	var header struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return cfg, false, err
	}
	encrypted := header.Format == "OShell.shared-vault"
	plain := data
	if encrypted {
		if len(master) == 0 {
			return cfg, true, ErrConfigurationLocked
		}
		var err error
		plain, err = OpenShared(data, master)
		if err != nil {
			return cfg, true, err
		}
		defer clear(plain)
	} else if header.Format != "" {
		return cfg, false, fmt.Errorf("不支持的配置格式")
	}
	if len(plain) > MaxArchive {
		return cfg, encrypted, fmt.Errorf("配置超过 16 MiB")
	}
	if err := json.Unmarshal(plain, &cfg); err != nil {
		return cfg, encrypted, err
	}
	if cfg.Version != 1 && cfg.Version != 2 && cfg.Version != 3 {
		return cfg, encrypted, fmt.Errorf("不支持的配置版本 %d", cfg.Version)
	}
	cfg.Version = 3
	if cfg.Settings.FontSize < 10 || cfg.Settings.FontSize > 26 {
		cfg.Settings.FontSize = 13
	}
	if cfg.Settings.ScrollbackMB < 1 || cfg.Settings.ScrollbackMB > 64 {
		cfg.Settings.ScrollbackMB = 4
	}
	cfg.NormalizeLinkRootOrder()
	return cfg, encrypted, cfg.Validate()
}
