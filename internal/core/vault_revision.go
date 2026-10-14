package core

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrVaultChanged = errors.New("密码库文件已变化，请重新载入后再保存")

func (v *Vault) checkDiskRevisionLocked() error {
	data, err := ReadBounded(filepath.Join(v.dir, "credentials.json"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	defer clear(data)
	current := ""
	if err == nil {
		current = configurationDigest(data)
	}
	if current != v.diskRevision {
		return ErrVaultChanged
	}
	return nil
}
