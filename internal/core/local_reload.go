package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
)

var ErrLocalDraftChanged = errors.New("本地有未保存的配置草稿，请先处理配置差异")

// ReloadLocalConfiguration adopts a coherent config/vault snapshot without
// publishing it. Protection changes must be handled by a fresh application start.
func (s *Store) ReloadLocalConfiguration(ctx context.Context, local Config, vault *Vault, validate func(Config) error) (Config, *Vault, error) {
	if s.LocalConflictDraftExists() {
		return Config{}, nil, ErrLocalDraftChanged
	}
	plan, err := s.PrepareLocalResolution(ctx, local, vault, nil)
	if err != nil {
		return Config{}, nil, err
	}
	defer plan.Close()
	if !reflect.DeepEqual(plan.Local, plan.Base) {
		return Config{}, nil, ErrLocalDraftChanged
	}
	master, _ := vault.State()
	protected, _ := s.ConfigurationProtection()
	raw, err := readConfigurationFile(filepath.Join(s.Dir, "config.json"))
	if err != nil {
		return Config{}, nil, err
	}
	defer clear(raw)
	if protected != IsSharedVault(raw) || master != plan.remoteVault.data.Master {
		return Config{}, nil, fmt.Errorf("密码保护已改变，请退出并重新打开应用；当前配置未被覆盖")
	}
	if validate != nil {
		if err := validate(plan.Remote); err != nil {
			return Config{}, nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return Config{}, nil, err
	}
	return plan.ReloadDisk(s)
}
